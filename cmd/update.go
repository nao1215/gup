package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/notify"
	"github.com/nao1215/gup/internal/pkgselect"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

const latestKeyword = "latest"

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update binaries installed by 'go install'",
		Example: `  gup update
  gup update --dry-run
  gup update --exclude foo,bar`,
		Long: `Update binaries installed by 'go install'

If you execute '$ gup update', gup gets the package path of all commands
under $GOPATH/bin and automatically updates commands to the latest version,
using the current installed Go toolchain.`,
		Run: func(cmd *cobra.Command, args []string) {
			OsExit(gup(defaultDependencies(), print.NewColorable(), cmd, args))
		},
		ValidArgsFunction: completePathBinaries,
	}
	cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
	cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
	cmd.Flags().StringSliceP("exclude", "e", []string{}, "specify binaries which should not be updated (delimiter: ',')")
	mustRegisterFlagCompletion(cmd, "exclude", completePathBinaries)
	cmd.Flags().StringSliceP("main", "m", []string{}, "specify binaries which update by @main or @master (delimiter: ',')")
	mustRegisterFlagCompletion(cmd, "main", completePathBinaries)
	cmd.Flags().StringSlice("master", []string{}, "specify binaries which update by @master (delimiter: ',')")
	mustRegisterFlagCompletion(cmd, "master", completePathBinaries)
	cmd.Flags().StringSlice(latestKeyword, []string{}, "specify binaries which update by @latest (delimiter: ',')")
	mustRegisterFlagCompletion(cmd, latestKeyword, completePathBinaries)
	// cmd.Flags().BoolP("main-all", "M", false, "update all binaries by @main or @master (delimiter: ',')")
	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "specify the number of CPU cores to use")
	mustRegisterFlagCompletion(cmd, "jobs", completeNCPUs)
	cmd.Flags().Bool("ignore-go-update", false, "ignore updates to the Go toolchain")
	cmd.Flags().Bool("json", false, "output result as machine-readable JSON")
	cmd.Flags().BoolP("quiet", "q", false, "suppress up-to-date lines; show only updated/failed binaries plus a summary")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read/write saved update channels")
	mustMarkFileFlagAsJSON(cmd)
	addTimeoutFlag(cmd)

	return cmd
}

// updateOpts holds the parsed command-line flags for the update command.
type updateOpts struct {
	dryRun         bool
	notify         bool
	cpus           int // already clamped to >= 1
	ignoreGoUpdate bool
	jsonOut        bool
	quiet          bool
	timeout        time.Duration
	excludePkgList []string
	mainPkgNames   []string
	masterPkgNames []string
	latestPkgNames []string
	confFile       string
}

// parseUpdateFlags reads every flag of the update command in one place so gup()
// handles a flag error exactly once. The returned cpus value is already clamped.
func parseUpdateFlags(cmd *cobra.Command) (updateOpts, error) {
	var opts updateOpts
	var err error

	if opts.dryRun, err = getFlagBool(cmd, "dry-run"); err != nil {
		return updateOpts{}, err
	}
	if opts.notify, err = getFlagBool(cmd, "notify"); err != nil {
		return updateOpts{}, err
	}
	if opts.cpus, err = getFlagInt(cmd, "jobs"); err != nil {
		return updateOpts{}, err
	}
	opts.cpus = clampJobs(opts.cpus)
	if opts.ignoreGoUpdate, err = getFlagBool(cmd, "ignore-go-update"); err != nil {
		return updateOpts{}, err
	}
	if opts.jsonOut, err = getFlagBool(cmd, "json"); err != nil {
		return updateOpts{}, err
	}
	if opts.quiet, err = getFlagBool(cmd, "quiet"); err != nil {
		return updateOpts{}, err
	}
	if opts.timeout, err = getTimeoutFlag(cmd); err != nil {
		return updateOpts{}, err
	}
	if opts.excludePkgList, err = getFlagStringSlice(cmd, "exclude"); err != nil {
		return updateOpts{}, err
	}
	if opts.mainPkgNames, err = getFlagStringSlice(cmd, "main"); err != nil {
		return updateOpts{}, err
	}
	if opts.masterPkgNames, err = getFlagStringSlice(cmd, "master"); err != nil {
		return updateOpts{}, err
	}
	if opts.latestPkgNames, err = getFlagStringSlice(cmd, latestKeyword); err != nil {
		return updateOpts{}, err
	}
	if opts.confFile, err = getFlagString(cmd, "file"); err != nil {
		return updateOpts{}, err
	}
	return opts, nil
}

