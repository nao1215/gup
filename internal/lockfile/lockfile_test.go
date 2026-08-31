// Package lockfile's tests split into two kinds, and the split is the point.
//
// The in-process ones cover the parts that are ordinary code: where a lock file
// lives, what the busy message says, how a set of locks is ordered. The ones
// that matter cover exclusion, and exclusion is a property BETWEEN processes - a
// lock that works perfectly inside one process and not across two looks
// identical from the inside. Those tests therefore run real child processes
// against a real lock file, by re-executing this test binary (see TestMain).
package lockfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	// cmdUpdate is the subcommand name these tests record as the lock's owner; it
	// is repeated often enough that goconst asks for a name.
	cmdUpdate = "update"
	// testLockPath is a path used only in message-formatting assertions.
	testLockPath = "/tmp/gup.lock"
)

// The environment variables that turn a re-execution of this test binary into a
// helper process instead of a test run. See TestMain.
const (
	// envHelperRole selects what the child does: hold a lock, or contend for one.
	envHelperRole = "GUP_LOCKFILE_TEST_ROLE"
	// envHelperLock is the lock file the child acts on.
	envHelperLock = "GUP_LOCKFILE_TEST_LOCK"
	// envHelperReady is the file the child creates once it holds the lock, so the
	// parent can wait for a fact rather than for a duration.
	envHelperReady = "GUP_LOCKFILE_TEST_READY"
	// envHelperCounter is the file the contending children read, increment and
	// write back while holding the lock.
	envHelperCounter = "GUP_LOCKFILE_TEST_COUNTER"
	// envHelperRounds is how many times a contending child takes the lock.
	envHelperRounds = "GUP_LOCKFILE_TEST_ROUNDS"
	// envHelperSet is the list of lock files a roleAcquireSet child takes
	// together, separated by the platform's list separator.
	envHelperSet = "GUP_LOCKFILE_TEST_SET"

	roleHold = "hold"
	// roleHoldDropped holds the lock the way a careless caller would: it throws
	// the Lock away and forces a garbage collection.
	roleHoldDropped = "hold-dropped"
	roleContend     = "contend"
	// roleAcquireSet takes a whole SET of locks at once, repeatedly. It is what
	// makes the acquisition ORDER testable: two children taking one pair of
	// resources by differently-sorting names deadlock unless they agree on which
	// to take first, and one process can never show that about two.
	roleAcquireSet = "acquire-set"
)

// TestMain turns this test binary into the helper process the cross-process
// tests need.
//
// Exclusion cannot be tested any other way. Two goroutines share a kernel lock's
// descriptor table, so they prove nothing about two gups; a fake process that
// only plants a lock FILE proves less than nothing here, because the file is not
// what excludes anyone. Re-executing the test binary gives a genuinely separate
// process running this exact code, on every operating system, with no fixture
// binary to build and keep in step.
func TestMain(m *testing.M) {
	switch os.Getenv(envHelperRole) {
	case roleHold:
		os.Exit(runHoldHelper(false))
	case roleHoldDropped:
		os.Exit(runHoldHelper(true))
	case roleContend:
		os.Exit(runContendHelper())
	case roleAcquireSet:
		os.Exit(runAcquireSetHelper())
	}
	os.Exit(m.Run())
}

