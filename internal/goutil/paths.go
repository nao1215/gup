package goutil

import (
	"errors"
	"fmt"
	"os"
)

// GoPaths has $GOBIN and $GOPATH.
type GoPaths struct {
	// GOBIN is $GOBIN
	GOBIN string
	// GOPATH is $GOPATH
	GOPATH string
	// TmpPath is tmporary path for dry run
	TmpPath string
}

// NewGoPaths return GoPaths instance.
func NewGoPaths() *GoPaths {
	return &GoPaths{
		GOBIN:  goBin(),
		GOPATH: goPath(),
	}
}

// StartDryRunMode change the GOBIN or GOPATH settings to install the binaries in the temporary directory.
func (gp *GoPaths) StartDryRunMode() error {
	tmpDir, err := osMkdirTemp("", "")
	if err != nil {
		return err
	}
	gp.TmpPath = tmpDir

	switch {
	case gp.GOBIN != "":
		if err := os.Setenv(keyGoBin, tmpDir); err != nil {
			// Avoid leaking the temp dir when the env mutation fails.
			_ = gp.removeTmpDir()
			// Wrap error to avoid OS dependent error message during testing.
			return fmt.Errorf(
				"failed to set GOBIN to env variable. key: %v, value: %v: %w",
				keyGoBin, tmpDir, err,
			)
		}
	case gp.GOPATH != "":
		if err := os.Setenv(keyGoPath, tmpDir); err != nil {
			_ = gp.removeTmpDir()
			return fmt.Errorf(
				"failed to set GOPATH to env variable. key: %v, value: %v: %w",
				keyGoPath, tmpDir, err,
			)
		}
	default:
		_ = gp.removeTmpDir()
		return errors.New("$GOPATH and $GOBIN is not set")
	}
	return nil
}

// EndDryRunMode restore the GOBIN or GOPATH settings and remove the temporary
// directory. The temp dir is always removed, even when restoring the env
// variable fails, so a failed restore does not leak the directory (see issue
// #297). Any restore and removal errors are joined into the returned error.
func (gp *GoPaths) EndDryRunMode() error {
	var restoreErr error
	switch {
	case gp.GOBIN != "":
		if err := os.Setenv(keyGoBin, gp.GOBIN); err != nil {
			// Wrap error to avoid OS dependent error message during testing.
			restoreErr = fmt.Errorf(
				"failed to set GOBIN to env variable. key: %v, value: %v: %w",
				keyGoBin, gp.GOBIN, err,
			)
		}
	case gp.GOPATH != "":
		if err := os.Setenv(keyGoPath, gp.GOPATH); err != nil {
			restoreErr = fmt.Errorf(
				"failed to set GOPATH to env variable. key: %v, value: %v: %w",
				keyGoPath, gp.GOPATH, err,
			)
		}
	default:
		restoreErr = errors.New("$GOPATH and $GOBIN is not set")
	}

	var removeErr error
	if err := gp.removeTmpDir(); err != nil {
		removeErr = fmt.Errorf("temporary directory for dry run remains: %w", err)
	}

	return errors.Join(restoreErr, removeErr)
}

// removeTmpDir remove tmporary directory for dry run.
func (gp *GoPaths) removeTmpDir() error {
	if gp.TmpPath != "" {
		return os.RemoveAll(gp.TmpPath)
	}
	return nil
}
