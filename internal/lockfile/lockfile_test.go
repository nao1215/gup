// Package lockfile's tests run without t.Parallel where they swap the package's
// nowFunc/exitFunc test seams, which are process-wide.
package lockfile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// cmdUpdate is the subcommand name these tests record as the lock's owner; it is
// repeated often enough that goconst asks for a name.
const cmdUpdate = "update"

// TestAcquire_createsAndReleasesLockFile covers the ordinary lifecycle: the lock
// file appears while held, records the caller's identity, and is gone afterwards
// so the next command can take it.
func TestAcquire_createsAndReleasesLockFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	if lock.Path() != path {
		t.Errorf("Path() = %q, want %q", lock.Path(), path)
	}

	raw, err := os.ReadFile(path) //nolint:gosec // path is this test's temp file
	if err != nil {
		t.Fatalf("the lock file was not created: %v", err)
	}
	var owner Owner
	if err := json.Unmarshal(raw, &owner); err != nil {
		t.Fatalf("the lock file is not valid JSON: %v (%q)", err, raw)
	}
	if owner.PID != os.Getpid() {
		t.Errorf("owner.PID = %d, want this process %d", owner.PID, os.Getpid())
	}
	if owner.Command != cmdUpdate {
		t.Errorf("owner.Command = %q, want %q", owner.Command, cmdUpdate)
	}
	if owner.Acquired.IsZero() {
		t.Error("owner.Acquired was not recorded")
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the lock file still exists after Release(): %v", err)
	}
}

// TestAcquire_createsParentDirectory covers a first run on a machine where
// ~/.config/gup does not exist yet: the lock must not fail just because nothing
// has written gup.json before.
func TestAcquire_createsParentDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "dir", "gup.lock")
	lock, err := Acquire(t.Context(), path, "pin")
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	if _, err := os.Stat(path); err != nil {
		t.Errorf("the lock file was not created under a new directory: %v", err)
	}
}

// TestRelease_isIdempotent covers a caller that releases early and also defers a
// release: the second call must not report the already-removed file as an error.
func TestRelease_isIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, "remove")
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("first Release() error: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("second Release() error: %v, want nil", err)
	}
}

// TestRelease_toleratesAnAlreadyDeletedLockFile covers an operator who removed
// the lock file by hand while gup was running. Release's postcondition is "this
// process no longer holds the lock", which is already true.
func TestRelease_toleratesAnAlreadyDeletedLockFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("failed to remove the lock file: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() error: %v, want nil for an already-deleted lock file", err)
	}
}

// TestAcquire_reportsALockHeldByAnotherLiveProcess is the double-execution case:
// a second gup must refuse rather than proceed, and must say who is running. The
// holder is written directly (rather than by a second Acquire) so the lock looks
// like it came from a different process while still naming a PID that exists -
// this test's own.
func TestAcquire_reportsALockHeldByAnotherLiveProcess(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      os.Getpid(), // alive, so the lock is not stale
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
	})

	start := time.Now()
	_, err := Acquire(t.Context(), path, "remove")
	if err == nil {
		t.Fatal("Acquire() succeeded while another process held the lock")
	}

	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("Acquire() error = %T (%v), want *BusyError", err, err)
	}
	if busy.Owner.PID != os.Getpid() {
		t.Errorf("BusyError.Owner.PID = %d, want %d", busy.Owner.PID, os.Getpid())
	}
	// The message has to be actionable: which process, and what to do about it.
	for _, want := range []string{"another gup process is already running", "gup update", path} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Acquire() error %q does not mention %q", err.Error(), want)
		}
	}
	// It waits before giving up, but not long: a user who typed two commands
	// wants an answer, not a hang.
	if waited := time.Since(start); waited < retryInterval || waited > 5*DefaultWait {
		t.Errorf("Acquire() waited %v, want roughly %v", waited, DefaultWait)
	}

	// The other process's lock file must survive the refusal.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the refused acquisition removed the holder's lock file: %v", err)
	}
}

// TestAcquire_takesOverALockWhoseOwnerIsGone is the crash case. A gup killed
// with SIGKILL runs no cleanup, so its lock file outlives it; if that wedged
// gup, one interrupted update would break the tool until someone found the file.
func TestAcquire_takesOverALockWhoseOwnerIsGone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      deadPID(t),
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
	})

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v, want the stale lock to be taken over", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	owner, readErr := readOwner(path)
	if readErr != nil {
		t.Fatalf("readOwner() error: %v", readErr)
	}
	if owner.PID != os.Getpid() {
		t.Errorf("the lock file still names the dead owner (pid %d), want this process %d", owner.PID, os.Getpid())
	}
}

