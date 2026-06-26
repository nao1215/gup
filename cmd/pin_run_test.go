package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

// stubPinPackageInfo swaps the installed-package lister for the duration of a
// test and restores it afterward.
func stubPinPackageInfo(t *testing.T, pkgs []goutil.Package) {
	t.Helper()
	orig := pinPackageInfo
	pinPackageInfo = func(*print.Printer) ([]goutil.Package, error) { return pkgs, nil }
	t.Cleanup(func() { pinPackageInfo = orig })
}

// writeUserConfig writes content to the user-level gup.json under the isolated
// XDG config home.
func writeUserConfig(t *testing.T, content string) {
	t.Helper()
	path := config.FilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

//nolint:paralleltest // swaps package globals and XDG env; must not run in parallel
func TestRunPin_writesPinnedConfig(t *testing.T) {
	setupXDGBase(t)
	stubPinPackageInfo(t, []goutil.Package{
		{Name: testBinTool, ImportPath: pinnedTestImport, Version: &goutil.Version{Current: "v0.9.0"}},
	})

	p, _ := newTestPrinter()
	if code := runPin(p, newPinCmd(), []string{testBinTool, testVersionOne}); code != 0 {
		t.Fatalf("runPin() = %d, want 0", code)
	}

	raw, err := os.ReadFile(config.FilePath())
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	got := string(raw)
	for _, want := range []string{`"schema_version": 2`, `"channel": "pinned"`, `"version": "v1.0.0"`, `"name": "tool"`} {
		if !strings.Contains(got, want) {
			t.Errorf("written config missing %q:\n%s", want, got)
		}
	}
}

//nolint:paralleltest // swaps package globals and XDG env; must not run in parallel
func TestRunPin_unmanagedToolFails(t *testing.T) {
	setupXDGBase(t)
	stubPinPackageInfo(t, []goutil.Package{
		{Name: "other", ImportPath: "example.com/other", Version: &goutil.Version{Current: testVersionOne}},
	})

	p, _ := newTestPrinter()
	if code := runPin(p, newPinCmd(), []string{testBinTool, testVersionOne}); code != 1 {
		t.Fatalf("runPin() for an unmanaged tool = %d, want 1", code)
	}
	// Nothing must be written when the pin target can't be resolved.
	if _, err := os.Stat(config.FilePath()); !os.IsNotExist(err) {
		t.Errorf("config must not be created on failure (stat err = %v)", err)
	}
}

//nolint:paralleltest // swaps package globals and XDG env; must not run in parallel
func TestRunPin_invalidVersionFails(t *testing.T) {
	setupXDGBase(t)
	stubPinPackageInfo(t, []goutil.Package{
		{Name: testBinTool, ImportPath: pinnedTestImport, Version: &goutil.Version{Current: testVersionOne}},
	})

	// "latest" is a channel keyword, not a concrete version: pin must reject it.
	p, _ := newTestPrinter()
	if code := runPin(p, newPinCmd(), []string{testBinTool, string(goutil.UpdateChannelLatest)}); code != 1 {
		t.Fatalf("runPin() with a channel keyword as version = %d, want 1", code)
	}
}

//nolint:paralleltest // swaps XDG env; must not run in parallel
func TestRunUnpin_clearsPin(t *testing.T) {
	setupXDGBase(t)
	writeUserConfig(t, `{"schema_version":2,"packages":[
		{"name":"tool","import_path":"example.com/tool","version":"v1.0.0","channel":"pinned"}
	]}`)

	p, _ := newTestPrinter()
	if code := runUnpin(p, newUnpinCmd(), []string{testBinTool}); code != 0 {
		t.Fatalf("runUnpin() = %d, want 0", code)
	}

	raw, err := os.ReadFile(config.FilePath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(raw)
	if strings.Contains(got, `"channel": "pinned"`) {
		t.Errorf("pin should be cleared:\n%s", got)
	}
	if !strings.Contains(got, `"channel": "latest"`) {
		t.Errorf("unpinned entry should be on the latest channel:\n%s", got)
	}
	// No pin remaining means the file drops back to schema_version 1.
	if !strings.Contains(got, `"schema_version": 1`) {
		t.Errorf("config with no pins should be schema_version 1:\n%s", got)
	}
}

//nolint:paralleltest // swaps XDG env; must not run in parallel
func TestRunUnpin_notPinnedIsIdempotent(t *testing.T) {
	setupXDGBase(t)
	writeUserConfig(t, `{"schema_version":1,"packages":[
		{"name":"tool","import_path":"example.com/tool","version":"v1.0.0","channel":"latest"}
	]}`)

	p, _ := newTestPrinter()
	if code := runUnpin(p, newUnpinCmd(), []string{testBinTool}); code != 0 {
		t.Fatalf("runUnpin() on a non-pinned tool = %d, want 0 (idempotent)", code)
	}
}

//nolint:paralleltest // swaps XDG env; must not run in parallel
func TestRunUnpin_missingToolName(t *testing.T) {
	setupXDGBase(t)
	// "@" strips to an empty target, which must be rejected.
	p, _ := newTestPrinter()
	if code := runUnpin(p, newUnpinCmd(), []string{"@"}); code != 1 {
		t.Fatalf("runUnpin() with an empty tool name = %d, want 1", code)
	}
}

//nolint:paralleltest // swaps XDG env; must not run in parallel
func TestRunUnpin_malformedConfigFails(t *testing.T) {
	setupXDGBase(t)
	writeUserConfig(t, `{"schema_version":1,"packages":[`) // truncated JSON

	p, _ := newTestPrinter()
	if code := runUnpin(p, newUnpinCmd(), []string{testBinTool}); code != 1 {
		t.Fatalf("runUnpin() with a malformed config = %d, want 1", code)
	}
}

//nolint:paralleltest // swaps package globals and XDG env; must not run in parallel
func TestRunPin_malformedConfigFails(t *testing.T) {
	setupXDGBase(t)
	stubPinPackageInfo(t, []goutil.Package{
		{Name: testBinTool, ImportPath: pinnedTestImport, Version: &goutil.Version{Current: testVersionOne}},
	})
	writeUserConfig(t, `{"schema_version":1,"packages":[`) // truncated JSON

	p, _ := newTestPrinter()
	if code := runPin(p, newPinCmd(), []string{testBinTool, testVersionOne}); code != 1 {
		t.Fatalf("runPin() with a malformed config = %d, want 1", code)
	}
}
