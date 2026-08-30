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
	if err := os.WriteFile(dst, []byte(testNameOld), 0o600); err != nil {
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

	if err := os.WriteFile(dst, []byte(testNameOld), 0o600); err != nil {
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
	if err := os.WriteFile(dst, []byte(testNameOld), 0o600); err != nil {
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
	if err := os.WriteFile(dst, []byte(testNameOld), 0o600); err != nil {
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
	if err := os.WriteFile(dst, []byte(testNameOld), 0o600); err != nil {
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

// Test_writeConfigFile_rejectsEmptyDirectory verifies the #367 contract: an
// existing empty directory passed as the destination is rejected, the directory
// survives, and no temp/backup artifacts are left behind.
func Test_writeConfigFile_rejectsEmptyDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target := filepath.Join(parent, "gup.json")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}

	err := writeConfigFile(target, []goutil.Package{{Name: testBinPosixer}})
	if err == nil {
		t.Fatal("writeConfigFile() should reject an existing directory path")
	}

	info, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatalf("directory should still exist after failed write: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("target should still be a directory, not replaced by a file")
	}
	assertNoTempFiles(t, parent, filepath.Base(target))
}

// Test_writeConfigFile_rejectsNonEmptyDirectory verifies #367 for a directory
// that has contents: the directory and its child are untouched and no *.bak-* is
// created next to it.
func Test_writeConfigFile_rejectsNonEmptyDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target := filepath.Join(parent, "gup.json")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(child, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeConfigFile(target, []goutil.Package{{Name: testBinPosixer}}); err == nil {
		t.Fatal("writeConfigFile() should reject a non-empty directory path")
	}

	if info, statErr := os.Stat(target); statErr != nil || !info.IsDir() {
		t.Fatalf("directory should survive failed write, stat err = %v", statErr)
	}
	if data, readErr := os.ReadFile(filepath.Clean(child)); readErr != nil || string(data) != "precious" {
		t.Fatalf("directory contents must be unchanged, read err = %v, data = %q", readErr, data)
	}
	assertNoTempFiles(t, parent, filepath.Base(target))
}

// Test_writeConfigFile_regularFileStillWorks is a regression guard ensuring the
// #367 directory check does not break the normal file-target path.
func Test_writeConfigFile_regularFileStillWorks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.json")

	pkg := goutil.Package{Name: testBinPosixer, ImportPath: testImportPathPosixer, Version: &goutil.Version{Current: "v0.1.0"}}
	if err := writeConfigFile(path, []goutil.Package{pkg}); err != nil {
		t.Fatalf("writeConfigFile() on a regular file path should succeed, got: %v", err)
	}
	if !fileExists(t, path) {
		t.Fatal("expected the config file to be written")
	}
	assertNoTempFiles(t, dir, filepath.Base(path))
}

// Test_writeConfigFile_preservesSymlink verifies that when gup.json is a symlink
// (e.g. a dotfile manager links it into place), writing the config rewrites the
// link's target file and keeps the symlink intact rather than replacing it with a
// regular file.
func Test_writeConfigFile_preservesSymlink(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("symlink behavior is POSIX-specific")
	}
	t.Parallel()

	dir := t.TempDir()
	realPath := filepath.Join(dir, "real-gup.json")
	if err := os.WriteFile(realPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "gup.json")
	if err := os.Symlink(realPath, link); err != nil {
		t.Fatal(err)
	}

	pkg := goutil.Package{Name: testBinAir, ImportPath: oldImport, Version: &goutil.Version{Current: testVersion123}}
	if err := writeConfigFile(link, []goutil.Package{pkg}); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	// The link must remain a symlink, not be replaced by a regular file.
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("gup.json symlink was replaced by a regular file")
	}

	// The config must have been written through the link to the real target.
	got, err := os.ReadFile(filepath.Clean(realPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), oldImport) {
		t.Fatalf("config was not written through the symlink to its target: %q", string(got))
	}
	assertNoTempFiles(t, dir, filepath.Base(link))
	// Temps are staged next to the resolved target, so check that basename too.
	assertNoTempFiles(t, dir, filepath.Base(realPath))
}

