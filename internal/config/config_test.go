package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/goutil"
)

func withTempXDG(t *testing.T) func() {
	t.Helper()

	origConfig := xdg.ConfigHome
	origData := xdg.DataHome
	origCache := xdg.CacheHome

	base := t.TempDir()
	xdg.ConfigHome = filepath.Join(base, "config")
	xdg.DataHome = filepath.Join(base, "data")
	xdg.CacheHome = filepath.Join(base, "cache")

	for _, dir := range []string{xdg.ConfigHome, xdg.DataHome, xdg.CacheHome} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create XDG dir %s: %v", dir, err)
		}
	}

	return func() {
		xdg.ConfigHome = origConfig
		xdg.DataHome = origData
		xdg.CacheHome = origCache
	}
}

func TestDirAndFilePath(t *testing.T) { //nolint:paralleltest // modifies xdg globals
	cleanup := withTempXDG(t)
	defer cleanup()

	if got := DirPath(); got != filepath.Join(xdg.ConfigHome, "gup") {
		t.Fatalf("DirPath() = %s, want %s", got, filepath.Join(xdg.ConfigHome, "gup"))
	}

	if got := FilePath(); got != filepath.Join(xdg.ConfigHome, "gup", ConfigFileName) {
		t.Fatalf("FilePath() = %s, want %s", got, filepath.Join(xdg.ConfigHome, "gup", ConfigFileName))
	}
}

func TestWriteConfFile(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	pkgs := []goutil.Package{
		{
			Name:          "foo",
			ImportPath:    "example.com/foo",
			Version:       &goutil.Version{Current: "v1.2.3"},
			UpdateChannel: goutil.UpdateChannelMain,
		},
		{
			Name:       "bar",
			ImportPath: "example.com/bar",
		},
		{
			Name:       "baz",
			ImportPath: "example.com/baz",
			Version:    &goutil.Version{Current: "(devel)"},
		},
	}

	if err := WriteConfFile(&buf, pkgs); err != nil {
		t.Fatalf("WriteConfFile() error = %v", err)
	}

	want := `{
  "schema_version": 1,
  "packages": [
    {
      "name": "foo",
      "import_path": "example.com/foo",
      "version": "v1.2.3",
      "channel": "main"
    },
    {
      "name": "bar",
      "import_path": "example.com/bar",
      "version": "latest",
      "channel": "latest"
    },
    {
      "name": "baz",
      "import_path": "example.com/baz",
      "version": "latest",
      "channel": "latest"
    }
  ]
}
`
	if got := buf.String(); got != want {
		t.Fatalf("WriteConfFile() output = %q, want %q", got, want)
	}
}

const (
	verLatest  = "latest"
	verUnknown = "unknown"
	verDevel   = "(devel)"
	verSemver  = "v1.2.3"
)