// gup is main sequence.
// All errors are handled in this function.
//
// deps carries the go-toolchain operations (version lookups and per-channel
// installs) so the update flow takes its collaborators explicitly. Production
// passes defaultDependencies(); tests inject fakes.
func gup(deps dependencies, p *print.Printer, cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		p.Err(err)
		return 1
	}

	opts, err := parseUpdateFlags(cmd)
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
	// --ignore-go-update so a transient "go version" failure does not force
	// every binary to reinstall (see issue #296).
	ignoreGoUpdate := opts.ignoreGoUpdate || !goVersionAvailable

	pkgselect.WarnMissing(missingTargets, func(msg string) { p.Warn(msg) })
	// In JSON mode the human-readable "Exclude ..." notice is suppressed so
	// STDOUT stays valid JSON (the notice goes to STDOUT via p.Info, which
	// would otherwise break machine-readable output; see issue #291).
	excludeNotify := func(string) {}
	if !opts.jsonOut {
		excludeNotify = func(msg string) { p.Info(msg) }
	}
	pkgs = pkgselect.Exclude(pkgs, opts.excludePkgList, excludeNotify)

	if len(pkgs) == 0 {
		// With explicit targets or --exclude, an empty result means the user
		// narrowed everything out: that is a usage error. Otherwise it is a normal
		// first-run condition handled the same way as check.
		return handleEmptyEnvironment(p, opts.confFile, opts.jsonOut,
			len(args) != 0 || len(opts.excludePkgList) != 0,
			"unable to update package: no package information or no package under $GOBIN")
	}

	// When both the user-level config and ./gup.json exist and no --file is
	// given, fail fast instead of silently choosing one (#342), consistent with
	// import and check.
	confReadPath, err := config.ResolveImportFilePath(opts.confFile)
	if err != nil {
		p.Err(err)
		return 1
	}
	confWritePath := configstate.ResolveWritePath(opts.confFile, confReadPath)

	// A malformed or unreadable config must fail fast instead of silently
	// falling back to @latest, which would update from the wrong channel and
	// then persist that downgrade back to gup.json (#369).
	confPkgs, err := configstate.ReadFileIfExists(confReadPath)
	if err != nil {
		p.Err(err)
		return 1
	}

	// missingTargets were already reported as "not found ... in $GOBIN" above;
	// pass them so ResolveChannels does not emit a second, redundant notice for a
	// name listed both as a positional target and in --main/--master/--latest.
	channelMap, pinnedMap, err := configstate.ResolveChannels(pkgs, confPkgs, opts.mainPkgNames, opts.masterPkgNames, opts.latestPkgNames,
		missingTargets, func(msg string) { p.Warn(msg) })
	if err != nil {
		p.Err(err)
		return 1
	}

	result, succeededPkgs, renamedPkgs := updateWithChannels(deps, p, pkgs, opts.dryRun, opts.notify, opts.cpus, ignoreGoUpdate, channelMap, pinnedMap, opts.timeout, opts.jsonOut, opts.quiet)

	if !opts.dryRun && (configstate.ShouldPersistChannels(opts.mainPkgNames, opts.masterPkgNames, opts.latestPkgNames) || len(renamedPkgs) > 0) {
		merged := configstate.MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
		if err := writeConfigFile(confWritePath, merged); err != nil {
			p.Warn("failed to write " + confWritePath + ": " + err.Error())
		}
	}

	return result
}

type updateResult struct {
	updated     bool
	pkg         goutil.Package
	err         error
	renamedFrom string // original binary name if renamed during update
	skipped     bool   // true when the package was intentionally skipped (no error)
	skipReason  string // human-readable reason when skipped is true
	status      string // machine-readable status for --json output (see jsonout.go)
}

