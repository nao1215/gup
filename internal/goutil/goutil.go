package goutil

import (
	"bytes"
	"context"
	"debug/buildinfo"
	stderrors "errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/nao1215/gup/internal/print"
	"github.com/pkg/errors"
)

const unknown = "unknown"

// UpdateChannel is the update source channel for go install.
type UpdateChannel string

const (
	// UpdateChannelLatest updates by @latest.
	UpdateChannelLatest UpdateChannel = "latest"
	// UpdateChannelMain updates by @main (and fallback to @master if main is missing).
	UpdateChannelMain UpdateChannel = "main"
	// UpdateChannelMaster updates by @master.
	UpdateChannelMaster UpdateChannel = "master"
)

// Internal variables to mock/monkey-patch behaviors in tests.
var (
	// goExe is the executable name for the go command.
	goExe = "go" //nolint:gochecknoglobals
	// keyGoBin is the key name of the env variable for "GOBIN".
	keyGoBin = "GOBIN" //nolint:gochecknoglobals
	// keyGoPath is the key name of the env variable for "GOPATH".
	keyGoPath = "GOPATH" //nolint:gochecknoglobals
	// osMkdirTemp is a copy of os.MkdirTemp to ease testing.
	osMkdirTemp = os.MkdirTemp //nolint:gochecknoglobals
	// goCommandContext builds the *exec.Cmd used to run the go toolchain. It is a
	// variable so tests can swap in the standard "helper process" pattern
	// (re-executing the test binary) and exercise the subprocess-driven helpers
	// deterministically without network access. Production code always uses the
	// default, which simply runs goExe with the given arguments.
	goCommandContext = func(ctx context.Context, args ...string) *exec.Cmd { //nolint:gochecknoglobals
		return exec.CommandContext(ctx, goExe, args...) //#nosec G204 -- args are built internally, not from untrusted input
	}
)

// GoPaths has $GOBIN and $GOPATH
type GoPaths struct {
	// GOBIN is $GOBIN
	GOBIN string
	// GOPATH is $GOPATH
	GOPATH string
	// TmpPath is tmporary path for dry run
	TmpPath string
}

// Package is package information
type Package struct {
	// Name is package name
	Name string
	// ImportPath is the package import path used by 'go install'
	ImportPath string
	// ModulePath is path where go.mod is stored.
	// May not be set if module path cannot be determined
	ModulePath string
	// Version store Package version (current and latest).
	Version *Version
	// GoVersion stores version of Go toolchain
	GoVersion *Version
	// UpdateChannel stores preferred update channel.
	UpdateChannel UpdateChannel
}

// Version is package version information.
type Version struct {
	// Current(before update) version
	Current string
	// Latest(after update) version
	Latest string
}

// NewVersion return Version instance.
func NewVersion() *Version {
	return &Version{
		Current: "",
		Latest:  "",
	}
}

// NormalizeUpdateChannel normalizes a user/config value into a valid channel.
// Unknown or blank values are treated as "latest".
func NormalizeUpdateChannel(channel string) UpdateChannel {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case string(UpdateChannelMain):
		return UpdateChannelMain
	case string(UpdateChannelMaster):
		return UpdateChannelMaster
	case string(UpdateChannelLatest):
		return UpdateChannelLatest
	default:
		return UpdateChannelLatest
	}
}

// SetLatestVer set package latest version.
func (p *Package) SetLatestVer() {
	p.Version.Latest = GetPackageVersion(p.Name)
}

// CurrentToLatestStr returns string about the current version and the latest version
func (p *Package) CurrentToLatestStr() string {
	if p.IsPackageUpToDate() && p.IsGoUpToDate() {
		return "Already up-to-date: " + color.GreenString(p.Version.Current) + " / " + color.GreenString(p.GoVersion.Current)
	}
	var ret string
	if p.Version.Current != p.Version.Latest {
		currentVer, latestVer := colorVersionPair(p.Version.Current, p.Version.Latest, "v")
		ret += currentVer + " to " + latestVer
	}
	if p.GoVersion.Current != p.GoVersion.Latest {
		if len(ret) != 0 {
			ret += ", "
		}
		currentGo, latestGo := colorVersionPair(p.GoVersion.Current, p.GoVersion.Latest, "go")
		ret += currentGo + " to " + latestGo
	}
	return ret
}

