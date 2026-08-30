//go:build windows

package lockfile

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// The byte range the lock is taken on.
//
// LockFileEx locks a RANGE, and a range another process holds exclusively also
// refuses that process's reads. The owner record gup writes for the "who is
// running" message lives at the start of the file, so locking from offset zero
// would make the one thing a waiter needs to read the one thing it cannot. The
// lock is therefore taken on a single byte far past any content the file will
// ever hold: 2^62, which no lock file reaches and no read touches.
const (
	lockOffsetLow  = 0
	lockOffsetHigh = 1 << 30 // together: byte 2^62
	lockByteCount  = 1
)

// lockRange is the overlapped structure naming the byte both operations act on.
func lockRange() *windows.Overlapped {
	return &windows.Overlapped{Offset: lockOffsetLow, OffsetHigh: lockOffsetHigh}
}

// tryLockFile takes an exclusive kernel lock on an open lock file without
// waiting, reporting whether it was granted.
//
// LockFileEx is the whole of the exclusion on Windows. The lock belongs to the
// file handle, so the kernel drops it when the handle closes - including the
// close every process gets when it dies, however it dies. That is what makes a
// crashed gup impossible to distinguish from a finished one: there is no
// leftover state to judge, and therefore no staleness rule, no heartbeat and no
// take-over.
//
// LOCKFILE_FAIL_IMMEDIATELY is what turns the call into a try: without it a
// contended lock blocks in the kernel, where neither the caller's deadline nor
// its context could reach it.
func tryLockFile(file *os.File) (bool, error) {
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, lockByteCount, 0, lockRange())
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, windows.ERROR_LOCK_VIOLATION), errors.Is(err, windows.ERROR_IO_PENDING):
		// Held by somebody else. ERROR_IO_PENDING is what an immediate-failure
		// request reports when the lock could only have been granted by waiting.
		return false, nil
	default:
		return false, err
	}
}

// unlockFile drops the kernel lock. Closing the handle would do it too; this is
// called first so the release is a deliberate step rather than a side effect of
// cleanup.
func unlockFile(file *os.File) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, lockByteCount, 0, lockRange())
}
