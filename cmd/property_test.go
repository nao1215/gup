//nolint:paralleltest // grouped with the rest of the cmd test suite
package cmd

import (
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/quick"
)

// Literals reused across the property tests, kept as constants to satisfy
// goconst (which counts occurrences across the whole package).
const (
	testGOOSLinux = "linux"
	testDotExe    = ".exe"
)

// quickConfig returns a quick.Config with a fixed RNG seed so that any failing
// input is reproducible across local and CI runs.
func quickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 500,
		Rand:     rand.New(rand.NewSource(1)),
	}
}

// binBaseName generates plausible binary base names: letters/digits with an
// optional case-mixed ".exe"/".EXE" suffix and optional surrounding spaces. It
// is bounded so the property checks stay fast and deterministic-ish.
type binBaseName string

// Generate implements quick.Generator for binBaseName.
func (binBaseName) Generate(rng *rand.Rand, _ int) reflect.Value {
	letters := []rune("abcdefABCDEF0123456789-_")
	n := 1 + rng.Intn(8)
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteRune(letters[rng.Intn(len(letters))])
	}
	name := sb.String()

	switch rng.Intn(4) {
	case 0:
		name += testDotExe
	case 1:
		name += ".EXE"
	case 2:
		name += ".Exe"
	default:
		// no suffix
	}
	if rng.Intn(2) == 0 {
		name = "  " + name + "  " // surrounding spaces to exercise TrimSpace
	}
	return reflect.ValueOf(binBaseName(name))
}

// TestNormalizeBinaryNameForMatch_properties asserts:
//   - idempotency: f(f(x)) == f(x) for all inputs;
//   - non-Windows hosts only trim whitespace (no case folding, no .exe strip);
//   - on Windows the result is lowercase with any ".exe" suffix removed.
//
// normalizeBinaryNameForMatch branches on runtime.GOOS, so the OS-specific
// assertions are gated on the host the test runs on while idempotency is
// universal.
func TestNormalizeBinaryNameForMatch_properties(t *testing.T) {
	idempotent := func(a binBaseName) bool {
		once := normalizeBinaryNameForMatch(string(a))
		twice := normalizeBinaryNameForMatch(once)
		return once == twice
	}
	if err := quick.Check(idempotent, quickConfig()); err != nil {
		t.Errorf("idempotency failed: %v", err)
	}

	// Case/.exe handling is exercised through the public function with explicit
	// inputs so the invariant is asserted regardless of the host OS via the
	// idempotency above; here we additionally check the host-specific contract.
	hostContract := func(a binBaseName) bool {
		in := string(a)
		got := normalizeBinaryNameForMatch(in)
		trimmed := strings.TrimSpace(in)

		if isWindowsHost() {
			lower := strings.ToLower(trimmed)
			want := strings.TrimSuffix(lower, testDotExe)
			return got == want
		}
		// Non-Windows: only whitespace is trimmed; case and .exe are preserved.
		return got == trimmed
	}
	if err := quick.Check(hostContract, quickConfig()); err != nil {
		t.Errorf("host-specific contract failed: %v", err)
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
		for i := 0; i < n; i++ {
			sb.WriteRune(letters[rng.Intn(len(letters))])
		}
		return sb.String()
	}
	parts := []string{"example.com", seg()}
	extra := rng.Intn(4)
	for i := 0; i < extra; i++ {
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

// isWindowsHost reports whether normalizeBinaryNameForMatch will take its
// Windows branch on the current host. It mirrors the runtime.GOOS check in the
// production code.
func isWindowsHost() bool {
	return runtime.GOOS == goosWindows
}
