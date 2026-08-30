// The tests in this file are about the two ways a path lies about which file it
// names: a symbolic link pointing somewhere else entirely, and two different
// names for one file. Both were real bugs. Locking through a link let gup
// truncate an arbitrary file the user could write; treating two names for one
// directory as two locks made gup wait out the timeout against itself and then
// report itself as another gup process.
package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// symlinkOrSkip links link to target, skipping when the platform refuses to let
// an unprivileged process create one - which is Windows without Developer Mode.
// Skipping is right rather than failing: a machine that cannot create the attack
// cannot be attacked that way either.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform will not let this process create a symlink: %v", err)
	}
}

// TestAcquire_refusesALockPathThatIsASymlink is the regression test for the
// worst thing this package could do.
//
// gup truncates its lock file to write the owner record into it. A lock path
// replaced with a link - `gup.json.lock -> ~/.ssh/authorized_keys`, or any other
// file gup has permission to write - therefore used to empty that file, with the
// command reporting success. The refusal has to come from the open itself: a
// check before opening leaves a window for the link to be planted in.
func TestAcquire_refusesALockPathThatIsASymlink(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()
	victim := filepath.Join(dir, "precious.txt")
	const content = "the file the link points at"
	if err := os.WriteFile(victim, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write the file the link points at: %v", err)
	}
	path := filepath.Join(dir, "gup.json.lock")
	symlinkOrSkip(t, victim, path)

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err == nil {
		_ = lock.Release()
		t.Fatal("Acquire() on a symlinked lock path succeeded, want a refusal")
	}
	if !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("Acquire() error = %v, want it to say the lock path is a link", err)
	}
	if !errors.Is(err, errLockPathIsSymlink) {
		t.Errorf("Acquire() error = %v, want it to wrap errLockPathIsSymlink", err)
	}

	got, err := os.ReadFile(victim) //nolint:gosec // a path this test created
	if err != nil {
		t.Fatalf("the file the link points at could not be read back: %v", err)
	}
	if string(got) != content {
		t.Errorf("the file the link points at now holds %q, want %q untouched", got, content)
	}
	// The link is still a link: gup refused it rather than replacing it.
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("the lock path could not be inspected: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Acquire() replaced the symlink instead of refusing it")
	}
}

// TestAcquireAll_refusesALockPathThatIsASymlink covers the same refusal in a
// set, including the rollback: the lock it did take must be free afterwards, or
// the next command waits on a resource nobody is using.
func TestAcquireAll_refusesALockPathThatIsASymlink(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()
	victim := filepath.Join(dir, "precious.txt")
	if err := os.WriteFile(victim, []byte("intact"), 0o600); err != nil {
		t.Fatalf("failed to write the file the link points at: %v", err)
	}
	linked := filepath.Join(dir, "linked.lock")
	symlinkOrSkip(t, victim, linked)
	plain := filepath.Join(dir, "plain.lock")

	if _, err := AcquireAll(t.Context(), cmdUpdate, plain, linked); err == nil {
		t.Fatal("AcquireAll() with a symlinked lock path succeeded, want a refusal")
	}
	got, err := os.ReadFile(victim) //nolint:gosec // a path this test created
	if err != nil {
		t.Fatalf("the file the link points at could not be read back: %v", err)
	}
	if string(got) != "intact" {
		t.Errorf("the file the link points at now holds %q, want it untouched", got)
	}

	held, err := AcquireAll(t.Context(), cmdUpdate, plain)
	if err != nil {
		t.Fatalf("AcquireAll() after a rolled-back set = %v, want success", err)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquireAll_treatsASymlinkedDirectoryAsOneLock is the `gup migrate BEFORE
// AFTER` case where the two arguments name one directory.
//
// Deduplicating the path STRINGS leaves both in the set. gup then takes the
// kernel lock on the first, asks for the same file again on a second descriptor,
// waits out the whole timeout, and reports itself as another gup process - a
// command that can never succeed, with a message pointing at a process that does
// not exist.
func TestAcquireAll_treatsASymlinkedDirectoryAsOneLock(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o750); err != nil {
		t.Fatalf("failed to create the directory the link points at: %v", err)
	}
	alias := filepath.Join(root, "alias")
	symlinkOrSkip(t, real, alias)

	start := time.Now()
	held, err := AcquireAll(t.Context(), "migrate", PathForDir(real), PathForDir(alias))
	if err != nil {
		t.Fatalf("AcquireAll() on one directory named twice = %v, want success", err)
	}
	defer func() {
		if err := held.Release(); err != nil {
			t.Errorf("Release() = %v, want nil", err)
		}
	}()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("AcquireAll() took %v: the two names contended against each other", elapsed)
	}
	if got := held.Paths(); len(got) != 1 {
		t.Errorf("Paths() = %v, want one lock for one directory", got)
	}
}

