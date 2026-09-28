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
	for i := range n {
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

// FuzzParseConf feeds arbitrary bytes to the gup.json decoder. gup.json is
// hand-editable and travels between machines via export/import, so the decoder
// must never panic and must uphold its contract for whatever it accepts:
//   - every entry has a trimmed, non-empty name, import path and version;
//   - the channel is one of latest/main/master/pinned, a pinned entry carries a
//     valid pin equal to its version, and no other entry carries a pin;
//   - writing the result back and reading it again is lossless (modulo the
//     documented version normalization), and a second write is byte-identical.
func FuzzParseConf(f *testing.F) {
	for _, seed := range []string{
		"",
		"   \n",
		"{}",
		"null",
		`{"schema_version":1,"packages":[]}`,
		`{"schema_version":1,"packages":[{"name":"gup","import_path":"github.com/nao1215/gup","version":"v0.7.0","channel":"latest"}]}`,
		`{"schema_version":1,"packages":[{"name":" gup ","import_path":" github.com/nao1215/gup ","version":"(devel)","channel":""}]}`,
		`{"schema_version":1,"packages":[{"name":"a","import_path":"example.com/a","version":"unknown","channel":"MAIN"}]}`,
		`{"schema_version":1,"packages":[{"name":"a","import_path":"example.com/a","version":"v1.0.0","channel":"pinned"}]}`,
		`{"schema_version":2,"packages":[{"name":"a","import_path":"example.com/a","version":"v1.2.3","channel":"pinned"},{"name":"b","import_path":"example.com/b","version":"v0.1.0","channel":"master"}]}`,
		`{"schema_version":2,"packages":[{"name":"a","import_path":"example.com/a","version":"v0.0.0-20240102150405-abcdef123456","channel":"pinned"}]}`,
		`{"schema_version":2,"packages":[{"name":"a","import_path":"example.com/a","version":"latest","channel":"pinned"}]}`,
		`{"schema_version":1,"packages":[{"name":"a","import_path":"example.com/a","version":"v1","channel":"nightly"}]}`,
		`{"schema_version":99,"packages":[]}`,
		`{"schema_version":1,"packages":[`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		pkgs, err := parseConf(raw, ConfigFileName)
		if err != nil {
			return
		}
		checkParsedConf(t, pkgs)

		var first bytes.Buffer
		if err := WriteConfFile(&first, pkgs); err != nil {
			t.Fatalf("WriteConfFile rejected packages parseConf accepted: %v\ninput: %q", err, raw)
		}
		again, err := parseConf(first.Bytes(), ConfigFileName)
		if err != nil {
			t.Fatalf("parseConf rejected WriteConfFile output: %v\ninput: %q\nwritten: %s", err, raw, first.String())
		}
		checkParsedConf(t, again)
		if len(again) != len(pkgs) {
			t.Fatalf("round trip changed package count: %d -> %d\ninput: %q", len(pkgs), len(again), raw)
		}
		for i := range pkgs {
			want := pkgs[i]
			if want.UpdateChannel != goutil.UpdateChannelPinned {
				want.Version = &goutil.Version{Current: normalizeConfVersion(want.Version.Current)}
			}
			got := again[i]
			if got.Name != want.Name || got.ImportPath != want.ImportPath ||
				got.UpdateChannel != want.UpdateChannel || got.PinnedVersion != want.PinnedVersion ||
				got.Version.Current != want.Version.Current {
				t.Fatalf("round trip changed entry %d: %+v -> %+v\ninput: %q", i, want, got, raw)
			}
		}

		var second bytes.Buffer
		if err := WriteConfFile(&second, again); err != nil {
			t.Fatalf("WriteConfFile failed on re-read packages: %v", err)
		}
		if !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatalf("WriteConfFile is not idempotent:\nfirst:  %s\nsecond: %s", first.String(), second.String())
		}
	})
}

// checkParsedConf asserts the per-entry contract of a successful parseConf.
func checkParsedConf(t *testing.T, pkgs []goutil.Package) {
	t.Helper()
	for i, p := range pkgs {
		for field, v := range map[string]string{"name": p.Name, "import_path": p.ImportPath, "version": p.Version.Current} {
			if v == "" || v != strings.TrimSpace(v) {
				t.Fatalf("entry %d: %s %q is empty or untrimmed", i, field, v)
			}
		}
		switch p.UpdateChannel {
		case goutil.UpdateChannelPinned:
			if err := goutil.ValidatePinnedVersion(p.PinnedVersion); err != nil {
				t.Fatalf("entry %d: accepted invalid pin %q: %v", i, p.PinnedVersion, err)
			}
			if p.PinnedVersion != p.Version.Current {
				t.Fatalf("entry %d: pin %q differs from version %q", i, p.PinnedVersion, p.Version.Current)
			}
		case goutil.UpdateChannelLatest, goutil.UpdateChannelMain, goutil.UpdateChannelMaster:
			if p.PinnedVersion != "" {
				t.Fatalf("entry %d: channel %q carries pin %q", i, p.UpdateChannel, p.PinnedVersion)
			}
		default:
			t.Fatalf("entry %d: unexpected channel %q", i, p.UpdateChannel)
		}
	}
}
