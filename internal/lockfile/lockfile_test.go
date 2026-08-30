// Package lockfile's tests run without t.Parallel where they swap the package's
// nowFunc test seam or set environment variables, which are process-wide.
package lockfile

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
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
const (
	cmdUpdate = "update"
	// ownerHolder is the nonce these tests give a planted lock, and
	// testLockPath a path used only in message-formatting assertions.
	ownerHolder  = "holder"
	testLockPath = "/tmp/gup.lock"
	// cmdRemove is the subcommand a planted successor's lock records, and
	// remoteHost the machine a planted lock claims to have been written on -
	// one whose PIDs mean nothing here, which is what makes the heartbeat the
	// only thing that can decide about it.
	cmdRemove  = "remove"
	remoteHost = "some-other-machine"
	// successorNonce identifies the lock a planted successor holds, the file a
	// previous owner must never delete or refresh.
	successorNonce = "successor-nonce"
)

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
	if got, want := PathForFile(filepath.Join("cfg", "gup.json")), filepath.Join("cfg", "gup.json")+".lock"; got != want {
		t.Errorf("PathForFile() = %q, want %q", got, want)
	}
}

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

	owner := readOwnerForTest(t, path)
	if owner.PID != os.Getpid() {
		t.Errorf("owner.PID = %d, want this process %d", owner.PID, os.Getpid())
	}
	if owner.Command != cmdUpdate {
		t.Errorf("owner.Command = %q, want %q", owner.Command, cmdUpdate)
	}
	if owner.Acquired.IsZero() {
		t.Error("owner.Acquired was not recorded")
	}
	if owner.Nonce == "" {
		t.Error("owner.Nonce was not recorded; the lock would not be identifiable after a take-over")
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the lock file still exists after Release(): %v", err)
	}
}

// TestAcquire_createsParentDirectory covers a first run on a machine where the
// target directory does not exist yet: the lock must not fail just because
// nothing has been written there before.
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

// TestRelease_doesNotDeleteASuccessorsLock is the one that matters most. When a
// lock is reclaimed as abandoned, the original holder is still running and will
// eventually release. If Release removed whatever file sits at its path, it
// would delete the successor's lock, and the process after that would walk
// straight into a critical section two others were already in.
func TestRelease_doesNotDeleteASuccessorsLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	first, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	// Stand in for "another gup reclaimed this lock and took it": the file at the
	// path is now someone else's acquisition.
	successor := Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Acquired: time.Now(), Nonce: successorNonce}
	writeOwner(t, path, successor)

	err = first.Release()
	var takenOver *TakenOverError
	if !errors.As(err, &takenOver) {
		t.Errorf("Release() error = %v, want *TakenOverError so the overlap is reported", err)
	}

	got := readOwnerForTest(t, path)
	if got.Nonce != successor.Nonce {
		t.Fatalf("the successor's lock was deleted or replaced by the previous owner's Release(); nonce = %q", got.Nonce)
	}
}

// TestRelease_leavesAnUnreadableLockFileAlone covers the same rule when the file
// cannot be parsed: unprovable ownership is not ownership, so it is not removed.
func TestRelease_leavesAnUnreadableLockFileAlone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), lockFileMode); err != nil {
		t.Fatalf("failed to corrupt the lock file: %v", err)
	}

	var takenOver *TakenOverError
	if err := lock.Release(); !errors.As(err, &takenOver) {
		t.Errorf("Release() error = %v, want *TakenOverError", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("an unreadable lock file was removed by a process that could not prove it owned it: %v", err)
	}
}

// TestHeartbeat_doesNotRefreshASuccessorsLock is the same ownership rule applied
// to the other destructive operation. A previous owner that kept touching its
// successor's file would keep that lock looking alive after the successor died,
// hiding the successor's death from everyone waiting.
func TestHeartbeat_doesNotRefreshASuccessorsLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce})

	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the successor's lock: %v", err)
	}
	// refresh is what the heartbeat calls on every tick, so this asserts the real
	// operation without waiting out a ticker.
	lock.refresh()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat the lock file: %v", err)
	}
	if !info.ModTime().Equal(old) {
		t.Error("the successor's lock was refreshed by its previous owner")
	}
}

// TestHeartbeat_keepsALongRunningLockFresh covers the case the staleness bound
// exists for: a lock nobody can attribute to a live local process must keep
// looking alive while its owner is working.
func TestHeartbeat_keepsALongRunningLockFresh(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}
	before := readAll(t, path)

	lock.refresh()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat the lock file: %v", err)
	}
	if !info.ModTime().After(old) {
		t.Error("the heartbeat did not move the lock file's modification time forward")
	}
	// A heartbeat says "still working"; it must not rewrite who is working.
	if got := readAll(t, path); got != before {
		t.Errorf("the heartbeat changed the owner record to %q, want %q", got, before)
	}
	if _, _, stale := inspect(path); stale {
		t.Error("a freshly touched lock file was judged abandoned")
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
		PID:      os.Getpid(), // alive, so the lock is not abandoned
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
		Nonce:    ownerHolder,
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
	for _, want := range []string{"another gup process is already running", "gup update", path} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Acquire() error %q does not mention %q", err.Error(), want)
		}
	}
	// Telling a user to delete a lock whose owner is alive invites the concurrent
	// run the lock exists to prevent.
	if strings.Contains(err.Error(), "Delete it by hand") {
		t.Errorf("Acquire() error advises deleting a live lock: %q", err.Error())
	}
	if waited := time.Since(start); waited < retryInterval || waited > 5*defaultWait {
		t.Errorf("Acquire() waited %v, want roughly %v", waited, defaultWait)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the refused acquisition removed the holder's lock file: %v", err)
	}
}