// runHoldHelper takes the lock, announces it, and holds it until the parent
// kills the process. It never releases: the tests that use it are about what
// happens when a holder dies without cleaning up.
//
// When drop is set it throws the Lock away and forces a garbage collection
// first, which is the mistake a long-running holder makes most easily. The
// kernel lock lives on a descriptor inside the Lock, and os.File closes itself
// from a finalizer once it is unreachable, so a package that did not keep its
// held locks reachable would quietly stop holding this one - with the helper
// still running and still announcing that it has it.
func runHoldHelper(drop bool) int {
	lock, err := Acquire(context.Background(), os.Getenv(envHelperLock), cmdUpdate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helper could not acquire: %v\n", err)
		return 1
	}
	if drop {
		lock = nil
		runtime.GC()
		runtime.GC()
	}
	if err := os.WriteFile(os.Getenv(envHelperReady), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil { //nolint:gosec // G703: the path comes from the parent test process, not a user.
		fmt.Fprintf(os.Stderr, "helper could not announce readiness: %v\n", err)
		return 1
	}

	// Held until the parent kills it, which is the point. A blocking receive would
	// be tidier and is not usable: the runtime would call it a deadlock, kill the
	// process, and hand the lock straight back to the test that is asserting it
	// cannot have it.
	time.Sleep(10 * time.Minute)
	runtime.KeepAlive(lock)
	return 0
}

// runContendHelper repeatedly takes the lock and performs a deliberately
// non-atomic read-modify-write on a shared counter.
//
// The read, the pause and the write are three separate operations on one file,
// so two processes running them at once lose an increment with near certainty.
// A final count equal to the number of increments attempted is therefore a
// statement about mutual exclusion, not about the counter.
func runContendHelper() int {
	rounds, err := strconv.Atoi(os.Getenv(envHelperRounds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "helper got an unusable round count: %v\n", err)
		return 1
	}
	counter := os.Getenv(envHelperCounter)
	for range rounds {
		lock, err := Acquire(context.Background(), os.Getenv(envHelperLock), cmdUpdate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "helper could not acquire: %v\n", err)
			return 1
		}
		if err := bumpCounter(counter); err != nil {
			_ = lock.Release()
			fmt.Fprintf(os.Stderr, "helper could not bump the counter: %v\n", err)
			return 1
		}
		if err := lock.Release(); err != nil {
			fmt.Fprintf(os.Stderr, "helper could not release: %v\n", err)
			return 1
		}
	}
	return 0
}

// runAcquireSetHelper takes a whole set of locks, gives it straight back, and
// does it again, so two children doing the same to one pair of resources spend
// as much time as possible overlapping.
//
// It announces readiness before the first round rather than after, because what
// the parent waits for here is a running process, not a held lock: the point is
// that both are trying at once.
func runAcquireSetHelper() int {
	rounds, err := strconv.Atoi(os.Getenv(envHelperRounds))
	if err != nil {
		fmt.Fprintf(os.Stderr, "helper got an unusable round count: %v\n", err)
		return 1
	}
	paths := filepath.SplitList(os.Getenv(envHelperSet))
	if err := os.WriteFile(os.Getenv(envHelperReady), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil { //nolint:gosec // G703: the path comes from the parent test process, not a user.
		fmt.Fprintf(os.Stderr, "helper could not announce readiness: %v\n", err)
		return 1
	}
	for range rounds {
		held, err := AcquireAll(context.Background(), cmdUpdate, paths...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "helper could not acquire %v: %v\n", paths, err)
			return 1
		}
		if err := held.Release(); err != nil {
			fmt.Fprintf(os.Stderr, "helper could not release %v: %v\n", paths, err)
			return 1
		}
	}
	return 0
}

// bumpCounter is the unsafe increment the contending helpers race on.
func bumpCounter(path string) error {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return err
	}
	// The window a lost update would fall into. Without it the three operations
	// are fast enough that a broken lock might still produce the right answer.
	time.Sleep(2 * time.Millisecond)
	//nolint:gosec // G703: the path comes from the parent test process, not a user.
	return os.WriteFile(path, []byte(strconv.Itoa(n+1)), 0o600)
}

// startHolder launches a child process that takes the lock at path and holds it
// until it is killed, returning once the child actually has it.
func startHolder(t *testing.T, path string) *exec.Cmd {
	t.Helper()
	return startHolderAs(t, path, roleHold)
}

// startHolderAs is startHolder with the holder's role chosen by the caller.
func startHolderAs(t *testing.T, path, role string) *exec.Cmd {
	t.Helper()

	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.CommandContext(t.Context(), os.Args[0]) //nolint:gosec // this test binary, re-executed
	cmd.Env = append(os.Environ(),
		envHelperRole+"="+role,
		envHelperLock+"="+path,
		envHelperReady+"="+ready,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start the lock holder: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	waitForFile(t, ready)
	return cmd
}

// waitForFile blocks until path exists, failing the test if it never does.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
}

// shortWait makes the acquisition timeout small enough that a test asserting a
// refusal finishes in milliseconds.
func shortWait(t *testing.T) {
	t.Helper()
	t.Setenv(envWait, "200ms")
}

// -----------------------------------------------------------------------------
// Cross-process exclusion: what the package exists for.
// -----------------------------------------------------------------------------

