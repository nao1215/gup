package config

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/nao1215/gup/internal/goutil"
)

// validPackageSet generates a slice of packages that ReadConfFile accepts:
// every entry has a non-empty, non-whitespace Name, ImportPath and Version.
// It is the input domain for the write -> read round-trip property.
type validPackageSet []goutil.Package

// Generate implements quick.Generator for validPackageSet.
func (validPackageSet) Generate(rng *rand.Rand, _ int) reflect.Value {
	n := rng.Intn(6) // 0..5 packages, including the empty set
	channels := []goutil.UpdateChannel{
		goutil.UpdateChannelLatest,
		goutil.UpdateChannelMain,
		goutil.UpdateChannelMaster,
		"", // exercises normalization to "latest"
		"SNAPSHOT",
	}
	versions := []string{verSemver, "v0.0.1", verLatest, "v2.0.0-rc.1"}

	pkgs := make(validPackageSet, 0, n)
	for i := 0; i < n; i++ {
		pkgs = append(pkgs, goutil.Package{
			Name:          fmt.Sprintf("tool%d", i),
			ImportPath:    fmt.Sprintf("example.com/owner/tool%d", i),
			Version:       &goutil.Version{Current: versions[rng.Intn(len(versions))]},
			UpdateChannel: channels[rng.Intn(len(channels))],
		})
	}
	return reflect.ValueOf(pkgs)
}

// normalizedView is the persisted, comparable projection of a package: the
// fields that survive a WriteConfFile -> ReadConfFile round-trip, each already
// run through the same normalization WriteConfFile applies.
type normalizedView struct {
	Name       string
	ImportPath string
	Version    string
	Channel    goutil.UpdateChannel
}

func viewOf(p goutil.Package) normalizedView {
	version := verLatest
	if p.Version != nil {
		version = normalizeConfVersion(p.Version.Current)
	}
	return normalizedView{
		Name:       p.Name,
		ImportPath: p.ImportPath,
		Version:    version,
		Channel:    goutil.NormalizeUpdateChannel(string(p.UpdateChannel)),
	}
}

func viewsOf(pkgs []goutil.Package) []normalizedView {
	out := make([]normalizedView, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, viewOf(p))
	}
	return out
}

// TestWriteThenReadConfFile_roundTrip is the property: for any valid package
// set, writing it and reading it back yields an equivalent set (modulo the
// documented normalization of version and channel performed on write).
func TestWriteThenReadConfFile_roundTrip(t *testing.T) { //nolint:paralleltest // uses a temp file per case
	roundTrip := func(in validPackageSet) bool {
		var buf bytes.Buffer
		if err := WriteConfFile(&buf, in); err != nil {
			t.Logf("WriteConfFile() error = %v", err)
			return false
		}

		// ReadConfFile reads from a path, so persist to a temp file.
		path := filepath.Join(t.TempDir(), "gup.json")
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Logf("failed to write temp conf file: %v", err)
			return false
		}

		got, err := ReadConfFile(path)
		if err != nil {
			t.Logf("ReadConfFile() error = %v", err)
			return false
		}

		want := viewsOf([]goutil.Package(in))
		gotViews := viewsOf(got)
		if !reflect.DeepEqual(want, gotViews) {
			t.Logf("round-trip mismatch:\n want=%+v\n got =%+v", want, gotViews)
			return false
		}
		return true
	}

	if err := quick.Check(roundTrip, &quick.Config{MaxCount: 300}); err != nil {
		t.Errorf("write->read round-trip property failed: %v", err)
	}
}

// TestWriteThenReadConfFile_roundTrip_emptySet covers the explicit empty-set
// boundary, where ReadConfFile returns an empty (non-nil) slice.
func TestWriteThenReadConfFile_roundTrip_emptySet(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := WriteConfFile(&buf, nil); err != nil {
		t.Fatalf("WriteConfFile() error = %v", err)
	}
	// The persisted form must declare zero packages.
	if !strings.Contains(buf.String(), `"packages": []`) {
		t.Fatalf("empty set should persist an empty packages array, got: %s", buf.String())
	}

	path := filepath.Join(t.TempDir(), "gup.json")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("failed to write temp conf file: %v", err)
	}
	got, err := ReadConfFile(path)
	if err != nil {
		t.Fatalf("ReadConfFile() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadConfFile() len = %d, want 0", len(got))
	}
}