// TestAcquire_keepsALockWhoseLocalOwnerIsAliveDespiteAStoppedHeartbeat is the
// rule an earlier version had backwards. Within the trust window the heartbeat
// is a fallback for owners whose liveness cannot be checked, not an expiry: a
// `gup update` suspended with Ctrl-Z, or a laptop resumed from sleep, stops the
// heartbeat without stopping the process, and stealing there puts two gups in
// the critical section at once.
func TestAcquire_keepsALockWhoseLocalOwnerIsAliveDespiteAStoppedHeartbeat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      os.Getpid(), // alive, on this host
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
		Nonce:    ownerHolder,
	})
	// Far past the staleness bound, but well inside the window in which a
	// recorded PID is still believed.
	old := time.Now().Add(-10 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}

	_, err := Acquire(t.Context(), path, cmdUpdate)
	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("Acquire() error = %v, want *BusyError: a live local owner keeps its lock across a heartbeat gap", err)
	}
	if got := readOwnerForTest(t, path); got.Nonce != ownerHolder {
		t.Error("the live owner's lock file was taken over")
	}
}

// TestAcquire_stopsBelievingAPIDOnceTheLockFileIsAncient is the other side of
// that rule, and the reason the trust is bounded at all. A PID outlives the
// process that owned it: after a SIGKILL the operating system eventually
// recycles the number onto something unrelated, and an unbounded check would
// answer "still held" forever. The lock would never be reclaimed by anything,
// while gup kept telling the user it reclaims abandoned locks by itself.
func TestAcquire_stopsBelievingAPIDOnceTheLockFileIsAncient(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		// This process is alive and on this host, standing in for a recycled PID
		// that now names something other than the gup that recorded it.
		PID:      os.Getpid(),
		Host:     hostname(t),
		Command:  cmdUpdate,
		Acquired: time.Now(),
		Nonce:    ownerHolder,
	})
	old := time.Now().Add(-2 * pidTrustMultiple * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v, want a lock nothing has touched in an age to be reclaimable", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	if got := readOwnerForTest(t, path); got.Nonce == ownerHolder {
		t.Error("the ancient lock file was not taken over")
	}
}

// TestPIDTrustWindow_isAGenerousMultipleOfStaleness pins the relationship rather
// than the number: the window has to be long enough that an ordinary heartbeat
// gap never reaches it, and finite so a recycled PID cannot wedge the lock.
func TestPIDTrustWindow_isAGenerousMultipleOfStaleness(t *testing.T) {
	t.Parallel()

	if got, want := pidTrustWindow(), pidTrustMultiple*staleAfter(); got != want {
		t.Errorf("pidTrustWindow() = %v, want %v", got, want)
	}
	if pidTrustWindow() <= staleAfter() {
		t.Error("the PID trust window is not longer than the staleness bound, so the PID check would never apply")
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
		Nonce:    "dead",
	})

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v, want the abandoned lock to be taken over", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	if got := readOwnerForTest(t, path); got.PID != os.Getpid() {
		t.Errorf("the lock file still names the dead owner (pid %d), want this process %d", got.PID, os.Getpid())
	}
}

// TestAcquire_takesOverALockWhoseHeartbeatStopped covers what the PID check
// cannot answer: a lock file from another host (a home directory shared over
// NFS), whose PID means nothing locally. Without the heartbeat fallback such a
// lock would block gup forever.
func TestAcquire_takesOverALockWhoseHeartbeatStopped(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{
		PID:      os.Getpid(), // alive HERE, but the record claims another machine
		Host:     remoteHost,
		Command:  cmdUpdate,
		Acquired: time.Now(),
		Nonce:    "remote",
	})
	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}

	lock, err := Acquire(t.Context(), path, "import")
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
		Host:     remoteHost,
		Command:  cmdUpdate,
		Acquired: time.Now(),
		Nonce:    "remote",
	})

	if _, err := Acquire(t.Context(), path, cmdUpdate); err == nil {
		t.Fatal("Acquire() took over a freshly heartbeated lock from another host")
	}
}

// TestAcquire_takesOverAnUnreadableLockFileOnceItAges covers a lock file
// truncated by a crash mid-write or corrupted on disk. While it is fresh it is
// assumed to be a live writer and respected; once it stops being touched it is
// reclaimed, so unparseable content cannot wedge gup permanently.
func TestAcquire_takesOverAnUnreadableLockFileOnceItAges(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	if err := os.WriteFile(path, []byte("{not json"), lockFileMode); err != nil {
		t.Fatalf("failed to write the corrupt lock file: %v", err)
	}

	if _, err := Acquire(t.Context(), path, cmdUpdate); err == nil {
		t.Fatal("Acquire() took over a freshly written but unreadable lock file")
	}

	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v, want an aged corrupt lock file to be reclaimed", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
}

