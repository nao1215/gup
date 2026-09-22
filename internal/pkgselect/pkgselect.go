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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/gup/internal/binname"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/nao1215/gup/internal/suggest"
)

// BinaryPaths returns the absolute paths of the binaries installed under $GOBIN
// (or $GOPATH/bin).
//
// A $GOBIN/$GOPATH/bin directory that does not exist yet is a normal first-run
// condition, not an error: it is reported as an empty list so list/check/update/
// export behave like any other empty installed-tool set instead of failing with
// a read-dir error (#350).
func BinaryPaths() ([]string, error) {
	goBin, err := goutil.GoBin()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't find installed binaries", err)
	}

	binList, err := goutil.BinaryPathList(goBin)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("%s: %w", "can't get binary-paths installed by 'go install'", err)
	}

	return binList, nil
}

// PackageInfo returns package information for every installed binary without
// reading the Go toolchain version. Use it for commands (list, export) that
// never compare Package.GoVersion, avoiding a needless "go version" subprocess.
func PackageInfo(p *print.Printer) ([]goutil.Package, error) {
	binList, err := BinaryPaths()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	return goutil.GetPackageInformationWithoutGoVersion(p, binList), nil
}

// Selection is what PackageInfoByTargets resolved for a command's targets.
type Selection struct {
	// Packages are the installed binaries matching the targets (every installed
	// binary when there are none).
	Packages []goutil.Package
	// Missing are the targets that match no installed binary, for "not found"
	// reporting.
	Missing []string
	// Installed names every binary in $GOBIN, whether or not it matched a target
	// or could be read. It is the candidate list for "did you mean" suggestions.
	Installed []string
	// GoVersionAvailable reports whether the installed Go version was detected.
	// When it is false, callers must disable Go-version comparison (see issue
	// #296).
	GoVersionAvailable bool
}

// PackageInfoByTargets resolves the installed binaries matching targets (all
// binaries when targets is empty) into a Selection.
//
// Missing is derived from the binary paths, not from the resolved packages, so
// a binary that exists in $GOBIN but whose build info can't be read (or that was
// not installed by 'go install') is never mislabeled as "not found"; it is
// present but unmanageable, and GetPackageInformation already warns about it.
func PackageInfoByTargets(p *print.Printer, targets []string) (Selection, error) {
	binList, err := BinaryPaths()
	if err != nil {
		return Selection{}, fmt.Errorf("%s: %w", "can't get package info", err)
	}

	filtered := FilterBinaryPaths(binList, targets)
	pkgs, goVersionAvailable := goutil.GetPackageInformation(p, filtered)
	return Selection{
		Packages:           pkgs,
		Missing:            MissingTargets(binList, targets),
		Installed:          InstalledNames(binList),
		GoVersionAvailable: goVersionAvailable,
	}, nil
}

// InstalledNames returns the binary names in binList as a user would type them:
// the base name, without the ".exe" suffix Windows gives every binary.
func InstalledNames(binList []string) []string {
	names := make([]string, 0, len(binList))
	for _, path := range binList {
		base := filepath.Base(path)
		if ext := filepath.Ext(base); strings.EqualFold(ext, ".exe") {
			base = strings.TrimSuffix(base, ext)
		}
		names = append(names, base)
	}
	return names
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

// MissingTargets returns the user-specified targets that match no installed
// binary in binList, preserving first-seen order and collapsing duplicate
// targets. The returned names are the trimmed originals, suitable for display.
//
// A target is "missing" only when no binary in $GOBIN shares its name. A binary
// that exists but never becomes a Package (build info unreadable, or not
// installed by 'go install') is NOT reported here: it is present, just
// unmanageable. Matching the resolved packages instead would mislabel such a
// binary as "not found", which is the wrong next step for the user.
func MissingTargets(binList, targets []string) []string {
	if len(targets) == 0 {
		return nil
	}

	present := make(map[string]struct{}, len(binList))
	for _, path := range binList {
		present[binname.NormalizeForMatch(filepath.Base(path))] = struct{}{}
	}

	var missing []string
	seen := make(map[string]struct{}, len(targets))
	for _, rawTarget := range targets {
		normalized := binname.NormalizeForMatch(rawTarget)
		if normalized == "" {
			continue
		}
		if _, dup := seen[normalized]; dup {
			continue
		}
		seen[normalized] = struct{}{}
		if _, ok := present[normalized]; !ok {
			missing = append(missing, strings.TrimSpace(rawTarget))
		}
	}
	return missing
}

// WarnMissing reports each missing target name (as returned by MissingTargets)
// through warn, using the standard "not found ... in $GOBIN" wording, followed by
// a "did you mean" suggestion when an installed binary is a likely typo target.
// warn is called once per name; centralizing the message keeps update and check
// consistent.
func WarnMissing(missing, installed []string, warn func(string)) {
	for _, name := range missing {
		warn("not found '" + name + "' package in $GOPATH/bin or $GOBIN" + didYouMean(name, installed))
	}
}

// WarnUnmatchedExcludes warns about each --exclude name that matches no
// installed binary but is a likely typo of one, so `--exclude lazygti` does not
// quietly update lazygit. An excluded name with no close match stays silent: it
// is normal for an exclude list shared between machines (Topgrade's
// gup_exclude) to name tools this machine never installed, and warning about
// those on every run would be noise.
func WarnUnmatchedExcludes(excludeList, installed []string, warn func(string)) {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[binname.NormalizeForMatch(name)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(excludeList))
	for _, raw := range excludeList {
		normalized := binname.NormalizeForMatch(raw)
		if normalized == "" {
			continue
		}
		if _, ok := present[normalized]; ok {
			continue
		}
		if _, dup := seen[normalized]; dup {
			continue
		}
		seen[normalized] = struct{}{}
		name := strings.TrimSpace(raw)
		if hint := didYouMean(name, installed); hint != "" {
			warn("--exclude '" + name + "' matches no installed binary" + hint)
		}
	}
}

// didYouMean renders the suggestion suffix for name, or "" when no installed
// binary is close enough to suggest.
func didYouMean(name string, installed []string) string {
	if match, ok := suggest.Closest(name, installed); ok {
		return "; did you mean '" + match + "'?"
	}
	return ""
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
