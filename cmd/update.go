package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/notify"
	"github.com/nao1215/gup/internal/print"
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
	cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
	if err := cmd.RegisterFlagCompletionFunc("jobs", completeNCPUs); err != nil {
		panic(err)
	}
	cmd.Flags().Bool("ignore-go-update", false, "Ignore updates to the Go toolchain")
	cmd.Flags().Bool("json", false, "output result as machine-readable JSON")
	cmd.Flags().BoolP("quiet", "q", false, "suppress up-to-date lines; show only updated/failed binaries plus a summary")
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

	pkgs, goVersionAvailable, err := getPackageInfoByTargets(args)
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

	pkgs = extractUserSpecifyPkg(pkgs, args)
	pkgs = excludePkgs(excludePkgList, pkgs, jsonOut)

	if len(pkgs) == 0 {
		print.Err("unable to update package: no package information or no package under $GOBIN")
		return 1
	}

	confReadPath, resolveErr := config.ResolveImportFilePath("")
	if resolveErr != nil {
		// update only reads the config for channel hints and is not the
		// command targeted by the ambiguity check, so fall back to the
		// user-level config instead of failing.
		confReadPath = config.FilePath()
	}
	confWritePath := config.FilePath()
	if fileutil.IsFile(confReadPath) {
		confWritePath = confReadPath
	}

	confPkgs, err := readConfFileIfExists(confReadPath)
	if err != nil {
		print.Warn(fmt.Sprintf("failed to read %s: %s (continuing without config)", confReadPath, err))
		confPkgs = []goutil.Package{}
	}

	channelMap, err := resolveUpdateChannels(pkgs, confPkgs, mainPkgNames, masterPkgNames, latestPkgNames)
	if err != nil {
		print.Err(err)
		return 1
	}

	result, succeededPkgs, renamedPkgs := updateWithChannels(pkgs, dryRun, notify, cpus, ignoreGoUpdate, channelMap, timeout, jsonOut, quiet)

	if !dryRun && (shouldPersistChannels(mainPkgNames, masterPkgNames, latestPkgNames) || len(renamedPkgs) > 0) {
		merged := mergeConfigPackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
		if err := writeConfigFile(confWritePath, merged); err != nil {
			print.Warn("failed to write " + confWritePath + ": " + err.Error())
		}
	}

	return result
}

// excludePkgs drops the binaries named in excludePkgList from pkgs. In JSON mode
// the human-readable "Exclude ..." notice is suppressed so STDOUT stays valid
// JSON (the notice goes to STDOUT via print.Info, which would otherwise break
// machine-readable output; see issue #291).
func excludePkgs(excludePkgList []string, pkgs []goutil.Package, jsonOut bool) []goutil.Package {
	excluded := make(map[string]struct{}, len(excludePkgList))
	for _, name := range excludePkgList {
		normalized := normalizeBinaryNameForMatch(name)
		if normalized == "" {
			continue
		}
		excluded[normalized] = struct{}{}
	}

	packageList := []goutil.Package{}
	for _, v := range pkgs {
		if _, ok := excluded[normalizeBinaryNameForMatch(v.Name)]; ok {
			if !jsonOut {
				print.Info(fmt.Sprintf("Exclude '%s' from the update target", v.Name))
			}
			continue
		}
		packageList = append(packageList, v)
	}
	return packageList
}

func normalizeBinaryNameForMatch(name string) string {
	name = strings.TrimSpace(name)
	if runtime.GOOS != goosWindows {
		return name
	}
	name = strings.ToLower(name)
	if strings.HasSuffix(name, ".exe") {
		return strings.TrimSuffix(name, ".exe")
	}
	return name
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

func updateWithChannels(pkgs []goutil.Package, dryRun, notification bool, cpus int, ignoreGoUpdate bool, channelMap map[string]goutil.UpdateChannel, timeout time.Duration, jsonOut, quiet bool) (exitCode int, succeeded []goutil.Package, renamed map[string]string) {
	dryRunManager := goutil.NewGoPaths()

	verCache := newLatestVerCache()

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
		channel := packageUpdateChannel(p.Name, p.UpdateChannel, channelMap)
		p.UpdateChannel = channel

		// Collect online channel version if possible; else always update
		shouldUpdate := true
		modulePathChanged := false
		if p.ModulePath != "" {
			ver, err := verCache.getByChannel(ctx, p.ModulePath, channel)
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

				ver, err = verCache.getByChannel(ctx, p.ModulePath, channel)
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
			goExe = ".exe"
		}
		if !strings.HasSuffix(strings.ToLower(binName), strings.ToLower(goExe)) {
			return binName + goExe
		}
	}
	return binName
}

