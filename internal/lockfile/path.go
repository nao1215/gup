// Path and identity: which file a lock is for, which file a name reaches, and
// which of the two a message should say.
//
// Everything here is about names being a poor way to talk about files. A lock is
// keyed on the identity the filesystem gives a file, never on the path that
// found it, because one file has many names - a symlinked directory, a relative
// path, an 8.3 alias, a different capitalization on macOS and Windows - and a
// lock that two names disagree about is not a lock. What the paths are still for
// is the mkdir, the open, and every message a user reads.

package lockfile

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/gup/internal/fileutil"
)

// DirLockName is the lock file guarding a binary directory. The leading dot
// keeps it out of gup's own $GOBIN listing, which skips dot-prefixed entries
// (see goutil.BinaryPathList), so the lock cannot be mistaken for a tool.
const DirLockName = ".gup.lock"

// IsReservedName reports whether name is a file gup keeps in a directory it
// manages, rather than something a user installed there.
//
// $GOBIN holds the lock guarding $GOBIN, and `gup remove` deletes files from
// $GOBIN by name. Without this, `gup remove .gup.lock --force` deletes the lock
// of the very command running it: the kernel lock survives (it lives on the open
// descriptor, not on the name), but the next gup creates a fresh file at the
// free name and locks that instead, and the two run side by side. The name is
// reserved rather than merely hidden, because "you cannot see it" is not the
// same as "you cannot name it".
//
// The comparison ignores case because macOS and Windows do: on a case-insensitive
// filesystem `.GUP.LOCK` opens the same file. Trailing dots and spaces are
// stripped for the same reason: Win32 strips them before the name reaches the
// filesystem, so `.gup.lock.` and `.gup.lock ` open the very file this refuses.
// Both foldings are applied on every platform - no one installs a tool by any of
// those names, and a rule that means different things per operating system is a
// rule that gets tested on one of them.
//
// This is the readable half of the refusal, not the load-bearing one. A name can
// reach a file in ways no amount of string folding covers - an 8.3 short name on
// NTFS, a hard link, a spelling a future Windows normalizes some other way - so
// `gup remove` also compares the file it is about to delete with the lock file
// itself (see SameFile). This gives that refusal its explanation; SameFile makes
// it true.
func IsReservedName(name string) bool {
	return strings.EqualFold(foldFileName(name), DirLockName)
}

// foldFileName reduces a file name the way the filesystem will before it decides
// which file the name means: surrounding whitespace goes, and so do trailing
// dots and spaces, which Win32 discards.
func foldFileName(name string) string {
	return strings.TrimRight(strings.TrimSpace(name), ". \t")
}

// SameFile reports whether two paths name one file on disk.
//
// It is what makes `gup remove` refuse to delete the lock guarding the $GOBIN it
// is deleting from, whatever the user typed: a name normalization gup does not
// know about, an 8.3 short name, a hard link. Removing that file would not
// release the kernel lock - the lock is on an open descriptor, not on a name -
// but it would free the NAME, and the next gup would create a fresh file there
// and lock that instead, leaving two commands rewriting one $GOBIN each
// believing it had the directory to itself.
//
// Both paths are stat'ed without following a final symbolic link, so a link that
// POINTS at the lock file is normally not the lock file: deleting the link
// leaves the lock where it is, and refusing it would refuse something harmless.
// Where a platform cannot tell a link's own identity from its target's, such a
// link is refused as well - an over-refusal of a deletion that could not have
// hurt anything, which is the safe direction to be wrong in. A path that cannot
// be stat'ed is not the same file as anything, because a file that is not there
// is not the one being protected.
func SameFile(a, b string) bool {
	first, err := os.Lstat(a)
	if err != nil {
		return false
	}
	second, err := os.Lstat(b)
	if err != nil {
		return false
	}
	return os.SameFile(first, second)
}

// normalizePath resolves a lock path to a cleaned absolute one.
//
// Every entry point normalizes through here. What the result is used for is the
// mkdir, the open, the order a set is acquired in, and every message naming the
// lock - not the identity of the lock itself, which comes from the open file.
// Failing is better than falling back to the relative path: a lock file the
// working directory can move out from under is not a lock.
func normalizePath(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("can not resolve the gup lock path %s: %w", path, err)
	}
	return abs, nil
}

// PathForDir returns the lock file guarding a directory whose contents gup
// mutates, such as a $GOBIN or a migrate destination.
//
// It needs no canonicalization, and that is a property of where it puts the
// lock rather than an omission. The lock file lives INSIDE the directory it
// guards, so every name that reaches the directory - a symlinked parent, an 8.3
// alias, a different capitalization on macOS and Windows - reaches the same
// .gup.lock inside it. The path strings differ; the file does not, which is the
// only thing a lock is keyed on.
func PathForDir(dir string) string { return filepath.Join(dir, DirLockName) }

// lockFileSuffix is what turns the name of a file gup rewrites into the name of
// the lock guarding it.
const lockFileSuffix = ".lock"

