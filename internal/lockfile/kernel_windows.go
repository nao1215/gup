//go:build windows

package lockfile

import (
	"errors"
	"os"
	"strings"

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

// openLockFile opens the lock file at path, creating it when it is not there
// yet, and returns it together with the identity the filesystem gives it.
//
// FILE_FLAG_OPEN_REPARSE_POINT is the whole of the symlink defense, and it is
// the reason this goes through CreateFile rather than os.OpenFile. gup truncates
// the lock file to write the owner record into it, so a lock path somebody
// replaced with a symlink or a junction - `gup.json.lock` pointing at a file in
// the user's profile - would be a way to empty an arbitrary file gup can write.
// Looking for a reparse point before opening would not fix it, because whoever
// can create one can create it after the look. The flag moves the question
// inside the open: what comes back is a handle to the reparse point ITSELF, not
// to whatever it names, so the check below runs against the thing gup has, and
// nothing has been written to anything by the time it fails.
//
// A hard link needs a different answer, because it is not a reparse point at
// all: `gup.json.lock` linked onto `gup.json` opens as the ordinary file it is.
// The link count reported alongside the file's identity is what refuses it, for
// the reasons in errLockPathIsHardLink.
//
// The share mode admits every other opener. A lock file no second gup can open
// would report a sharing violation instead of the "another gup is running"
// message, which is the one thing the file is there to make possible.
func openLockFile(path string) (*os.File, fileID, error) {
	name, err := windows.UTF16PtrFromString(extendedPath(path))
	if err != nil {
		return nil, fileID{}, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		// Windows says little about why an open failed: a directory refuses it
		// with a bare access denial, and a reparse point opened for writing may
		// too. The path is consulted only to explain a failure that has already
		// happened - nothing was written, and nothing below depends on the answer
		// still being true - so the refusal reads the same whether the reparse
		// point was rejected here or by the attribute check further down.
		//nolint:gosec // G703: the path is gup's own lock file, already normalized
		// by normalizePath, and this only explains a failure that has happened.
		if info, statErr := os.Lstat(path); statErr == nil {
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				return nil, fileID{}, errLockPathIsSymlink
			case info.IsDir():
				return nil, fileID{}, errLockPathIsDirectory
			case !info.Mode().IsRegular():
				return nil, fileID{}, errLockPathIsNotRegular
			}
		}
		return nil, fileID{}, err
	}

	// What kind of handle this is has to be settled BEFORE anything asks for file
	// metadata: GetFileInformationByHandle is documented for files on disk, and a
	// character device or a pipe at the lock path would make it fail with an error
	// about the call rather than about the path. This is the counterpart of the
	// Unix side reading the file kind off the descriptor, and it is settled on the
	// handle for the same reason - it answers about the thing gup has open.
	kind, err := windows.GetFileType(handle)
	if err != nil || kind != windows.FILE_TYPE_DISK {
		_ = windows.CloseHandle(handle)
		return nil, fileID{}, errLockPathIsNotRegular
	}

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fileID{}, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(handle)
		return nil, fileID{}, errLockPathIsSymlink
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		_ = windows.CloseHandle(handle)
		return nil, fileID{}, errLockPathIsDirectory
	}
	// A file with more than one name is not a lock file gup made. NTFS has hard
	// links, and one pointed at a gup.json gives that file a second name ending in
	// .lock which no reparse-point check can object to: the handle is to an
	// ordinary file, and truncating it for the owner record would empty the user's
	// configuration. The link count comes from the same GetFileInformationByHandle
	// call as the identity, so it describes the handle gup holds rather than a
	// path that could have changed underneath it. A count of zero is not a second
	// name - a volume that does not track link counts reports it.
	if info.NumberOfLinks > 1 {
		_ = windows.CloseHandle(handle)
		return nil, fileID{}, errLockPathIsHardLink
	}
	return os.NewFile(uintptr(handle), path), fileID{
		device: uint64(info.VolumeSerialNumber),
		// The file index is one 64-bit number the API reports in halves.
		index: uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow),
	}, nil
}

// maxWin32Path is the length at which a path needs the extended-length form.
// MAX_PATH is 260; the cutoff is 12 lower because that is the limit Win32
// applies to the directory a file is created in, and it is the same number Go's
// own os package uses.
const maxWin32Path = 248

// extendedPath rewrites a long path into the \\?\ form CreateFile accepts.
// os.OpenFile does this for its callers; CreateFile, which openLockFile has to
// use instead, does not.
//
// It is applied only past the limit, because the prefix also turns OFF the
// normalization Win32 performs - trailing dots and spaces stop being stripped -
// and gup's short paths are better served by the ordinary rules. The path comes
// from normalizePath, so it is already absolute, cleaned, and backslash-spelled,
// which is what the prefixed form requires.
func extendedPath(path string) string {
	if len(path) < maxWin32Path || strings.HasPrefix(path, `\\?\`) {
		return path
	}
	// A UNC path spells its prefix differently: \\server\share becomes
	// \\?\UNC\server\share. \\.\ names a device, which a lock file never is,
	// so it is left alone rather than mangled into a UNC path.
	if unc, found := strings.CutPrefix(path, `\\`); found && !strings.HasPrefix(unc, `.\`) {
		return `\\?\UNC\` + unc
	}
	return `\\?\` + path
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
