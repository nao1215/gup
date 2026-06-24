// Package pkgselect discovers the binaries installed by 'go install' and selects
// which of them a command should act on. It holds the package-selection logic
// (target filtering, exclusion, user-specified narrowing) that the update,
// check, list, export and migrate commands share, keeping the cmd layer a thin
// wiring/output shell over this reusable core.
//
// Functions that would otherwise print progress take a callback (notify/warn)
// so the caller owns the output sink. This mirrors configstate.ResolveChannels
// and keeps the selection logic free of any dependency on the print package,
// which in turn lets JSON mode suppress human-readable notices simply by passing
// a no-op (see issue #291).
package pkgselect

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nao1215/gup/internal/binname"
	"github.com/nao1215/gup/internal/goutil"
)

// BinaryPaths returns the absolute paths of the binaries installed under $GOBIN
// (or $GOPATH/bin).
func BinaryPaths() ([]string, error) {
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

// PackageInfo returns package information for every installed binary without
// reading the Go toolchain version. Use it for commands (list, export) that
// never compare Package.GoVersion, avoiding a needless "go version" subprocess.
func PackageInfo() ([]goutil.Package, error) {
	binList, err := BinaryPaths()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	return goutil.GetPackageInformationWithoutGoVersion(binList), nil
}

// PackageInfoByTargets returns package information for the installed binaries
// matching targets (all binaries when targets is empty) together with a bool
// reporting whether the installed Go version was detected. When the bool is
// false, callers must disable Go-version comparison (see issue #296).
func PackageInfoByTargets(targets []string) ([]goutil.Package, bool, error) {
	binList, err := BinaryPaths()
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	filtered := FilterBinaryPaths(binList, targets)
	pkgs, goVersionAvailable := goutil.GetPackageInformation(filtered)
	return pkgs, goVersionAvailable, nil
}

// FilterBinaryPaths returns the subset of binList whose base name matches one of
// targets. When targets is empty the whole list is returned; when every target
// is blank an empty list is returned. Matching uses binname.NormalizeForMatch,
// so names are trimmed (and on Windows compared case-insensitively without the
// ".exe" suffix).
func FilterBinaryPaths(binList, targets []string) []string {
	if len(targets) == 0 {
		return binList
	}

	targetSet := make(map[string]struct{}, len(targets))
	for _, rawTarget := range targets {
		target := binname.NormalizeForMatch(rawTarget)
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
		base := binname.NormalizeForMatch(filepath.Base(path))
		if _, ok := targetSet[base]; ok {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

// ExtractByTargets returns the packages whose names match targets, preserving
// the order of pkgs. When targets is empty pkgs is returned unchanged. For each
// target that matches no package, warn is called exactly once (duplicate targets
// are collapsed).
func ExtractByTargets(pkgs []goutil.Package, targets []string, warn func(string)) []goutil.Package {
	result := []goutil.Package{}
	if len(targets) == 0 {
		return pkgs
	}

	targetSet := make(map[string]string, len(targets)) // normalized target -> original (first seen)
	targetOrder := make([]string, 0, len(targets))
	for _, rawTarget := range targets {
		target := binname.NormalizeForMatch(rawTarget)
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
		pkg := binname.NormalizeForMatch(v.Name)
		if _, ok := targetSet[pkg]; ok {
			result = append(result, v)
			matched[pkg] = struct{}{}
		}
	}

	for _, target := range targetOrder {
		if _, ok := matched[target]; !ok {
			warn("not found '" + targetSet[target] + "' package in $GOPATH/bin or $GOBIN")
		}
	}
	return result
}

// Exclude returns pkgs with the binaries named in excludeList removed. For each
// excluded package it calls notify with a human-readable notice; pass a no-op to
// suppress it (e.g. JSON mode, where STDOUT must stay valid JSON — issue #291).
// notify is never called when nothing is excluded.
func Exclude(pkgs []goutil.Package, excludeList []string, notify func(string)) []goutil.Package {
	excluded := make(map[string]struct{}, len(excludeList))
	for _, name := range excludeList {
		normalized := binname.NormalizeForMatch(name)
		if normalized == "" {
			continue
		}
		excluded[normalized] = struct{}{}
	}

	packageList := []goutil.Package{}
	for _, v := range pkgs {
		if _, ok := excluded[binname.NormalizeForMatch(v.Name)]; ok {
			notify(fmt.Sprintf("Exclude '%s' from the update target", v.Name))
			continue
		}
		packageList = append(packageList, v)
	}
	return packageList
}