func updateWithChannels(deps dependencies, pr *print.Printer, pkgs []goutil.Package, dryRun, notification bool, cpus int, ignoreGoUpdate bool, channelMap map[string]goutil.UpdateChannel, pinnedMap map[string]string, timeout time.Duration, jsonOut, quiet bool) (exitCode int, succeeded []goutil.Package, renamed map[string]string) {
	dryRunManager := goutil.NewGoPaths()

	verCache := deps.newVerCache()

	if !jsonOut && !quiet {
		pr.Info("update binary under $GOPATH/bin or $GOBIN")
	}
	if dryRun {
		if err := dryRunManager.StartDryRunMode(); err != nil {
			pr.Err(fmt.Errorf("can not change to dry run mode: %w", err))
			notify.Warn(pr, "gup", "Can not change to dry run mode")
			return 1, nil, nil
		}
		// Restore the environment and remove the temp dir via defer so it runs
		// even if a package update panics (see issue #297).
		defer func() {
			if err := dryRunManager.EndDryRunMode(); err != nil {
				pr.Err(fmt.Errorf("can not change dry run mode to normal mode: %w", err))
				exitCode = 1
			}
		}()
	}

	updater := func(ctx context.Context, p goutil.Package) updateResult {
		originalName := p.Name
		// Resolve the update channel up front so the skip/update decision is
		// derived from the version the selected channel would install, not from
		// @latest. Without this, a package tracked on @main/@master would
		// piggyback on the @latest lookup and could be skipped or updated
		// incorrectly (see issue #292).
		channel := configstate.PackageChannel(p.Name, p.UpdateChannel, channelMap)
		p.UpdateChannel = channel

		// A pinned package is installed at its exact recorded version and never
		// resolves @latest/@main/@master, so it is handled entirely separately from
		// the channel-version lookup below.
		if channel == goutil.UpdateChannelPinned {
			p.PinnedVersion = pinnedMap[p.Name]
			return updatePinned(deps, ctx, p, ignoreGoUpdate)
		}

		// Collect online channel version if possible; else always update
		shouldUpdate := true
		modulePathChanged := false
		if p.ModulePath != "" {
			ver, err := verCache.Get(ctx, p.ModulePath, channel)
			if err != nil {
				newPkg, changed := resolveModulePathChange(p, err)
				if !changed {
					return updateResult{
						updated: false,
						pkg:     p,
						err:     fmt.Errorf("%s: %w", p.Name, err),
					}
				}
				modulePathChanged = true
				p = newPkg

				ver, err = verCache.Get(ctx, p.ModulePath, channel)
				if err != nil {
					return updateResult{
						updated: false,
						pkg:     p,
						err:     fmt.Errorf("%s: %w", p.Name, err),
					}
				}
			}
			p.Version.Latest = ver

			// Check if we should update the package
			shouldUpdate = modulePathChanged || !p.IsPackageUpToDate() || (!ignoreGoUpdate && !p.IsGoUpToDate())
		}

		if !shouldUpdate {
			return updateResult{
				updated: false,
				pkg:     p,
				err:     nil,
				status:  statusUpToDate,
			}
		}

		// Run the update
		var updateErr error
		installedViaRetry := false
		if p.ImportPath == "" {
			updateErr = fmt.Errorf("%s is not installed by 'go install' (or permission incorrect)", p.Name)
		} else {
			if err := installWithSelectedVersion(deps, ctx, p.ImportPath, channel); err != nil {
				newPkg, changed := resolveModulePathChange(p, err)
				if !changed {
					updateErr = fmt.Errorf("%s: %w", p.Name, err)
				} else {
					installedViaRetry = true
					p = newPkg
					if retryErr := installWithSelectedVersion(deps, ctx, p.ImportPath, channel); retryErr != nil {
						updateErr = fmt.Errorf("%s: %w", originalName, retryErr)
					} else {
						newName := binaryNameFromImportPath(p.ImportPath)
						if err := removeOldBinaryIfRenamed(originalName, newName); err != nil {
							updateErr = fmt.Errorf("%s: %w", originalName, err)
						}
						p.Name = newName
						p.UpdateChannel = channel
					}
				}
			}
		}

		if updateErr == nil {
			// For @latest with no module-path surprises, p.Version.Latest
			// is already correct from verCache; skip expensive buildinfo.ReadFile.
			if p.UpdateChannel != goutil.UpdateChannelLatest || modulePathChanged || installedViaRetry {
				p.SetLatestVer()
			}
		}
		var renamed string
		if updateErr == nil && p.Name != originalName {
			renamed = originalName
		}
		status := statusUpdated
		if updateErr != nil {
			status = statusError
		}
		return updateResult{
			updated:     updateErr == nil,
			pkg:         p,
			err:         updateErr,
			renamedFrom: renamed,
			status:      status,
		}
	}

	var onResult func(prefix string, v updateResult)
	if !jsonOut {
		// In quiet mode show only binaries that were actually updated.
		onResult = resultLineRenderer(pr, quiet,
			func(v updateResult) bool { return v.updated },
			updateResultStr)
	}

	// update all packages
	result, results := executePackages(pr, pkgs, cpus, timeout, updater, onResult)

	if jsonOut {
		if err := encodeJSONPackages(pr, resultsToJSONPackages(results)); err != nil {
			pr.Err(err)
			result = 1
		}
	} else if quiet {
		pr.Info(summarizeResults(results, false))
	}

	desktopNotifyIfNeeded(pr, result, notification)

	succeededPkgs, renamedPkgs := succeededAndRenamed(results)
	return result, succeededPkgs, renamedPkgs
}

