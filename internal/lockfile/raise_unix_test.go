//go:build !windows

package lockfile

import (
	"os"
	"syscall"
	"testing"
)

// raiseInterrupt sends this process a real SIGINT, so the test exercises the
// signal.Notify registration and the guard's goroutine rather than only the
// channel behind them.
//
// It is safe to send a process-wide signal from a test because the guard
// releases EVERY registered lock: Go pauses parallel tests until all serial
// tests in the package have finished, and this test is serial, so no other
// test's lock can be registered while the signal is delivered.
func raiseInterrupt(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT to this process: %v", err)
	}
}
