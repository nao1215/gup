package goutil

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/nao1215/gup/internal/parallel"
	"github.com/nao1215/gup/internal/print"
)

var goVersionRegex = regexp.MustCompile(`(^|\s)(go[1-9]\S+)`)

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

// isStandardLibraryCommand reports whether an import path is a Go standard
// library command such as "cmd/go" or "cmd/gofmt". These ship with the Go
// toolchain and "go install <path>@latest" rejects them with "argument must not
// be a package in the standard library", so gup must skip them.
//
// This matters when Go is installed via mise (and similar tools), which places
// the toolchain commands in $GOBIN alongside user-installed binaries, so gup
// finds them and would otherwise try to reinstall them (issue #206). All Go
// commands live under the "cmd/" import-path prefix, and no third-party module
// can claim that prefix (its import path always starts with a host element),
// making this check unambiguous.
func isStandardLibraryCommand(importPath string) bool {
	return importPath == "cmd" || strings.HasPrefix(importPath, "cmd/")
}

// shouldManageBinary reports whether gup can manage a binary, given its import
// path and main module path from build info. It skips Go standard library
// commands (which cannot be reinstalled with "go install <path>@latest"; issue
// #206) and binaries with no recorded main module, such as GOPATH-mode or local
// "go build" binaries (issue #299).
//
// The standard library check is independent of the main module path so a
// toolchain command is skipped even if its build info ever records a main module,
// rather than relying solely on the empty-main-module heuristic.
func shouldManageBinary(importPath, mainModulePath string) bool {
	if isStandardLibraryCommand(importPath) {
		return false
	}
	return isModuleBinary(mainModulePath)
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
func GetPackageInformation(p *print.Printer, binList []string) ([]Package, bool) {
	goVer, err := GetInstalledGoVersion()
	if err != nil {
		p.Warn(fmt.Sprintf("failed to detect installed Go version (%v); "+
			"skipping Go-version comparison this run. Module versions are still checked.", err))
		return collectPackageInformation(p, binList, unknown), false
	}
	return collectPackageInformation(p, binList, goVer), true
}

// GetPackageInformationWithoutGoVersion is like GetPackageInformation but skips
// the "go version" subprocess. Use it for commands (list, export, migrate) that
// never read Package.GoVersion, avoiding a needless subprocess per invocation.
func GetPackageInformationWithoutGoVersion(p *print.Printer, binList []string) []Package {
	return collectPackageInformation(p, binList, unknown)
}

// collectPackageInformation reads build info for each binary in parallel and
// stamps goVer as the latest Go toolchain version on every package. It delegates
// the bounded worker pool to internal/parallel.Run so the concurrency logic is
// not duplicated. No context or timeout is used because buildinfo.ReadFile is a
// fast local read, so onCancel never fires.
func collectPackageInformation(p *print.Printer, binList []string, goVer string) []Package {
	if len(binList) == 0 {
		return nil
	}

	type indexedPkg struct {
		pkg Package
		ok  bool
	}

	results := parallel.Run(
		context.Background(),
		binList,
		runtime.NumCPU(),
		0, // no timeout: buildinfo.ReadFile is a fast local read
		func(_ context.Context, v string) indexedPkg {
			info, err := buildinfo.ReadFile(v)
			if err != nil {
				p.Warn(err)
				return indexedPkg{}
			}
			if !shouldManageBinary(info.Path, info.Main.Path) {
				return indexedPkg{}
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
			return indexedPkg{pkg: pkg, ok: true}
		},
		func(_ string, _ error) indexedPkg { return indexedPkg{} },
		nil,
	)

	pkgs := make([]Package, 0, len(binList))
	for _, r := range results {
		if r.ok {
			pkgs = append(pkgs, r.pkg)
		}
	}
	return pkgs
}

// GetPackageVersion return golang package version.
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
