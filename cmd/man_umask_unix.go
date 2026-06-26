//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// applyUmask returns perm with the current process umask cleared, mirroring how
// os.Create (which opens with mode 0666) lets the OS apply the umask to a newly
// created file. There is no read-only umask syscall, so we set it to 0 and
// immediately restore the previous value to read it; man generation runs
// sequentially, so the momentary change is safe.
func applyUmask(perm os.FileMode) os.FileMode {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	// A umask only occupies the low permission bits, so the conversion cannot
	// overflow; mask to os.ModePerm to make that explicit for the conversion.
	return perm &^ (os.FileMode(mask) & os.ModePerm) //nolint:gosec // umask is a small permission bitmask
}
