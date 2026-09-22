package suggest

import "testing"

const (
	lazygit = "lazygit"
	abc     = "abc"
	toolA   = "toola"
	toolB   = "toolb"
)

func TestDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{abc, "", 3},
		{"", abc, 3},
		{lazygit, lazygit, 0},
		{lazygit, "lazygti", 1}, // adjacent swap is one edit
		{lazygit, "lazygi", 1},  // deletion
		{lazygit, "lazyygit", 1},
		{lazygit, "lazyxit", 1}, // substitution
		{"kitten", "sitting", 3},
		{"ca", abc, 3}, // optimal string alignment, not unrestricted Damerau
		{"日本語", "日本", 1},
	}
	for _, tt := range tests {
		if got := Distance(tt.a, tt.b); got != tt.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := Distance(tt.b, tt.a); got != tt.want {
			t.Errorf("Distance(%q, %q) = %d, want %d (symmetry)", tt.b, tt.a, got, tt.want)
		}
	}
}

func TestClosest(t *testing.T) {
	t.Parallel()

	installed := []string{"gopls", "golangci-lint", lazygit, "gh", "air", "sqly"}
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{"swapped runes", "lazygti", lazygit, true},
		{"missing rune", "golangci-lnt", "golangci-lint", true},
		{"case differs only", "LazyGit", lazygit, true},
		{"exact match is not a suggestion", lazygit, "", false},
		{"three edits on a long name", "lazyxyz", "", false},
		{"two edits on a long name within limit", "lazyit", lazygit, true},
		{"short name allows one edit", "sqyl", "sqly", true},
		{"short name refuses two edits", "sq", "", false},
		{"nothing close", "kind", "", false},
		{"blank input", "   ", "", false},
		{"trimmed input", "  gopl  ", "gopls", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Closest(tt.input, installed)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("Closest(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestClosest_tieBreaksAlphabetically(t *testing.T) {
	t.Parallel()

	// "tool" is one edit from both; the alphabetically first wins, whatever order
	// the candidates arrive in.
	for _, candidates := range [][]string{{toolB, toolA}, {toolA, toolB}} {
		if got, _ := Closest("tool", candidates); got != toolA {
			t.Errorf("Closest(tool, %v) = %q, want toola", candidates, got)
		}
	}
}

func TestClosest_noCandidates(t *testing.T) {
	t.Parallel()

	if got, ok := Closest(lazygit, nil); ok || got != "" {
		t.Errorf("Closest with no candidates = (%q, %v), want (\"\", false)", got, ok)
	}
}