// TestAcquireAll_treatsAHardLinkAsOneLock covers the second name a file can have
// without any path trickery at all. It is the case no amount of path cleaning
// reaches: the two names are equally real, and only the filesystem's own
// identity for the file says they are the same lock.
func TestAcquireAll_treatsAHardLinkAsOneLock(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "first.lock")
	if err := os.WriteFile(first, nil, lockFileMode); err != nil {
		t.Fatalf("failed to create the lock file: %v", err)
	}
	second := filepath.Join(dir, "second.lock")
	if err := os.Link(first, second); err != nil {
		t.Skipf("this filesystem does not support hard links: %v", err)
	}

	start := time.Now()
	held, err := AcquireAll(t.Context(), cmdUpdate, first, second)
	if err != nil {
		t.Fatalf("AcquireAll() on one file named twice = %v, want success", err)
	}
	defer func() {
		if err := held.Release(); err != nil {
			t.Errorf("Release() = %v, want nil", err)
		}
	}()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("AcquireAll() took %v: the two names contended against each other", elapsed)
	}
	if got := held.Paths(); len(got) != 1 {
		t.Errorf("Paths() = %v, want one lock for one file", got)
	}
}

// TestAcquireAll_treatsACaseVariantAsOneLock covers macOS and Windows, where the
// filesystem is normally case-insensitive and `$GOBIN` spelled two ways is one
// directory. On a case-sensitive filesystem the two names really are two files,
// and the test says so by skipping.
func TestAcquireAll_treatsACaseVariantAsOneLock(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()
	lower := filepath.Join(dir, "gobin")
	if err := os.MkdirAll(lower, 0o750); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}
	upper := filepath.Join(dir, "GOBIN")
	if _, err := os.Stat(upper); err != nil {
		t.Skip("this filesystem is case-sensitive, so the two spellings are two directories")
	}

	held, err := AcquireAll(t.Context(), cmdUpdate, PathForDir(lower), PathForDir(upper))
	if err != nil {
		t.Fatalf("AcquireAll() on one directory spelled two ways = %v, want success", err)
	}
	defer func() {
		if err := held.Release(); err != nil {
			t.Errorf("Release() = %v, want nil", err)
		}
	}()
	if got := held.Paths(); len(got) != 1 {
		t.Errorf("Paths() = %v, want one lock for one directory", got)
	}
}

// TestSameFile covers what `gup remove` refuses a file by. Names are the thing
// an attacker controls, so the refusal is decided on the file the name reaches.
func TestSameFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lock := filepath.Join(dir, DirLockName)
	if err := os.WriteFile(lock, nil, lockFileMode); err != nil {
		t.Fatalf("failed to create the lock file: %v", err)
	}
	other := filepath.Join(dir, "gopls")
	if err := os.WriteFile(other, []byte("a tool"), lockFileMode); err != nil {
		t.Fatalf("failed to create the other file: %v", err)
	}

	if !SameFile(lock, lock) {
		t.Error("SameFile() says a file is not itself")
	}
	if SameFile(lock, other) {
		t.Error("SameFile() says two different files are one")
	}
	if SameFile(lock, filepath.Join(dir, "never-existed")) {
		t.Error("SameFile() says a missing path is an existing file")
	}
	if SameFile(filepath.Join(dir, "never-existed"), lock) {
		t.Error("SameFile() says an existing file is a missing path")
	}

	// A second name for one file is the same file, whichever way it was made.
	alias := filepath.Join(dir, "alias")
	if err := os.Link(lock, alias); err != nil {
		t.Logf("this filesystem does not support hard links: %v", err)
	} else if !SameFile(alias, lock) {
		t.Error("SameFile() says a hard link is a different file")
	}

	// A link POINTING at the lock file is not the lock file: deleting the link
	// leaves the lock where it is, so refusing it would refuse something harmless.
	pointer := filepath.Join(dir, "pointer")
	if err := os.Symlink(lock, pointer); err != nil {
		t.Logf("this platform will not let this process create a symlink: %v", err)
	} else if SameFile(pointer, lock) {
		t.Error("SameFile() treats a symlink to the lock file as the lock file")
	}
}
