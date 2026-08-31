//go:build !windows

package lockfile

import (
	"errors"

	"golang.org/x/sys/unix"
)

// canonicalTargetPath returns the one spelling of path that every gup process
// derives the same lock file name from.
//
// On Unix there is nothing to rewrite, and that is the whole finding rather than
// a gap. A name reaches a file through symbolic links, which the caller has
// already followed, and through nothing else: there is no 8.3 alias, no
// normalization the kernel performs on the way in, and a case-insensitive mount
// answers to a differently capitalized name with the SAME file - so the lock
// beside it is the same file too, whichever spelling produced it.
//
// One alias is left, and it is the one that cannot be rewritten: a hard link.
// Two names for one inode are equally real, in two different directories if you
// like, so `--file a/gup.json` and `--file b/gup.json` would put their locks in
// two different directories and neither would be more canonical than the other.
// The link count is what says so, and a file that has a second name is refused
// rather than locked at a name that only sometimes means it. It is also a file
// gup could not write correctly anyway: gup replaces gup.json by renaming a
// temporary file over it, which breaks the link, so the other name would go on
// naming the contents from before the command ran.
//
// A path that is not there yet is its own canonical name. Nothing has a second
// name until it exists, and the file the command is about to create will be
// created at exactly this path.
func canonicalTargetPath(path string) (string, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) {
			return path, nil
		}
		return "", err
	}
	// The link count means "second name" only for a regular file. Directories
	// carry one per entry beneath them, which says nothing about aliases, and a
	// count of zero is what a filesystem that does not track them reports.
	if stat.Mode&unix.S_IFMT == unix.S_IFREG && widen(stat.Nlink) > 1 {
		return "", errTargetHasSecondName
	}
	return path, nil
}
