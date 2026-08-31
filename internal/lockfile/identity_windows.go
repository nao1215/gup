//go:build windows

package lockfile

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// finalPathFlags asks GetFinalPathNameByHandle for the normalized path with a
// drive letter: FILE_NAME_NORMALIZED (0x0) with VOLUME_NAME_DOS (0x0). Both are
// zero, and both are named here because "0" at a call site says nothing.
const finalPathFlags = 0

// canonicalTargetPath returns the one spelling of path that every gup process
// derives the same lock file name from.
//
// Windows is where a file answers to the most names, and two of them split a
// sibling lock silently:
//
//   - NTFS keeps an 8.3 alias for a long name, so a gup.json is also GUP~1.JSO
//     (and a directory in the path has one too). Its sibling lock would be
//     GUP~1.JSO.lock, a different file from gup.json.lock.
//   - Win32 strips trailing dots and spaces before the filesystem sees a name,
//     so `--file gup.json.` writes gup.json - while the lock built from it,
//     gup.json..lock, ends in "lock" and has nothing to strip, so it is a
//     different file from gup.json.lock.
//
// The operating system is the only thing that knows which file a name means, so
// it is asked: an open handle plus GetFinalPathNameByHandle returns the long,
// normalized, drive-lettered path of the file itself, with every alias in every
// component resolved. Two processes that spelled the name differently get the
// same answer, and therefore the same lock.
//
// A hard link has no such answer - two names for one file are equally real - so
// it is refused instead (see errTargetHasSecondName).
//
// A path that does not exist yet has no alias to resolve, because 8.3 names and
// normalization describe files that are there. Its deepest existing ancestor is
// canonicalized instead, and the components below it are folded the way Win32
// would fold them, so `--file <dir>/gup.json.` and `--file <dir>/gup.json` agree
// on the lock before either has created anything.
func canonicalTargetPath(path string) (string, error) {
	final, links, isDir, err := describeByHandle(path)
	if err == nil {
		if !isDir && links > 1 {
			return "", errTargetHasSecondName
		}
		return final, nil
	}
	if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return "", err
	}

	parent := filepath.Dir(path)
	if parent == path {
		// A volume root that could not be opened: there is nothing above it to
		// canonicalize, so the cleaned path is the best name there is.
		return path, nil
	}
	canonicalParent, err := canonicalTargetPath(parent)
	if err != nil {
		return "", err
	}
	base := filepath.Base(path)
	if folded := foldFileName(base); folded != "" {
		base = folded
	}
	return filepath.Join(canonicalParent, base), nil
}

// describeByHandle opens path for its metadata alone and reports the normalized
// path Windows itself gives the file, its link count, and whether it is a
// directory.
//
// The open asks for FILE_READ_ATTRIBUTES only, so it works on a file another
// process is writing and on one this user may not read, and it shares
// everything, so it never becomes the reason a concurrent gup fails.
// FILE_FLAG_BACKUP_SEMANTICS is what lets a directory be opened at all, which
// the ancestor walk above needs. Reparse points are deliberately NOT opened as
// themselves here: a gup.json linked into place must lock the file the write
// lands on, which is the target.
func describeByHandle(path string) (string, uint32, bool, error) {
	name, err := windows.UTF16PtrFromString(extendedPath(path))
	if err != nil {
		return "", 0, false, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", 0, false, err
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return "", 0, false, err
	}
	isDir := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0

	final, err := finalPath(handle)
	if err != nil {
		return "", 0, false, err
	}
	return final, info.NumberOfLinks, isDir, nil
}

// finalPath reads the normalized path of an open handle, growing the buffer when
// the first ask was too small. GetFinalPathNameByHandle reports the length it
// needs in exactly that case, so the retry happens at most once.
func finalPath(handle windows.Handle) (string, error) {
	size := uint32(windows.MAX_PATH)
	for range 2 {
		buf := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], size, finalPathFlags)
		if err != nil {
			return "", err
		}
		if n < size {
			return trimExtendedPrefix(windows.UTF16ToString(buf[:n])), nil
		}
		// Too small: n is the length it needs, plus room for the terminator.
		size = n + 1
	}
	return "", errors.New("the normalized path of the gup lock target kept growing")
}

// trimExtendedPrefix turns the \\?\ form GetFinalPathNameByHandle always returns
// back into the ordinary spelling.
//
// The prefix is not dropped for cosmetics. It travels into every message naming
// the lock, and it is the same prefix extendedPath puts back when a path is long
// enough to need it - so keeping it would leave gup reporting `\\?\C:\...` for
// short paths that never needed the form, while changing nothing about which
// file is opened.
func trimExtendedPrefix(path string) string {
	if rest, found := strings.CutPrefix(path, `\\?\UNC\`); found {
		return `\\` + rest
	}
	if rest, found := strings.CutPrefix(path, `\\?\`); found {
		return rest
	}
	return path
}
