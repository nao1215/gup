package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/pkgselect"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the latest version of the binary installed by 'go install'",
		Example: `  gup check
  gup check --quiet`,
		Long: `Check the latest version and build toolchain of the binary installed by 'go install'

check subcommand checks if the binary is the latest version
and if it has been built with the current version of go installed,
and displays the name of the binary that needs to be updated.
It does not update them.`,
		ValidArgsFunction: completePathBinaries,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(check(defaultDependencies(), printerFor(cmd), cmd, args))
		},
	}

	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "specify the number of CPU cores to use")
	mustRegisterFlagCompletion(cmd, "jobs", completeNCPUs)
	cmd.Flags().Bool("ignore-go-update", false, "ignore updates to the Go toolchain")
	cmd.Flags().Bool("json", false, "output result as machine-readable JSON")
	cmd.Flags().BoolP("quiet", "q", false, "suppress up-to-date lines; show only update-available/failed binaries plus a summary")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read saved update channels from")
	mustMarkFileFlagAsJSON(cmd)
	addTimeoutFlag(cmd)

	return cmd
}

// checkOpts holds the parsed command-line flags for the check command.
type checkOpts struct {
	cpus           int // already clamped to >= 1
	ignoreGoUpdate bool
	jsonOut        bool
	quiet          bool
	timeout        time.Duration
	confFile       string
}

// parseCheckFlags reads every flag of the check command in one place so check()
// handles a flag error exactly once. The returned cpus value is already clamped.
func parseCheckFlags(cmd *cobra.Command) (checkOpts, error) {
	var opts checkOpts
	var err error

	if opts.cpus, err = getFlagInt(cmd, "jobs"); err != nil {
		return checkOpts{}, err
	}
	opts.cpus = clampJobs(opts.cpus)
	if opts.ignoreGoUpdate, err = getFlagBool(cmd, "ignore-go-update"); err != nil {
		return checkOpts{}, err
	}
	if opts.jsonOut, err = getFlagBool(cmd, "json"); err != nil {
		return checkOpts{}, err
	}
	if opts.quiet, err = getFlagBool(cmd, "quiet"); err != nil {
		return checkOpts{}, err
	}
	if opts.timeout, err = getTimeoutFlag(cmd); err != nil {
		return checkOpts{}, err
	}
	if opts.confFile, err = getFlagString(cmd, "file"); err != nil {
		return checkOpts{}, err
	}
	return opts, nil
}

// check runs the check command. deps carries the go-toolchain version lookups so
// the flow takes its collaborators explicitly: production passes
// defaultDependencies(); tests inject fakes.
func check(deps dependencies, p *print.Printer, cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		p.Err(err)
		return 1
	}

	opts, err := parseCheckFlags(cmd)
	if err != nil {
		p.Err(err)
		return 1
	}

	pkgs, missingTargets, goVersionAvailable, err := pkgselect.PackageInfoByTargets(p, args)
	if err != nil {
		p.Err(err)
		return 1
	}
	// When the installed Go version can't be detected, behave as
	// --ignore-go-update so check does not report every binary as outdated
	// (see issue #296).
	ignoreGoUpdate := opts.ignoreGoUpdate || !goVersionAvailable
	pkgselect.WarnMissing(missingTargets, func(msg string) { p.Warn(msg) })

	if len(pkgs) == 0 {
		return handleEmptyEnvironment(p, opts.confFile, opts.jsonOut, len(args) != 0,
			"unable to check package: no package information")
	}

	pkgs, err = configstate.ResolveAndApplyChannels(pkgs, opts.confFile)
	if err != nil {
		p.Err(err)
		return 1
	}
	if opts.jsonOut {
		return doCheckJSON(deps, p, pkgs, opts.cpus, opts.timeout, ignoreGoUpdate)
	}
	return doCheck(deps, p, pkgs, opts.cpus, opts.timeout, ignoreGoUpdate, opts.quiet)
}

func doCheck(deps dependencies, p *print.Printer, pkgs []goutil.Package, cpus int, timeout time.Duration, ignoreGoUpdate, quiet bool) int {
	return doCheckWith(deps, p, pkgs, cpus, timeout, ignoreGoUpdate, quiet, false)
}

// doCheckJSON runs the same check as doCheck but emits a JSON array of package
// records to STDOUT instead of human-readable progress lines.
func doCheckJSON(deps dependencies, p *print.Printer, pkgs []goutil.Package, cpus int, timeout time.Duration, ignoreGoUpdate bool) int {
	return doCheckWith(deps, p, pkgs, cpus, timeout, ignoreGoUpdate, false, true)
}