// VersionCheckResultStr returns string about command version check.
func (p *Package) VersionCheckResultStr() string {
	if p.IsPackageUpToDate() && p.IsGoUpToDate() {
		return "Already up-to-date: " + color.GreenString(p.Version.Current) + " / " + color.GreenString(p.GoVersion.Current)
	}
	var ret string
	currentVer, latestVer := colorVersionPair(p.Version.Current, p.Version.Latest, "v")
	if p.Version.Current == p.Version.Latest {
		ret += currentVer
	} else {
		ret += "current: " + currentVer + ", latest: " + latestVer
	}
	ret += " / "
	currentGo, latestGo := colorVersionPair(p.GoVersion.Current, p.GoVersion.Latest, "go")
	if p.GoVersion.Current == p.GoVersion.Latest {
		ret += currentGo
	} else {
		ret += "current: " + currentGo + ", installed: " + latestGo
	}
	return ret
}

func colorVersionPair(current, latest, prefix string) (string, string) {
	upToDate := versionUpToDate
	if prefix == "go" {
		upToDate = goVersionUpToDate
	}

	currentNoPrefix := strings.TrimPrefix(current, prefix)
	latestNoPrefix := strings.TrimPrefix(latest, prefix)
	currentUpToDate := upToDate(currentNoPrefix, latestNoPrefix)
	latestUpToDate := upToDate(latestNoPrefix, currentNoPrefix)

	switch {
	case currentUpToDate && latestUpToDate:
		return color.GreenString(current), color.GreenString(latest)
	case currentUpToDate:
		return color.GreenString(current), color.YellowString(latest)
	case latestUpToDate:
		return color.YellowString(current), color.GreenString(latest)
	default:
		return color.YellowString(current), color.YellowString(latest)
	}
}

// IsPackageUpToDate checks if the Package (set by the package author) version is up to date.
// Returns true if current >= available.
func (p *Package) IsPackageUpToDate() bool {
	return versionUpToDate(
		strings.TrimPrefix(p.Version.Current, "v"),
		strings.TrimPrefix(p.Version.Latest, "v"),
	)
}

// IsGoUpToDate checks if the Golang runtime version is up to date.
// Returns true if current >= available.
func (p *Package) IsGoUpToDate() bool {
	return goVersionUpToDate(
		strings.TrimPrefix(p.GoVersion.Current, "go"),
		strings.TrimPrefix(p.GoVersion.Latest, "go"),
	)
}

// goVersionUpToDate compares Go toolchain versions.
// Some custom toolchains append non-semver separators (e.g. "X:nodwarf5"),
// so normalize known separators before semver comparison.
func goVersionUpToDate(current, available string) bool {
	current = strings.TrimSpace(current)
	available = strings.TrimSpace(available)
	if current == unknown || available == unknown {
		return false
	}
	// If both strings are exactly the same, treat as up-to-date even when
	// custom tags are not strict semver.
	if current == available {
		return true
	}
	return versionUpToDate(
		normalizeGoVersionForCompare(current),
		normalizeGoVersionForCompare(available),
	)
}

func normalizeGoVersionForCompare(ver string) string {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return ver
	}

	var b strings.Builder
	b.Grow(len(ver))
	for _, r := range ver {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '.',
			r == '-',
			r == '+':
			b.WriteRune(r)
		default:
			b.WriteByte('.')
		}
	}
	return b.String()
}

// versionUpToDate parses versions and compares them.
// Returns true if current >= available.
func versionUpToDate(current, available string) bool {
	if current == unknown || available == unknown {
		return false // unknown version is not up to date
	}

	currentVer, err := version.NewVersion(current)
	if err != nil {
		return false // invalid version is not up to date
	}
	availableVer, err := version.NewVersion(available)
	if err != nil {
		return false // invalid version is not up to date
	}

	if currentVer.GreaterThanOrEqual(availableVer) {
		return true
	}
	return false
}

// NewGoPaths return GoPaths instance.
func NewGoPaths() *GoPaths {
	return &GoPaths{
		GOBIN:  goBin(),
		GOPATH: goPath(),
	}
}

