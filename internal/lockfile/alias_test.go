// The tests in this file are about the second half of "a lock is the file, not
// the path": deriving the lock's own name from a resource's name, and agreeing
// with every other process on the order a set of them is taken in.
//
// Both were real bugs, and both fail silently. A gup.json reached by a second
// name produced a second lock file, so two gups rewrote one configuration each
// holding a lock the other could not see. And a set of locks ordered by PATH is
// only agreed on while every process spells the paths the same way: two that
// reach one pair of resources through differently-sorting names each take the
// one the other is waiting for, and both end in a busy error about a contention
// that was theirs alone.
package lockfile

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestPathForFile_refusesAFileThatHasASecondName covers the alias that cannot be
// canonicalized away.
//
// The lock guarding a gup.json is its sibling, named after it, so the name gup
// was given decides which file the lock is. Two hard links to one configuration
// are two equally real names in two possibly different directories, so
// `--file a/gup.json` and `--file b/gup.json` would take two different locks and
// never contend - two gups rewriting one file, both reporting success, one
// result lost. There is no canonical name to prefer, so gup refuses rather than
// guessing: a command that stops with a reason is better than one that runs
// unprotected.
func TestPathForFile_refusesAFileThatHasASecondName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := filepath.Join(dir, "gup.json")
	if err := os.WriteFile(config, []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}
	// A lock is derived for it happily while it has one name.
	if _, err := PathForFile(config); err != nil {
		t.Fatalf("PathForFile() on an ordinary file = %v, want a lock path", err)
	}

	second := filepath.Join(dir, "also-gup.json")
	if err := os.Link(config, second); err != nil {
		t.Skipf("this filesystem does not support hard links: %v", err)
	}

	for _, name := range []string{config, second} {
		got, err := PathForFile(name)
		if err == nil {
			t.Fatalf("PathForFile(%q) = %q, want a refusal: the file has two names and neither is the one to lock",
				name, got)
		}
		if !strings.Contains(err.Error(), "hard link") {
			t.Errorf("PathForFile(%q) error = %v, want it to say the file has a second name", name, err)
		}
		if !strings.Contains(err.Error(), filepath.Base(name)) {
			t.Errorf("PathForFile(%q) error = %v, want it to name the file", name, err)
		}
	}
}

// TestPathForFile_locksAFileThatDoesNotExistYet is the ordinary first run: there
// is no file to ask the filesystem about, and there is nothing it could be an
// alias of either, so the lock sits beside the path the command is about to
// create.
func TestPathForFile_locksAFileThatDoesNotExistYet(t *testing.T) {
	t.Parallel()

	config := filepath.Join(t.TempDir(), "sub", "gup.json")
	got, err := PathForFile(config)
	if err != nil {
		t.Fatalf("PathForFile() on a path that does not exist = %v, want a lock path", err)
	}
	// Only the last component is asserted literally. The directories above it come
	// back as the operating system spells them, which on Windows is the long form
	// of a temporary directory the test framework may well have named by its 8.3
	// alias - the very rewriting this is here to allow.
	if want := filepath.Base(config) + lockFileSuffix; filepath.Base(got) != want {
		t.Errorf("PathForFile() = %q, want a %q beside the file", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("PathForFile() = %q, want an absolute path: a lock a working directory can move is not a lock", got)
	}
	again, err := PathForFile(config)
	if err != nil {
		t.Fatalf("PathForFile() = %v the second time, want a lock path", err)
	}
	if again != got {
		t.Errorf("PathForFile() = %q and then %q for one path", got, again)
	}
}

// TestPathForFile_followsASymlinkToTheFileTheWriteLands covers the config a
// dotfile manager linked into place. gup writes THROUGH the link so the link
// survives, so the lock has to follow it too: a lock beside the link would let
// `--file link/gup.json` and `--file real/gup.json` rewrite one file without
// ever contending.
func TestPathForFile_followsASymlinkToTheFileTheWriteLands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	real := filepath.Join(dir, "real-gup.json")
	if err := os.WriteFile(real, []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}
	link := filepath.Join(dir, "link-gup.json")
	symlinkOrSkip(t, real, link)

	viaLink, err := PathForFile(link)
	if err != nil {
		t.Fatalf("PathForFile(link) = %v, want a lock path", err)
	}
	viaReal, err := PathForFile(real)
	if err != nil {
		t.Fatalf("PathForFile(real) = %v, want a lock path", err)
	}
	if viaLink != viaReal {
		t.Errorf("PathForFile() = %q through the link and %q directly; the two would never contend", viaLink, viaReal)
	}
}

// TestAcquireAll_agreesOnOrderWithAnotherProcessSpellingThePathsDifferently is
// the cross-process half of the ordering rule, and the only place it can be
// shown: one process taking a set twice proves nothing about two taking it at
// once.
//
// Both children lock the same two directories. One names them directly; the
// other reaches the first through a symlink whose name sorts on the far side of
// the second, so ordering by PATH puts them in opposite orders - each takes the
// resource the other is waiting for, sits there for the whole acquisition
// timeout, and reports a busy error naming the other. Ordering by the identity
// the filesystem gives the file is what makes the two agree: one waits briefly
// for the other and both finish.
func TestAcquireAll_agreesOnOrderWithAnotherProcessSpellingThePathsDifferently(t *testing.T) {
	// Long enough that a genuine handover is never mistaken for a deadlock, and
	// short enough that a deadlocking build fails rather than hangs.
	t.Setenv(envWait, "5s")

	root := t.TempDir()
	first := filepath.Join(root, "d1")
	second := filepath.Join(root, "d2")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}
	// An alias for the FIRST directory whose name sorts after the second's, so
	// the two children's path orders are reverses of each other.
	alias := filepath.Join(root, "z1")
	symlinkOrSkip(t, first, alias)

	runAcquireSetPair(t,
		[]string{PathForDir(first), PathForDir(second)},
		[]string{PathForDir(second), PathForDir(alias)})
}