// TestAcquire_doesNotSpinWhenAnAbandonedLockCannotBeRemoved is the stopping
// guarantee. An abandoned lock in a directory this user cannot write is the one
// combination where the take-over fails every time; an earlier version looped
// straight back to the top without consulting the deadline or the context, so it
// burned a core until the machine was rebooted.
func TestAcquire_doesNotSpinWhenAnAbandonedLockCannotBeRemoved(t *testing.T) {
	t.Parallel()
	requireUnprivilegedPOSIX(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.lock")
	writeOwner(t, path, Owner{PID: deadPID(t), Host: hostname(t), Command: cmdUpdate, Nonce: "dead"})
	// Read+execute only: the file can be opened and read, but not renamed away.
	//nolint:gosec // G302: 0o500 is a DIRECTORY mode, and denying write is the point.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("failed to make the directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restoring a directory mode

	start := time.Now()
	_, err := Acquire(t.Context(), path, cmdUpdate)
	elapsed := time.Since(start)

	var reclaimErr *ReclaimError
	if !errors.As(err, &reclaimErr) {
		t.Fatalf("Acquire() error = %v, want *ReclaimError naming the file that cannot be removed", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Acquire() error %q does not name the lock file", err)
	}
	// A permission failure cannot be waited out, so it must be reported at once
	// rather than after the full wait.
	if elapsed > defaultWait {
		t.Errorf("Acquire() took %v to report an unrecoverable take-over, want well under %v", elapsed, defaultWait)
	}
}

// TestAcquire_honorsContextCancellationWhileWaiting covers Ctrl-C (or a
// --timeout expiring) while gup is waiting its turn: the wait must end at once
// with the context's error rather than running out the full timeout.
func TestAcquire_honorsContextCancellationWhileWaiting(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: ownerHolder})

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(2 * retryInterval)
		cancel()
	}()

	start := time.Now()
	_, err := Acquire(ctx, path, cmdUpdate)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Acquire() error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > defaultWait {
		t.Errorf("Acquire() waited %v after cancellation, want it to return promptly", elapsed)
	}
}

// TestAcquire_honorsAnAlreadyCancelledContext covers the caller that arrives
// with a dead context: no filesystem work should happen at all.
func TestAcquire_honorsAnAlreadyCancelledContext(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: ownerHolder})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := Acquire(ctx, path, cmdUpdate); !errors.Is(err, context.Canceled) {
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

// TestAcquire_rejectsADirectoryLockPath covers a misconfigured path (or a
// directory literally named like the lock): the error must name the problem
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

// TestAcquireAll_takesEverythingOrNothing covers the multi-resource case. A
// command that writes both a $GOBIN and a gup.json needs both, and a partial
// hold left behind by a failed acquisition would block the resource it did get
// for no reason.
func TestAcquireAll_takesEverythingOrNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	free := filepath.Join(dir, "free.lock")
	taken := filepath.Join(dir, "taken.lock")
	writeOwner(t, taken, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: ownerHolder})

	if _, err := AcquireAll(t.Context(), cmdUpdate, free, taken); err == nil {
		t.Fatal("AcquireAll() succeeded even though one lock was held")
	}
	if _, err := os.Stat(free); !os.IsNotExist(err) {
		t.Errorf("AcquireAll() left the first lock held after failing on the second: %v", err)
	}
}

// TestAcquireAll_ordersAndDeduplicates covers the deadlock rule: two commands
// asking for the same resources in different orders must acquire them in the
// same order, and a resource named twice must be locked once.
func TestAcquireAll_ordersAndDeduplicates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	a := filepath.Join(dir, "a.lock")
	b := filepath.Join(dir, "b.lock")

	forward, err := AcquireAll(t.Context(), cmdUpdate, a, b, a)
	if err != nil {
		t.Fatalf("AcquireAll() error: %v", err)
	}
	if got := len(forward.Paths()); got != 2 {
		t.Errorf("AcquireAll() held %d locks, want 2 after deduplication", got)
	}
	forwardPaths := forward.Paths()
	if err := forward.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	reverse, err := AcquireAll(t.Context(), cmdUpdate, b, a)
	if err != nil {
		t.Fatalf("AcquireAll() error: %v", err)
	}
	t.Cleanup(func() { _ = reverse.Release() })

	if !slicesEqual(forwardPaths, reverse.Paths()) {
		t.Errorf("acquisition order differs by argument order: %v vs %v; that is how two processes deadlock",
			forwardPaths, reverse.Paths())
	}
}

// TestAcquireAll_withNoPathsIsANoOp covers a command that writes nothing, such
// as `gup update --dry-run`: it must not create or contend for anything.
func TestAcquireAll_withNoPathsIsANoOp(t *testing.T) {
	t.Parallel()

	held, err := AcquireAll(t.Context(), cmdUpdate)
	if err != nil {
		t.Fatalf("AcquireAll() with no paths error: %v", err)
	}
	if got := held.Paths(); len(got) != 0 {
		t.Errorf("AcquireAll() with no paths held %v", got)
	}
	if err := held.Release(); err != nil {
		t.Errorf("Release() error: %v", err)
	}
}

// TestBusyError_degradesWithoutOwnerDetails covers the message when the lock
// file could not be parsed: it must still tell the user which file to look at
// rather than printing a half-built sentence.
func TestBusyError_degradesWithoutOwnerDetails(t *testing.T) {
	t.Parallel()

	got := (&BusyError{Path: testLockPath}).Error()
	if strings.Contains(got, "pid") {
		t.Errorf("BusyError.Error() = %q, want no pid clause when the owner is unknown", got)
	}
	if !strings.Contains(got, testLockPath) {
		t.Errorf("BusyError.Error() = %q, want it to name the lock file", got)
	}
}

