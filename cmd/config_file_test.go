package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
)

// oldImport is the import path used by config-file tests that exercise the
// air package rename. It is defined at package level so the literal is not
// duplicated across test cases (goconst).
const oldImport = "github.com/cosmtrek/air/cmd/air"

func TestRenameWithBackupSwap_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "gup.json.tmp")
	dst := filepath.Join(dir, "gup.json")

	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := renameWithBackupSwap(src, dst); err != nil {
		t.Fatalf("renameWithBackupSwap() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Clean(dst))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("updated content = %q, want %q", string(got), "new")
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("src file should be moved, stat err = %v", err)
	}
}

func TestRenameWithBackupSwap_RestoreOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "missing.tmp")
	dst := filepath.Join(dir, "gup.json")

	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := renameWithBackupSwap(src, dst); err == nil {
		t.Fatal("renameWithBackupSwap() should return error")
	}

	got, err := os.ReadFile(filepath.Clean(dst))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("restored content = %q, want %q", string(got), "old")
	}
}

func Test_shouldRetryRenameWithReplace(t *testing.T) {
	t.Parallel()

	dst := filepath.Join(t.TempDir(), "gup.json")
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !shouldRetryRenameWithReplace(os.ErrExist, dst) {
		t.Fatal("shouldRetryRenameWithReplace() should return true for os.ErrExist")
	}

	got := shouldRetryRenameWithReplace(os.ErrNotExist, dst)
	if runtime.GOOS == goosWindows {
		if !got {
			t.Fatal("shouldRetryRenameWithReplace() should return true on Windows when dst exists")
		}
		return
	}
	if got {
		t.Fatal("shouldRetryRenameWithReplace() should return false on non-Windows for non-exist error")
	}
}

func Test_renameWithReplace_errorWhenSrcMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "missing.tmp")
	dst := filepath.Join(dir, "gup.json")
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := renameWithReplace(src, dst); err == nil {
		t.Fatal("renameWithReplace() should return error when source file does not exist")
	}
}

func Test_writeConfigFile_success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.json")

	pkgs := []goutil.Package{
		{
			Name:       "air",
			ImportPath: oldImport,
			Version:    &goutil.Version{Current: "v1.2.3"},
		},
	}

	if err := writeConfigFile(path, pkgs); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("config file should exist: %v", err)
	}
	if !strings.Contains(string(got), oldImport) {
		t.Fatalf("config file content = %q, want it to contain the import path", string(got))
	}

	assertNoTempFiles(t, dir, filepath.Base(path))
}

func Test_writeConfigFile_idempotentOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.json")

	pkgs := []goutil.Package{
		{
			Name:       "air",
			ImportPath: oldImport,
			Version:    &goutil.Version{Current: "v1.2.3"},
		},
	}

	if err := writeConfigFile(path, pkgs); err != nil {
		t.Fatalf("first writeConfigFile() error = %v", err)
	}
	first, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	if err := writeConfigFile(path, pkgs); err != nil {
		t.Fatalf("second writeConfigFile() error = %v", err)
	}
	second, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	if string(first) != string(second) {
		t.Fatalf("overwrite changed content: first = %q, second = %q", string(first), string(second))
	}

	assertNoTempFiles(t, dir, filepath.Base(path))
}

//nolint:paralleltest // mutates package-level renameFunc
func Test_renameWithBackupSwap_restoreFailure(t *testing.T) {
	origRename := renameFunc
	t.Cleanup(func() { renameFunc = origRename })

	dir := t.TempDir()
	src := filepath.Join(dir, "gup.json.tmp")
	dst := filepath.Join(dir, "gup.json")

	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	updateErr := errors.New("forced update rename failure")
	restoreErr := errors.New("forced restore rename failure")

	// First call (dst -> backup) succeeds, second (src -> dst) fails,
	// third (backup -> dst restore) also fails: the config-loss worst case.
	calls := 0
	renameFunc = func(oldpath, newpath string) error {
		calls++
		switch calls {
		case 1:
			return origRename(oldpath, newpath)
		case 2:
			return updateErr
		default:
			return restoreErr
		}
	}

	err := renameWithBackupSwap(src, dst)
	if err == nil {
		t.Fatal("renameWithBackupSwap() should return error on restore failure")
	}
	if !errors.Is(err, updateErr) {
		t.Fatalf("error = %v, want it to wrap the update error", err)
	}
	if !errors.Is(err, restoreErr) {
		t.Fatalf("error = %v, want it to wrap the restore error", err)
	}
}

//nolint:paralleltest // mutates package-level renameFunc
func Test_writeConfigFile_noStrayFilesOnRenameFailure(t *testing.T) {
	origRename := renameFunc
	t.Cleanup(func() { renameFunc = origRename })

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.json")

	renameFunc = func(_, _ string) error {
		return errors.New("forced rename failure")
	}

	err := writeConfigFile(path, []goutil.Package{{Name: "dummy"}})
	if err == nil {
		t.Fatal("writeConfigFile() should return error when rename fails")
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("config file should not exist after failed write, stat err = %v", statErr)
	}
	assertNoTempFiles(t, dir, filepath.Base(path))
}

// assertNoTempFiles fails if any leftover temporary or backup files matching the
// config file's temp/backup naming pattern remain in dir.
func assertNoTempFiles(t *testing.T, dir, base string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, base+".tmp-") || strings.HasPrefix(name, base+".bak-") {
			t.Fatalf("stray temp/backup file left behind: %s", name)
		}
	}
}
