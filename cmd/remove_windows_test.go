//go:build windows

// These tests exist because the thing they are about only happens on Windows:
// the name a program passes to the filesystem is not the name the filesystem
// looks up. Win32 strips trailing dots and spaces on the way in, and NTFS
// answers to an 8.3 alias as well as to the name a file was created with. Both
// are ways to reach `.gup.lock` while spelling something else, and both are
// invisible on Linux and macOS - so they are checked on the machine where they
// are real.
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/lockfile"
	"golang.org/x/sys/windows"
)

// Test_windowsNameNormalizationReachesTheLockFile establishes the threat the
// rest of this file is about, rather than assuming it. If Windows ever stopped
// folding these spellings, this would say so instead of the refusals below
// quietly becoming tests of nothing.
func Test_windowsNameNormalizationReachesTheLockFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lock := filepath.Join(dir, lockfile.DirLockName)
	if err := os.WriteFile(lock, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, spelling := range []string{lock + ".", lock + " ", lock + "..."} {
		if !lockfile.SameFile(spelling, lock) {
			t.Errorf("%q does not reach the lock file on this Windows: the refusal it drives is untested", spelling)
		}
	}
}

// Test_removeLoop_refusesTheLockFileSpelledWithATrailingDot is the regression
// test for a real bypass of the name check.
//
// `.gup.lock.` is not `.gup.lock` to any string comparison, and it is the same
// file to Windows. gup composes exactly that spelling on its own whenever $GOEXE
// is a bare dot, because a name that does not already end in $GOEXE gets it
// appended. Before this was fixed, `gup remove .gup.lock` with $GOEXE=. deleted
// the lock the running command was holding: the kernel lock survived on the open
// handle, the NAME did not, and the next gup created its own file there and
// locked that instead.
func Test_removeLoop_refusesTheLockFileSpelledWithATrailingDot(t *testing.T) {
	gobin := t.TempDir()
	lock := filepath.Join(gobin, lockfile.DirLockName)
	if err := os.WriteFile(lock, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOEXE", ".")
	p, buf := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{lockfile.DirLockName}); got != 1 {
		t.Errorf("removeLoop() = %d, want 1", got)
	}
	if !fileutil.IsFile(lock) {
		t.Fatal("removeLoop() deleted gup's own lock file through a trailing dot")
	}
	if out := buf.String(); !strings.Contains(out, "gup's own lock file") {
		t.Errorf("removeLoop() output %q does not explain the refusal", out)
	}
}

// Test_removeLoop_refusesTheLockFileByItsShortName covers the spelling no string
// rule can be written against: NTFS keeps an 8.3 alias for `.gup.lock`, and it
// looks nothing like it. Only asking the filesystem which file the name reaches
// catches this one.
//
// 8.3 alias generation is off on some volumes and can be off machine-wide, in
// which case GetShortPathName hands back the long name and there is no second
// spelling to test.
func Test_removeLoop_refusesTheLockFileByItsShortName(t *testing.T) {
	gobin := t.TempDir()
	lock := filepath.Join(gobin, lockfile.DirLockName)
	if err := os.WriteFile(lock, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	short := filepath.Base(shortPathName(t, lock))
	if strings.EqualFold(short, lockfile.DirLockName) {
		t.Skip("this volume keeps no 8.3 alias, so there is no short name to refuse")
	}
	// $GOEXE is pinned to the alias's own extension so removeLoop appends nothing
	// to it: the point is the name reaching the lock file, not what gup adds.
	ext := filepath.Ext(short)
	if ext == "" {
		t.Skipf("the 8.3 alias %q has no extension for $GOEXE to match", short)
	}
	t.Setenv("GOEXE", ext)

	p, buf := newTestPrinter()
	if got := removeLoop(p, gobin, true, []string{short}); got != 1 {
		t.Errorf("removeLoop(%q) = %d, want 1", short, got)
	}
	if !fileutil.IsFile(lock) {
		t.Fatalf("removeLoop() deleted gup's own lock file through its 8.3 alias %q", short)
	}
	if out := buf.String(); !strings.Contains(out, "gup's own lock file") {
		t.Errorf("removeLoop(%q) output %q does not explain the refusal", short, out)
	}
}

// shortPathName returns the 8.3 alias Windows keeps for path, or path itself
// where the volume keeps none.
func shortPathName(t *testing.T, path string) string {
	t.Helper()

	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("failed to convert %s: %v", path, err)
	}
	const bufLen = windows.MAX_PATH
	buf := make([]uint16, bufLen)
	n, err := windows.GetShortPathName(long, &buf[0], bufLen)
	if err != nil || n == 0 || n > bufLen {
		t.Skipf("GetShortPathName(%s) is unavailable here: %v", path, err)
	}
	return windows.UTF16ToString(buf[:n])
}
