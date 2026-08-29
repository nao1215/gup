//go:build !windows

package lockfile

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process with this PID currently exists.
//
// Signal 0 performs the kernel's existence and permission checks without
// delivering anything, which is the POSIX way to ask this question and needs no
// CGO. EPERM means the process exists but belongs to another user - still alive,
// so the lock is not stale. ESRCH is the only answer that means "gone"; any
// other error is treated as alive, because wrongly declaring a lock stale would
// let two gup processes run at once, while wrongly declaring it live only makes
// the caller wait out staleAfter.
func processAlive(pid int) bool {
	// A non-positive PID is not a process: kill(2) reads 0 and negative values as
	// "the whole process group" and "every process I may signal", which would
	// answer a question nobody asked and report a corrupt lock file as live.
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}
