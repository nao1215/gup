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

	"github.com/nao1215/gup/internal/print"
)

// Package is package information
type Package struct {
	// Name is package name
	Name string
	// ImportPath is import path for 'go install'
	ImportPath string
}

// CanUseGoCmd check whether go command install in the system.
func CanUseGoCmd() error {
	_, err := exec.LookPath("go")
	return err
}

// Install execute "$ go install <importPath>"
func Install(importPath string) error {
	if err := exec.Command("go", "install", importPath+"@latest").Run(); err != nil {
		return errors.New("can't install " + importPath)
	}
	return nil
}

// GoPath return GOPATH environment variable.
func GoPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		return gopath
	}
	return build.Default.GOPATH
}

// GoBin return $GOPATH/bin directory path.
func GoBin() (string, error) {
	goBin := os.Getenv("GOBIN")
	if goBin != "" {
		return goBin, nil
	}

	goPath := GoPath()
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
		if e.IsDir() {
			print.Warn("$GOPATH/bin or $GOBIN contains the directory")
		} else {
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
		path := extractPackagePath(out)
		pkg := Package{
			Name:       filepath.Base(v),
			ImportPath: path,
		}
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

// extractPackagePath extract package path from result of "$ go version -m".
func extractPackagePath(lines []string) string {
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
