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
	"github.com/nao1215/gup/internal/vercache"
	"github.com/spf13/cobra"
)

var (
	getLatestVerCtx        = goutil.GetLatestVerWithContext        //nolint:gochecknoglobals // swapped in tests
	getVerByRefCtx         = goutil.GetVerWithContext              //nolint:gochecknoglobals // swapped in tests
	installLatestCtx       = goutil.InstallLatestWithContext       //nolint:gochecknoglobals // swapped in tests
	installMainOrMasterCtx = goutil.InstallMainOrMasterWithContext //nolint:gochecknoglobals // swapped in tests
	installByVersionUpdCtx = goutil.InstallWithContext             //nolint:gochecknoglobals // swapped in tests
)

const latestKeyword = "latest"

// newVerCache builds the per-(module,channel) version cache used by update and
// check, wiring the package-level lookup seams into vercache's channel policy.
// The seams are read at call time (via closures) so tests that swap them before
// invoking a command still take effect.
func newVerCache() *vercache.Cache {
	return vercache.New(vercache.ChannelResolver(
		func(ctx context.Context, modulePath string) (string, error) {
			return getLatestVerCtx(ctx, modulePath)
		},
		func(ctx context.Context, modulePath, ref string) (string, error) {
			return getVerByRefCtx(ctx, modulePath, ref)
		},
	))
}

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
			OsExit(gup(cmd, args))
		},
		ValidArgsFunction: completePathBinaries,
	}
	cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
	cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
	cmd.Flags().StringSliceP("exclude", "e", []string{}, "specify binaries which should not be updated (delimiter: ',')")
	if err := cmd.RegisterFlagCompletionFunc("exclude", completePathBinaries); err != nil {
		panic(err)
	}
	cmd.Flags().StringSliceP("main", "m", []string{}, "specify binaries which update by @main or @master (delimiter: ',')")
	if err := cmd.RegisterFlagCompletionFunc("main", completePathBinaries); err != nil {
		panic(err)
	}
	cmd.Flags().StringSlice("master", []string{}, "specify binaries which update by @master (delimiter: ',')")
	if err := cmd.RegisterFlagCompletionFunc("master", completePathBinaries); err != nil {
		panic(err)
	}
	cmd.Flags().StringSlice(latestKeyword, []string{}, "specify binaries which update by @latest (delimiter: ',')")
	if err := cmd.RegisterFlagCompletionFunc(latestKeyword, completePathBinaries); err != nil {
		panic(err)
	}
	// cmd.Flags().BoolP("main-all", "M", false, "update all binaries by @main or @master (delimiter: ',')")
	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "specify the number of CPU cores to use")
	if err := cmd.RegisterFlagCompletionFunc("jobs", completeNCPUs); err != nil {
		panic(err)
	}
	cmd.Flags().Bool("ignore-go-update", false, "ignore updates to the Go toolchain")
	cmd.Flags().Bool("json", false, "output result as machine-readable JSON")
	cmd.Flags().BoolP("quiet", "q", false, "suppress up-to-date lines; show only updated/failed binaries plus a summary")
	cmd.Flags().StringP("file", "f", "", "specify gup.json file path to read/write saved update channels")
	if err := cmd.MarkFlagFilename("file", "json"); err != nil {
		panic(err)
	}
	addTimeoutFlag(cmd)

	return cmd
}