func packageUpdateChannel(name string, fallback goutil.UpdateChannel, channelMap map[string]goutil.UpdateChannel) goutil.UpdateChannel {
	if channel, ok := channelMap[name]; ok {
		return goutil.NormalizeUpdateChannel(string(channel))
	}
	return goutil.NormalizeUpdateChannel(string(fallback))
}

func readConfFileIfExists(path string) ([]goutil.Package, error) {
	if !fileutil.IsFile(path) {
		return []goutil.Package{}, nil
	}
	pkgs, err := config.ReadConfFile(path)
	if err != nil {
		return nil, err
	}
	return pkgs, nil
}

func shouldPersistChannels(mainPkgNames, masterPkgNames, latestPkgNames []string) bool {
	return len(mainPkgNames) > 0 || len(masterPkgNames) > 0 || len(latestPkgNames) > 0
}

func resolveUpdateChannels(
	pkgs []goutil.Package,
	confPkgs []goutil.Package,
	mainPkgNames []string,
	masterPkgNames []string,
	latestPkgNames []string,
) (map[string]goutil.UpdateChannel, error) {
	channelMap := make(map[string]goutil.UpdateChannel, len(pkgs))
	normalizedToActual := make(map[string]string, len(pkgs))
	for _, p := range pkgs {
		channelMap[p.Name] = goutil.UpdateChannelLatest
		normalizedToActual[normalizeBinaryNameForMatch(p.Name)] = p.Name
	}
	for _, p := range confPkgs {
		if actual, ok := normalizedToActual[normalizeBinaryNameForMatch(p.Name)]; ok {
			channelMap[actual] = goutil.NormalizeUpdateChannel(string(p.UpdateChannel))
		}
	}

	assignedByFlag := map[string]string{}
	apply := func(flag string, names []string, channel goutil.UpdateChannel) error {
		for _, raw := range names {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			normalized := normalizeBinaryNameForMatch(name)
			if prevFlag, ok := assignedByFlag[normalized]; ok && prevFlag != flag {
				return fmt.Errorf("same binary (%s) is specified in both --%s and --%s", name, prevFlag, flag)
			}
			assignedByFlag[normalized] = flag

			actual, ok := normalizedToActual[normalized]
			if !ok {
				print.Warn("not found '" + name + "' package in update target")
				continue
			}
			channelMap[actual] = channel
		}
		return nil
	}

	if err := apply("main", mainPkgNames, goutil.UpdateChannelMain); err != nil {
		return nil, err
	}
	if err := apply("master", masterPkgNames, goutil.UpdateChannelMaster); err != nil {
		return nil, err
	}
	if err := apply(latestKeyword, latestPkgNames, goutil.UpdateChannelLatest); err != nil {
		return nil, err
	}
	return channelMap, nil
}

func mergeConfigPackages(confPkgs []goutil.Package, succeededPkgs []goutil.Package, channelMap map[string]goutil.UpdateChannel, renamedPkgs map[string]string) []goutil.Package {
	pkgByName := map[string]goutil.Package{}
	for _, p := range confPkgs {
		pkgByName[p.Name] = sanitizeConfigPackage(p)
	}
	for _, p := range succeededPkgs {
		if p.Name == "" || p.ImportPath == "" {
			continue
		}
		channel := packageUpdateChannel(p.Name, p.UpdateChannel, channelMap)
		pkgByName[p.Name] = goutil.Package{
			Name:          p.Name,
			ImportPath:    p.ImportPath,
			Version:       &goutil.Version{Current: persistedVersion(p)},
			UpdateChannel: channel,
		}
	}
	// Remove stale entries when a binary was renamed during update
	for oldName := range renamedPkgs {
		delete(pkgByName, oldName)
	}
	for name, channel := range channelMap {
		p, ok := pkgByName[name]
		if !ok {
			continue
		}
		p.UpdateChannel = goutil.NormalizeUpdateChannel(string(channel))
		pkgByName[name] = sanitizeConfigPackage(p)
	}

	names := make([]string, 0, len(pkgByName))
	for name := range pkgByName {
		names = append(names, name)
	}
	sort.Strings(names)

	merged := make([]goutil.Package, 0, len(names))
	for _, name := range names {
		merged = append(merged, pkgByName[name])
	}
	return merged
}