// TestReclaimError_saysWhatToDo covers the one case where deleting the file by
// hand IS the right advice, and the case where there is no underlying error to
// quote.
func TestReclaimError_saysWhatToDo(t *testing.T) {
	t.Parallel()

	withCause := (&ReclaimError{Path: testLockPath, Err: fs.ErrPermission}).Error()
	if !strings.Contains(withCause, "Delete it by hand") || !strings.Contains(withCause, "permission") {
		t.Errorf("ReclaimError.Error() = %q, want the cause and the remedy", withCause)
	}
	if !errors.Is(&ReclaimError{Err: fs.ErrPermission}, fs.ErrPermission) {
		t.Error("ReclaimError does not unwrap to its cause")
	}
	if got := (&ReclaimError{Path: testLockPath}).Error(); !strings.Contains(got, "re-creating") {
		t.Errorf("ReclaimError.Error() without a cause = %q, want it to explain the race", got)
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

// TestDurationFromEnv covers the timing overrides the end-to-end suite depends
// on, and their refusal to accept nonsense: a typo in a test knob must fall back
// to the shipped default rather than disable the lock's bounds.
func TestDurationFromEnv(t *testing.T) {
	tests := map[string]struct {
		value string
		want  time.Duration
	}{
		"unset":        {value: "", want: defaultWait},
		"valid":        {value: "250ms", want: 250 * time.Millisecond},
		"padded":       {value: "  1s  ", want: time.Second},
		"unparseable":  {value: "soon", want: defaultWait},
		"zero":         {value: "0s", want: defaultWait},
		"negative":     {value: "-1s", want: defaultWait},
		"bare integer": {value: "5", want: defaultWait},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envWait, tt.value)
			if got := waitTimeout(); got != tt.want {
				t.Errorf("waitTimeout() with %s=%q = %v, want %v", envWait, tt.value, got, tt.want)
			}
		})
	}
}

// TestTimingOverrides_shortenTheWait proves the override reaches the acquisition
// path, which is what lets the end-to-end suite test waiting and staleness in
// seconds instead of a minute.
func TestTimingOverrides_shortenTheWait(t *testing.T) {
	t.Setenv(envWait, "100ms")

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: ownerHolder})

	start := time.Now()
	if _, err := Acquire(context.Background(), path, cmdUpdate); err == nil {
		t.Fatal("Acquire() succeeded against a held lock")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Acquire() waited %v, want roughly the 100ms override", elapsed)
	}
}

// TestStaleOverride_shortensReclaim is the same for the staleness bound: a lock
// nobody can attribute is reclaimed after the override rather than the default
// minute.
func TestStaleOverride_shortensReclaim(t *testing.T) {
	t.Setenv(envStale, "50ms")

	path := filepath.Join(t.TempDir(), "gup.lock")
	if err := os.WriteFile(path, []byte("{not json"), lockFileMode); err != nil {
		t.Fatalf("failed to write the corrupt lock file: %v", err)
	}
	old := time.Now().Add(-time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}

	lock, err := Acquire(context.Background(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v, want the override to make the lock reclaimable", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
}

// writeOwner plants a lock file describing owner, standing in for a lock taken
// by a different gup process.
func writeOwner(t *testing.T, path string, owner Owner) {
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

// readAll returns a file's whole content, failing the test if it cannot be read.
func readAll(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // a path this test just created
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(raw)
}

// readOwnerForTest reads the owner record, failing the test if it cannot.
func readOwnerForTest(t *testing.T, path string) Owner {
	t.Helper()
	owner, err := readOwner(path)
	if err != nil {
		t.Fatalf("failed to read the lock file %s: %v", path, err)
	}
	return owner
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

// requireUnprivilegedPOSIX skips a test that depends on directory permissions
// actually denying something. Windows does not express them this way, and root
// bypasses them entirely, so on either the test would pass without testing.
func requireUnprivilegedPOSIX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not deny rename on Windows the way POSIX modes do")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which bypasses the directory permissions this test relies on")
	}
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

// TestAcquireAll_normalizesBeforeDeduplicating covers two spellings of one path.
// Acquire normalizes before keying the in-process registry, so if AcquireAll
// deduplicated the raw strings the two would take separate slots for one file
// and the second would wait out the whole timeout against the first.
func TestAcquireAll_normalizesBeforeDeduplicating(t *testing.T) { //nolint:paralleltest // changes the working directory
	dir := t.TempDir()
	t.Chdir(dir)

	start := time.Now()
	held, err := AcquireAll(t.Context(), cmdUpdate, "x.lock", "./x.lock", filepath.Join(dir, "x.lock"))
	if err != nil {
		t.Fatalf("AcquireAll() error: %v", err)
	}
	t.Cleanup(func() { _ = held.Release() })

	if got := held.Paths(); len(got) != 1 {
		t.Errorf("AcquireAll() held %v, want one lock for three spellings of one path", got)
	}
	// The failure mode is a wait, not an error, so the elapsed time is the
	// assertion that matters.
	if elapsed := time.Since(start); elapsed > defaultWait {
		t.Errorf("AcquireAll() took %v, which means the spellings contended with each other", elapsed)
	}
}

// TestNormalizePath covers the shared normalization every entry point uses.
func TestNormalizePath(t *testing.T) { //nolint:paralleltest // changes the working directory
	dir := t.TempDir()
	t.Chdir(dir)

	first, err := normalizePath("x.lock")
	if err != nil {
		t.Fatalf("normalizePath() error: %v", err)
	}
	second, err := normalizePath("./x.lock")
	if err != nil {
		t.Fatalf("normalizePath() error: %v", err)
	}
	if first != second {
		t.Errorf("normalizePath() gave %q and %q for the same file", first, second)
	}
	if !filepath.IsAbs(first) {
		t.Errorf("normalizePath() = %q, want an absolute path", first)
	}
}

// TestReclaim_leavesALockCreatedAfterTheVerdictAlone is the take-over's
// equivalent of the rule Release follows. Judging a lock abandoned and removing
// it are two operations, and between them the file can be replaced: a faster
// process may have reclaimed it and created its own, which is alive and being
// relied on. Removing THAT would put two gups in the critical section, so the
// take-over must prove it is removing what it judged.
func TestReclaim_leavesALockCreatedAfterTheVerdictAlone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	abandoned := Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: "abandoned"}
	writeOwner(t, path, abandoned)
	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the abandoned lock: %v", err)
	}

	_, observed, stale := inspect(path)
	if !stale {
		t.Fatal("a lock file nobody has touched for twice the staleness bound was not judged abandoned")
	}

	// The window this closes: another process reclaimed the file and took the
	// lock between the verdict above and the take-over below.
	successor := Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Acquired: time.Now(), Nonce: successorNonce}
	writeOwner(t, path, successor)

	if err := reclaim(path, observed); !errors.Is(err, errLockChanged) {
		t.Errorf("reclaim() error = %v, want errLockChanged so the caller looks again", err)
	}
	if got := readOwnerForTest(t, path); got.Nonce != successor.Nonce {
		t.Fatalf("the successor's lock was taken over on a verdict about a different file; nonce = %q", got.Nonce)
	}
}