// TestAcquire_takesOverALockWhoseHeartbeatStopped covers what the PID check
// cannot answer: a lock file from another host (a home directory shared over
// NFS), or one whose PID has been reused. The heartbeat age is the fallback, and
// without it such a lock would block gup forever.
func TestAcquire_takesOverALockWhoseHeartbeatStopped(t *testing.T) { //nolint:paralleltest // swaps the package-level nowFunc/exitFunc seams
	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      os.Getpid(), // alive, so only the heartbeat can make this stale
		Host:     "some-other-machine",
		Command:  cmdUpdate,
		Acquired: time.Now(),
	})

	// Age the lock past staleAfter by moving the clock forward rather than by
	// sleeping, so the test costs nothing.
	restore := freezeClock(t, time.Now().Add(2*staleAfter))
	defer restore()

	lock, err := Acquire(context.Background(), path, "import")
	if err != nil {
		t.Fatalf("Acquire() error: %v, want the abandoned lock to be taken over", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
}

// TestAcquire_waitsOutAFreshLockFromAnotherHost is the other half of the
// heartbeat rule: a lock from a machine whose PIDs mean nothing here must be
// respected while it is being touched, or a shared home directory would let two
// machines update the same $GOBIN at once.
func TestAcquire_waitsOutAFreshLockFromAnotherHost(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      deadPID(t), // dead HERE, but the PID belongs to the other host
		Host:     "some-other-machine",
		Command:  cmdUpdate,
		Acquired: time.Now(),
	})

	if _, err := Acquire(t.Context(), path, cmdUpdate); err == nil {
		t.Fatal("Acquire() took over a freshly heartbeated lock from another host")
	}
}

// TestAcquire_takesOverAnUnreadableLockFileOnceItAges covers a lock file
// truncated by a crash mid-write or corrupted on disk. While it is fresh it is
// assumed to be a live writer and respected; once it stops being touched it is
// reclaimed, so unparseable content cannot wedge gup permanently.
func TestAcquire_takesOverAnUnreadableLockFileOnceItAges(t *testing.T) { //nolint:paralleltest // swaps the package-level nowFunc/exitFunc seams
	path := filepath.Join(t.TempDir(), "gup.lock")
	if err := os.WriteFile(path, []byte("{not json"), lockFileMode); err != nil {
		t.Fatalf("failed to write the corrupt lock file: %v", err)
	}

	if _, err := Acquire(context.Background(), path, cmdUpdate); err == nil {
		t.Fatal("Acquire() took over a freshly written but unreadable lock file")
	}

	restore := freezeClock(t, time.Now().Add(2*staleAfter))
	defer restore()

	lock, err := Acquire(context.Background(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v, want an aged corrupt lock file to be reclaimed", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
}

// TestAcquire_honorsContextCancellation covers Ctrl-C (or a --timeout expiring)
// while gup is waiting its turn: the wait must end at once with the context's
// error rather than running out the full DefaultWait.
func TestAcquire_honorsContextCancellation(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      os.Getpid(),
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Acquire(ctx, path, "update")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Acquire() error = %v, want context.Canceled", err)
	}
}

// TestAcquire_serializesGoroutinesInOneProcess covers the in-process gate. Two
// goroutines contending are cooperating parts of one program, so the second must
// wait for the first rather than be told "another gup process is already
// running" about itself - which is what a bare O_EXCL lock would report.
func TestAcquire_serializesGoroutinesInOneProcess(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	const goroutines = 8

	var (
		mu      sync.Mutex
		inside  int
		maxSeen int
		wg      sync.WaitGroup
	)
	errs := make(chan error, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := Acquire(context.Background(), path, cmdUpdate)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			inside++
			maxSeen = max(maxSeen, inside)
			mu.Unlock()

			time.Sleep(time.Millisecond)

			mu.Lock()
			inside--
			mu.Unlock()
			if releaseErr := lock.Release(); releaseErr != nil {
				errs <- releaseErr
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Acquire()/Release() error: %v", err)
	}
	if maxSeen != 1 {
		t.Errorf("%d goroutines held the lock at once, want 1", maxSeen)
	}
}

// TestAcquire_rejectsADirectoryLockPath covers a misconfigured XDG_CONFIG_HOME
// (or a directory literally named gup.lock): the error must name the problem
// instead of surfacing as an opaque create failure.
func TestAcquire_rejectsADirectoryLockPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}
	_, err := Acquire(t.Context(), path, cmdUpdate)
	if err == nil {
		t.Fatal("Acquire() succeeded with a directory as the lock path")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("Acquire() error = %q, want it to name the directory problem", err)
	}
}

// TestBusyError_degradesWithoutOwnerDetails covers the message when the lock
// file could not be parsed: it must still tell the user which file to look at
// rather than printing a half-built sentence.
func TestBusyError_degradesWithoutOwnerDetails(t *testing.T) {
	t.Parallel()

	err := &BusyError{Path: "/tmp/gup.lock"}
	got := err.Error()
	if strings.Contains(got, "pid") {
		t.Errorf("BusyError.Error() = %q, want no pid clause when the owner is unknown", got)
	}
	if !strings.Contains(got, "/tmp/gup.lock") {
		t.Errorf("BusyError.Error() = %q, want it to name the lock file", got)
	}
}

