//nolint:paralleltest // grouped with the rest of the cmd test suite
package cmd

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// Literals reused across the property tests, kept as constants to satisfy
// goconst (which counts occurrences across the whole package).
const (
	testGOOSLinux = "linux"
)

// quickConfig returns a quick.Config with a fixed RNG seed so that any failing
// input is reproducible across local and CI runs.
func quickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 500,
		Rand:     rand.New(rand.NewSource(1)), //nolint:gosec // G404: a fixed non-crypto seed is intentional for reproducible property tests
	}
}

// importPathInput generates import paths with several path segments so the
// derived binary name is the final element.
type importPathInput string

// Generate implements quick.Generator for importPathInput.
func (importPathInput) Generate(rng *rand.Rand, _ int) reflect.Value {
	letters := []rune("abcdefghij0123456789")
	seg := func() string {
		n := 1 + rng.Intn(6)
		var sb strings.Builder
		for range n {
			sb.WriteRune(letters[rng.Intn(len(letters))])
		}
		return sb.String()
	}
	first := seg()
	extra := rng.Intn(4)
	parts := make([]string, 0, 2+extra)
	parts = append(parts, "example.com", first)
	for range extra {
		parts = append(parts, seg())
	}
	return reflect.ValueOf(importPathInput(strings.Join(parts, "/")))
}

// TestBinaryNameFromImportPath_properties asserts:
//   - the result never contains a path separator (it is always a base name);
//   - the transform is stable for already-normalized input, i.e. running it on
//     an import path whose base name is the previous result returns that same
//     name (f is a fixed point on its own output), for both linux and windows.
func TestBinaryNameFromImportPath_properties(t *testing.T) {
	// Property: result is always a bare base name (no '/' or filepath separator).
	noSeparator := func(p importPathInput) bool {
		for _, goos := range []string{testGOOSLinux, goosWindows} {
			got := binaryNameFromImportPathWith(string(p), goos, "")
			if strings.ContainsRune(got, '/') || strings.ContainsRune(got, '\\') {
				return false
			}
		}
		return true
	}
	if err := quick.Check(noSeparator, quickConfig()); err != nil {
		t.Errorf("no-separator property failed: %v", err)
	}

	// Property: stable for already-normalized input. Feeding the previous result
	// back in (as a single-segment import path) yields the same binary name.
	stable := func(p importPathInput) bool {
		for _, goos := range []string{testGOOSLinux, goosWindows} {
			first := binaryNameFromImportPathWith(string(p), goos, "")
			second := binaryNameFromImportPathWith(first, goos, "")
			if first != second {
				return false
			}
		}
		return true
	}
	if err := quick.Check(stable, quickConfig()); err != nil {
		t.Errorf("stability property failed: %v", err)
	}

	// The default (host) wrapper must also produce a separator-free name.
	if got := binaryNameFromImportPath("example.com/owner/cmd/tool"); strings.ContainsAny(got, "/\\") {
		t.Errorf("binaryNameFromImportPath() leaked a separator: %q", got)
	}
}