// gup is main sequence.
// All errors are handled in this function.
func gup(cmd *cobra.Command, args []string) int {
	if err := ensureGoCommandAvailable(); err != nil {
		print.Err(err)
		return 1
	}

	dryRun, err := getFlagBool(cmd, "dry-run")
	if err != nil {
		print.Err(err)
		return 1
	}

	notify, err := getFlagBool(cmd, "notify")
	if err != nil {
		print.Err(err)
		return 1
	}

	cpus, err := getFlagInt(cmd, "jobs")
	if err != nil {
		print.Err(err)
		return 1
	}
	cpus = clampJobs(cpus)

	ignoreGoUpdate, err := getFlagBool(cmd, "ignore-go-update")
	if err != nil {
		print.Err(err)
		return 1
	}

	jsonOut, err := getFlagBool(cmd, "json")
	if err != nil {
		print.Err(err)
		return 1
	}

	quiet, err := getFlagBool(cmd, "quiet")
	if err != nil {
		print.Err(err)
		return 1
	}

	timeout, err := getTimeoutFlag(cmd)
	if err != nil {
		print.Err(err)
		return 1
	}

	pkgs, goVersionAvailable, err := pkgselect.PackageInfoByTargets(args)
	if err != nil {
		print.Err(err)
		return 1
	}
	// When the installed Go version can't be detected, behave as
	// --ignore-go-update so a transient "go version" failure does not force
	// every binary to reinstall (see issue #296).
	ignoreGoUpdate = ignoreGoUpdate || !goVersionAvailable

	excludePkgList, err := getFlagStringSlice(cmd, "exclude")
	if err != nil {
		print.Err(err)
		return 1
	}

	mainPkgNames, err := getFlagStringSlice(cmd, "main")
	if err != nil {
		print.Err(err)
		return 1
	}
	masterPkgNames, err := getFlagStringSlice(cmd, "master")
	if err != nil {
		print.Err(err)
		return 1
	}
	latestPkgNames, err := getFlagStringSlice(cmd, latestKeyword)
	if err != nil {
		print.Err(err)
		return 1
	}

	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		print.Err(err)
		return 1
	}

	pkgs = pkgselect.ExtractByTargets(pkgs, args, func(msg string) { print.Warn(msg) })
	// In JSON mode the human-readable "Exclude ..." notice is suppressed so
	// STDOUT stays valid JSON (the notice goes to STDOUT via print.Info, which
	// would otherwise break machine-readable output; see issue #291).
	excludeNotify := func(string) {}
	if !jsonOut {
		excludeNotify = func(msg string) { print.Info(msg) }
	}
	pkgs = pkgselect.Exclude(pkgs, excludePkgList, excludeNotify)

	if len(pkgs) == 0 {
		// With explicit targets or --exclude, an empty result means the user
		// narrowed everything out: that is a usage error.
		if len(args) != 0 || len(excludePkgList) != 0 {
			print.Err("unable to update package: no package information or no package under $GOBIN")
			return 1
		}
		// An explicitly named --file must be validated even when no binaries are
		// installed: honoring explicit user input must not depend on unrelated
		// environment state (#368).
		if err := configstate.ValidateExplicitFile(confFile); err != nil {
			print.Err(err)
			return 1
		}
		// Otherwise the environment simply has no manageable binaries yet, which
		// is a normal first-run condition, not an error (#350): emit an empty
		// JSON array or an informational note and exit 0.
		if jsonOut {
			if err := encodeJSONPackages(nil); err != nil {
				print.Err(err)
				return 1
			}
			return 0
		}
		print.Info(emptyEnvMessage)
		return 0
	}

	// When both the user-level config and ./gup.json exist and no --file is
	// given, fail fast instead of silently choosing one (#342), consistent with
	// import and check.
	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		print.Err(err)
		return 1
	}
	confWritePath := configstate.ResolveWritePath(confFile, confReadPath)

	// A malformed or unreadable config must fail fast instead of silently
	// falling back to @latest, which would update from the wrong channel and
	// then persist that downgrade back to gup.json (#369).
	confPkgs, err := configstate.ReadFileIfExists(confReadPath)
	if err != nil {
		print.Err(err)
		return 1
	}

	channelMap, err := configstate.ResolveChannels(pkgs, confPkgs, mainPkgNames, masterPkgNames, latestPkgNames,
		func(msg string) { print.Warn(msg) })
	if err != nil {
		print.Err(err)
		return 1
	}

	result, succeededPkgs, renamedPkgs := updateWithChannels(pkgs, dryRun, notify, cpus, ignoreGoUpdate, channelMap, timeout, jsonOut, quiet)

	if !dryRun && (configstate.ShouldPersistChannels(mainPkgNames, masterPkgNames, latestPkgNames) || len(renamedPkgs) > 0) {
		merged := configstate.MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
		if err := writeConfigFile(confWritePath, merged); err != nil {
			print.Warn("failed to write " + confWritePath + ": " + err.Error())
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
	inputIndex  int    // position of this package in the original input slice (#365)
}

func updateWithChannels(pkgs []goutil.Package, dryRun, notification bool, cpus int, ignoreGoUpdate bool, channelMap map[string]goutil.UpdateChannel, timeout time.Duration, jsonOut, quiet bool) (exitCode int, succeeded []goutil.Package, renamed map[string]string) {
	dryRunManager := goutil.NewGoPaths()

	verCache := newVerCache()

	if !jsonOut && !quiet {
		print.Info("update binary under $GOPATH/bin or $GOBIN")
	}
	if dryRun {
		if err := dryRunManager.StartDryRunMode(); err != nil {
			print.Err(fmt.Errorf("can not change to dry run mode: %w", err))
			notify.Warn("gup", "Can not change to dry run mode")
			return 1, nil, nil
		}
		// Restore the environment and remove the temp dir via defer so it runs
		// even if a package update panics (see issue #297).
		defer func() {
			if err := dryRunManager.EndDryRunMode(); err != nil {
				print.Err(fmt.Errorf("can not change dry run mode to normal mode: %w", err))
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
			if err := installWithSelectedVersion(ctx, p.ImportPath, channel); err != nil {
				newPkg, changed := resolveModulePathChange(p, err)
				if !changed {
					updateErr = fmt.Errorf("%s: %w", p.Name, err)
				} else {
					installedViaRetry = true
					p = newPkg
					if retryErr := installWithSelectedVersion(ctx, p.ImportPath, channel); retryErr != nil {
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
		onResult = func(prefix string, v updateResult) {
			if quiet {
				// In quiet mode show only binaries that were actually updated,
				// without the [i/n] progress counter (which would be sparse).
				if v.updated {
					print.Info(fmt.Sprintf("%s (%s)", v.pkg.ImportPath, v.pkg.CurrentToLatestStr()))
				}
				return
			}
			print.Info(fmt.Sprintf("%s %s (%s)", prefix, v.pkg.ImportPath, v.pkg.CurrentToLatestStr()))
		}
	}

	// update all packages
	result, results := executePackages(pkgs, cpus, timeout, updater, onResult)

	if jsonOut {
		if err := encodeJSONPackages(resultsToJSONPackages(results)); err != nil {
			print.Err(err)
			result = 1
		}
	} else if quiet {
		print.Info(summarizeResults(results, false))
	}

	desktopNotifyIfNeeded(result, notification)

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

func desktopNotifyIfNeeded(result int, enable bool) {
	if enable {
		if result == 0 {
			notify.Info("gup", "All update success")
		} else {
			notify.Warn("gup", "Some package can't update")
		}
	}
}

func installWithSelectedVersion(ctx context.Context, importPath string, channel goutil.UpdateChannel) error {
	switch goutil.NormalizeUpdateChannel(string(channel)) {
	case goutil.UpdateChannelLatest:
		return installLatestCtx(ctx, importPath)
	case goutil.UpdateChannelMain:
		return installMainOrMasterCtx(ctx, importPath)
	case goutil.UpdateChannelMaster:
		return installByVersionUpdCtx(ctx, importPath, "master")
	default:
		return installLatestCtx(ctx, importPath)
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
