//go:build windows

package lockfile

import (
	"errors"
	"syscall"
)

const (
	// stillActive is the exit code GetExitCodeProcess reports for a process that
	// has not exited yet (STILL_ACTIVE / STATUS_PENDING).
	stillActive = 259
	// errorInvalidParameter is Win32's ERROR_INVALID_PARAMETER, what OpenProcess
	// returns for a PID that names no process. The Go standard library declares
	// it unexported (syscall._ERROR_INVALID_PARAMETER), so the value is repeated
	// here rather than reached for.
	errorInvalidParameter = syscall.Errno(87)
)

// processAlive reports whether a process with this PID currently exists.
//
// Windows has no kill(2), so the equivalent question is asked by opening a
// handle and reading the exit code. os.FindProcess is not usable for this: on
// Windows it succeeds for any PID it can open and its Signal method cannot probe
// a process this program did not start.
//
// The bias matches the Unix implementation: only a definite "this PID does not
// exist" answer makes a lock stale. Access denied means the process exists and
// belongs to someone else, and any other failure is treated as alive, because
// wrongly declaring a lock stale would let two gup processes run at once, while
// wrongly declaring it live only makes the caller wait out staleAfter.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER is what Windows returns for a PID that no longer
		// names a process; anything else (notably ERROR_ACCESS_DENIED) means the
		// process is there but out of reach.
		return !errors.Is(err, errorInvalidParameter)
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return true
	}
	return code == stillActive
}