// TestReclaim_leavesALockItsOwnerRefreshedAlone is the same rule for the other
// way the verdict can go out of date: the owner's heartbeat runs. The content is
// unchanged, so only the modification time - the very thing the verdict was
// computed from - says the owner is still there.
func TestReclaim_leavesALockItsOwnerRefreshedAlone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock: %v", err)
	}

	_, observed, stale := inspect(path)
	if !stale {
		t.Fatal("a lock file nobody has touched for twice the staleness bound was not judged abandoned")
	}

	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("failed to refresh the lock: %v", err)
	}

	if err := reclaim(path, observed); !errors.Is(err, errLockChanged) {
		t.Errorf("reclaim() error = %v, want errLockChanged so the caller looks again", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("a refreshed lock file was taken over: %v", err)
	}
}

// TestReclaim_removesTheFileItJudged is the other half: when nothing has
// changed, the take-over must actually happen. A rule that only ever refuses
// would turn every abandoned lock file into a permanently wedged tool.
func TestReclaim_removesTheFileItJudged(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock: %v", err)
	}

	_, observed, _ := inspect(path)
	if err := reclaim(path, observed); err != nil {
		t.Fatalf("reclaim() error = %v, want the abandoned file to be taken over", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the abandoned lock file survived the take-over: %v", err)
	}
}

// TestReclaim_ofAVanishedFileIsANoOp covers the file that disappeared between
// the failed create and the look at it. There is nothing to take over, and
// nothing may be assumed about whatever appears at the path next.
func TestReclaim_ofAVanishedFileIsANoOp(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	_, observed, stale := inspect(path)
	if !stale {
		t.Fatal("a lock file that is not there was not reported as free")
	}

	// A lock created after the look must not be removed by a take-over decided
	// before it existed.
	writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce})
	if err := reclaim(path, observed); err != nil {
		t.Errorf("reclaim() of a vanished file = %v, want no error", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a lock created after the look was removed: %v", err)
	}
}

// TestOwnsFile covers the check every destructive step makes before it acts, and
// the one createLockFile makes about its own write: a lock file is this lock's
// only when it carries this lock's nonce.
func TestOwnsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mine := filepath.Join(dir, "mine.lock")
	writeOwner(t, mine, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: "mine"})
	if !ownsFile(mine, "mine") {
		t.Error("a lock file carrying this nonce was not recognized")
	}
	if ownsFile(mine, "someone-else") {
		t.Error("a lock file carrying another nonce was claimed")
	}

	// A record with no nonce at all - written by a gup too old to record one, or
	// truncated - proves nothing and must never match.
	empty := filepath.Join(dir, "empty.lock")
	writeOwner(t, empty, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate})
	if ownsFile(empty, "") {
		t.Error("a lock file with no nonce was claimed by a lock with no nonce")
	}
	if ownsFile(filepath.Join(dir, "missing.lock"), "mine") {
		t.Error("a lock file that does not exist was claimed")
	}
}

// TestTakenOverError_saysTheRunsOverlapped covers the message for the outcome
// nobody wants but everybody needs told: this command's lock was reclaimed while
// it worked, so another gup may have been running alongside it. Naming the file
// is what lets a user see which resource was shared.
func TestTakenOverError_saysTheRunsOverlapped(t *testing.T) {
	t.Parallel()

	got := (&TakenOverError{Path: testLockPath}).Error()
	for _, want := range []string{testLockPath, "taken over", "overlapped"} {
		if !strings.Contains(got, want) {
			t.Errorf("TakenOverError.Error() = %q, want it to contain %q", got, want)
		}
	}
}

// TestHeartbeat_doesNotRefreshAnUnreadableLockFile covers the file a crash
// truncated, or one an operator replaced with something that is not a lock at
// all: ownership cannot be proved, so it is not this lock's to keep alive.
func TestHeartbeat_doesNotRefreshAnUnreadableLockFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	if err := os.WriteFile(path, []byte("{this is not json"), 0o600); err != nil {
		t.Fatalf("failed to damage the lock file: %v", err)
	}
	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock file: %v", err)
	}

	lock.refresh()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat the lock file: %v", err)
	}
	if !info.ModTime().Equal(old) {
		t.Error("a lock file that proves nothing was kept alive by the heartbeat")
	}
}

// TestHeartbeat_toleratesALockFileThatIsGone covers the file an operator deleted
// while the command ran. There is nothing to keep alive and nothing to report:
// the lock is about to be released, and interrupting a user's update over a
// heartbeat would be a worse answer than letting the lock age out.
func TestHeartbeat_toleratesALockFileThatIsGone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	if err := os.Remove(path); err != nil {
		t.Fatalf("failed to remove the lock file: %v", err)
	}
	lock.refresh()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the heartbeat re-created a lock file nobody holds: %v", err)
	}
}