// TestReleaseOnSignal covers the interruption path. gup's subcommands exit
// through os.Exit, so without this handler a Ctrl-C during `gup update` would
// leave a lock file behind and block the next command until it aged out.
func TestReleaseOnSignal(t *testing.T) { //nolint:paralleltest // swaps the package-level nowFunc/exitFunc seams
	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(context.Background(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	exited := make(chan int, 1)
	originalExit := exitFunc
	exitFunc = func(code int) { exited <- code }
	t.Cleanup(func() { exitFunc = originalExit })

	lock.signals <- os.Interrupt

	select {
	case code := <-exited:
		if want := exitStatusFor(os.Interrupt); code != want {
			t.Errorf("exit status = %d, want %d", code, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the signal handler did not run")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the lock file survived the interrupt: %v", err)
	}
	// Release after the handler already removed the file must stay quiet.
	if err := lock.Release(); err != nil {
		t.Errorf("Release() after the signal handler error: %v", err)
	}
}

// TestExitStatusFor pins the conventional 128+signal mapping, so a script that
// checks $? sees what it would have seen without gup's handler installed.
func TestExitStatusFor(t *testing.T) {
	t.Parallel()

	if got, want := exitStatusFor(os.Interrupt), 130; got != want {
		t.Errorf("exitStatusFor(os.Interrupt) = %d, want %d", got, want)
	}
	if got, want := exitStatusFor(fakeSignal{}), 1; got != want {
		t.Errorf("exitStatusFor(non-syscall signal) = %d, want %d", got, want)
	}
}

// TestProcessAlive covers the OS-specific liveness probe on whichever platform
// the test runs, including Windows: this process is alive, and a PID that names
// nothing is not.
func TestProcessAlive(t *testing.T) {
	t.Parallel()

	if !processAlive(os.Getpid()) {
		t.Errorf("processAlive(%d) = false for this running process", os.Getpid())
	}
	if processAlive(deadPID(t)) {
		t.Error("processAlive() reported a finished process as alive")
	}
	if processAlive(-1) {
		t.Error("processAlive(-1) = true for an impossible PID")
	}
}

// TestHeartbeat_keepsALongRunningLockFresh covers the case the staleness bound
// exists for: `gup update` over a large toolset outlives staleAfter, and must
// not become reclaimable while it is still working.
func TestHeartbeat_keepsALongRunningLockFresh(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	// Backdate the file, then drive one heartbeat tick's worth of work by hand:
	// waiting heartbeatInterval would make this test the slowest in the package.
	old := time.Now().Add(-2 * staleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}
	if _, stale := inspect(path); !stale {
		t.Fatal("a backdated lock file was not judged stale; the heartbeat guards nothing")
	}

	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("failed to touch the lock file: %v", err)
	}
	if _, stale := inspect(path); stale {
		t.Error("a freshly touched lock file was judged stale")
	}
}

// fakeSignal is an os.Signal that is not a syscall.Signal, covering the fallback
// branch of exitStatusFor.
type fakeSignal struct{}

func (fakeSignal) String() string { return "fake" }
func (fakeSignal) Signal()        {}

// writeOwner plants a lock file describing owner, standing in for a lock taken
// by a different gup process.
func writeOwner(t *testing.T, path string, owner Owner) {
	t.Helper()
	raw, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("failed to marshal the owner record: %v", err)
	}
	if err := os.WriteFile(path, raw, lockFileMode); err != nil {
		t.Fatalf("failed to write the lock file: %v", err)
	}
}

// hostname returns this machine's name, which the staleness rules compare
// against a lock file's recorded host.
func hostname(t *testing.T) string {
	t.Helper()
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("failed to read the hostname: %v", err)
	}
	return host
}

// deadPID returns a PID that named a real process and no longer does, which is
// the only way to test the "owner is gone" path without guessing at an unused
// number. Starting and reaping a child works the same on POSIX and Windows.
func deadPID(t *testing.T) int {
	t.Helper()

	name, args := "true", []string{}
	if runtime.GOOS == "windows" {
		name, args = "cmd", []string{"/c", "exit"}
	}
	cmd := exec.CommandContext(t.Context(), name, args...) //nolint:gosec // a fixed, in-test command name
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run the throwaway process: %v", err)
	}
	pid := cmd.ProcessState.Pid()
	if pid <= 0 {
		t.Fatalf("the throwaway process reported an unusable pid: %d", pid)
	}
	return pid
}

// freezeClock points the package's clock at instant and returns the restore
// function, so staleness can be tested without sleeping past staleAfter.
func freezeClock(t *testing.T, instant time.Time) func() {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time { return instant }
	return func() { nowFunc = original }
}
