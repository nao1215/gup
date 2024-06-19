package goutil

import (
	"bytes"
	"debug/buildinfo"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/gorky/file"
	"github.com/nao1215/gup/internal/print"
	"github.com/pkg/errors"
)

// Internal variables to mock/monkey-patch behaviors in tests.
var (
	// goExe is the executable name for the go command.
	goExe = "go"
	// keyGoBin is the key name of the env variable for "GOBIN".
	keyGoBin = "GOBIN"
	// keyGoPath is the key name of the env variable for "GOPATH".
	keyGoPath = "GOPATH"
	// osMkdirTemp is a copy of os.MkdirTemp to ease testing.
	osMkdirTemp = os.MkdirTemp
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
	// ImportPath is import path for 'go install'
	ImportPath string
	// ModulePath is path where go.mod is stored
	ModulePath string
	// Version store Package version (current and latest).
	Version *Version
	// GoVersion stores version of Go toolchain
	GoVersion *Version
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

// SetLatestVer set package latest version.
func (p *Package) SetLatestVer() {
	p.Version.Latest = GetPackageVersion(p.Name)
}

// CurrentToLatestStr returns string about the current version and the latest version
func (p *Package) CurrentToLatestStr() string {
	if p.IsAlreadyUpToDate() {
		return "Already up-to-date: " + color.GreenString(p.Version.Latest) + " / " + color.GreenString(p.GoVersion.Current)
	}
	var ret string
	if p.Version.Current != p.Version.Latest {
		ret += color.GreenString(p.Version.Current) + " to " + color.YellowString(p.Version.Latest)
	}
	if p.GoVersion.Current != p.GoVersion.Latest {
		if len(ret) != 0 {
			ret += ", "
		}
		ret += color.GreenString(p.GoVersion.Current) + " to " + color.YellowString(p.GoVersion.Latest)
	}
	return ret
}

// VersionCheckResultStr returns string about command version check.
func (p *Package) VersionCheckResultStr() string {
	if p.IsAlreadyUpToDate() {
		return "Already up-to-date: " + color.GreenString(p.Version.Latest) + " / " + color.GreenString(p.GoVersion.Current)
	}
	var ret string
	// TODO: yellow only if latest > current
	if p.Version.Current == p.Version.Latest {
		ret += color.GreenString(p.Version.Current)
	} else {
		ret += "current: " + color.GreenString(p.Version.Current) + ", latest: "
		if versionUpToDate(p.Version.Current, p.Version.Latest) {
			ret += color.GreenString(p.Version.Latest)
		} else {
			ret += color.YellowString(p.Version.Latest)
		}
	}
	ret += " / "
	if p.GoVersion.Current == p.GoVersion.Latest {
		ret += color.GreenString(p.GoVersion.Current)
	} else {
		ret += "current: " + color.GreenString(p.GoVersion.Current) + ", installed: "
		if versionUpToDate(p.GoVersion.Current, p.GoVersion.Latest) {
			ret += color.GreenString(p.GoVersion.Latest)
		} else {
			ret += color.YellowString(p.GoVersion.Latest)
		}
	}
	return ret
}

// IsAlreadyUpToDate return whether binary is already up to date or not.
func (p *Package) IsAlreadyUpToDate() bool {
	if p.Version.Current == p.Version.Latest && p.GoVersion.Current == p.GoVersion.Latest {
		return true
	}

	return versionUpToDate(
		strings.TrimLeft(p.Version.Current, "v"),
		strings.TrimLeft(p.Version.Latest, "v"),
	) && versionUpToDate(
		strings.TrimLeft(p.GoVersion.Current, "go"),
		strings.TrimLeft(p.GoVersion.Latest, "go"),
	)
}

func versionUpToDate(current, available string) bool {
	return current >= available
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

	if gp.GOBIN != "" {
		if err := os.Setenv(keyGoBin, tmpDir); err != nil {
			// Wrap error to avoid OS dependent error message during testing.
			return errors.Wrapf(
				err,
				"failed to set GOBIN to env variable. key: %v, value: %v",
				keyGoBin, tmpDir,
			)
		}
	} else if gp.GOPATH != "" {
		if err := os.Setenv(keyGoPath, tmpDir); err != nil {
			return errors.Wrapf(
				err,
				"failed to set GOPATH to env variable. key: %v, value: %v",
				keyGoPath, tmpDir,
			)
		}
	} else {
		return errors.New("$GOPATH and $GOBIN is not set")
	}
	return nil
}

// EndDryRunMode restore the GOBIN or GOPATH settings.
func (gp *GoPaths) EndDryRunMode() error {
	if gp.GOBIN != "" {
		if err := os.Setenv(keyGoBin, gp.GOBIN); err != nil {
			// Wrap error to avoid OS dependent error message during testing.
			return errors.Wrapf(
				err,
				"failed to set GOBIN to env variable. key: %v, value: %v",
				keyGoBin, gp.GOBIN,
			)
		}
	} else if gp.GOPATH != "" {
		if err := os.Setenv(keyGoPath, gp.GOPATH); err != nil {
			return errors.Wrapf(
				err,
				"failed to set GOPATH to env variable. key: %v, value: %v",
				keyGoPath, gp.GOPATH,
			)
		}
	} else {
		return errors.New("$GOPATH and $GOBIN is not set")
	}

	if err := gp.removeTmpDir(); err != nil {
		return errors.Wrap(err, "temporary directory for dry run remains")
	}
	return nil
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
	return install(importPath, "latest")
}

// InstallMainOrMaster execute "$ go install <importPath>@main" or "$ go install <importPath>@master"
func InstallMainOrMaster(importPath string) error {
	mainErr := install(importPath, "main")
	if mainErr != nil {
		// Previous error is "invalid version: unknown revision main". Not return this error.
		masterErr := install(importPath, "master")
		if masterErr == nil {
			return nil
		}
		const errMsg = "cannot update with @master or @main using the 'gup'. please update manually."
		if strings.Contains(mainErr.Error(), "unknown revision main") {
			return fmt.Errorf("%s\n%w", errMsg, masterErr)
		} else if strings.Contains(masterErr.Error(), "unknown revision master") {
			return fmt.Errorf("%s\n%w", errMsg, mainErr)
		}
		return fmt.Errorf("%s\n%s\n%w", errMsg, mainErr.Error(), masterErr)
	}
	return nil
}

// install execute "$ go install <importPath>@<version>"
func install(importPath, version string) error {
	if importPath == "command-line-arguments" {
		return errors.New("is devel-binary copied from local environment")
	}

	var stderr bytes.Buffer
	cmd := exec.Command(goExe, "install", fmt.Sprintf("%s@%s", importPath, version)) //#nosec
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("can't install %s:\n%s", importPath, stderr.String())
	}
	return nil
}

