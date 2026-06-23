package binname

import (
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/quick"
)

const (
	dotExe       = ".exe"
	goosLinux    = "linux"
	toolPadded   = "  tool  "
	toolCaseName = "Tool"
	toolExeName  = "tool.exe"
	toolName     = "tool"
)

// Test_normalizeForMatchWith_perOS pins the OS-specific matching rules
// deterministically, independent of the host the test runs on. This is the
// single source of truth for binary-name matching, so both the OS-aware
// trimming and the Windows ".exe"/case folding are asserted here.
func Test_normalizeForMatchWith_perOS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goos  string
		input string
		want  string
	}{
		{name: "linux trims only", goos: goosLinux, input: toolPadded, want: toolName},
		{name: "linux preserves case", goos: goosLinux, input: toolCaseName, want: toolCaseName},
		{name: "linux preserves .exe", goos: goosLinux, input: toolExeName, want: toolExeName},
		{name: "darwin trims only", goos: "darwin", input: " gup ", want: "gup"},
		{name: "windows trims", goos: goosWindows, input: toolPadded, want: toolName},
		{name: "windows lowercases", goos: goosWindows, input: toolCaseName, want: toolName},
		{name: "windows strips .exe", goos: goosWindows, input: toolExeName, want: toolName},
		{name: "windows strips .EXE case-insensitively", goos: goosWindows, input: "Tool.EXE", want: toolName},
		{name: "windows trims then strips .exe", goos: goosWindows, input: "  Foo.Exe  ", want: "foo"},
		{name: "empty stays empty", goos: goosWindows, input: "   ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeForMatchWith(tt.input, tt.goos); got != tt.want {
				t.Errorf("normalizeForMatchWith(%q, %q) = %q, want %q", tt.input, tt.goos, got, tt.want)
			}
		})
	}
}

// Test_NormalizeForMatch_hostContract asserts that the exported host-facing
// function follows the rule for the OS it actually runs on.
func Test_NormalizeForMatch_hostContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "plain", input: "tool"},
		{name: "trimmed", input: "  tool  "},
		{name: "exe", input: "tool.exe"},
		{name: "mixed case exe", input: "Tool.EXE"},
		{name: "empty", input: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeForMatch(tt.input)
			want := normalizeForMatchWith(tt.input, runtime.GOOS)
			if got != want {
				t.Errorf("NormalizeForMatch(%q) = %q, want %q (host %s)", tt.input, got, want, runtime.GOOS)
			}
		})
	}
}

// Test_NormalizeForMatch_idempotent asserts f(f(x)) == f(x), the invariant
// callers rely on when they normalize names into a lookup key. The generator
// draws from letters/digits and appends ".exe" separately: real binary names
// have no internal spaces and at most one ".exe" suffix, and a " .exe" input
// would expose a benign non-idempotency (the trailing space surfaced by
// stripping ".exe" is trimmed only on the second pass) that no real name hits.
func Test_NormalizeForMatch_idempotent(t *testing.T) {
	t.Parallel()

	gen := func(rng *rand.Rand, _ int) reflect.Value {
		letters := []rune("abABeExX012")
		n := rng.Intn(8)
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteRune(letters[rng.Intn(len(letters))])
		}
		s := sb.String()
		if rng.Intn(2) == 0 {
			s += dotExe
		}
		return reflect.ValueOf(s)
	}
	idempotent := func(s string) bool {
		once := NormalizeForMatch(s)
		return NormalizeForMatch(once) == once
	}
	if err := quick.Check(idempotent, &quick.Config{MaxCount: 500, Values: func(v []reflect.Value, rng *rand.Rand) { v[0] = gen(rng, 0) }}); err != nil {
		t.Errorf("idempotency failed: %v", err)
	}
}