// StartDryRunMode change the GOBIN or GOPATH settings to install the binaries in the temporary directory.
func (gp *GoPaths) StartDryRunMode() error {
	tmpDir, err := osMkdirTemp("", "")
	if err != nil {
		return err
	}
	gp.TmpPath = tmpDir

	switch {
	case gp.GOBIN != "":
		if err := os.Setenv(keyGoBin, tmpDir); err != nil {
			// Avoid leaking the temp dir when the env mutation fails.
			_ = gp.removeTmpDir()
			// Wrap error to avoid OS dependent error message during testing.
			return errors.Wrapf(
				err,
				"failed to set GOBIN to env variable. key: %v, value: %v",
				keyGoBin, tmpDir,
			)
		}
	case gp.GOPATH != "":
		if err := os.Setenv(keyGoPath, tmpDir); err != nil {
			_ = gp.removeTmpDir()
			return errors.Wrapf(
				err,
				"failed to set GOPATH to env variable. key: %v, value: %v",
				keyGoPath, tmpDir,
			)
		}
	default:
		_ = gp.removeTmpDir()
		return errors.New("$GOPATH and $GOBIN is not set")
	}
	return nil
}

// EndDryRunMode restore the GOBIN or GOPATH settings and remove the temporary
// directory. The temp dir is always removed, even when restoring the env
// variable fails, so a failed restore does not leak the directory (see issue
// #297). Any restore and removal errors are joined into the returned error.
func (gp *GoPaths) EndDryRunMode() error {
	var restoreErr error
	switch {
	case gp.GOBIN != "":
		if err := os.Setenv(keyGoBin, gp.GOBIN); err != nil {
			// Wrap error to avoid OS dependent error message during testing.
			restoreErr = errors.Wrapf(
				err,
				"failed to set GOBIN to env variable. key: %v, value: %v",
				keyGoBin, gp.GOBIN,
			)
		}
	case gp.GOPATH != "":
		if err := os.Setenv(keyGoPath, gp.GOPATH); err != nil {
			restoreErr = errors.Wrapf(
				err,
				"failed to set GOPATH to env variable. key: %v, value: %v",
				keyGoPath, gp.GOPATH,
			)
		}
	default:
		restoreErr = errors.New("$GOPATH and $GOBIN is not set")
	}

	var removeErr error
	if err := gp.removeTmpDir(); err != nil {
		removeErr = errors.Wrap(err, "temporary directory for dry run remains")
	}

	return stderrors.Join(restoreErr, removeErr)
}

// removeTmpDir remove tmporary directory for dry run
func (gp *GoPaths) removeTmpDir() error {
	if gp.TmpPath != "" {
		return os.RemoveAll(gp.TmpPath)
	}
	return nil
}

// CanUseGoCmd check whether go command install in the system.
func CanUseGoCmd() error {
	_, err := exec.LookPath(goExe)
	return err
}

// InstallLatest execute "$ go install <importPath>@latest"
func InstallLatest(importPath string) error {
	return InstallLatestWithContext(context.Background(), importPath)
}

// InstallLatestWithContext executes "$ go install <importPath>@latest".
func InstallLatestWithContext(ctx context.Context, importPath string) error {
	return InstallWithContext(ctx, importPath, "latest")
}

// InstallMainOrMaster execute "$ go install <importPath>@main" or "$ go install <importPath>@master"
func InstallMainOrMaster(importPath string) error {
	return InstallMainOrMasterWithContext(context.Background(), importPath)
}

// IsBranchNotFound reports whether err indicates that the given branch (e.g.
// "main") does not exist in the module's repository, as opposed to a build,
// network, authentication, or other failure. The go toolchain reports a missing
// branch as "unknown revision <branch>". This is the only condition under which
// gup is allowed to fall back from @main to @master (#340); any other failure
// must surface as-is so a wrong-branch version is never silently installed.
func IsBranchNotFound(err error, branch string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unknown revision "+branch)
}

// InstallMainOrMasterWithContext executes "$ go install <importPath>@main"
// or "$ go install <importPath>@master" with context cancellation support.
//
// The @master fallback is taken only when @main fails because the main branch
// does not exist. Build failures, network/proxy/auth errors, timeouts, and
// cancellations on @main are returned as-is and never trigger a @master install
// (#340).
func InstallMainOrMasterWithContext(ctx context.Context, importPath string) error {
	mainErr := InstallWithContext(ctx, importPath, "main")
	if mainErr == nil {
		return nil
	}
	// A canceled/expired context would just hit @master too; surface the @main
	// error instead of retrying.
	if ctx != nil && ctx.Err() != nil {
		return mainErr
	}
	// Only a missing main branch justifies trying @master.
	if !IsBranchNotFound(mainErr, "main") {
		return mainErr
	}

	masterErr := InstallWithContext(ctx, importPath, "master")
	if masterErr == nil {
		return nil
	}
	const errMsg = "cannot update with @master or @main using the 'gup'. please update manually."
	return fmt.Errorf("%s\n%w", errMsg, masterErr)
}