// succeededAndRenamed derives the successfully-processed packages and the
// old->new rename map from execution results, so the config-persistence logic
// works identically in human-readable and --json modes.
func succeededAndRenamed(results []updateResult) ([]goutil.Package, map[string]string) {
	succeededPkgs := make([]goutil.Package, 0, len(results))
	renamedPkgs := map[string]string{} // oldName -> newName
	for _, v := range results {
		if v.err != nil {
			continue
		}
		succeededPkgs = append(succeededPkgs, v.pkg)
		if v.renamedFrom != "" {
			renamedPkgs[v.renamedFrom] = v.pkg.Name
		}
	}
	return succeededPkgs, renamedPkgs
}

func desktopNotifyIfNeeded(p *print.Printer, result int, enable bool) {
	if enable {
		if result == 0 {
			notify.Info(p, "gup", "All update success")
		} else {
			notify.Warn(p, "gup", "Some package can't update")
		}
	}
}

// updateResultStr renders the per-binary update line, using the pinned-specific
// description for a pinned package and the normal current->latest string
// otherwise.
func updateResultStr(p goutil.Package) string {
	if p.IsPinned() {
		return pinnedResultStr(p)
	}
	return currentToLatestStr(p)
}

// updatePinned installs (or keeps) a pinned package at its exact recorded
// version. It never resolves @latest/@main/@master: the only version that is
// ever installed is p.PinnedVersion. The pin locks the module version, not the
// Go build, so the package is still reinstalled (at the pinned version) when the
// installed binary was built with an older Go toolchain - unless ignoreGoUpdate
// is set, exactly like an unpinned package. It is kept only when the installed
// version matches the pin and the Go toolchain is current; otherwise it is
// reinstalled at the pinned version (which may be a downgrade). On dry-run the
// install runs into the throwaway GOBIN like every other update, so the
// kept/reinstalled outcome is still shown.
func updatePinned(deps dependencies, ctx context.Context, p goutil.Package, ignoreGoUpdate bool) updateResult {
	pinnedVer := strings.TrimSpace(p.PinnedVersion)
	if pinnedVer == "" {
		// Defensive: a pinned channel without a target should be impossible because
		// config validation rejects it, but never silently fall back to @latest.
		return updateResult{
			updated: false,
			pkg:     p,
			err:     fmt.Errorf("%s: pinned package has no recorded version", p.Name),
			status:  statusError,
		}
	}

	if p.Version == nil {
		p.Version = &goutil.Version{}
	}
	p.Version.Latest = pinnedVer

	goOutdated := !ignoreGoUpdate && p.GoVersion != nil && !p.IsGoUpToDate()
	if p.PinSatisfied() && !goOutdated {
		// Hide any Go-version delta we are intentionally not acting on, so the
		// human line reads a clean "pinned <ver>" instead of a phantom rebuild.
		if p.GoVersion != nil {
			p.GoVersion.Latest = p.GoVersion.Current
		}
		return updateResult{
			updated:    false,
			pkg:        p,
			skipped:    true,
			skipReason: "pinned",
			status:     statusPinned,
		}
	}

	if p.ImportPath == "" {
		return updateResult{
			updated: false,
			pkg:     p,
			err:     fmt.Errorf("%s is not installed by 'go install' (or permission incorrect)", p.Name),
			status:  statusError,
		}
	}

	if err := deps.installByVersion(ctx, p.ImportPath, pinnedVer); err != nil {
		return updateResult{
			updated: false,
			pkg:     p,
			err:     fmt.Errorf("%s: %w", p.Name, err),
			status:  statusError,
		}
	}

	// The reinstalled binary now matches the pinned version and was built with the
	// current Go toolchain.
	p.Version.Current = pinnedVer
	if p.GoVersion != nil {
		p.GoVersion.Current = p.GoVersion.Latest
	}
	return updateResult{
		updated: true,
		pkg:     p,
		status:  statusUpdated,
	}
}

