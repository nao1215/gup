//go:build !windows

package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestAcquire_takesTheLockAfterAnInterruptedHolderExits covers Ctrl-C.
//
// gup installs no lock-releasing signal handler, and that is deliberate:
// releasing the lock from a handler would free the resource while the
// interrupted command is still unwinding, so a gup started in that moment would
// run alongside it. The lock is held until the process is gone - and because it
// is the kernel's, "gone" is all it takes. A holder with no handler at all, like
// this one, is killed by the default disposition and its lock goes with it.
func TestAcquire_takesTheLockAfterAnInterruptedHolderExits(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), ".gup.lock")
	holder := startHolder(t, path)

	if err := holder.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("failed to interrupt the holder: %v", err)
	}
	if _, err := holder.Process.Wait(); err != nil {
		t.Fatalf("failed to reap the holder: %v", err)
	}
	// Nothing raced the process to the file: it is exactly where its owner left
	// it, and it holds nothing.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the interrupted holder's lock file is gone: %v", err)
	}

	lock, err := Acquire(t.Context(), path, "remove")
	if err != nil {
		t.Fatalf("Acquire() after the holder was interrupted = %v, want success", err)
	}
	if err := lock.Release(); err != nil {
		t.Errorf("Release() = %v, want nil", err)
	}
}

// TestAcquire_reportsBusyOnlyAfterWaiting covers the timeout itself. Waiting is
// there to absorb the moment between one command releasing and the next
// starting, so a refusal that arrived instantly would turn a handover into a
// failure - and one that never arrived would hang the second terminal.
func TestAcquire_reportsBusyOnlyAfterWaiting(t *testing.T) {
	const wait = 400 * time.Millisecond
	t.Setenv(envWait, wait.String())
	path := filepath.Join(t.TempDir(), ".gup.lock")
	startHolder(t, path)

	start := time.Now()
	if _, err := Acquire(t.Context(), path, "remove"); err == nil {
		t.Fatal("Acquire() succeeded while another process held the lock")
	}
	elapsed := time.Since(start)
	if elapsed < wait {
		t.Errorf("Acquire() gave up after %v, want it to wait at least %v", elapsed, wait)
	}
	if elapsed > 10*wait {
		t.Errorf("Acquire() waited %v, far past the %v timeout", elapsed, wait)
	}
}

// TestAcquire_reportsALockFileItCanNotOpen covers a directory gup may not write
// in. Retrying cannot fix a permission problem, so it is reported rather than
// waited out, and the message names the file.
func TestAcquire_reportsALockFileItCanNotOpen(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	if os.Geteuid() == 0 {
		t.Skip("running as root, which bypasses the directory permissions this test relies on")
	}
	shortWait(t)

	dir := t.TempDir()
	//nolint:gosec // G302: this is a directory mode, and denying writes is the point.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("failed to seal the directory: %v", err)
	}
	//nolint:gosec // G302: restoring the mode so t.TempDir can clean up.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	path := filepath.Join(dir, ".gup.lock")
	start := time.Now()
	_, err := Acquire(t.Context(), path, "update")
	if err == nil {
		t.Fatal("Acquire() in an unwritable directory succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "can not open the gup lock file") {
		t.Errorf("Acquire() error = %v, want it to name the file it could not open", err)
	}
	// Reported, not waited out: a permission problem does not resolve by retrying,
	// and waiting would replace a clear diagnosis with a timeout.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Acquire() spent %v retrying a permission failure", elapsed)
	}
	var busy *BusyError
	if errors.As(err, &busy) {
		t.Error("a permission failure was reported as another gup process")
	}
}
