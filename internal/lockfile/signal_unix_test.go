//go:build !windows

package lockfile

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestSignal_leavesTheLockHeld pins the decision this package makes about
// interruption, which is to make none.
//
// Releasing a lock from a signal handler looks tidy and is not safe: deleting
// the file does not stop the command, which is still installing binaries and
// rewriting gup.json while the process winds down. A second gup started in that
// gap runs concurrently with the first, and neither of them can tell. So the
// lock stays until the process is gone - a state the next gup reads off the
// operating system rather than off a file this one raced to delete.
//
// The test installs its own handler for the same reason gup's long-running
// commands do: to keep the signal from killing the process outright, and to
// prove the signal really was delivered while the lock file was still there.
func TestSignal_leavesTheLockHeld(t *testing.T) { //nolint:paralleltest // sends this process a real signal
	path := filepath.Join(t.TempDir(), "gup.lock")
	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	// Stands in for the signal-cancelling context gup's commands install: the
	// work is what reacts to a signal, not the lock.
	received := make(chan os.Signal, 1)
	signal.Notify(received, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(received) })

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT to this process: %v", err)
	}
	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("the interrupt was never delivered")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the lock file was released by the signal, while the command holding it is still running: %v", err)
	}
	// It is still this lock's, too: released and re-taken would be just as wrong.
	if !ownsFile(path, lock.nonce) {
		t.Error("the lock file at the path is no longer the one this lock holds")
	}
}
