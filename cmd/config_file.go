package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
)

var writeConfFile = config.WriteConfFile //nolint:gochecknoglobals // swapped in tests

var renameFunc = os.Rename //nolint:gochecknoglobals // swapped in tests to simulate rename failures

func writeConfigFile(path string, pkgs []goutil.Package) (err error) {
	path = filepath.Clean(path)
	// Reject an existing directory before any temp/backup files are created, so
	// a mistaken path (e.g. 'export --file <dir>') cannot replace a directory
	// tree with a regular file (#367).
	if fileutil.IsDir(path) {
		return fmt.Errorf("%s is a directory, not a file", path)
	}
	// Resolve symlinks at the destination so the atomic rename rewrites the link's
	// target rather than replacing the link itself with a regular file. Dotfile
	// managers (stow, chezmoi, yadm) commonly symlink gup.json into place; this
	// preserves the link - including a dangling link whose target does not exist
	// yet, the state right after a dotfile manager links a file before its first
	// write.
	resolvedPath, err := fileutil.ResolveSymlinkTarget(path)
	if err != nil {
		return fmt.Errorf("can not resolve config path %s: %w", path, err)
	}
	path = resolvedPath
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, fileutil.FileModeCreatingDir); err != nil {
		return fmt.Errorf("%s: %w", "can not make config directory", err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("%s %s: %w", "can't create temp file for", path, err)
	}
	tmpPath := file.Name()
	defer func() {
		if file != nil {
			if closeErr := file.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if err = writeConfFile(file, pkgs); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("%s %s: %w", "can't sync temp config file for", path, err)
	}
	if err = file.Close(); err != nil {
		file = nil
		return fmt.Errorf("%s %s: %w", "can't close temp config file for", path, err)
	}
	file = nil

	if err = renameWithReplace(tmpPath, path); err != nil {
		return fmt.Errorf("%s %s: %w", "can't update", path, err)
	}

	return nil
}

// renameWithReplace puts the freshly written file in place of the old one.
//
// The plain rename is what everything else in gup relies on: it replaces the
// destination atomically on every platform gup targets (Go asks Windows for
// FILE_RENAME_REPLACE_IF_EXISTS), so a reader holding no lock sees either the
// whole previous file or the whole next one, and never the gap between them.
//
// The fallbacks below exist because a destination can refuse to be replaced.
// They are ordered by how much of that guarantee they keep: clearing a read-only
// destination and replacing it atomically keeps all of it, and the backup swap -
// which moves the old file away before putting the new one in its place, leaving
// the path empty in between - is the last resort it has always been.
func renameWithReplace(src, dst string) error {
	err := renameFunc(src, dst)
	if err == nil {
		return nil
	}
	// Asked before the platform gate below, because a read-only destination is a
	// property of the file rather than of the operating system, and clearing it
	// keeps the replace atomic where the swap could not.
	if replaceErr := replaceReadOnlyDestination(src, dst); replaceErr == nil {
		return nil
	}
	if !shouldRetryRenameWithReplace(err, dst) {
		return err
	}
	return renameWithBackupSwap(src, dst)
}

// replaceReadOnlyDestination retries the atomic replace with the destination's
// read-only bit cleared, and puts that bit back on the file that replaces it.
//
// This is the case the backup swap was written for. On Windows a read-only
// destination makes the replace fail with a permission error, and a swap makes
// the write succeed at the cost of a moment where gup.json does not exist -
// which the unlocked readers (`gup check`, `gup list`, `gup import`) would read
// as "no config". Clearing the bit for the length of one rename costs nothing
// and keeps the file continuously present.
func replaceReadOnlyDestination(src, dst string) error {
	info, err := os.Stat(dst)
	if err != nil {
		return err
	}
	const ownerWrite = 0o200
	if info.Mode().Perm()&ownerWrite != 0 {
		// Not a read-only destination, so this is not what stopped the replace.
		return errNotReadOnlyDestination
	}
	if err := os.Chmod(dst, info.Mode()|ownerWrite); err != nil {
		return err
	}
	if err := renameFunc(src, dst); err != nil {
		// Put the destination back as it was found: the write failed, so nothing
		// about the file the user has should have changed.
		_ = os.Chmod(dst, info.Mode())
		return err
	}
	// The new file inherits the temp file's mode, so the destination's read-only
	// intent is restored onto it rather than silently dropped.
	return os.Chmod(dst, info.Mode())
}

// errNotReadOnlyDestination reports that the replace failed for some reason
// other than a destination that cannot be written, so clearing permissions is
// not the retry to make.
var errNotReadOnlyDestination = errors.New("the destination is not read-only")

func renameWithBackupSwap(src, dst string) error {
	backupPath, err := prepareBackupPath(dst)
	if err != nil {
		return err
	}

	if err = renameFunc(dst, backupPath); err != nil {
		return err
	}
	if err = renameFunc(src, dst); err != nil {
		if restoreErr := renameFunc(backupPath, dst); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("can't restore original file %s after failed update: %w", dst, restoreErr))
		}
		return err
	}

	_ = os.Remove(backupPath)
	return nil
}

func prepareBackupPath(dst string) (string, error) {
	backupFile, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".bak-*")
	if err != nil {
		return "", err
	}
	backupPath := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		_ = os.Remove(backupPath)
		return "", err
	}
	if err := os.Remove(backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

func shouldRetryRenameWithReplace(renameErr error, dst string) bool {
	if os.IsExist(renameErr) {
		return true
	}
	if runtime.GOOS != goosWindows {
		return false
	}
	_, err := os.Stat(dst)
	return err == nil
}
