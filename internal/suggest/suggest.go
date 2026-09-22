// Package suggest finds the installed binary a mistyped name most likely meant.
//
// gup takes binary names from the command line (update/check targets, and
// update's --exclude), and a typo there is otherwise silent or reported only as
// "not found". The suggestion is what turns that into a next step: "did you mean
// 'lazygit'?". It is deliberately conservative, because a wrong suggestion is
// worse than none: only a close match is offered, and a short name has to be
// closer still.
package suggest

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Closest returns the candidate closest to name by edit distance, and whether
// one is close enough to suggest. Comparison ignores case, so "LazyGit" still
// finds "lazygit", which on a case-sensitive file system is exactly the name the
// user missed. The name itself, spelled identically, is never suggested: echoing
// it back helps nobody. Ties are broken alphabetically so the result is
// deterministic.
func Closest(name string, candidates []string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	target := strings.ToLower(trimmed)
	if target == "" {
		return "", false
	}
	limit := maxDistance(target)

	sorted := append([]string(nil), candidates...)
	sort.Strings(sorted)

	best, bestDist := "", limit+1
	for _, c := range sorted {
		cand := strings.ToLower(c)
		if c == trimmed || cand == "" {
			continue
		}
		if d := Distance(target, cand); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, best != ""
}

const (
	// shortNameRunes is the longest name held to shortNameEdits.
	shortNameRunes = 4
	// shortNameEdits and longNameEdits are how far a suggestion may be from a
	// short and a longer name.
	shortNameEdits = 1
	longNameEdits  = 2
)

// maxDistance is how many edits a suggestion may be away from name. One edit on
// a name of four runes or fewer, two above that: "gh" is one edit from "go",
// "gp", "gs", ..., and suggesting any of them for a short name is a guess.
func maxDistance(name string) int {
	if utf8.RuneCountInString(name) <= shortNameRunes {
		return shortNameEdits
	}
	return longNameEdits
}

// Distance is the optimal string alignment distance between a and b: the
// Levenshtein distance (insertions, deletions, substitutions) with one more
// operation, swapping two adjacent runes. The swap is the typo people actually
// make ("lazygti"), and plain Levenshtein charges it two edits.
func Distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	// d[i][j] is the distance between ra[:i] and rb[:j].
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}
