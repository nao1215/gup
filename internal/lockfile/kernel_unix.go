//go:build !windows

package lockfile

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLockFile takes an exclusive kernel lock on an open lock file without
// waiting, reporting whether it was granted.
//
// flock(2) is the whole of the exclusion on Unix. The lock lives on the open
// file description in the kernel, not on anything gup writes, so it is released
// by the kernel when the descriptor closes - including the close every process
// gets when it dies, however it dies. That is what makes a crashed gup
// impossible to distinguish from a finished one: there is no leftover state to
// judge, and therefore no staleness rule, no heartbeat and no take-over.
//
// It is per-descriptor rather than per-process, so two descriptors opened by one
// gup exclude each other exactly as two processes would. The in-process registry
// exists to give that case a clearer message, not to make it safe.
func tryLockFile(file *os.File) (bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, unix.EWOULDBLOCK):
		// Held by somebody else. EAGAIN is the same errno on every platform gup
		// builds for, so this one case covers both spellings.
		return false, nil
	case errors.Is(err, unix.EINTR):
		// A signal arrived before the kernel answered. Nothing is held, and the
		// caller's retry loop asks again.
		return false, nil
	default:
		return false, err
	}
}

// unlockFile drops the kernel lock. Closing the file would do it too; this is
// called first so the release is a deliberate step rather than a side effect of
// cleanup.
func unlockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