// TestAcquireAll_agreesOnOrderWithAnotherProcessCapitalizingThePathsDifferently
// is the same property where symlinks are not available: macOS and Windows have
// case-insensitive filesystems, so `$GOBIN` spelled two ways is one directory,
// and the two spellings sort on opposite sides of a second directory. This is
// the case Windows can actually run, which matters because Windows is where a
// path has the most spellings.
func TestAcquireAll_agreesOnOrderWithAnotherProcessCapitalizingThePathsDifferently(t *testing.T) {
	t.Setenv(envWait, "5s")

	root := t.TempDir()
	// Uppercase sorts before lowercase, so "B1" comes before "C2" and "b1" after:
	// one child orders the pair one way and the other the opposite way.
	lower := filepath.Join(root, "b1")
	other := filepath.Join(root, "C2")
	for _, dir := range []string{lower, other} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}
	upper := filepath.Join(root, "B1")
	if _, err := os.Stat(upper); err != nil {
		t.Skip("this filesystem is case-sensitive, so the two spellings are two directories")
	}

	runAcquireSetPair(t,
		[]string{PathForDir(lower), PathForDir(other)},
		[]string{PathForDir(upper), PathForDir(other)})
}

// runAcquireSetPair starts two child processes taking the given sets of locks
// over and over, and fails if either one does not finish cleanly.
//
// A child that cannot agree with the other on the order reports a busy error and
// exits non-zero, so "both exited 0" is the assertion: neither waited out the
// timeout against a process that was waiting for it.
func runAcquireSetPair(t *testing.T, left, right []string) {
	t.Helper()

	// Enough rounds that the two overlap many times over; few enough that the
	// test stays fast when they cooperate.
	const rounds = 20
	done := make(chan error, 2)
	for _, set := range [][]string{left, right} {
		go func() { done <- runAcquireSetChild(t, set, rounds) }()
	}

	deadline := time.After(60 * time.Second)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("a process taking %v could not: %v", left, err)
			}
		case <-deadline:
			t.Fatal("the two processes never finished: they are each holding what the other is waiting for")
		}
	}
}

// runAcquireSetChild re-executes this test binary as a process that takes one
// set of locks, repeatedly, and reports how it exited.
func runAcquireSetChild(t *testing.T, paths []string, rounds int) error {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), os.Args[0]) //nolint:gosec // this test binary, re-executed
	cmd.Env = append(os.Environ(),
		envHelperRole+"="+roleAcquireSet,
		envHelperSet+"="+strings.Join(paths, string(os.PathListSeparator)),
		envHelperRounds+"="+strconv.Itoa(rounds),
		envHelperReady+"="+filepath.Join(t.TempDir(), "ready"),
		envWait+"="+os.Getenv(envWait),
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// TestAcquireAll_releasesWhatItHeldWhenTheContextIsCanceled covers the rollback
// a cancellation has to perform. A gup interrupted while waiting for the second
// of two locks must not walk away still holding the first: the next command
// would wait out its whole timeout against a process that has already given up.
func TestAcquireAll_releasesWhatItHeldWhenTheContextIsCanceled(t *testing.T) {
	// Long enough that the cancellation, not the timeout, is what ends the wait.
	t.Setenv(envWait, "30s")

	dir := t.TempDir()
	free := filepath.Join(dir, "aaa-free.lock")
	taken := filepath.Join(dir, "zzz-taken.lock")
	startHolder(t, taken)

	ctx, cancel := context.WithCancel(t.Context())
	waiting := make(chan error, 1)
	go func() {
		_, err := AcquireAll(ctx, cmdUpdate, free, taken)
		waiting <- err
	}()

	// The other lock is held for good, so this call can only be waiting.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-waiting:
		if err == nil {
			t.Fatal("AcquireAll() succeeded although one lock is held by another process")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("AcquireAll() error = %v, want a cancellation", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("AcquireAll() ignored the cancellation")
	}

	// The lock it had already taken is free RIGHT NOW, not after any timeout.
	shortWait(t)
	start := time.Now()
	held, err := AcquireAll(t.Context(), "remove", free)
	if err != nil {
		t.Fatalf("the lock a canceled AcquireAll() had taken is still held: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("re-taking the rolled-back lock took %v, want it to be free immediately", elapsed)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquireAll_releasesWhatItHeldWhenALaterLockIsRefused covers the same
// rollback for the other way an acquisition ends early: a path in the set that
// cannot be opened at all. Everything taken before it has to be given back, or a
// command that failed leaves the resource locked until the process exits.
func TestAcquireAll_releasesWhatItHeldWhenALaterLockIsRefused(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)

	dir := t.TempDir()
	good := filepath.Join(dir, "good.lock")
	// A directory can never be a lock file, so opening it is refused - and the
	// refusal happens while the good lock above may already be held.
	bad := filepath.Join(dir, "a-directory.lock")
	if err := os.MkdirAll(bad, 0o750); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}

	if _, err := AcquireAll(t.Context(), cmdUpdate, good, bad); err == nil {
		t.Fatal("AcquireAll() with an unusable path succeeded, want a refusal")
	}

	start := time.Now()
	held, err := AcquireAll(t.Context(), "remove", good)
	if err != nil {
		t.Fatalf("the lock the failed AcquireAll() had taken is still held: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("re-taking the rolled-back lock took %v, want it to be free immediately", elapsed)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}