// Install executes "$ go install <importPath>@<version>".
func Install(importPath, version string) error {
	return InstallWithContext(context.Background(), importPath, version)
}

// InstallWithContext executes "$ go install <importPath>@<version>".
func InstallWithContext(ctx context.Context, importPath, version string) error {
	if importPath == "command-line-arguments" {
		return errors.New("is devel-binary copied from local environment")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var stderr bytes.Buffer
	cmd := goCommandContext(ctx, "install", fmt.Sprintf("%s@%s", importPath, version))
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return fmt.Errorf("install of %s timed out; run `go install %s@%s` manually or raise --timeout (0 disables it): %w", importPath, importPath, version, ctxErr)
			}
			return fmt.Errorf("install of %s canceled: %w", importPath, ctxErr)
		}
		// A killed subprocess (e.g. SIGKILL) often writes nothing to stderr, so
		// fall back to err (e.g. "signal: killed") to always name a cause.
		detail := stderr.String()
		if strings.TrimSpace(detail) == "" {
			detail = err.Error()
		}
		return fmt.Errorf("can't install %s:\n%s", importPath, detail)
	}
	return nil
}

// GetLatestVer execute "$ go list -m -f {{.Version}} <importPath>@latest"
func GetLatestVer(modulePath string) (string, error) {
	return GetLatestVerWithContext(context.Background(), modulePath)
}

// GetLatestVerWithContext execute "$ go list -m -f {{.Version}} <importPath>@latest"
// with context cancellation support.
func GetLatestVerWithContext(ctx context.Context, modulePath string) (string, error) {
	return GetVerWithContext(ctx, modulePath, "latest")
}

// GetVerWithContext execute "$ go list -m -f {{.Version}} <modulePath>@<ref>"
// with context cancellation support. ref is the version selector understood by
// the go toolchain, such as "latest", "main", "master" or a concrete version.
func GetVerWithContext(ctx context.Context, modulePath, ref string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var stderr bytes.Buffer
	cmd := goCommandContext(ctx, "list", "-m", "-f", "{{.Version}}", modulePath+"@"+ref)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return "", fmt.Errorf("version check of %s timed out; run `go list -m %s@%s` manually or raise --timeout (0 disables it): %w", modulePath, modulePath, ref, ctxErr)
			}
			return "", fmt.Errorf("version check of %s canceled: %w", modulePath, ctxErr)
		}
		// A killed subprocess (e.g. SIGKILL) often writes nothing to stderr, so
		// fall back to err (e.g. "signal: killed") to always name a cause.
		detail := stderr.String()
		if strings.TrimSpace(detail) == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("can't check %s:\n%s", modulePath, detail)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// goPath return GOPATH environment variable.
func goPath() string {
	gopath := os.Getenv(keyGoPath)
	if gopath != "" {
		return gopath
	}
	out, err := exec.CommandContext(context.Background(), goExe, "env", keyGoPath).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return build.Default.GOPATH
}

// goBin return GOBIN environment variable.
func goBin() string {
	return os.Getenv(keyGoBin)
}

// GoBin return $GOPATH/bin directory path.
func GoBin() (string, error) {
	goBin := goBin()
	if goBin != "" {
		return goBin, nil
	}

	goPath := goPath()
	if goPath == "" {
		return "", errors.New("$GOPATH is not set")
	}
	return filepath.Join(goPath, "bin"), nil
}

// BinaryPathList return list of binary paths.
func BinaryPathList(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	list := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		list = append(list, filepath.Join(path, e.Name()))
	}
	return list, nil
}

// isModuleBinary reports whether a binary was produced by
// "go install <module>@<version>" and therefore records a main module path that
// gup can manage. The argument is the main module path from the binary's build
// info (debug/buildinfo's Main.Path), not its import path.
//
// Standard library and toolchain binaries (e.g. cmd/gofmt) record no main
// module, as do GOPATH-mode or local "go build" binaries; those are skipped.
// Using the recorded module instead of a "dotless first import-path element"
// heuristic avoids misclassifying third-party binaries whose host has no dot,
// such as localhost/... or an internal registry hostname (issue #299).
func isModuleBinary(mainModulePath string) bool {
	return mainModulePath != ""
}

