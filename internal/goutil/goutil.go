package goutil

import (
	"errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/print"
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

// SetCurrentVer set package current version.
func (p *Package) SetCurrentVer() {
	p.Version.Current = GetPackageVersion(p.Name)
}

// SetLatestVer set package latest version.
func (p *Package) SetLatestVer() {
	p.Version.Latest = GetPackageVersion(p.Name)
}

// CurrentToLatestStr returns string about the current version and the latest version
func (p *Package) CurrentToLatestStr() string {
	if IsAlreadyUpToDate(*p.Version) {
		return "Already up-to-date: " + color.GreenString(p.Version.Latest)
	}
	return color.GreenString(p.Version.Current) + " to " + color.YellowString(p.Version.Latest)
}

// VersionCheckResultStr returns string about command version check.
func (p *Package) VersionCheckResultStr() string {
	if IsAlreadyUpToDate(*p.Version) {
		return "Already up-to-date: " + color.GreenString(p.Version.Latest)
	}
	return "current: " + color.GreenString(p.Version.Current) + ", latest: " + color.YellowString(p.Version.Latest)
}

// IsAlreadyUpToDate return whether binary is already up to date or not.
func IsAlreadyUpToDate(ver Version) bool {
	return ver.Current == ver.Latest
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
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	if gp.GOBIN != "" {
		if err := os.Setenv("GOBIN", tmpDir); err != nil {
			return err
		}
	} else if gp.GOPATH != "" {
		if err := os.Setenv("GOPATH", tmpDir); err != nil {
			return err
		}
	} else {
		return errors.New("$GOPATH and $GOBIN is not set")
	}
	return nil
}

// EndDryRunMode restore the GOBIN or GOPATH settings.
func (gp *GoPaths) EndDryRunMode() error {
	if gp.GOBIN != "" {
		if err := os.Setenv("GOBIN", gp.GOBIN); err != nil {
			return err
		}
	} else if gp.GOPATH != "" {
		if err := os.Setenv("GOPATH", gp.GOPATH); err != nil {
			return err
		}
	} else {
		return errors.New("$GOPATH and $GOBIN is not set")
	}

	if err := gp.removeTmpDir(); err != nil {
		return fmt.Errorf("%s: %w", "temporary directory for dry run remains", err)
	}
	return nil
}

// removeTmpDir remove tmporary directory for dry run
func (gp *GoPaths) removeTmpDir() error {
	if gp.TmpPath != "" {
		if err := os.RemoveAll(gp.TmpPath); err != nil {
			return err
		}
	}
	return nil
}

// CanUseGoCmd check whether go command install in the system.
func CanUseGoCmd() error {
	_, err := exec.LookPath("go")
	return err
}

// Install execute "$ go install <importPath>@latest"
func Install(importPath string) error {
	if importPath == "command-line-arguments" {
		return errors.New("is devel-binary copied from local environment")
	}
	if err := exec.Command("go", "install", importPath+"@latest").Run(); err != nil {
		return fmt.Errorf("can't install %s: %w", importPath, err)
	}
	return nil
}

// GetLatestVer execute "$ go list -m -f {{.Version}} <importPath>@latest"
func GetLatestVer(modulePath string) (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", modulePath+"@latest").Output()
	if err != nil {
		return "", errors.New("can't check " + modulePath)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// goPath return GOPATH environment variable.
func goPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		return gopath
	}
	return build.Default.GOPATH
}

// goBin return GOBIN environment variable.
func goBin() string {
	return os.Getenv("GOBIN")
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

// GoVersionWithOptionM return result of "$ go version -m"
func GoVersionWithOptionM(bin string) ([]string, error) {
	out, err := exec.Command("go", "version", "-m", bin).Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(out), "\n"), nil
}

// BinaryPathList return list of binary paths.
func BinaryPathList(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	list := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			list = append(list, filepath.Join(path, e.Name()))
		}
	}
	return list, nil
}

// GetPackageInformation return golang package information.
func GetPackageInformation(binList []string) []Package {
	pkgs := []Package{}
	for _, v := range binList {
		out, err := GoVersionWithOptionM(v)
		if err != nil {
			print.Warn(fmt.Errorf("%s: %w", "can not get package path", err))
			continue
		}
		path := extractImportPath(out)
		mod := extractModulePath(out)
		pkg := Package{
			Name:       filepath.Base(v),
			ImportPath: path,
			ModulePath: mod,
			Version:    NewVersion(),
		}
		pkg.SetCurrentVer()
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

	out, err := GoVersionWithOptionM(filepath.Join(goBin, cmdName))
	if err != nil {
		return "unknown"
	}

	for _, v := range out {
		vv := strings.TrimSpace(v)
		if len(v) != len(vv) && strings.HasPrefix(vv, "mod") {
			//         mod     github.com/nao1215/subaru       v1.0.2  h1:LU9/1bFyqef3re6FVSFgTFMSXCZvrmDpmX3KQtlHzXA=
			v = strings.TrimLeft(vv, "mod")
			v = strings.TrimSpace(v)

			//github.com/nao1215/subaru       v1.0.2  h1:LU9/1bFyqef3re6FVSFgTFMSXCZvrmDpmX3KQtlHzXA=
			r := regexp.MustCompile(`^[^\s]+(\s)`)
			v = r.ReplaceAllString(v, "")

			// v1.0.2  h1:LU9/1bFyqef3re6FVSFgTFMSXCZvrmDpmX3KQtlHzXA=
			r = regexp.MustCompile(`(\s)[^\s]+$`)

			// v1.0.2
			return r.ReplaceAllString(v, "")
		}
	}
	return "unknown"
}

// extractImportPath extract package import path from result of "$ go version -m".
func extractImportPath(lines []string) string {
	for _, v := range lines {
		vv := strings.TrimSpace(v)
		if len(v) != len(vv) && strings.HasPrefix(vv, "path") {
			vv = strings.TrimLeft(vv, "path")
			vv = strings.TrimSpace(vv)
			return strings.TrimRight(vv, "\n")
		}
	}
	return ""
}

// extractModulePath extract package module path from result of "$ go version -m".
func extractModulePath(lines []string) string {
	for _, v := range lines {
		vv := strings.TrimSpace(v)
		if len(v) != len(vv) && strings.HasPrefix(vv, "mod") {
			//         mod     github.com/nao1215/subaru       v1.0.2  h1:LU9/1bFyqef3re6FVSFgTFMSXCZvrmDpmX3KQtlHzXA=
			v = strings.TrimLeft(vv, "mod")
			v = strings.TrimSpace(v)

			//github.com/nao1215/subaru       v1.0.2  h1:LU9/1bFyqef3re6FVSFgTFMSXCZvrmDpmX3KQtlHzXA=
			r := regexp.MustCompile(`(\s).*$`)
			return r.ReplaceAllString(v, "")
		}
	}
	return ""
}