func installWithSelectedVersion(deps dependencies, ctx context.Context, importPath string, channel goutil.UpdateChannel) error {
	switch goutil.NormalizeUpdateChannel(string(channel)) {
	case goutil.UpdateChannelLatest:
		return deps.installLatest(ctx, importPath)
	case goutil.UpdateChannelMain:
		return deps.installMainOrMaster(ctx, importPath)
	case goutil.UpdateChannelMaster:
		return deps.installByVersion(ctx, importPath, "master")
	case goutil.UpdateChannelPinned:
		// Pinned packages are installed via updatePinned, never here; never silently
		// degrade a pin to @latest.
		return fmt.Errorf("pinned package %s must be installed at its recorded version, not via channel install", importPath)
	default:
		return deps.installLatest(ctx, importPath)
	}
}

func resolveModulePathChange(pkg goutil.Package, err error) (goutil.Package, bool) {
	declaredPath, requiredPath, ok := goutil.DetectModulePathMismatch(err)
	if !ok {
		return pkg, false
	}

	pkg.ImportPath = replaceImportPathPrefix(pkg.ImportPath, requiredPath, declaredPath)
	pkg.ModulePath = declaredPath
	return pkg, true
}

func replaceImportPathPrefix(importPath, oldModulePath, newModulePath string) string {
	switch {
	case importPath == "":
		return newModulePath
	case importPath == oldModulePath:
		return newModulePath
	case strings.HasPrefix(importPath, oldModulePath+"/"):
		return newModulePath + strings.TrimPrefix(importPath, oldModulePath)
	default:
		return importPath
	}
}

func removeOldBinaryIfRenamed(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	if !isSafeBinaryName(oldName) || !isSafeBinaryName(newName) {
		return fmt.Errorf("refusing to remove binary with unsafe name: old=%q new=%q", oldName, newName)
	}

	goBin, err := goutil.GoBin()
	if err != nil {
		return fmt.Errorf("can't find installed binaries: %w", err)
	}

	oldBinaryPath := filepath.Join(goBin, oldName)
	if _, err := os.Stat(oldBinaryPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("can't stat old binary %s: %w", oldBinaryPath, err)
	}

	if err := os.Remove(oldBinaryPath); err != nil {
		return fmt.Errorf("can't remove old binary %s: %w", oldBinaryPath, err)
	}
	return nil
}

func binaryNameFromImportPath(importPath string) string {
	return binaryNameFromImportPathWith(importPath, runtime.GOOS, os.Getenv("GOEXE"))
}

func binaryNameFromImportPathWith(importPath, goos, goExe string) string {
	binName := filepath.Base(importPath)
	if goos == goosWindows {
		goExe = strings.TrimSpace(goExe)
		if goExe == "" {
			goExe = exeSuffix
		}
		if !strings.HasSuffix(strings.ToLower(binName), strings.ToLower(goExe)) {
			return binName + goExe
		}
	}
	return binName
}

func completePathBinaries(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	binList, _ := pkgselect.BinaryPaths()
	prefix := toComplete
	if runtime.GOOS == goosWindows {
		prefix = strings.ToLower(prefix)
	}
	filtered := make([]string, 0, len(binList))
	for i, b := range binList {
		binList[i] = filepath.Base(b)
		candidate := binList[i]
		if runtime.GOOS == goosWindows {
			candidate = strings.ToLower(candidate)
		}
		if prefix != "" && !strings.HasPrefix(candidate, prefix) {
			continue
		}
		filtered = append(filtered, binList[i])
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}
