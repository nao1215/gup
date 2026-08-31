// The process-wide bookkeeping a lock needs to survive its own caller.
//
// Two registries live here, and neither is the lock. heldLocks keeps an acquired
// lock reachable, because the kernel lock lives on a descriptor inside it and an
// os.File closes itself from a finalizer - so a caller that dropped the value
// would have its lock released by the garbage collector while it worked.
// inProcessLocks serializes two acquisitions of one file inside this process,
// because the kernel lock is per descriptor and cannot say whose descriptor the
// other one is: without it, one gup asking twice reports itself as "another gup
// process".

package lockfile

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// heldLocks keeps every lock this process holds reachable for as long as it
// holds it.
//
// This is not bookkeeping; it is what makes the lock survive a caller that
// forgets about it. The kernel lock lives on the descriptor inside a Lock, and
// os.File closes itself from a finalizer once it becomes unreachable - so a
// caller that acquires a lock and drops the value would have the lock released
// on its behalf at the next garbage collection, silently, while it carried on
// working. That is the one outcome this package exists to prevent, and it must
// not depend on every caller holding the value carefully: a long-running holder
// that never intends to release is exactly the shape of code that drops it.
//
// The entry goes when Release does, so a released lock is collectable again. One
// that is never released stays reachable for the life of the process, which is
// precisely how long it is held.
var heldLocks = struct { //nolint:gochecknoglobals // process-wide by definition

	mu    sync.Mutex
	locks map[*Lock]struct{}
}{locks: map[*Lock]struct{}{}}

// rememberHeldLock keeps lock reachable until it is released.
func rememberHeldLock(lock *Lock) {
	heldLocks.mu.Lock()
	defer heldLocks.mu.Unlock()
	heldLocks.locks[lock] = struct{}{}
}

// forgetHeldLock lets a released lock be collected.
func forgetHeldLock(lock *Lock) {
	heldLocks.mu.Lock()
	defer heldLocks.mu.Unlock()
	delete(heldLocks.locks, lock)
}

// inProcessLocks serializes acquisitions of the same path inside one process.
// The kernel lock alone would report a second acquisition in this process as a
// foreign conflict, since it is taken per descriptor and cannot say whose
// descriptor the other one is.
//
// It is keyed on the FILE rather than on the path, so two goroutines that
// reached one lock file by different names - a symlinked directory, a relative
// path - queue behind each other instead of racing on the kernel lock and
// reporting this process as another one.
var inProcessLocks = struct { //nolint:gochecknoglobals // process-wide by definition

	mu   sync.Mutex
	held map[fileID]chan struct{}
}{held: map[fileID]chan struct{}{}}

// acquireInProcess claims the in-process slot for a lock file and returns the
// function that hands it back. path is carried only for the message. The wait is
// bounded by the same timeout the cross-process wait uses: an unbounded wait here
// would turn a caller that forgot to release into a hang with no diagnosis,
// which is worse than a clear timeout.
func acquireInProcess(ctx context.Context, id fileID, path string) (func(), error) {
	timeout := time.NewTimer(waitTimeout())
	defer timeout.Stop()

	for {
		inProcessLocks.mu.Lock()
		if released, held := inProcessLocks.held[id]; held {
			inProcessLocks.mu.Unlock()
			select {
			case <-released:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timeout.C:
				return nil, fmt.Errorf("timed out waiting for another operation in this gup process to release %s", path)
			}
		}
		released := make(chan struct{})
		inProcessLocks.held[id] = released
		inProcessLocks.mu.Unlock()

		var once sync.Once
		return func() {
			once.Do(func() {
				inProcessLocks.mu.Lock()
				delete(inProcessLocks.held, id)
				inProcessLocks.mu.Unlock()
				close(released)
			})
		}, nil
	}
}
