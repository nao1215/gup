// Package e2e holds the bootstrap for gup's offline end-to-end suite: the OS
// classification of the atago specs (this file) and the runner that builds gup,
// starts the offline module proxy, and hands the right specs to atago
// (e2e/runner).
package e2e

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// MatrixFileName is the committed classification table, relative to the e2e
// directory.
const MatrixFileName = "os_matrix.tsv"

// SpecDirName holds the atago specs, relative to the e2e directory.
const SpecDirName = "atago"

// SupportedOS lists the operating systems the classification covers. It matches
// the platforms gup is tested and released on.
var SupportedOS = []string{"linux", "darwin", "windows"} //nolint:gochecknoglobals // the fixed platform list

// Classification records which operating systems one spec file runs on, and why
// it does not run on the others.
type Classification struct {
	// Spec is the spec file name, relative to e2e/atago.
	Spec string
	// OS is the set of GOOS values the spec runs on.
	OS map[string]bool
	// Reason explains the exclusions. It is required whenever the spec does not
	// run everywhere: an untested platform with no recorded reason is the thing
	// this table exists to prevent.
	Reason string
}

// RunsOn reports whether the spec is classified for goos.
func (c Classification) RunsOn(goos string) bool { return c.OS[goos] }

// LoadMatrix parses the classification table under dir (the e2e directory).
//
// The table is what stops the Windows and macOS legs from being an unstated
// subset that quietly shrinks. Every spec file has a row, every row that
// excludes a platform carries a written reason, and a test fails when a spec is
// added without a decision -- so a new spec is either portable or its
// portability gap is on the record.
func LoadMatrix(dir string) ([]Classification, error) {
	path := filepath.Join(dir, MatrixFileName)
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("can not read the OS matrix: %w", err)
	}
	defer func() { _ = file.Close() }()

	var out []Classification
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimRight(scanner.Text(), " \t")
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s:%d: want <spec>\\t<os-list>[\\t<reason>], got %q", path, line, text)
		}
		spec := strings.TrimSpace(fields[0])
		if seen[spec] {
			return nil, fmt.Errorf("%s:%d: %s is classified twice", path, line, spec)
		}
		seen[spec] = true

		osSet := map[string]bool{}
		for goos := range strings.SplitSeq(fields[1], ",") {
			goos = strings.TrimSpace(goos)
			if goos == "" {
				continue
			}
			if !slices.Contains(SupportedOS, goos) {
				return nil, fmt.Errorf("%s:%d: %s names the unknown OS %q", path, line, spec, goos)
			}
			osSet[goos] = true
		}
		if len(osSet) == 0 {
			return nil, fmt.Errorf("%s:%d: %s lists no operating system", path, line, spec)
		}

		reason := ""
		if len(fields) > 2 {
			reason = strings.TrimSpace(fields[2])
		}
		if len(osSet) < len(SupportedOS) && reason == "" {
			return nil, fmt.Errorf("%s:%d: %s does not run everywhere and gives no reason", path, line, spec)
		}
		out = append(out, Classification{Spec: spec, OS: osSet, Reason: reason})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("can not read %s: %w", path, err)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Spec < out[j].Spec })
	return out, nil
}

// SpecFiles lists the spec files present under dir/atago.
func SpecFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, SpecDirName))
	if err != nil {
		return nil, fmt.Errorf("can not list the atago specs: %w", err)
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".atago.yaml") {
			continue
		}
		out = append(out, entry.Name())
	}
	sort.Strings(out)
	return out, nil
}

// TargetsFor returns the spec paths (under dir) to run on goos, in the order
// they are classified.
func TargetsFor(dir, goos string) ([]string, error) {
	matrix, err := LoadMatrix(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range matrix {
		if entry.RunsOn(goos) {
			out = append(out, filepath.Join(dir, SpecDirName, entry.Spec))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no spec is classified to run on %s", goos)
	}
	return out, nil
}
