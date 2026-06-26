//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// processUmask is the umask captured once at package initialization, while the
// program is still single-threaded. Reading the umask requires temporarily
// setting it (there is no read-only syscall), so capturing it here - rather than
// on every call - avoids mutating the process-global umask at a point where other
// goroutines might concurrently create files and observe a transient 0 mask. It
// is a var so tests can substitute a known value.
var processUmask = readProcessUmask() //nolint:gochecknoglobals // captured once at startup; overridden in tests

// readProcessUmask returns the current process umask, restoring it immediately.
func readProcessUmask() int {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return mask
}

// applyUmask returns perm with the cached process umask cleared, mirroring how
// os.Create (which opens with mode 0666) lets the OS apply the umask to a newly
// created file.
func applyUmask(perm os.FileMode) os.FileMode {
	// A umask only occupies the low permission bits, so the conversion cannot
	// overflow; mask to os.ModePerm to make that explicit for the conversion.
	return perm &^ (os.FileMode(processUmask) & os.ModePerm) //nolint:gosec // umask is a small permission bitmask
}
