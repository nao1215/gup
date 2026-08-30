//go:build !windows

package lockfile

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// TestAcquire_refusesALockPathThatIsNotARegularFile covers the rest of what can
// sit at a lock path once symlinks are ruled out. A FIFO is the one an
// unprivileged user can plant, and opening one for writing is how a command that
// should have failed instead blocks forever, or writes gup's owner record into
// somebody's reader. The kind of file is read back from the descriptor rather
// than from the path, so the answer is about the file gup actually opened.
func TestAcquire_refusesALockPathThatIsNotARegularFile(t *testing.T) { //nolint:paralleltest // sets the wait timeout
	shortWait(t)
	path := filepath.Join(t.TempDir(), "gup.json.lock")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Skipf("this platform does not support FIFOs here: %v", err)
	}

	lock, err := Acquire(t.Context(), path, cmdUpdate)
	if err == nil {
		_ = lock.Release()
		t.Fatal("Acquire() on a FIFO succeeded, want a refusal")
	}
	if !errors.Is(err, errLockPathIsNotRegular) {
		t.Errorf("Acquire() error = %v, want it to wrap errLockPathIsNotRegular", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Acquire() error = %v, want it to name %s", err, path)
	}
}