// TestRelease_reportsALockFileItCannotRemove covers the directory that denies
// the removal - the same permissions problem the take-over reports, on the way
// out instead of the way in. The work is done either way, so the caller is told
// rather than failed, but it must be told: the file left behind will make the
// next gup wait until it ages out.
func TestRelease_reportsALockFileItCannotRemove(t *testing.T) { //nolint:paralleltest // changes a directory's mode
	requireUnprivilegedPOSIX(t)

	dir := t.TempDir()
	lock, err := Acquire(t.Context(), filepath.Join(dir, "gup.lock"), cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	// Read+execute only: the lock file can still be read, but not moved or deleted.
	//nolint:gosec // G302: 0o500 is a DIRECTORY mode, and denying write is the point.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("failed to make the directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restoring a directory mode

	err = lock.Release()
	if err == nil {
		t.Fatal("Release() reported success for a lock file it could not remove")
	}
	if !strings.Contains(err.Error(), "can not remove the gup lock file") {
		t.Errorf("Release() error = %v, want it to name the file that is still there", err)
	}
}

// TestRestore_doesNotDisplaceALockTakenWhileTheFileWasAside is the rule that
// keeps putting a file back from being the same bug as taking it. A file is
// detached before it can be identified, and a process that acquires the path in
// that window holds a real lock: overwriting it would leave that process working
// with no lock and no way to find out, next to whoever holds the restored one.
func TestRestore_doesNotDisplaceALockTakenWhileTheFileWasAside(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.lock")
	aside := filepath.Join(dir, "gup.lock.stale-1-1")
	writeOwner(t, aside, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
	newcomer := Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce}
	writeOwner(t, path, newcomer)

	restore(aside, path)

	if got := readOwnerForTest(t, path); got.Nonce != newcomer.Nonce {
		t.Errorf("the lock at the path is now %q, want the newcomer's %q", got.Nonce, newcomer.Nonce)
	}
	if _, err := os.Stat(aside); !os.IsNotExist(err) {
		t.Errorf("the detached file was left lying around under a name no owner recognizes: %v", err)
	}
}

// TestRestore_putsTheFileBackWhenThePathIsFree is the other half: with nothing
// at the path, the owner whose file was detached must get it back, content and
// modification time intact, or a lock that was only being examined would have
// been destroyed by examining it.
func TestRestore_putsTheFileBackWhenThePathIsFree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gup.lock")
	writeOwner(t, path, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat the lock file: %v", err)
	}
	content := readAll(t, path)

	aside, err := detach(path, "stale")
	if err != nil || aside == "" {
		t.Fatalf("detach() = %q, %v", aside, err)
	}
	restore(aside, path)

	if got := readAll(t, path); got != content {
		t.Errorf("the restored lock file reads %q, want %q", got, content)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the lock file was not put back: %v", err)
	}
	// The modification time is what a waiter judges staleness from, so restoring
	// must not look like a heartbeat.
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("restore moved the modification time from %v to %v", before.ModTime(), after.ModTime())
	}
}

// TestReclaim_leavesALockChangedJustBeforeTheTakeOverAlone covers the look taken
// immediately before the file is moved. Without it a lock whose owner refreshed
// it after the verdict is detached first and identified second, so its owner is
// briefly missing from its own path even though the file goes back.
func TestReclaim_leavesALockChangedJustBeforeTheTakeOverAlone(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gup.lock")
	writeOwner(t, path, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
	old := time.Now().Add(-2 * defaultStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate the lock: %v", err)
	}
	_, observed, _ := inspect(path)

	// The owner's heartbeat runs: same bytes, newer modification time.
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("failed to refresh the lock: %v", err)
	}

	if err := reclaim(path, observed); !errors.Is(err, errLockChanged) {
		t.Errorf("reclaim() error = %v, want errLockChanged", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the refreshed lock file is gone: %v", err)
	}
	if !info.ModTime().Equal(now.Truncate(0)) && info.ModTime().Before(old.Add(time.Second)) {
		t.Errorf("the lock file's modification time is %v, want the refreshed one", info.ModTime())
	}
}

// TestAcquireAll_reportsARollbackFailureAlongsideTheAcquisitionFailure covers
// the error that used to be dropped. When a later lock cannot be taken, the ones
// already held are handed back - and a hand-back that failed leaves a resource
// held by nobody, which the next command waits on. Reporting only the
// acquisition failure hides the one thing the user would have to clean up.
//
// The situation has to be staged against a running acquisition: the first lock
// is taken, something reclaims it while the second is still being waited for,
// and the hand-back then finds a stranger's file at its path. A stand-in that
// overwrites the file can land too early - inside the window where the
// acquisition is still proving the file it created is its own, which correctly
// fails the acquisition instead - so the attempt is repeated until it lands
// where it was aimed, rather than asserted on whichever way it fell.
func TestAcquireAll_reportsARollbackFailureAlongsideTheAcquisitionFailure(t *testing.T) {
	t.Setenv(envWait, "500ms")

	for attempt := range 5 {
		err := rollbackAgainstAReclaimedFirstLock(t)
		var busy *BusyError
		if errors.As(err, &busy) && strings.HasSuffix(busy.Path, "a.lock") {
			// The overwrite landed while the first lock was still being taken, so
			// this run never reached the hand-back. Aim again.
			t.Logf("attempt %d overwrote the first lock before it was held; retrying", attempt+1)
			continue
		}
		if !errors.As(err, &busy) {
			t.Fatalf("AcquireAll() error = %v, want it to report the lock that is held", err)
		}
		var takenOver *TakenOverError
		if !errors.As(err, &takenOver) {
			t.Fatalf("AcquireAll() error = %v, want it to also report the lock it could not hand back", err)
		}
		return
	}
	t.Skip("the reclaim never landed between the acquisition and the hand-back")
}

// rollbackAgainstAReclaimedFirstLock runs one attempt at the situation above and
// returns what AcquireAll reported.
func rollbackAgainstAReclaimedFirstLock(t *testing.T) error {
	t.Helper()

	dir := t.TempDir()
	// Sorted order decides acquisition order: "a" is taken first, "b" cannot be
	// taken, and the hand-back of "a" is what this is about.
	first := filepath.Join(dir, "a.lock")
	second := filepath.Join(dir, "b.lock")
	writeOwner(t, second, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: ownerHolder})

	// Stands in for another gup reclaiming the first lock while this process is
	// still waiting for the second: the file at that path stops being ours, so
	// handing it back reports the overlap instead of deleting a stranger's lock.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			if owner, err := readOwner(first); err == nil && owner.Nonce != "" {
				// Let the acquisition finish proving the file is its own.
				time.Sleep(100 * time.Millisecond)
				writeOwner(t, first, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce})
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	_, err := AcquireAll(t.Context(), cmdUpdate, first, second)
	<-done
	if err == nil {
		t.Fatal("AcquireAll() succeeded against a lock held by a live process")
	}
	return err
}

