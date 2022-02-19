package goutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/print"
	"github.com/nao1215/gup/internal/shell"
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
func Install(importPath string) {
	if err := exec.Command("go", "install", importPath+"@latest").Run(); err != nil {
		print.Err(err)
	}
}

// GoPath return GOPATH environment variable.
func GoPath() string {
	return os.Getenv("GOPATH")
}

// GoBin return $GOPATH/bin directory path.
func GoBin() (string, error) {
	goPath := GoPath()
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
			print.Warn("$GOPATH/bin contains the directory")
		} else {
			list = append(list, e.Name())
		}
	}
	return list, nil
}

// GetPackageInformation return golang package information.
func GetPackageInformation(binList []string) ([]Package, error) {
	historyFileList := shell.AllShellHistoryFilePath()

	history := []string{}
	for _, f := range historyFileList {
		if !file.IsFile(f) { // exist check
			continue
		}
		l, err := file.ReadFileToList(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", "can't not read '"+f+"'", err)
		}
		history = append(history, l...)
	}

	installHist := extractInstallHistory(history)
	pkgs := []Package{}
	for _, b := range binList {
		pkg := Package{}
		pkg.Name = b
		for _, v := range installHist {
			if strings.Contains(v, pkg.Name) {
				v = extractPackagePathFromHistroy(v)
				pkg.ImportPath = v
				break
			}
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// extractInstallHistory returns only the history of executing "go install".
func extractInstallHistory(history []string) []string {
	r := regexp.MustCompile(`go\s+install`)

	extract := []string{}
	for _, v := range history {
		if r.MatchString(v) {
			extract = append(extract, v)
		}
	}
	return extract
}

// extractPackagePathFromHistroy extract package path from history.
func extractPackagePathFromHistroy(hist string) string {
	r := regexp.MustCompile(`go\s+install`)
	h := r.ReplaceAllString(hist, "")

	r = regexp.MustCompile(`@.*`)
	h = r.ReplaceAllString(h, "")

	h = strings.ReplaceAll(h, "\n", "")
	h = strings.TrimSpace(h)
	return h
}