func Test_normalizeConfVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{name: "empty becomes latest", in: "", want: verLatest},
		{name: "whitespace becomes latest", in: "   ", want: verLatest},
		{name: "(devel) becomes latest", in: verDevel, want: verLatest},
		{name: "devel becomes latest", in: "devel", want: verLatest},
		{name: "unknown becomes latest", in: verUnknown, want: verLatest},
		{name: "unknown with whitespace becomes latest", in: "  unknown  ", want: verLatest},
		{name: "latest stays latest", in: verLatest, want: verLatest},
		{name: "real version is kept", in: verSemver, want: verSemver},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeConfVersion(tt.in); got != tt.want {
				t.Errorf("normalizeConfVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Test_WriteConfFile_normalizesUnknownVersion verifies that "unknown" never
// reaches gup.json: it is normalized to "latest" like "(devel)" (issue #300).
func Test_WriteConfFile_normalizesUnknownVersion(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	pkgs := []goutil.Package{
		{
			Name:       "tool",
			ImportPath: "example.com/tool",
			Version:    &goutil.Version{Current: verUnknown},
		},
	}
	if err := WriteConfFile(&buf, pkgs); err != nil {
		t.Fatalf("WriteConfFile() error = %v", err)
	}
	if strings.Contains(buf.String(), verUnknown) {
		t.Errorf("gup.json must not persist %q, got:\n%s", verUnknown, buf.String())
	}
	if !strings.Contains(buf.String(), `"version": "latest"`) {
		t.Errorf("%q should be normalized to %q, got:\n%s", verUnknown, verLatest, buf.String())
	}
}

func TestReadConfFile(t *testing.T) { //nolint:paralleltest // modifies xdg globals
	cleanup := withTempXDG(t)
	defer cleanup()

	confPath := filepath.Join(xdg.ConfigHome, "gup.json")
	content := `{
  "schema_version": 1,
  "packages": [
    {
      "name": "foo",
      "import_path": "example.com/foo",
      "version": "v1.2.3",
      "channel": "main"
    },
    {
      "name": "bar",
      "import_path": "example.com/bar",
      "version": "v4.5.6",
      "channel": "master"
    }
  ]
}`
	if err := os.WriteFile(confPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp conf file: %v", err)
	}

	pkgs, err := ReadConfFile(confPath)
	if err != nil {
		t.Fatalf("ReadConfFile() error = %v", err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("ReadConfFile() len = %d, want 2", len(pkgs))
	}
	if pkgs[0].Name != "foo" || pkgs[0].ImportPath != "example.com/foo" {
		t.Fatalf("first pkg mismatch: %+v", pkgs[0])
	}
	if pkgs[0].Version == nil || pkgs[0].Version.Current != "v1.2.3" {
		t.Fatalf("first pkg version mismatch: %+v", pkgs[0].Version)
	}
	if pkgs[0].UpdateChannel != goutil.UpdateChannelMain {
		t.Fatalf("first pkg channel mismatch: %s", pkgs[0].UpdateChannel)
	}
	if pkgs[1].Name != "bar" || pkgs[1].ImportPath != "example.com/bar" {
		t.Fatalf("second pkg mismatch: %+v", pkgs[1])
	}
	if pkgs[1].Version == nil || pkgs[1].Version.Current != "v4.5.6" {
		t.Fatalf("second pkg version mismatch: %+v", pkgs[1].Version)
	}
	if pkgs[1].UpdateChannel != goutil.UpdateChannelMaster {
		t.Fatalf("second pkg channel mismatch: %s", pkgs[1].UpdateChannel)
	}
}

func TestReadConfFile_Empty(t *testing.T) {
	t.Parallel()

	confPath := filepath.Join(t.TempDir(), "gup.json")
	if err := os.WriteFile(confPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write temp conf file: %v", err)
	}

	pkgs, err := ReadConfFile(confPath)
	if err != nil {
		t.Fatalf("ReadConfFile() error = %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("ReadConfFile() len = %d, want 0", len(pkgs))
	}
}

func TestReadConfFile_InvalidFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "invalid json",
			content: `{"schema_version": 1,`,
		},
		{
			name:    "unsupported schema",
			content: `{"schema_version": 99, "packages": []}`,
		},
		{
			name: "invalid package entry",
			content: `{
  "schema_version": 1,
  "packages": [
    {
      "name": "",
      "import_path": "example.com/foo",
      "version": "v1.2.3",
      "channel": "latest"
    }
  ]
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			confPath := filepath.Join(t.TempDir(), "gup.json")
			if err := os.WriteFile(confPath, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("failed to write temp conf file: %v", err)
			}

			if _, err := ReadConfFile(confPath); err == nil {
				t.Fatal("ReadConfFile() should return error for invalid format")
			}
		})
	}
}

func TestResolveImportFilePath(t *testing.T) { //nolint:paralleltest // changes working dir
	cleanup := withTempXDG(t)
	defer cleanup()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	custom := filepath.Join(t.TempDir(), "custom.json")
	if got, err := ResolveImportFilePath(custom); err != nil || got != custom {
		t.Fatalf("ResolveImportFilePath(explicit) = (%s, %v), want (%s, nil)", got, err, custom)
	}

	// Neither candidate exists: returns the user-level path so the caller can
	// report it as "not found".
	if got, err := ResolveImportFilePath(""); err != nil || got != FilePath() {
		t.Fatalf("ResolveImportFilePath(none) = (%s, %v), want (%s, nil)", got, err, FilePath())
	}

	// Only ./gup.json exists: it is used.
	local := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(local, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ResolveImportFilePath(""); err != nil || got != LocalFilePath() {
		t.Fatalf("ResolveImportFilePath(local) = (%s, %v), want (%s, nil)", got, err, LocalFilePath())
	}

	// Both candidates exist: ambiguous, must return an error mentioning --file.
	xdgFile := FilePath()
	if err := os.MkdirAll(filepath.Dir(xdgFile), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgFile, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveImportFilePath("")
	if err == nil {
		t.Fatalf("ResolveImportFilePath(ambiguous) = (%s, nil), want error", got)
	}
	if got != "" {
		t.Fatalf("ResolveImportFilePath(ambiguous) path = %s, want empty", got)
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Fatalf("ResolveImportFilePath(ambiguous) error = %q, want it to mention --file", err.Error())
	}

	// An explicit path still wins even when both candidates exist.
	if got, err := ResolveImportFilePath(custom); err != nil || got != custom {
		t.Fatalf("ResolveImportFilePath(explicit over ambiguous) = (%s, %v), want (%s, nil)", got, err, custom)
	}
}

// TestResolveImportFilePathDirectoryCandidate verifies that an auto-detected
// gup.json candidate (./gup.json or the XDG user-level config) is rejected with
// an error when it is a directory instead of a file. Previously such a directory
// was silently ignored because IsFile returns false for directories, so import
// fell back to "not found" or the other candidate without telling the user.
func TestResolveImportFilePathDirectoryCandidate(t *testing.T) { //nolint:paralleltest // changes working dir
	t.Run("local ./gup.json is a directory", func(t *testing.T) { //nolint:paralleltest // changes working dir
		cleanup := withTempXDG(t)
		defer cleanup()

		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(wd)
		})

		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		if err := os.Mkdir(filepath.Join(tmpDir, ConfigFileName), 0o750); err != nil {
			t.Fatal(err)
		}

		got, err := ResolveImportFilePath("")
		if err == nil {
			t.Fatalf("ResolveImportFilePath(local dir) = (%s, nil), want error", got)
		}
		if got != "" {
			t.Fatalf("ResolveImportFilePath(local dir) path = %s, want empty", got)
		}
		if !strings.Contains(err.Error(), "directory") {
			t.Fatalf("ResolveImportFilePath(local dir) error = %q, want it to mention it is a directory", err.Error())
		}
	})

	t.Run("XDG gup.json is a directory", func(t *testing.T) { //nolint:paralleltest // changes working dir
		cleanup := withTempXDG(t)
		defer cleanup()

		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(wd)
		})

		// Run from a directory with no local ./gup.json so only the XDG
		// candidate is in play.
		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		xdgFile := FilePath()
		if err := os.MkdirAll(xdgFile, 0o750); err != nil {
			t.Fatal(err)
		}

		got, err := ResolveImportFilePath("")
		if err == nil {
			t.Fatalf("ResolveImportFilePath(xdg dir) = (%s, nil), want error", got)
		}
		if got != "" {
			t.Fatalf("ResolveImportFilePath(xdg dir) path = %s, want empty", got)
		}
		if !strings.Contains(err.Error(), "directory") {
			t.Fatalf("ResolveImportFilePath(xdg dir) error = %q, want it to mention it is a directory", err.Error())
		}
	})
}

func TestResolveExportFilePath(t *testing.T) {
	t.Parallel()

	custom := filepath.Join(t.TempDir(), "custom.json")
	if got := ResolveExportFilePath(custom); got != custom {
		t.Fatalf("ResolveExportFilePath(explicit) = %s, want %s", got, custom)
	}
	if got := ResolveExportFilePath(""); got != FilePath() {
		t.Fatalf("ResolveExportFilePath(default) = %s, want %s", got, FilePath())
	}
}
