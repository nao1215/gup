//nolint:paralleltest,goconst // keep consistent with the rest of this package's tests; version literals are clearer inline
package goutil

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/hashicorp/go-version"
)

// quickConfig is the shared testing/quick configuration. MaxCount is kept
// bounded so the property-based tests stay fast and non-flaky.
func quickConfig() *quick.Config {
	return &quick.Config{MaxCount: 500}
}

// semverString is a bounded, parseable version string generator. Values it
// produces are always accepted by version.NewVersion, which lets the properties
// focus on comparison behavior rather than parse failures.
type semverString string

// Generate implements quick.Generator for semverString.
func (semverString) Generate(rng *rand.Rand, _ int) reflect.Value {
	major := rng.Intn(20)
	minor := rng.Intn(40)
	patch := rng.Intn(40)
	s := fmt.Sprintf("%d.%d.%d", major, minor, patch)

	switch rng.Intn(6) {
	case 0:
		s = "v" + s // go-style "v" prefix
	case 1:
		s += fmt.Sprintf("-rc.%d", rng.Intn(5)) // prerelease
	case 2:
		s += fmt.Sprintf("+build.%d", rng.Intn(5)) // build metadata
	case 3:
		s = fmt.Sprintf("%d.%d", major, minor) // shorter form
	case 4:
		s = fmt.Sprintf("%d.%d.%d-0.20220908165354-f7355b5d2afa", major, minor, patch) // pseudo-version-ish
	default:
		// keep plain "x.y.z"
	}
	return reflect.ValueOf(semverString(s))
}

// TestVersionUpToDate_properties asserts the order-theoretic invariants of
// versionUpToDate for parseable inputs:
//   - reflexivity: upToDate(a, a) == true
//   - antisymmetry/totality: for parseable a, b at least one direction holds,
//     and when both hold the versions compare equal.
func TestVersionUpToDate_properties(t *testing.T) {
	reflexive := func(a semverString) bool {
		return versionUpToDate(string(a), string(a))
	}
	if err := quick.Check(reflexive, quickConfig()); err != nil {
		t.Errorf("reflexivity failed: %v", err)
	}

	// Totality + antisymmetry: with two parseable versions, the comparison is a
	// total order, so upToDate(a,b) || upToDate(b,a) is always true, and both
	// can be true only when the versions are equal.
	total := func(a, b semverString) bool {
		ab := versionUpToDate(string(a), string(b))
		ba := versionUpToDate(string(b), string(a))
		if !ab && !ba {
			return false // a total order must order every parseable pair
		}
		if ab && ba {
			// Both >= each other means they must be equal under the parser.
			va, errA := version.NewVersion(string(a))
			vb, errB := version.NewVersion(string(b))
			if errA != nil || errB != nil {
				return false
			}
			return va.Equal(vb)
		}
		return true
	}
	if err := quick.Check(total, quickConfig()); err != nil {
		t.Errorf("totality/antisymmetry failed: %v", err)
	}

	// Boundary pairs exercise carry/format boundaries the property generators may
	// not hit often. Each must respect the same invariants.
	boundaryPairs := []struct{ a, b string }{
		{"v1.9.0", "v1.10.0"}, // numeric carry: 9 < 10
		{"1.9.0", "1.10.0"},
		{"v1.0", "v1.0.0"}, // short form vs full form (equal)
		{"1.0", "1.0.0"},
		{"v1.2.3-rc.1", "v1.2.3"},    // prerelease < release
		{"v1.2.3+build.1", "v1.2.3"}, // build metadata ignored for ordering
		{"v1.9.1-0.20220908165354-f7355b5d2afa", "v1.9.0"},
	}
	for _, p := range boundaryPairs {
		if !versionUpToDate(p.a, p.a) {
			t.Errorf("reflexivity boundary failed for %q", p.a)
		}
		ab := versionUpToDate(p.a, p.b)
		ba := versionUpToDate(p.b, p.a)
		if !ab && !ba {
			t.Errorf("totality boundary failed for %q vs %q", p.a, p.b)
		}
	}
}

// TestGoVersionUpToDate_properties mirrors the version invariants for the Go
// toolchain comparator, including custom-tag normalization.
func TestGoVersionUpToDate_properties(t *testing.T) {
	reflexive := func(a semverString) bool {
		return goVersionUpToDate(string(a), string(a))
	}
	if err := quick.Check(reflexive, quickConfig()); err != nil {
		t.Errorf("reflexivity failed: %v", err)
	}

	// Equal strings (even with custom separators) are always up to date.
	for _, s := range []string{"1.25.0-X:nodwarf5", "1.26.0-X~tag", "go-ish"} {
		if !goVersionUpToDate(s, s) {
			t.Errorf("goVersionUpToDate(%q,%q) should be reflexive true", s, s)
		}
	}

	total := func(a, b semverString) bool {
		ab := goVersionUpToDate(string(a), string(b))
		ba := goVersionUpToDate(string(b), string(a))
		// For parseable, distinct semver-ish strings the comparator is total.
		return ab || ba
	}
	if err := quick.Check(total, quickConfig()); err != nil {
		t.Errorf("totality failed: %v", err)
	}
}

// TestNormalizeGoVersionForCompare_properties asserts the key contract of the
// normalizer: its output is always parseable by version.NewVersion when the
// input contained at least one semver-significant character, and normalization
// is idempotent (normalize(normalize(x)) == normalize(x)).
func TestNormalizeGoVersionForCompare_properties(t *testing.T) {
	idempotent := func(a goVersionInput) bool {
		once := normalizeGoVersionForCompare(string(a))
		twice := normalizeGoVersionForCompare(once)
		return once == twice
	}
	if err := quick.Check(idempotent, quickConfig()); err != nil {
		t.Errorf("idempotency failed: %v", err)
	}

	// Any normalized output of a real-looking Go version string must parse.
	parseable := func(a semverString) bool {
		out := normalizeGoVersionForCompare(string(a))
		_, err := version.NewVersion(out)
		return err == nil
	}
	if err := quick.Check(parseable, quickConfig()); err != nil {
		t.Errorf("normalized output should be parseable: %v", err)
	}

	// Custom-tag Go versions must normalize to a parseable form.
	for _, s := range []string{"1.25.0-X:nodwarf5", "1.26.0-X~nodwarf5", " 1.26.0-X:nodwarf5 "} {
		out := normalizeGoVersionForCompare(s)
		if _, err := version.NewVersion(out); err != nil {
			t.Errorf("normalizeGoVersionForCompare(%q) = %q is not parseable: %v", s, out, err)
		}
	}
}

// goVersionInput generates arbitrary Go version-ish strings (including custom
// separators like ':' and '~') to exercise normalization idempotency on a wider
// input space than strictly-parseable semver.
type goVersionInput string

// Generate implements quick.Generator for goVersionInput.
func (goVersionInput) Generate(rng *rand.Rand, _ int) reflect.Value {
	base := fmt.Sprintf("%d.%d.%d", rng.Intn(30), rng.Intn(30), rng.Intn(30))
	tags := []string{"", "-X:nodwarf5", "-X~tag", "+meta", "-rc.1", " spaced "}
	return reflect.ValueOf(goVersionInput(base + tags[rng.Intn(len(tags))]))
}
