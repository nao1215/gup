//go:build !windows

package lockfile

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// openLockFile opens the lock file at path, creating it when it is not there
// yet, and returns it together with the identity the filesystem gives it.
//
// The open is O_NOFOLLOW, and that is the whole of the symlink defence. gup
// truncates the lock file to write the owner record into it, so a lock path
// somebody replaced with a link - `gup.json.lock -> ~/.ssh/authorized_keys` -
// would be a way to empty an arbitrary file that gup has permission to write.
// Checking for a link before opening would not fix it: whoever can create the
// link can create it in the window between the check and the open. O_NOFOLLOW
// moves the check inside the kernel's own open, where there is no window: either
// the final component is not a link and gup has the file it named, or the open
// fails outright.
//
// The kind of file is then read back from the descriptor rather than from the
// path, for the same reason. A FIFO or a device at the lock path is not a lock
// file, and answering that question about the thing already open answers it
// about the thing gup is going to write.
//
// O_NONBLOCK is there so the open itself cannot hang: a FIFO or a device at the
// lock path would otherwise block inside the kernel, where the acquisition
// timeout cannot reach it, on a path gup is about to reject anyway. It is
// cleared again below, because it means something to later reads and writes and
// nothing to the regular file this ends up with.
func openLockFile(path string) (*os.File, fileID, error) {
	flags := unix.O_RDWR | unix.O_CREAT | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	for {
		fd, err := unix.Open(path, flags, uint32(lockFileMode))
		if errors.Is(err, unix.EINTR) {
			// A signal arrived before the kernel answered. Nothing is open, so
			// asking again is the whole recovery.
			continue
		}
		if err != nil {
			return nil, fileID{}, classifyOpenError(err)
		}

		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil {
			_ = unix.Close(fd)
			return nil, fileID{}, err
		}
		if kind := stat.Mode & unix.S_IFMT; kind != unix.S_IFREG {
			_ = unix.Close(fd)
			if kind == unix.S_IFDIR {
				return nil, fileID{}, errLockPathIsDirectory
			}
			return nil, fileID{}, errLockPathIsNotRegular
		}
		if err := unix.SetNonblock(fd, false); err != nil {
			_ = unix.Close(fd)
			return nil, fileID{}, err
		}
		file := os.NewFile(uintptr(fd), path)
		return file, fileID{device: widen(stat.Dev), index: widen(stat.Ino)}, nil
	}
}

// widen brings st_dev and st_ino to the one shape fileID compares them in.
//
// It is generic because the supported platforms do not agree on how to spell
// them: st_dev is uint64 on Linux and int32 on macOS, and a plain conversion
// would be redundant on one and required on the other. These are identities
// rather than quantities, so widening never loses anything that mattered.
func widen[T ~int32 | ~uint32 | ~int64 | ~uint64](value T) uint64 {
	return uint64(value)
}

// classifyOpenError turns the errno an open failed with into the reason gup
// reports, so a refused symlink reads as a refused symlink rather than as
// "too many levels of symbolic links".
func classifyOpenError(err error) error {
	switch {
	case errors.Is(err, unix.ELOOP), errors.Is(err, unix.EMLINK):
		// ELOOP is what Linux returns for O_NOFOLLOW on a link; the BSDs, macOS
		// among them, return EMLINK for the same refusal.
		return errLockPathIsSymlink
	case errors.Is(err, unix.EISDIR):
		return errLockPathIsDirectory
	default:
		return err
	}
}

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