// TestAcquire_refusesALockHeldByAnotherProcess is the central guarantee. A
// second process must be told the resource is taken rather than allowed to
// proceed alongside the first.
func TestAcquire_refusesALockHeldByAnotherProcess(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	holder := startHolder(t, path)

	lock, err := Acquire(t.Context(), path, "remove")
	if err == nil {
		_ = lock.Release()
		t.Fatal("Acquire() succeeded while another process held the lock")
	}
	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("Acquire() error = %v, want *BusyError", err)
	}
	// The message names the process actually holding it, not some other one.
	if busy.Owner.PID != holder.Process.Pid {
		t.Errorf("BusyError names pid %d, want the holder's %d", busy.Owner.PID, holder.Process.Pid)
	}
	if busy.Owner.Command != cmdUpdate {
		t.Errorf("BusyError names command %q, want %q", busy.Owner.Command, cmdUpdate)
	}
}

// TestAcquire_takesTheLockAsSoonAsAKilledHolderIsGone covers the crash case,
// which is the one every lock-file scheme gets wrong. A SIGKILLed process runs
// no cleanup, so its lock FILE is still there with a complete owner record in
// it; the kernel dropped the lock as it reaped the process, so the next gup must
// walk straight in - with no staleness bound to wait out and nothing to delete.
func TestAcquire_takesTheLockAsSoonAsAKilledHolderIsGone(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	holder := startHolder(t, path)

	if err := holder.Process.Kill(); err != nil {
		t.Fatalf("failed to kill the holder: %v", err)
	}
	if _, err := holder.Process.Wait(); err != nil {
		t.Fatalf("failed to reap the holder: %v", err)
	}
	// The file the dead process left behind, owner record and all.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the killed holder's lock file is gone: %v", err)
	}

	lock, err := Acquire(t.Context(), path, "remove")
	if err != nil {
		t.Fatalf("Acquire() after the holder was killed = %v, want success", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquire_ignoresALockFileNobodyHolds states the same thing from the other
// side: a lock file is not a lock. One left over from any earlier run, however
// convincing its contents, holds nothing - so nothing about it needs to be
// judged, aged out, or removed by a user.
func TestAcquire_ignoresALockFileNobodyHolds(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	plantOwnerRecord(t, path, Owner{
		PID:      os.Getpid(), // a PID that is very much alive
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
	})

	start := time.Now()
	lock, err := Acquire(t.Context(), path, "remove")
	if err != nil {
		t.Fatalf("Acquire() over an unheld lock file = %v, want success", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Acquire() waited %v over a file nobody holds, want no wait", elapsed)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquire_neverLetsTwoProcessesInAtOnce runs several processes through a
// read-modify-write that loses an increment whenever two of them overlap. The
// lock is what makes the arithmetic come out.
func TestAcquire_neverLetsTwoProcessesInAtOnce(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	const (
		children = 4
		rounds   = 5
	)
	dir := t.TempDir()
	path := filepath.Join(dir, ".gup.lock")
	counter := filepath.Join(dir, "counter")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatalf("failed to seed the counter: %v", err)
	}

	procs := make([]*exec.Cmd, 0, children)
	for range children {
		cmd := exec.CommandContext(t.Context(), os.Args[0]) //nolint:gosec // this test binary, re-executed
		cmd.Env = append(os.Environ(),
			envHelperRole+"="+roleContend,
			envHelperLock+"="+path,
			envHelperCounter+"="+counter,
			envHelperRounds+"="+strconv.Itoa(rounds),
			// Long enough that a child waiting its turn behind the others never
			// gives up: this test is about exclusion, not about the timeout.
			envWait+"=60s",
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start a contender: %v", err)
		}
		procs = append(procs, cmd)
	}
	for i, cmd := range procs {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("contender %d failed: %v", i, err)
		}
	}

	raw, err := os.ReadFile(filepath.Clean(counter))
	if err != nil {
		t.Fatalf("failed to read the counter: %v", err)
	}
	got, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("the counter is not a number: %v", err)
	}
	if want := children * rounds; got != want {
		t.Errorf("counter = %d, want %d: %d increments were lost to overlapping processes", got, want, want-got)
	}
}

// TestRelease_handsTheLockToTheNextProcess covers the ordinary handover: a
// released lock is available at once, without the file having been deleted.
func TestRelease_handsTheLockToTheNextProcess(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	holder := startHolder(t, path)

	// Refused while held...
	if _, err := Acquire(t.Context(), path, "remove"); err == nil {
		t.Fatal("Acquire() succeeded while another process held the lock")
	}

	if err := holder.Process.Kill(); err != nil {
		t.Fatalf("failed to kill the holder: %v", err)
	}
	if _, err := holder.Process.Wait(); err != nil {
		t.Fatalf("failed to reap the holder: %v", err)
	}

	// ...and granted once it is not.
	lock, err := Acquire(t.Context(), path, "remove")
	if err != nil {
		t.Fatalf("Acquire() after the holder let go = %v, want success", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the lock file was deleted on release: %v", err)
	}
}

// -----------------------------------------------------------------------------
// The lock file itself.
// -----------------------------------------------------------------------------

// TestPathFor pins where a lock lives relative to what it guards. The lock has
// to travel with the resource: a lock derived from gup's config directory would
// not serialize two processes that share a $GOBIN but were started with
// different XDG_CONFIG_HOME values.
func TestPathFor(t *testing.T) {
	t.Parallel()

	if got, want := PathForDir(filepath.Join("home", "bin")), filepath.Join("home", "bin", ".gup.lock"); got != want {
		t.Errorf("PathForDir() = %q, want %q", got, want)
	}
	// The dot prefix is what keeps the lock out of gup's own $GOBIN listing.
	if !strings.HasPrefix(filepath.Base(PathForDir("bin")), ".") {
		t.Error("the directory lock is not dot-prefixed, so it would show up as an installed binary")
	}
	// The file lock is derived from the name the operating system agrees the file
	// has, so it comes back absolute even when the caller spelled it relatively -
	// two processes in different working directories must not derive two locks for
	// one file.
	config := filepath.Join(t.TempDir(), "gup.json")
	got, err := PathForFile(config)
	if err != nil {
		t.Fatalf("PathForFile() error: %v", err)
	}
	if want := filepath.Base(config) + ".lock"; filepath.Base(got) != want {
		t.Errorf("PathForFile() = %q, want a %q beside the file", got, want)
	}
	if dir := filepath.Dir(got); !strings.EqualFold(filepath.Base(dir), filepath.Base(filepath.Dir(config))) {
		t.Errorf("PathForFile() = %q, want the lock in the file's own directory %q", got, filepath.Dir(config))
	}
}

// TestIsReservedName covers the name `gup remove` must refuse. The lock lives in
// $GOBIN, and `gup remove` deletes from $GOBIN by name, so the name has to be
// reserved rather than merely hidden.
func TestIsReservedName(t *testing.T) {
	t.Parallel()

	reserved := []string{
		".gup.lock", " .gup.lock ", ".GUP.LOCK", ".Gup.Lock",
		// Win32 strips trailing dots and spaces before the filesystem ever sees
		// the name, so every one of these opens the file above. Folding them the
		// same way on every platform keeps the rule one rule.
		".gup.lock.", ".gup.lock...", ".gup.lock ", ".gup.lock . .", ".GUP.LOCK.",
	}
	for _, name := range reserved {
		if !IsReservedName(name) {
			t.Errorf("IsReservedName(%q) = false, want true", name)
		}
	}
	allowed := []string{"gup.lock", "gup", "gopls", "", ".gup", ".gup.lock.bak", "gup.json.lock"}
	for _, name := range allowed {
		if IsReservedName(name) {
			t.Errorf("IsReservedName(%q) = true, want false", name)
		}
	}
	// The reserved name is the one PathForDir actually creates - not a constant
	// that could drift away from it.
	if !IsReservedName(filepath.Base(PathForDir("bin"))) {
		t.Error("the file PathForDir names is not reserved")
	}
}

// TestAcquire_recordsTheHolder covers the descriptive half of the lock file: it
// says who has it, so a waiting process can name them.
func TestAcquire_recordsTheHolder(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() = %v, want success", err)
	}
	if lock.Path() != path {
		t.Errorf("Path() = %q, want %q", lock.Path(), path)
	}
	owner := readPlantedOwner(t, path)
	if owner.PID != os.Getpid() {
		t.Errorf("recorded pid = %d, want %d", owner.PID, os.Getpid())
	}
	if owner.Command != cmdUpdate {
		t.Errorf("recorded command = %q, want %q", owner.Command, cmdUpdate)
	}
	if owner.Host != hostname(t) {
		t.Errorf("recorded host = %q, want %q", owner.Host, hostname(t))
	}
	if owner.Acquired.IsZero() {
		t.Error("recorded acquisition time is zero")
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() = %v, want nil", err)
	}
	// The record goes when the lock does, so no waiter can ever read one naming a
	// process that has already let go.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the lock file was deleted on release: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("the released lock file still holds %d bytes, want an empty file", info.Size())
	}
}

// TestAcquire_toleratesAnUnreadableOwnerRecord covers a lock file somebody
// filled with rubbish. The verdict comes from the kernel, so the only thing a
// corrupt record can cost is detail in the message.
func TestAcquire_toleratesAnUnreadableOwnerRecord(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	if err := os.WriteFile(path, []byte("{this is not json"), lockFileMode); err != nil {
		t.Fatalf("failed to plant the corrupt lock file: %v", err)
	}

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() over a corrupt lock file = %v, want success", err)
	}
	// It was overwritten with a usable record rather than appended to.
	if owner := readPlantedOwner(t, path); owner.PID != os.Getpid() {
		t.Errorf("recorded pid = %d, want %d", owner.PID, os.Getpid())
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquire_createsParentDirectory covers a $GOBIN that does not exist yet -
// the first run of every command, and exactly when two processes are most likely
// to collide.
func TestAcquire_createsParentDirectory(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), "not", "there", "yet", ".gup.lock")

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() into a missing directory = %v, want success", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the lock file was not created: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquire_rejectsADirectoryLockPath fails with a clear message rather than
// whatever the operating system says about opening a directory for writing.
func TestAcquire_rejectsADirectoryLockPath(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()

	if _, err := Acquire(t.Context(), dir, cmdUpdate); err == nil {
		t.Fatal("Acquire() on a directory succeeded, want an error")
	} else if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("Acquire() error = %v, want it to say the path is a directory", err)
	}
}