// TestRestoreByClaiming covers restore's fallback for filesystems with no hard
// links, which the test machine almost certainly has - so the fallback is driven
// directly. It must follow the same rule as the Link path, and be atomic about
// it: the path is filled by the operation that claims it, never by a check
// followed by a write.
func TestRestoreByClaiming(t *testing.T) {
	t.Parallel()

	t.Run("restores into a free path, modification time and all", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "gup.lock")
		aside := filepath.Join(dir, "gup.lock.stale-1-1")
		writeOwner(t, aside, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
		old := time.Now().Add(-30 * time.Second).Truncate(time.Second)
		if err := os.Chtimes(aside, old, old); err != nil {
			t.Fatalf("failed to backdate the detached file: %v", err)
		}
		content := readAll(t, aside)

		restoreByClaiming(aside, path)

		if got := readAll(t, path); got != content {
			t.Errorf("the restored lock file reads %q, want %q", got, content)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("the lock file was not put back: %v", err)
		}
		// Staleness is measured from this, so a restored lock must not look
		// freshly touched - that would hide an owner's death from every waiter.
		if !info.ModTime().Equal(old) {
			t.Errorf("the restored lock file's modification time is %v, want %v", info.ModTime(), old)
		}
		if _, err := os.Stat(aside); !os.IsNotExist(err) {
			t.Errorf("the detached file was left behind: %v", err)
		}
	})

	t.Run("leaves an occupied path alone", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "gup.lock")
		aside := filepath.Join(dir, "gup.lock.stale-1-1")
		writeOwner(t, aside, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
		writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce})
		before, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat the successor's lock: %v", err)
		}

		restoreByClaiming(aside, path)

		if got := readOwnerForTest(t, path); got.Nonce != successorNonce {
			t.Errorf("the lock at the path is %q, want the newcomer's %q", got.Nonce, successorNonce)
		}
		// The old fallback wrote the detached file's content and then wound the
		// modification time back, both by name. Landing that on a successor's lock
		// backdates a live lock into staleness, so a third process reclaims it.
		after, err := os.Stat(path)
		if err != nil {
			t.Fatalf("the successor's lock is gone: %v", err)
		}
		if !after.ModTime().Equal(before.ModTime()) {
			t.Errorf("the successor's lock was re-timed from %v to %v", before.ModTime(), after.ModTime())
		}
		if _, err := os.Stat(aside); !os.IsNotExist(err) {
			t.Errorf("the detached file was left behind: %v", err)
		}
	})

	t.Run("discards a detached file it cannot read", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "gup.lock")

		restoreByClaiming(filepath.Join(dir, "not-there"), path)

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("a lock file was created out of nothing: %v", err)
		}
	})
}

// TestDiscardOwnUnwritten covers the cleanup that runs when the owner record
// could not be written into a lock file this process had already created.
//
// The dangerous case is the second one. A write that fails can take longer than
// the staleness bound - a device that stalls before reporting an error is
// exactly how that happens - and by the time it returns, a waiter may have
// judged the anonymous file abandoned, removed it, and created its own lock at
// the same name. Removing that path by name deletes a lock another gup is
// actively working under, and the process after it walks straight in.
func TestDiscardOwnUnwritten(t *testing.T) {
	t.Parallel()

	t.Run("removes the anonymous file it created", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "gup.lock")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("failed to plant the anonymous lock file: %v", err)
		}

		discardOwnUnwritten(path, "mine")

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the half-created lock file was left behind: %v", err)
		}
	})

	t.Run("removes a file that carries its own nonce", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "gup.lock")
		writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdUpdate, Nonce: "mine"})

		discardOwnUnwritten(path, "mine")

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the lock file this process created was left behind: %v", err)
		}
	})

	t.Run("leaves a successor's lock exactly where it is", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "gup.lock")
		writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce})
		content := readAll(t, path)
		before, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat the successor's lock: %v", err)
		}

		discardOwnUnwritten(path, "mine")

		if got := readAll(t, path); got != content {
			t.Errorf("the successor's lock reads %q, want %q", got, content)
		}
		after, err := os.Stat(path)
		if err != nil {
			t.Fatalf("the successor's lock was deleted: %v", err)
		}
		if !after.ModTime().Equal(before.ModTime()) {
			t.Errorf("the successor's lock was re-timed from %v to %v", before.ModTime(), after.ModTime())
		}
		if leftovers := asideFiles(t, filepath.Dir(path)); len(leftovers) != 0 {
			t.Errorf("detached files were left lying around: %v", leftovers)
		}
	})
}