// GetPackageInformation return golang package information including the latest
// installed Go toolchain version. Use it for commands that compare Go versions
// (check, update). Binary info is read in parallel using a worker pool.
//
// The second return value reports whether the installed Go version was
// detected. When it is false, callers must disable Go-version comparison
// (behave as --ignore-go-update); otherwise a transient "go version" failure
// stamps "unknown" on every package and forces a needless reinstall of all
// binaries (see issue #296).
func GetPackageInformation(binList []string) ([]Package, bool) {
	goVer, err := GetInstalledGoVersion()
	if err != nil {
		print.Warn(fmt.Sprintf("failed to detect installed Go version (%v); "+
			"skipping Go-version comparison this run. Module versions are still checked.", err))
		return collectPackageInformation(binList, unknown), false
	}
	return collectPackageInformation(binList, goVer), true
}

// GetPackageInformationWithoutGoVersion is like GetPackageInformation but skips
// the "go version" subprocess. Use it for commands (list, export, migrate) that
// never read Package.GoVersion, avoiding a needless subprocess per invocation.
func GetPackageInformationWithoutGoVersion(binList []string) []Package {
	return collectPackageInformation(binList, unknown)
}

// collectPackageInformation reads build info for each binary in parallel and
// stamps goVer as the latest Go toolchain version on every package.
func collectPackageInformation(binList []string, goVer string) []Package {
	if len(binList) == 0 {
		return nil
	}

	type indexedPkg struct {
		pkg Package
		ok  bool
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > len(binList) {
		numWorkers = len(binList)
	}

	results := make([]indexedPkg, len(binList))
	jobs := make(chan int, len(binList))
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				v := binList[i]
				info, err := buildinfo.ReadFile(v)
				if err != nil {
					print.Warn(err)
					continue
				}
				if !isModuleBinary(info.Main.Path) {
					continue
				}
				pkg := Package{
					Name:       filepath.Base(v),
					ImportPath: info.Path,
					ModulePath: info.Main.Path,
					Version:    NewVersion(),
					GoVersion:  NewVersion(),
				}
				pkg.Version.Current = info.Main.Version
				pkg.GoVersion.Current, _, _ = strings.Cut(info.GoVersion, " ")
				pkg.GoVersion.Latest = goVer
				results[i] = indexedPkg{pkg: pkg, ok: true}
			}
		}()
	}

	for i := range binList {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	pkgs := make([]Package, 0, len(binList))
	for _, r := range results {
		if r.ok {
			pkgs = append(pkgs, r.pkg)
		}
	}
	return pkgs
}

// GetPackageVersion return golang package version
func GetPackageVersion(cmdName string) string {
	goBin, err := GoBin()
	if err != nil {
		return unknown
	}
	info, err := buildinfo.ReadFile(filepath.Join(goBin, cmdName))
	if err != nil {
		return unknown
	}
	return info.Main.Version
}

var goVersionRegex = regexp.MustCompile(`(^|\s)(go[1-9]\S+)`)
var moduleDeclaresPathRegex = regexp.MustCompile(`(?m)module declares its path as:\s*(\S+)`)
var requiredAsPathRegex = regexp.MustCompile(`(?m)but was required as:\s*(\S+)`)

// GetInstalledGoVersion return installed go version.
func GetInstalledGoVersion() (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := goCommandContext(context.Background(), "version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("can't check go version:\n%s", stderr.String())
	}

	if m := goVersionRegex.FindStringSubmatch(stdout.String()); m != nil {
		return m[2], nil
	}

	return "", fmt.Errorf("can't find go version string in %q", strings.TrimSpace(stdout.String()))
}

// DetectModulePathMismatch detects module path mismatch errors from go command output.
// It returns:
//   - declaredPath: module path declared in go.mod
//   - requiredPath: module path that was originally required
//   - ok: true when both paths are detected and they differ
func DetectModulePathMismatch(err error) (declaredPath, requiredPath string, ok bool) {
	if err == nil {
		return "", "", false
	}

	declared := moduleDeclaresPathRegex.FindStringSubmatch(err.Error())
	required := requiredAsPathRegex.FindStringSubmatch(err.Error())
	if len(declared) < 2 || len(required) < 2 {
		return "", "", false
	}

	declaredPath = strings.TrimSpace(declared[1])
	requiredPath = strings.TrimSpace(required[1])
	if declaredPath == "" || requiredPath == "" || declaredPath == requiredPath {
		return "", "", false
	}
	return declaredPath, requiredPath, true
}
