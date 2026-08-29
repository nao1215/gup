//go:build windows

package lockfile

import (
	"os"
	"testing"
)

// raiseInterrupt stands in for a real SIGINT on Windows, which has no way for a
// process to send a console interrupt to itself. The guard's goroutine and its
// release-everything behavior are still exercised; what is not covered here is
// the signal.Notify delivery itself, which the end-to-end suite checks by
// interrupting a real gup process.
func raiseInterrupt(t *testing.T) {
	t.Helper()
	signalGuard.mu.Lock()
	ch := signalGuard.ch
	signalGuard.mu.Unlock()
	if ch == nil {
		t.Fatal("no signal watcher is installed")
	}
	ch <- os.Interrupt
}