// TestCreateLockFile_doesNotDeleteALockTakenWhileItsOwnRecordWasBeingWritten
// drives the same race through createLockFile itself rather than through the
// cleanup in isolation, so the two stay wired together.
//
// The take-over is staged from inside the failing write, which is what makes
// this deterministic: the stand-in reclaims the anonymous file and installs a
// successor's lock at exactly the moment the real race would, then reports the
// write error that sends createLockFile into its cleanup. No sleeping, and no
// dependence on which of two goroutines happens to run first.
func TestCreateLockFile_doesNotDeleteALockTakenWhileItsOwnRecordWasBeingWritten(t *testing.T) { //nolint:paralleltest // swaps a package-level test seam
	path := filepath.Join(t.TempDir(), "gup.lock")
	successor := Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce}

	original := writeOwnerRecord
	t.Cleanup(func() { writeOwnerRecord = original })
	writeOwnerRecord = func(_ *os.File, _ []byte) error {
		// Another gup judged the anonymous file abandoned and took the name over.
		if err := os.Remove(path); err != nil {
			t.Errorf("failed to stand in for the take-over: %v", err)
		}
		writeOwner(t, path, successor)
		return errors.New("the device reported a write error")
	}

	lock, err := createLockFile(path, cmdUpdate)
	if err == nil {
		t.Fatalf("createLockFile() succeeded despite a failed write; lock = %v", lock)
	}
	if got := readOwnerForTest(t, path); got.Nonce != successorNonce {
		t.Fatalf("the lock at the path is %q, want the successor's %q", got.Nonce, successorNonce)
	}
	if leftovers := asideFiles(t, filepath.Dir(path)); len(leftovers) != 0 {
		t.Errorf("detached files were left lying around: %v", leftovers)
	}
}

// TestCreateLockFile_cleansUpAfterAFailedWrite is the other half: with nobody
// else involved, the file this process created must not be left behind, or the
// next gup waits out the staleness bound on rubbish.
func TestCreateLockFile_cleansUpAfterAFailedWrite(t *testing.T) { //nolint:paralleltest // swaps a package-level test seam
	path := filepath.Join(t.TempDir(), "gup.lock")

	original := writeOwnerRecord
	t.Cleanup(func() { writeOwnerRecord = original })
	writeOwnerRecord = func(*os.File, []byte) error { return errors.New("the device reported a write error") }

	if lock, err := createLockFile(path, cmdUpdate); err == nil {
		t.Fatalf("createLockFile() succeeded despite a failed write; lock = %v", lock)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the half-created lock file was left behind: %v", err)
	}
	if leftovers := asideFiles(t, filepath.Dir(path)); len(leftovers) != 0 {
		t.Errorf("detached files were left lying around: %v", leftovers)
	}
}

// TestRestoreByClaiming_neverLeavesASuccessorsLockWearingAnotherOwnersTimestamp
// drives the one window the fallback has: a lock created at the path after the
// name was claimed and before the detached file is renamed onto it.
//
// The invariant is that the file left at the path is consistent - whichever of
// the two locks survives, it wears its OWN modification time. The fallback this
// replaced could not hold that: it wrote the detached file's bytes through a
// descriptor whose name had already been taken over, and then wound the path's
// modification time back to the detached file's, leaving a successor's lock
// backdated into staleness for every waiter to reclaim while its owner worked.
func TestRestoreByClaiming_neverLeavesASuccessorsLockWearingAnotherOwnersTimestamp(t *testing.T) { //nolint:paralleltest // swaps a package-level test seam
	dir := t.TempDir()
	path := filepath.Join(dir, "gup.lock")
	aside := filepath.Join(dir, "gup.lock.stale-1-1")
	writeOwner(t, aside, Owner{PID: 1, Host: remoteHost, Command: cmdUpdate, Nonce: ownerHolder})
	old := time.Now().Add(-30 * time.Second).Truncate(time.Second)
	if err := os.Chtimes(aside, old, old); err != nil {
		t.Fatalf("failed to backdate the detached file: %v", err)
	}

	successorMod := time.Now().Truncate(time.Second)
	original := restoreRaceHook
	t.Cleanup(func() { restoreRaceHook = original })
	restoreRaceHook = func() {
		// Another gup takes the claimed name over: it removes the empty claim and
		// puts its own lock there, exactly as a reclaim would.
		if err := os.Remove(path); err != nil {
			t.Errorf("failed to stand in for the take-over: %v", err)
		}
		writeOwner(t, path, Owner{PID: os.Getpid(), Host: hostname(t), Command: cmdRemove, Nonce: successorNonce})
		if err := os.Chtimes(path, successorMod, successorMod); err != nil {
			t.Errorf("failed to time the successor's lock: %v", err)
		}
	}

	restoreByClaiming(aside, path)

	got := readOwnerForTest(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("no lock file was left at the path: %v", err)
	}
	switch got.Nonce {
	case ownerHolder:
		if !info.ModTime().Equal(old) {
			t.Errorf("the restored lock reads %q but is timed %v, want %v", got.Nonce, info.ModTime(), old)
		}
	case successorNonce:
		if !info.ModTime().Equal(successorMod) {
			t.Errorf("the successor's lock was re-timed to %v, want its own %v", info.ModTime(), successorMod)
		}
	default:
		t.Errorf("the lock at the path belongs to nobody: %+v", got)
	}
	if leftovers := asideFiles(t, dir); len(leftovers) != 0 {
		t.Errorf("detached files were left lying around: %v", leftovers)
	}
}

// asideFiles lists the detached lock files left in dir. Every path that gets
// renamed aside has to end up either back at its name or removed; one left
// behind is a lock file nobody will ever reclaim, because no owner recognizes
// the name.
func asideFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to list %s: %v", dir, err)
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, ".stale-") || strings.Contains(name, ".release-") ||
			strings.Contains(name, ".unwritten-") {
			out = append(out, name)
		}
	}
	return out
}