// PathForFile returns the lock file guarding a single file gup rewrites, such as
// a gup.json. It sits beside the file so the lock travels with the resource,
// which is what makes two processes with different configuration directories but
// the same --file contend for the same lock.
//
// Unlike PathForDir, this one has to be canonicalized, and the reason is the
// SIBLING placement it depends on. The lock is a second file in the same
// directory whose name is built from the resource's own, so the name gup was
// given decides which file the lock is - and one file answers to many names:
//
//   - On Windows, NTFS keeps an 8.3 alias for a long name, so a gup.json also
//     answers to something like GUP~1.JSO, whose sibling lock would be
//     GUP~1.JSO.lock rather than gup.json.lock.
//   - Also on Windows, Win32 strips trailing dots and spaces before the
//     filesystem ever sees a name, so `--file gup.json.` writes gup.json - while
//     its sibling lock, gup.json..lock, has no trailing dot to strip and is
//     therefore a different file from gup.json.lock.
//   - Everywhere, a hard link makes two equally real names for one file, in two
//     different directories if you like, and neither is more canonical than the
//     other.
//
// Every one of those splits the lock: two gups writing one file, each holding a
// lock the other cannot see, which is the exact failure the lock exists to
// prevent and the one it would fail at silently.
//
// The first two are answered by asking the operating system which file the name
// means and building the lock name from the answer (see canonicalTargetPath).
// The third has no answer to give - a file with two names has no canonical one -
// so it is REFUSED rather than guessed at. Continuing unlocked is not on the
// table: an unprotected write is what this package exists to make impossible,
// and a command that stops with a reason a user can act on is strictly better
// than one that quietly runs alongside another.
//
// A symbolic link at file is followed first, so a gup.json linked into place by
// a dotfile manager locks the file the write lands on rather than the link.
func PathForFile(file string) (string, error) {
	resolved, err := fileutil.ResolveSymlinkTarget(file)
	if err != nil {
		return "", fmt.Errorf("can not resolve the gup lock path for %s: %w", file, err)
	}
	normalized, err := normalizePath(resolved)
	if err != nil {
		return "", err
	}
	canonical, err := canonicalTargetPath(normalized)
	if err != nil {
		return "", fmt.Errorf("can not lock %s: %w", normalized, err)
	}
	return canonical + lockFileSuffix, nil
}

// fileID is the identity a filesystem gives a file, as opposed to the many paths
// that may reach it. It is what this package keys locks on: two spellings of one
// file - through a symlinked directory, a relative path, a differently
// capitalized $GOBIN - produce one fileID, and a lock is one per file.
type fileID struct {
	// device is the volume the file lives on: st_dev on Unix, the volume serial
	// number on Windows.
	device uint64
	// index is the file's number within that volume: the inode on Unix, and on
	// Windows the file index, which the API hands over as two 32-bit halves.
	index uint64
}

// The reasons a lock path cannot be used, kept as values so a caller can tell
// them apart and Acquire can wrap them all into one "can not open" message.
var (
	// errLockPathIsSymlink refuses to write through a link. gup created its lock
	// files, so a link where one should be is somebody else's doing, and
	// truncating whatever it points at is exactly what must not happen.
	errLockPathIsSymlink = errors.New(
		"the lock path is a symbolic link, and gup will not write through one:" +
			" delete it, or point gup at a directory it owns")
	// errLockPathIsHardLink refuses a lock file that is a second name for some
	// other file. A symbolic link announces itself; a hard link does not - the two
	// names are equally real, and the one gup opens is a perfectly ordinary
	// regular file. `gup.json.lock` hard-linked onto `gup.json` would therefore
	// pass every check a symlink fails, and the first thing gup does with a lock
	// it has taken is truncate it to write the owner record: the configuration
	// file would be emptied and the owner JSON written over it, by a command that
	// reported success. The link count is what tells the two apart, because a lock
	// file gup created has exactly one name.
	errLockPathIsHardLink = errors.New(
		"the lock path is a hard link to another file, and gup will not truncate a file it does not own:" +
			" delete the lock file while no gup is running, or point gup at a directory it owns")
	// errLockPathIsDirectory names the mistake of locking a directory itself
	// rather than the lock file inside it.
	errLockPathIsDirectory = errors.New("the lock path is a directory, not a file")
	// errLockPathIsNotRegular covers the rest: a FIFO, a socket, a device.
	errLockPathIsNotRegular = errors.New("the lock path is not a regular file")
	// errTargetHasSecondName refuses to derive a lock from the name of a file
	// that has more than one. PathForFile builds the lock's name out of the
	// resource's, so a hard-linked gup.json reached by its other name yields a
	// different lock file: two gups rewriting one configuration, each holding a
	// lock the other cannot see. Unlike an 8.3 alias or a trailing dot, there is
	// no canonical name to rewrite this one to - both names are equally real - so
	// the only honest answers are to refuse or to run unprotected, and running
	// unprotected is the thing this package exists to prevent.
	errTargetHasSecondName = errors.New(
		"the file has a second name (a hard link), so gup can not tell which lock protects it:" +
			" remove the extra name while no gup is running, or point gup at a file that has only one")
)

// compareFileID orders two files the way every process ordering the same two
// files will, whatever names they reached them by. The volume comes first and
// the number within it second, which is an order with no meaning at all beyond
// being the same one everywhere - which is the entire requirement.
func compareFileID(a, b fileID) int {
	if a.device != b.device {
		return cmp.Compare(a.device, b.device)
	}
	return cmp.Compare(a.index, b.index)
}