func sanitizeConfigPackage(p goutil.Package) goutil.Package {
	version := latestKeyword
	if p.Version != nil {
		v := strings.TrimSpace(p.Version.Current)
		if v != "" {
			version = v
		}
	}

	return goutil.Package{
		Name:          strings.TrimSpace(p.Name),
		ImportPath:    strings.TrimSpace(p.ImportPath),
		Version:       &goutil.Version{Current: version},
		UpdateChannel: goutil.NormalizeUpdateChannel(string(p.UpdateChannel)),
	}
}

func persistedVersion(p goutil.Package) string {
	if p.Version == nil {
		return latestKeyword
	}
	if latest := strings.TrimSpace(p.Version.Latest); latest != "" && latest != "unknown" {
		return latest
	}
	if current := strings.TrimSpace(p.Version.Current); current != "" {
		return current
	}
	return latestKeyword
}

func getBinaryPathList() ([]string, error) {
	goBin, err := goutil.GoBin()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't find installed binaries", err)
	}

	binList, err := goutil.BinaryPathList(goBin)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get binary-paths installed by 'go install'", err)
	}

	return binList, nil
}

func getPackageInfo() ([]goutil.Package, error) {
	binList, err := getBinaryPathList()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	// list and export never read GoVersion, so skip the "go version" subprocess.
	return goutil.GetPackageInformationWithoutGoVersion(binList), nil
}

// getPackageInfoByTargets returns the packages to process and a bool reporting
// whether the installed Go version was detected. When the bool is false,
// callers must disable Go-version comparison (see issue #296).
func getPackageInfoByTargets(targets []string) ([]goutil.Package, bool, error) {
	binList, err := getBinaryPathList()
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	filtered := filterBinaryPathListByTargets(binList, targets)
	pkgs, goVersionAvailable := goutil.GetPackageInformation(filtered)
	return pkgs, goVersionAvailable, nil
}

func filterBinaryPathListByTargets(binList, targets []string) []string {
	if len(targets) == 0 {
		return binList
	}

	targetSet := make(map[string]struct{}, len(targets))
	for _, rawTarget := range targets {
		target := normalizeBinaryNameForMatch(rawTarget)
		if target == "" {
			continue
		}
		targetSet[target] = struct{}{}
	}
	if len(targetSet) == 0 {
		return []string{}
	}

	filtered := make([]string, 0, len(targetSet))
	for _, path := range binList {
		base := normalizeBinaryNameForMatch(filepath.Base(path))
		if _, ok := targetSet[base]; ok {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func extractUserSpecifyPkg(pkgs []goutil.Package, targets []string) []goutil.Package {
	result := []goutil.Package{}
	if len(targets) == 0 {
		return pkgs
	}

	targetSet := make(map[string]string, len(targets)) // normalized target -> original (first seen)
	targetOrder := make([]string, 0, len(targets))
	for _, rawTarget := range targets {
		target := normalizeBinaryNameForMatch(rawTarget)
		if target == "" {
			continue
		}
		if _, exists := targetSet[target]; !exists {
			targetSet[target] = strings.TrimSpace(rawTarget)
			targetOrder = append(targetOrder, target)
		}
	}

	matched := make(map[string]struct{}, len(targetSet))
	for _, v := range pkgs {
		pkg := normalizeBinaryNameForMatch(v.Name)
		if _, ok := targetSet[pkg]; ok {
			result = append(result, v)
			matched[pkg] = struct{}{}
		}
	}

	for _, target := range targetOrder {
		if _, ok := matched[target]; !ok {
			print.Warn("not found '" + targetSet[target] + "' package in $GOPATH/bin or $GOBIN")
		}
	}
	return result
}

func completePathBinaries(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	binList, _ := getBinaryPathList()
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
