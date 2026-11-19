//go:build windows

package cmd

import "os"

// applyUmask is a no-op on Windows, which has no POSIX umask; os.Chmod only
// toggles the read-only bit there, so the requested mode is returned unchanged.
func applyUmask(perm os.FileMode) os.FileMode {
	return perm
}
