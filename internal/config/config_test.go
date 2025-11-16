package config

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestDirAndFilePath(t *testing.T) {
	t.Parallel()
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
		{Name: "foo", ImportPath: "example.com/foo"},
		{Name: "bar", ImportPath: "example.com/bar"},
	}

	if err := WriteConfFile(&buf, pkgs); err != nil {
		t.Fatalf("WriteConfFile() error = %v", err)
	}

	want := "foo = example.com/foo\nbar = example.com/bar\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteConfFile() output = %q, want %q", got, want)
	}
}

func TestReadConfFile(t *testing.T) {
	t.Parallel()
	cleanup := withTempXDG(t)
	defer cleanup()

	confPath := filepath.Join(xdg.ConfigHome, "gup.conf")
	content := "foo = example.com/foo\nbar = example.com/bar\n"
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
	if pkgs[1].Name != "bar" || pkgs[1].ImportPath != "example.com/bar" {
		t.Fatalf("second pkg mismatch: %+v", pkgs[1])
	}
}