func doCheckWith(deps dependencies, p *print.Printer, pkgs []goutil.Package, cpus int, timeout time.Duration, ignoreGoUpdate, quiet, jsonOut bool) int {
	verCache := deps.newVerCache()

	if !jsonOut && !quiet {
		p.Info("check binary under $GOPATH/bin or $GOBIN")
	}

	checker := func(ctx context.Context, p goutil.Package) updateResult {
		// A pinned package is compared against its recorded version, never against
		// @latest: reporting "update available" for a pin would be wrong.
		if p.IsPinned() {
			return checkPinned(p, ignoreGoUpdate)
		}

		var err error
		status := statusUpToDate
		if p.ModulePath == "" {
			err = fmt.Errorf("%s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {
			var latestVer string
			modulePathChanged := false
			latestVer, err = verCache.Get(ctx, p.ModulePath, p.UpdateChannel)
			if err != nil {
				newPkg, changed := resolveModulePathChange(p, err)
				if !changed {
					err = fmt.Errorf("%s %w", p.Name, err)
				} else {
					modulePathChanged = true
					p = newPkg
					latestVer, err = verCache.Get(ctx, p.ModulePath, p.UpdateChannel)
					if err != nil {
						err = fmt.Errorf("%s %w", p.Name, err)
					}
				}
			}
			if err == nil {
				p.Version.Latest = latestVer

				shouldUpdate := modulePathChanged || !p.IsPackageUpToDate() || (!ignoreGoUpdate && !p.IsGoUpToDate())
				if shouldUpdate {
					status = statusUpdateAvailable
				}
			}
		}

		return updateResult{
			pkg:    p,
			err:    err,
			status: status,
		}
	}

	var onResult func(prefix string, v updateResult)
	if !jsonOut {
		// In quiet mode show only binaries with an available update.
		onResult = resultLineRenderer(p, quiet,
			func(v updateResult) bool {
				return v.status == statusUpdateAvailable || v.status == statusPinMismatch
			},
			checkResultStr)
	}

	result, results := executePackages(p, pkgs, cpus, timeout, checker, onResult)

	if jsonOut {
		if err := encodeJSONPackages(p, resultsToJSONPackages(results)); err != nil {
			p.Err(err)
			return 1
		}
		return result
	}

	printUpdatablePkgInfo(p, collectNeedUpdatePkgs(results))
	if quiet {
		p.Info(summarizeResults(results, true))
	}
	return result
}

// checkResultStr renders the per-binary check line, using the pinned-specific
// description for a pinned package and the normal version-check string
// otherwise.
func checkResultStr(p goutil.Package) string {
	if p.IsPinned() {
		return pinnedResultStr(p)
	}
	return versionCheckResultStr(p)
}

// checkPinned reports the state of a pinned package without consulting @latest:
// "pinned" when the installed version matches the pin and the Go toolchain is
// current (or Go updates are ignored), "pin-mismatch" otherwise (the binary
// would be reinstalled at the pinned version by 'gup update', either to correct
// the version or to rebuild with the current Go toolchain). The pin locks the
// module version, not the Go build, so a Go-toolchain delta is surfaced just as
// it is for unpinned packages.
func checkPinned(p goutil.Package, ignoreGoUpdate bool) updateResult {
	if p.Version == nil {
		p.Version = &goutil.Version{}
	}
	goOutdated := !ignoreGoUpdate && p.GoVersion != nil && !p.IsGoUpToDate()
	if p.PinSatisfied() && !goOutdated {
		// Hide a Go delta we are intentionally ignoring so the line reads cleanly.
		if p.GoVersion != nil {
			p.GoVersion.Latest = p.GoVersion.Current
		}
		return updateResult{pkg: p, status: statusPinned}
	}
	return updateResult{pkg: p, status: statusPinMismatch}
}

// collectNeedUpdatePkgs returns the packages from successful results whose
// status indicates an available update, preserving completion order. A pinned
// package whose installed version differs from its pin is included so the
// follow-up "run gup update ..." hint covers it too.
func collectNeedUpdatePkgs(results []updateResult) []goutil.Package {
	needUpdate := make([]goutil.Package, 0, len(results))
	for _, v := range results {
		if v.err == nil && (v.status == statusUpdateAvailable || v.status == statusPinMismatch) {
			needUpdate = append(needUpdate, v.pkg)
		}
	}
	return needUpdate
}

func printUpdatablePkgInfo(p *print.Printer, pkgs []goutil.Package) {
	if len(pkgs) == 0 {
		return
	}

	var b strings.Builder
	for _, v := range pkgs {
		b.WriteString(v.Name)
		b.WriteString(" ")
	}

	const indentSpaces = 11
	// Emit the blank separator line through the Printer (not fmt.Println, which
	// writes to the real stdout) so all of this command's output flows through
	// one sink and is captured together in tests.
	p.Info("")
	p.Info("If you want to update binaries, run the following command.\n" +
		strings.Repeat(" ", indentSpaces) +
		"$ gup update " + b.String())
}