// TestRelease_isIdempotent covers a caller that defers Release and also releases
// early, which is the normal shape of the code that uses this.
func TestRelease_isIdempotent(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() = %v, want success", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("first Release() = %v, want nil", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("second Release() = %v, want nil", err)
	}
	// A nil lock releases cleanly too, so a failed acquisition's defer is safe.
	var missing *Lock
	if err := missing.Release(); err != nil {
		t.Errorf("(*Lock)(nil).Release() = %v, want nil", err)
	}
}

// -----------------------------------------------------------------------------
// Waiting, cancellation, and in-process serialization.
// -----------------------------------------------------------------------------

// TestAcquire_serializesGoroutinesInOneProcess covers two acquisitions inside
// one gup. The kernel lock is taken per descriptor, so it would report the
// second as a foreign conflict; the in-process registry is what makes it wait
// instead.
func TestAcquire_serializesGoroutinesInOneProcess(t *testing.T) {
	t.Setenv(envWait, "10s")
	path := filepath.Join(t.TempDir(), ".gup.lock")

	var concurrent, peak atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := Acquire(context.Background(), path, cmdUpdate)
			if err != nil {
				t.Errorf("Acquire() = %v, want success", err)
				return
			}
			defer func() {
				concurrent.Add(-1)
				if err := lock.Release(); err != nil {
					t.Errorf("Release() = %v, want nil", err)
				}
			}()
			if now := concurrent.Add(1); now > peak.Load() {
				peak.Store(now)
			}
			time.Sleep(2 * time.Millisecond)
		}()
	}
	wg.Wait()

	if peak.Load() != 1 {
		t.Errorf("%d goroutines held the lock at once, want 1", peak.Load())
	}
}

