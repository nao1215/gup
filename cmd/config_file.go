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

func renameWithReplace(src, dst string) error {
	if err := renameFunc(src, dst); err != nil {
		// Windows cannot overwrite an existing file with os.Rename.
		// Retry via destination backup swap when the destination likely exists.
		if !shouldRetryRenameWithReplace(err, dst) {
			return err
		}
		return renameWithBackupSwap(src, dst)
	}
	return nil
}

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
