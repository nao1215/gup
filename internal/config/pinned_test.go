package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
)

const (
	pinTestVersion = "v1.62.0"
	pinTestImport  = "example.com/a"
	pinTestV100    = "v1.0.0"
)

// writeTempConf writes content to a temp gup.json and returns its path.
func writeTempConf(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gup.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestReadConfFile_schemaV1_latestMainMaster(t *testing.T) {
	t.Parallel()
	path := writeTempConf(t, `{"schema_version":1,"packages":[
		{"name":"a","import_path":"example.com/a","version":"v1.0.0","channel":"latest"},
		{"name":"b","import_path":"example.com/b","version":"v1.0.0","channel":"main"},
		{"name":"c","import_path":"example.com/c","version":"v1.0.0","channel":"master"}
	]}`)
	pkgs, err := ReadConfFile(path)
	if err != nil {
		t.Fatalf("ReadConfFile() error: %v", err)
	}
	want := map[string]goutil.UpdateChannel{
		"a": goutil.UpdateChannelLatest,
		"b": goutil.UpdateChannelMain,
		"c": goutil.UpdateChannelMaster,
	}
	for _, p := range pkgs {
		if p.UpdateChannel != want[p.Name] {
			t.Errorf("%s channel = %q, want %q", p.Name, p.UpdateChannel, want[p.Name])
		}
		if p.PinnedVersion != "" {
			t.Errorf("%s PinnedVersion = %q, want empty", p.Name, p.PinnedVersion)
		}
	}
}

func TestReadConfFile_missingChannelDefaultsLatest(t *testing.T) {
	t.Parallel()
	path := writeTempConf(t, `{"schema_version":1,"packages":[
		{"name":"a","import_path":"example.com/a","version":"v1.0.0"}
	]}`)
	pkgs, err := ReadConfFile(path)
	if err != nil {
		t.Fatalf("ReadConfFile() error: %v", err)
	}
	if pkgs[0].UpdateChannel != goutil.UpdateChannelLatest {
		t.Errorf("missing channel = %q, want latest", pkgs[0].UpdateChannel)
	}
}

func TestReadConfFile_schemaV2_pinnedLoads(t *testing.T) {
	t.Parallel()
	path := writeTempConf(t, `{"schema_version":2,"packages":[
		{"name":"golangci-lint","import_path":"example.com/golangci-lint","version":"v1.62.0","channel":"pinned"}
	]}`)
	pkgs, err := ReadConfFile(path)
	if err != nil {
		t.Fatalf("ReadConfFile() error: %v", err)
	}
	p := pkgs[0]
	if p.UpdateChannel != goutil.UpdateChannelPinned {
		t.Errorf("channel = %q, want pinned", p.UpdateChannel)
	}
	if p.PinnedVersion != pinTestVersion {
		t.Errorf("PinnedVersion = %q, want %s", p.PinnedVersion, pinTestVersion)
	}
	if p.Version == nil || p.Version.Current != pinTestVersion {
		t.Errorf("Version.Current = %v, want %s (so import installs the exact version)", p.Version, pinTestVersion)
	}
}

func TestReadConfFile_pinnedRejectsBadVersions(t *testing.T) {
	t.Parallel()
	for _, ver := range []string{"latest", "main", "master", "pinned", "(devel)", "unknown"} {
		t.Run(ver, func(t *testing.T) {
			t.Parallel()
			path := writeTempConf(t, `{"schema_version":2,"packages":[
				{"name":"a","import_path":"example.com/a","version":"`+ver+`","channel":"pinned"}
			]}`)
			if _, err := ReadConfFile(path); err == nil {
				t.Fatalf("ReadConfFile() with pinned version %q expected error, got nil", ver)
			}
		})
	}
}

func TestReadConfFile_pinnedInSchemaV1IsRejected(t *testing.T) {
	t.Parallel()
	// channel "pinned" must not appear under schema_version 1: an older gup would
	// normalize the unknown channel to @latest and silently unpin it. Reading such
	// a file must fail fast, never degrade to latest.
	path := writeTempConf(t, `{"schema_version":1,"packages":[
		{"name":"a","import_path":"example.com/a","version":"v1.0.0","channel":"pinned"}
	]}`)
	_, err := ReadConfFile(path)
	if err == nil {
		t.Fatal("ReadConfFile() with pinned in schema_version 1 expected error, got nil")
	}
	if !strings.Contains(err.Error(), "pinned") {
		t.Errorf("error = %v, want it to mention pinned", err)
	}
}

func TestReadConfFile_unknownChannelIsErrorNotLatest(t *testing.T) {
	t.Parallel()
	path := writeTempConf(t, `{"schema_version":1,"packages":[
		{"name":"a","import_path":"example.com/a","version":"v1.0.0","channel":"stable"}
	]}`)
	_, err := ReadConfFile(path)
	if err == nil {
		t.Fatal("ReadConfFile() with unknown channel expected error (not silent latest), got nil")
	}
}

func TestReadConfFile_unsupportedSchemaFails(t *testing.T) {
	t.Parallel()
	path := writeTempConf(t, `{"schema_version":99,"packages":[]}`)
	if _, err := ReadConfFile(path); err == nil {
		t.Fatal("ReadConfFile() with schema_version 99 expected error, got nil")
	}
}

func TestReadConfFile_malformedJSONFails(t *testing.T) {
	t.Parallel()
	path := writeTempConf(t, `{"schema_version":1,"packages":[`)
	if _, err := ReadConfFile(path); err == nil {
		t.Fatal("ReadConfFile() with malformed JSON expected error, got nil")
	}
}

func TestWriteConfFile_pinnedUsesSchemaV2(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pkgs := []goutil.Package{
		{Name: "a", ImportPath: pinTestImport, Version: &goutil.Version{Current: pinTestV100}, UpdateChannel: goutil.UpdateChannelLatest},
		{Name: "b", ImportPath: "example.com/b", Version: &goutil.Version{Current: "v9.9.9"}, UpdateChannel: goutil.UpdateChannelPinned, PinnedVersion: pinTestVersion},
	}
	if err := WriteConfFile(&buf, pkgs); err != nil {
		t.Fatalf("WriteConfFile() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"schema_version": 2`) {
		t.Errorf("output should use schema_version 2 when a package is pinned:\n%s", out)
	}
	if !strings.Contains(out, `"channel": "pinned"`) {
		t.Errorf("output should record channel pinned:\n%s", out)
	}
	// The pinned target version must be written, not the installed current version.
	if !strings.Contains(out, `"version": "`+pinTestVersion+`"`) {
		t.Errorf("output should persist the pinned target %s, not the installed v9.9.9:\n%s", pinTestVersion, out)
	}
	if strings.Contains(out, "v9.9.9") {
		t.Errorf("output must not leak the installed version for a pinned package:\n%s", out)
	}
}

func TestWriteConfFile_noPinKeepsSchemaV1(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pkgs := []goutil.Package{
		{Name: "a", ImportPath: pinTestImport, Version: &goutil.Version{Current: pinTestV100}, UpdateChannel: goutil.UpdateChannelLatest},
		{Name: "b", ImportPath: "example.com/b", Version: &goutil.Version{Current: pinTestV100}, UpdateChannel: goutil.UpdateChannelMain},
	}
	if err := WriteConfFile(&buf, pkgs); err != nil {
		t.Fatalf("WriteConfFile() error: %v", err)
	}
	if !strings.Contains(buf.String(), `"schema_version": 1`) {
		t.Errorf("output should stay schema_version 1 when nothing is pinned:\n%s", buf.String())
	}
}

func TestWriteConfFile_pinnedWithBadVersionFails(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pkgs := []goutil.Package{
		{Name: "a", ImportPath: pinTestImport, UpdateChannel: goutil.UpdateChannelPinned, PinnedVersion: ""},
	}
	if err := WriteConfFile(&buf, pkgs); err == nil {
		t.Fatal("WriteConfFile() with empty pinned version expected error (never write an unsafe pin), got nil")
	}
}

// TestPinnedRoundTrip proves a pinned package survives a write -> read cycle with
// its channel and concrete version intact.
func TestPinnedRoundTrip(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	in := []goutil.Package{
		{Name: "a", ImportPath: pinTestImport, Version: &goutil.Version{Current: "v2.0.0"}, UpdateChannel: goutil.UpdateChannelPinned, PinnedVersion: pinTestVersion},
	}
	if err := WriteConfFile(&buf, in); err != nil {
		t.Fatalf("WriteConfFile() error: %v", err)
	}
	path := writeTempConf(t, buf.String())
	out, err := ReadConfFile(path)
	if err != nil {
		t.Fatalf("ReadConfFile() error: %v", err)
	}
	if out[0].UpdateChannel != goutil.UpdateChannelPinned || out[0].PinnedVersion != pinTestVersion {
		t.Errorf("round trip lost pin: channel=%q pinned=%q", out[0].UpdateChannel, out[0].PinnedVersion)
	}
}