// TestAcquire_honorsContextCancellationWhileWaiting covers Ctrl-C arriving while
// a command queues behind another one: it must return then, not at the end of
// the wait.
func TestAcquire_honorsContextCancellationWhileWaiting(t *testing.T) {
	t.Setenv(envWait, "60s")
	path := filepath.Join(t.TempDir(), ".gup.lock")
	startHolder(t, path)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := Acquire(ctx, path, "remove"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Errorf("Acquire() returned after %v, want it to stop when the context was canceled", elapsed)
	}
}

// TestAcquire_honorsAnAlreadyCancelledContext covers the same thing before the
// first attempt, so a canceled command creates no lock file at all.
func TestAcquire_honorsAnAlreadyCancelledContext(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := Acquire(ctx, path, cmdUpdate); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire() error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a canceled acquisition left %s behind", path)
	}
}

// TestWaitTimeout covers the override the end-to-end suite drives contention
// with, and the fallback that keeps a typo in it from breaking the lock.
func TestWaitTimeout(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "unset", value: "", want: defaultWait},
		{name: "override", value: "150ms", want: 150 * time.Millisecond},
		{name: "unparseable falls back", value: "soon", want: defaultWait},
		{name: "zero falls back", value: "0s", want: defaultWait},
		{name: "negative falls back", value: "-1s", want: defaultWait},
		{name: "surrounding space is trimmed", value: "  2s  ", want: 2 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envWait, tt.value)
			if got := waitTimeout(); got != tt.want {
				t.Errorf("waitTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Taking several locks at once.
// -----------------------------------------------------------------------------

// TestAcquireAll_takesEverythingOrNothing covers the rollback: a set that could
// not be taken whole must leave nothing held, or the next command waits on a
// resource nobody is using.
//
// The held one has to be the one taken LAST, or the acquisition fails before it
// has anything to roll back. Which that is comes from AcquisitionOrder rather
// than from how the paths sort: the order is the filesystem's (see AcquireAll).
func TestAcquireAll_takesEverythingOrNothing(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()
	first, second := lockOrder(t, filepath.Join(dir, "a.lock"), filepath.Join(dir, "b.lock"))
	startHolder(t, second)

	if _, err := AcquireAll(t.Context(), cmdUpdate, first, second); err == nil {
		t.Fatal("AcquireAll() succeeded although one lock was held")
	}
	// The one it did take was handed back, so this succeeds.
	held, err := AcquireAll(t.Context(), cmdUpdate, first)
	if err != nil {
		t.Fatalf("AcquireAll() after a rolled-back set = %v, want success", err)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquireAll_ordersAndDeduplicates covers the two properties that make
// deadlock impossible: one file is one lock, and the same set is always taken in
// the same order however the caller ordered it.
//
// The order itself is deliberately not asserted against a literal. It is the
// order of the files' identities, which the filesystem hands out and nothing can
// predict - and predicting it is not what makes the lock safe. Two processes
// AGREEING on it is, so that is what this asserts: the same set, given in two
// different orders, comes back in one.
func TestAcquireAll_ordersAndDeduplicates(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.lock")
	b := filepath.Join(dir, "b.lock")

	first := acquireAllPaths(t, b, a, b)
	if len(first) != 2 {
		t.Fatalf("Paths() = %v, want the two distinct files (b was named twice)", first)
	}
	sorted := append([]string(nil), first...)
	slices.Sort(sorted)
	if want := []string{a, b}; !slicesEqual(sorted, want) {
		t.Errorf("locked %v, want exactly %v", sorted, want)
	}

	second := acquireAllPaths(t, a, b)
	if !slicesEqual(first, second) {
		t.Errorf("the same set was taken as %v and then as %v; two processes ordering it differently deadlock",
			first, second)
	}
}

// acquireAllPaths takes a set of locks, reports the order they were taken in,
// and gives them back.
func acquireAllPaths(t *testing.T, paths ...string) []string {
	t.Helper()
	held, err := AcquireAll(t.Context(), cmdUpdate, paths...)
	if err != nil {
		t.Fatalf("AcquireAll(%v) = %v, want success", paths, err)
	}
	order := held.Paths()
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
	return order
}

// TestAcquireAll_withNoPathsIsANoOp covers a command that changes nothing: it
// contends for nothing rather than inventing a resource to lock.
func TestAcquireAll_withNoPathsIsANoOp(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)

	held, err := AcquireAll(t.Context(), cmdUpdate)
	if err != nil {
		t.Fatalf("AcquireAll() with no paths = %v, want success", err)
	}
	if got := held.Paths(); len(got) != 0 {
		t.Errorf("Paths() = %v, want empty", got)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
	// A nil MultiLock releases cleanly, so a failed AcquireAll's defer is safe.
	var missing *MultiLock
	if got := missing.Paths(); got != nil {
		t.Errorf("(*MultiLock)(nil).Paths() = %v, want nil", got)
	}
	if err := missing.Release(); err != nil {
		t.Errorf("(*MultiLock)(nil).Release() = %v, want nil", err)
	}
}

// TestAcquireAll_normalizesBeforeDeduplicating covers two spellings of one path.
// Acquire normalizes before keying the in-process registry, so if AcquireAll
// deduplicated the raw strings the two would take separate slots for one file
// and the second would wait out the whole timeout against the first.
func TestAcquireAll_normalizesBeforeDeduplicating(t *testing.T) { //nolint:paralleltest // changes the working directory
	shortWait(t)
	dir := t.TempDir()
	t.Chdir(dir)

	start := time.Now()
	held, err := AcquireAll(t.Context(), cmdUpdate, "gup.json.lock", filepath.Join(dir, "gup.json.lock"))
	if err != nil {
		t.Fatalf("AcquireAll() = %v, want success", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("AcquireAll() took %v: the two spellings contended against each other", elapsed)
	}
	if got := held.Paths(); len(got) != 1 {
		t.Errorf("Paths() = %v, want one entry", got)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestNormalizePath covers the key the in-process registry uses: two spellings
// of one path must produce one key, or a lock does not lock.
func TestNormalizePath(t *testing.T) { //nolint:paralleltest // changes the working directory
	dir := t.TempDir()
	t.Chdir(dir)

	relative, err := normalizePath(filepath.Join(".", "gup.json.lock"))
	if err != nil {
		t.Fatalf("normalizePath() = %v, want success", err)
	}
	absolute, err := normalizePath(filepath.Join(dir, "gup.json.lock"))
	if err != nil {
		t.Fatalf("normalizePath() = %v, want success", err)
	}
	if relative != absolute {
		t.Errorf("normalizePath() gave %q and %q for one path", relative, absolute)
	}
}

// -----------------------------------------------------------------------------
// Messages.
// -----------------------------------------------------------------------------

// TestBusyError_namesTheOtherProcess covers the message a user actually sees
// when two commands overlap.
func TestBusyError_namesTheOtherProcess(t *testing.T) {
	t.Parallel()

	err := &BusyError{
		Path: testLockPath,
		Owner: Owner{
			PID:      4242,
			Host:     "some-machine",
			Command:  cmdUpdate,
			Acquired: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}
	msg := err.Error()
	for _, want := range []string{
		"another gup process is already running",
		"pid 4242",
		"some-machine",
		`"gup update"`,
		"2026-01-02T03:04:05Z",
		testLockPath,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("BusyError message %q does not mention %q", msg, want)
		}
	}
	// It must never send a user to delete the lock file: the lock is the kernel's,
	// so deleting the file while the other process runs buys them the overlap the
	// lock exists to prevent.
	for _, forbidden := range []string{"delete it by hand", "remove the lock file", "rm "} {
		if strings.Contains(strings.ToLower(msg), forbidden) {
			t.Errorf("BusyError message %q tells the user to delete the lock file", msg)
		}
	}
}

// TestBusyError_degradesWithoutOwnerDetails covers a lock file whose record
// could not be read. The refusal came from the kernel, so the message must still
// be a refusal - just a thinner one.
func TestBusyError_degradesWithoutOwnerDetails(t *testing.T) {
	t.Parallel()

	msg := (&BusyError{Path: testLockPath}).Error()
	if !strings.Contains(msg, "another gup process is already running") {
		t.Errorf("BusyError message %q does not report the conflict", msg)
	}
	if strings.Contains(msg, "pid") {
		t.Errorf("BusyError message %q invents a pid it does not have", msg)
	}
	if !strings.Contains(msg, testLockPath) {
		t.Errorf("BusyError message %q does not name the lock file", msg)
	}
}

// -----------------------------------------------------------------------------
// Helpers.
// -----------------------------------------------------------------------------

// plantOwnerRecord writes a lock file's descriptive record without taking the
// lock, standing in for the file a dead process leaves behind.
func plantOwnerRecord(t *testing.T, path string, owner Owner) {
	t.Helper()
	raw, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("failed to marshal the owner record: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("failed to create the lock directory: %v", err)
	}
	if err := os.WriteFile(path, raw, lockFileMode); err != nil {
		t.Fatalf("failed to write the lock file: %v", err)
	}
}

// readPlantedOwner reads a lock file's record by path, failing the test if it
// cannot be parsed.
func readPlantedOwner(t *testing.T, path string) Owner {
	t.Helper()
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	owner := readOwner(file)
	if owner.PID == 0 {
		t.Fatalf("no owner record could be read from %s", path)
	}
	return owner
}

// hostname returns this machine's name, which the owner record carries.
func hostname(t *testing.T) string {
	t.Helper()
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("failed to read the hostname: %v", err)
	}
	return host
}

// slicesEqual reports whether two string slices have the same contents in the
// same order.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestAcquire_timesOutAgainstAnotherOperationInThisProcess covers a caller that
// asks for a lock this gup already holds and never gives back. Waiting forever
// would turn a programming mistake into a hang with no diagnosis; the message
// says where the lock actually is.
func TestAcquire_timesOutAgainstAnotherOperationInThisProcess(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")

	held, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() = %v, want success", err)
	}
	defer func() { _ = held.Release() }()

	_, err = Acquire(t.Context(), path, "remove")
	if err == nil {
		t.Fatal("a second Acquire() in this process succeeded, want a timeout")
	}
	if !strings.Contains(err.Error(), "another operation in this gup process") {
		t.Errorf("Acquire() error = %v, want it to name the in-process holder", err)
	}
	// It must not be reported as a foreign conflict: telling a user another gup is
	// running when it is this one sends them looking for a process that is not
	// there.
	var busy *BusyError
	if errors.As(err, &busy) {
		t.Error("an in-process conflict was reported as another gup process")
	}
}

// TestAcquire_reportsAPathItCanNotCreate covers a lock whose parent cannot be a
// directory, which is what a $GOBIN with a regular file in its way looks like.
func TestAcquire_reportsAPathItCanNotCreate(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("in the way"), 0o600); err != nil {
		t.Fatalf("failed to write the blocking file: %v", err)
	}

	if _, err := Acquire(t.Context(), filepath.Join(blocker, ".gup.lock"), cmdUpdate); err == nil {
		t.Fatal("Acquire() under a regular file succeeded, want an error")
	} else if !strings.Contains(err.Error(), "can not create directory") {
		t.Errorf("Acquire() error = %v, want it to name the directory it could not create", err)
	}
}

// TestReadOwner covers what the "who is running" message is built from,
// including the inputs that leave it with nothing to say. None of these can
// change the verdict - the kernel has already given that - so each one must
// degrade rather than fail.
func TestReadOwner(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
		wantPID int
	}{
		{name: "a complete record", content: `{"pid":77,"host":"h","command":"update"}`, wantPID: 77},
		{name: "an empty file", content: "", wantPID: 0},
		{name: "not json at all", content: "{truncated", wantPID: 0},
		{name: "json of the wrong shape", content: `["update"]`, wantPID: 0},
		{name: "longer than the bound", content: `{"pid":88,"command":"` + strings.Repeat("x", ownerRecordLimit) + `"}`, wantPID: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, strings.ReplaceAll(tt.name, " ", "-")+".lock")
			if err := os.WriteFile(path, []byte(tt.content), lockFileMode); err != nil {
				t.Fatalf("failed to write %s: %v", path, err)
			}
			file, err := os.Open(filepath.Clean(path))
			if err != nil {
				t.Fatalf("failed to open %s: %v", path, err)
			}
			defer func() { _ = file.Close() }()

			if got := readOwner(file).PID; got != tt.wantPID {
				t.Errorf("readOwner().PID = %d, want %d", got, tt.wantPID)
			}
		})
	}
}

// TestAcquire_keepsHoldingALockTheCallerDropped is the regression test for a
// hazard the package's own helpers walked into.
//
// The kernel lock lives on a descriptor inside the returned Lock, and os.File
// closes itself from a finalizer once it becomes unreachable. So a caller that
// takes a lock and then drops the value - a long-running holder that never
// means to release it - would have the lock released for it at the next garbage
// collection, with nothing anywhere reporting that it had happened. Holding a
// lock has to survive the caller forgetting about it, because "held until you
// release it or the process ends" is the whole contract.
func TestAcquire_keepsHoldingALockTheCallerDropped(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	startHolderAs(t, path, roleHoldDropped)

	lock, err := Acquire(t.Context(), path, "remove")
	if err == nil {
		_ = lock.Release()
		t.Fatal("Acquire() succeeded: the holder's lock was collected out from under it")
	}
	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("Acquire() error = %v, want *BusyError", err)
	}
}