// Test_writeConfigFile_preservesDanglingSymlink verifies that when gup.json is a
// symlink whose target does not exist yet (a dotfile tool linked it before the
// target was written), the config write creates the link target instead of
// replacing the symlink with a regular file.
func Test_writeConfigFile_preservesDanglingSymlink(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("symlink behavior is POSIX-specific")
	}
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "real-gup.json")
	link := filepath.Join(dir, "gup.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	pkg := goutil.Package{Name: testBinAir, ImportPath: oldImport, Version: &goutil.Version{Current: testVersion123}}
	if err := writeConfigFile(link, []goutil.Package{pkg}); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dangling gup.json symlink was replaced by a regular file")
	}
	got, err := os.ReadFile(filepath.Clean(target))
	if err != nil {
		t.Fatalf("link target was not created through the symlink: %v", err)
	}
	if !strings.Contains(string(got), oldImport) {
		t.Fatalf("config was not written to the link target: %q", string(got))
	}
	assertNoTempFiles(t, dir, filepath.Base(link))
	// Temps are staged next to the resolved target, so check that basename too.
	assertNoTempFiles(t, dir, filepath.Base(target))
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
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

// TestWriteConfigFile_createTempFailure covers the temp-file creation error
// branch when the destination directory is not writable.
//
//nolint:paralleltest // relies on process-wide filesystem state
func TestWriteConfigFile_createTempFailure(t *testing.T) {
	skipIfDirWriteFaultUnsupported(t)
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(roDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) }) //nolint:gosec // restore dir perms so t.TempDir cleanup can remove it

	err := writeConfigFile(filepath.Join(roDir, "gup.json"), []goutil.Package{{Name: "x"}})
	if err == nil {
		t.Fatal("writeConfigFile() err = nil, want error creating temp file in a read-only dir")
	}
}

// TestWriteConfigFile_backupSwapPath drives the rename-backup-swap fallback used
// when os.Rename can not overwrite the destination (the Windows path), by making
// the first rename report os.ErrExist so shouldRetryRenameWithReplace triggers
// the backup swap; the swap itself uses the real rename.
//
//nolint:paralleltest // mutates package-level renameFunc
func TestWriteConfigFile_backupSwapPath(t *testing.T) {
	origRename := renameFunc
	t.Cleanup(func() { renameFunc = origRename })

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.json")
	if err := os.WriteFile(path, []byte(testNameOld), 0o600); err != nil {
		t.Fatal(err)
	}

	calls := 0
	renameFunc = func(oldpath, newpath string) error {
		calls++
		if calls == 1 {
			// First tmp->dst rename: pretend the destination can not be
			// overwritten, so renameWithReplace falls back to the backup swap.
			return os.ErrExist
		}
		return origRename(oldpath, newpath)
	}

	if err := writeConfigFile(path, []goutil.Package{{Name: "x", ImportPath: "example.com/x", Version: &goutil.Version{Current: testVersionOne}}}); err != nil {
		t.Fatalf("writeConfigFile() err = %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "example.com/x") {
		t.Fatalf("config not written through backup swap: %s", data)
	}
}

// Test_writeConfigFile_replacesAReadOnlyConfigInPlace covers the destination the
// backup swap was written for. A read-only gup.json must still be rewritten -
// and rewritten by an atomic replace, because the swap moves the old file away
// first, and gup's unlocked readers (check, list, import) would read that moment
// as "no config".
func Test_writeConfigFile_replacesAReadOnlyConfigInPlace(t *testing.T) { //nolint:paralleltest // mutates package-level renameFunc
	origRename := renameFunc
	t.Cleanup(func() { renameFunc = origRename })

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"packages":[]}`), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	// POSIX lets a rename replace a read-only file, so the refusal Windows gives
	// is simulated here: the first attempt fails, and the retry after the
	// read-only bit is cleared must be the one that succeeds.
	firstAttempt := true
	renameFunc = func(oldpath, newpath string) error {
		if firstAttempt {
			firstAttempt = false
			return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: os.ErrPermission}
		}
		return origRename(oldpath, newpath)
	}

	if err := writeConfigFile(path, []goutil.Package{{Name: testBinDummy, ImportPath: "example.com/dummy"}}); err != nil {
		t.Fatalf("writeConfigFile() error = %v", err)
	}

	// No swap happened, so nothing was ever moved out of the way: the backup the
	// swap would have created is not there, and neither is a temp file.
	assertNoTempFiles(t, dir, filepath.Base(path))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the config file is gone: %v", err)
	}
	// The read-only intent survives the write rather than being silently dropped.
	if info.Mode().Perm()&0o200 != 0 {
		t.Errorf("gup.json is now mode %v, want the read-only mode it had", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path) //nolint:gosec // a path this test just created
	if err != nil {
		t.Fatalf("failed to read the config file: %v", err)
	}
	if !strings.Contains(string(raw), "example.com/dummy") {
		t.Errorf("gup.json = %q, want the newly written package", raw)
	}
}
