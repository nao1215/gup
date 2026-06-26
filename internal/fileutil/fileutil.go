// Package fileutil provides simple file system helper functions.
package fileutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// FileModeCreatingDir is the permission used for creating directories.
	FileModeCreatingDir fs.FileMode = 0750
	// FileModeCreatingFile is the permission used for creating files.
	FileModeCreatingFile fs.FileMode = 0600
)

// IsFile reports whether the path exists and is a file.
func IsFile(path string) bool {
	stat, err := os.Stat(path)
	return (err == nil) && (!stat.IsDir())
}

// IsDir reports whether the path exists and is a directory.
func IsDir(path string) bool {
	stat, err := os.Stat(path)
	return (err == nil) && (stat.IsDir())
}

// IsHiddenFile reports whether the path is a hidden file (name starts with ".").
func IsHiddenFile(filePath string) bool {
	_, name := filepath.Split(filePath)
	return IsFile(filePath) && strings.HasPrefix(name, ".")
}

// ResolveSymlinkTarget follows the chain of symlinks at path and returns the
// final target, even when that target does not exist yet (a dangling symlink -
// e.g. a dotfile manager that linked a config file before its target was ever
// written). This lets an atomic write (temp file + rename) create or replace the
// link's target rather than the link itself, preserving the symlink.
// filepath.EvalSymlinks cannot be used because it requires the whole path to
// exist and so fails on a dangling link. A non-symlink path (including a missing
// plain file) is returned unchanged; intermediate directory symlinks need no
// handling because the OS resolves them transparently during create/rename.
// Exhausting the hop limit means a symlink cycle: that is reported as an error so
// the caller aborts, rather than returning a still-symlink path that the atomic
// rename would then clobber into a regular file.
func ResolveSymlinkTarget(path string) (string, error) {
	for range 255 {
		info, err := os.Lstat(path)
		if err != nil {
			// path does not exist yet (e.g. a dangling link's final target): it is
			// the destination to create, so stop here with no error.
			return path, nil //nolint:nilerr // a missing target is the intended write path
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		dest, err := os.Readlink(path)
		if err != nil {
			// The link is unreadable; fall back to treating path as the destination
			// so the caller surfaces a concrete write error rather than this one.
			return path, nil //nolint:nilerr // fall back to writing at path
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		path = filepath.Clean(dest)
	}
	return "", fmt.Errorf("symlink chain too deep (possible cycle) at %s", path)
}