// GetLatestVer execute "$ go list -m -f {{.Version}} <importPath>@latest"
func GetLatestVer(modulePath string) (string, error) {
	out, err := exec.Command(goExe, "list", "-m", "-f", "{{.Version}}", modulePath+"@latest").Output() //#nosec
	if err != nil {
		return "", errors.New("can't check " + modulePath)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// goPath return GOPATH environment variable.
func goPath() string {
	gopath := os.Getenv(keyGoPath)
	if gopath != "" {
		return gopath
	}
	out, err := exec.Command(goExe, "env", keyGoPath).Output()
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

		path := filepath.Join(path, e.Name())
		if file.IsHiddenFile(path) {
			continue
		}
		list = append(list, path)
	}
	return list, nil
}

// GetPackageInformation return golang package information.
func GetPackageInformation(binList []string) []Package {
	pkgs := []Package{}
	goVer, err := GetInstalledGoVersion()
	if err != nil {
		goVer = "unknown"
	}
	for _, v := range binList {
		info, err := buildinfo.ReadFile(v)
		if err != nil {
			print.Warn(err)
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
		pkg.GoVersion.Current = info.GoVersion
		pkg.GoVersion.Latest = goVer
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

// GetPackageVersion return golang package version
func GetPackageVersion(cmdName string) string {
	goBin, err := GoBin()
	if err != nil {
		return "unknown"
	}
	info, err := buildinfo.ReadFile(filepath.Join(goBin, cmdName))
	if err != nil {
		return "unknown"
	}
	return info.Main.Version
}

var goVersionRegex = regexp.MustCompile(`(^|\s)(go[1-9]\S+)`)

func GetInstalledGoVersion() (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(goExe, "version")
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
