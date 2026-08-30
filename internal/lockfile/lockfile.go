// Package lockfile provides the cross-platform lock that keeps two gup
// processes from mutating the same state at the same time.
//
// gup's state is the binaries under a $GOBIN and the gup.json describing them. A
// command that rewrites gup.json already writes it atomically (temp file +
// rename), so a reader never sees a half-written file - but atomicity is not
// mutual exclusion. Two `gup update` runs interleaved still lose one of the two
// results: both read the same gup.json, both install, and whichever finishes
// last writes a file describing only what it did. `gup remove` deleting a binary
// that a concurrent `gup update` is reinstalling is the same class of race with
// a worse outcome. This package is what makes the mutating commands take turns.
//
// A lock is scoped to the RESOURCE it protects, not to gup's configuration
// directory: PathForDir guards a binary directory, PathForFile guards a gup.json.
// That distinction matters because the two are relocated independently - a user
// who points XDG_CONFIG_HOME at a per-project directory still shares one $GOBIN
// with every other project, and two commands writing the same --file may be
// started from different configuration directories entirely. A lock derived from
// the configuration directory alone would serialize neither.
//
// # The lock is the kernel's, not a file gup interprets
//
// Exclusion here is flock(2) on Unix and LockFileEx on Windows: the operating
// system's own advisory lock, taken on a descriptor gup keeps open for as long
// as it holds the resource. Everything that makes lock files hard follows from
// that one choice, by not existing.
//
//   - A lock is released by the kernel when the descriptor closes. A process
//     that exits, crashes, or is killed with SIGKILL closes every descriptor on
//     the way out, so its lock is gone the instant it is - with no cleanup to
//     run, and nothing for a successor to detect.
//   - There is therefore no such thing as a stale lock. No heartbeat proves the
//     owner is alive, no PID is consulted, no staleness bound has to be tuned,
//     and no take-over has to prove it is removing the file it examined. A
//     process that loses a race simply never had the lock.
//   - The lock file is never deleted, and that is deliberate. Unlinking a file
//     another process has already opened and is waiting on would hand two
//     processes a lock on two different inodes at the same path - exactly the
//     overlap this package exists to prevent. The file left behind holds no
//     authority at all: it is a name for the kernel to hang a lock on, and an
//     empty one when nobody holds it.
//
// One consequence is worth naming, because it is invisible in the API: a lock
// this process holds is kept reachable by the package itself (see heldLocks).
// The descriptor carrying the kernel lock is inside the Lock, and an os.File
// closes itself from a finalizer once nothing refers to it, so a caller that
// took a lock and dropped the value would otherwise have it released by the
// garbage collector while it was still working.
//
// What gup writes INTO the lock file is a courtesy, not a mechanism. The holder
// records its PID, host and subcommand so a waiting process can say who it is
// waiting for; a waiter that cannot read that record loses nothing but detail in
// an error message, because the verdict came from the kernel before the file was
// ever read.
package lockfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nao1215/gup/internal/fileutil"
)

const (
	// defaultWait is how long Acquire keeps retrying before reporting that
	// another gup process holds the lock. It is short on purpose: waiting is only
	// there to absorb the moment between one command releasing and the next
	// starting, not to queue behind a `gup update` that may run for minutes. A
	// user who ran two gup commands at once wants to be told so, not to watch a
	// second terminal hang.
	defaultWait = 3 * time.Second

	// retryInterval is the gap between acquisition attempts while waiting.
	retryInterval = 50 * time.Millisecond

	// lockFileMode is the permission of a newly created lock file. It carries the
	// invoking user's PID and command line, so it is owner-readable only, like
	// every other file gup writes.
	lockFileMode fs.FileMode = 0600

	// DirLockName is the lock file guarding a binary directory. The leading dot
	// keeps it out of gup's own $GOBIN listing, which skips dot-prefixed entries
	// (see goutil.BinaryPathList), so the lock cannot be mistaken for a tool.
	DirLockName = ".gup.lock"
)

// envWait names the environment variable that overrides the wait timeout. It
// exists so the end-to-end suite can exercise contention in milliseconds instead
// of seconds: behavior that can only be tested by waiting is behavior that does
// not get tested. It is deliberately undocumented for users - the default is the
// contract.
const envWait = "GUP_LOCK_WAIT"

// waitTimeout returns the acquisition timeout. A malformed override falls back
// to the default rather than failing: this is a test knob, and a typo in it must
// not break the lock.
func waitTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv(envWait))
	if raw == "" {
		return defaultWait
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultWait
	}
	return d
}

// IsReservedName reports whether name is a file gup keeps in a directory it
// manages, rather than something a user installed there.
//
// $GOBIN holds the lock guarding $GOBIN, and `gup remove` deletes files from
// $GOBIN by name. Without this, `gup remove .gup.lock --force` deletes the lock
// of the very command running it: the kernel lock survives (it lives on the open
// descriptor, not on the name), but the next gup creates a fresh file at the
// free name and locks that instead, and the two run side by side. The name is
// reserved rather than merely hidden, because "you cannot see it" is not the
// same as "you cannot name it".
//
// The comparison ignores case because macOS and Windows do: on a case-insensitive
// filesystem `.GUP.LOCK` opens the same file. Rejecting it on Linux too costs
// nothing - no one installs a tool by that name.
func IsReservedName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), DirLockName)
}

// normalizePath resolves a lock path to a cleaned absolute one.
//
// Every entry point normalizes through here, and they must all agree: the
// in-process registry is keyed by the result, so if one caller keyed "x.lock"
// and another "./x.lock" they would take two different in-process slots for one
// file, and the second acquisition would wait out the whole timeout against
// itself. Failing is better than falling back to the relative path, because a
// key that is ambiguous is a lock that does not lock.
func normalizePath(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("can not resolve the gup lock path %s: %w", path, err)
	}
	return abs, nil
}

// PathForDir returns the lock file guarding a directory whose contents gup
// mutates, such as a $GOBIN or a migrate destination.
func PathForDir(dir string) string { return filepath.Join(dir, DirLockName) }

// PathForFile returns the lock file guarding a single file gup rewrites, such as
// a gup.json. It sits beside the file so the lock travels with the resource,
// which is what makes two processes with different configuration directories but
// the same --file contend for the same lock.
func PathForFile(file string) string { return file + ".lock" }

// Owner records who holds a lock. The holder writes it into the lock file so a
// waiting process can name the command it is waiting for.
//
// It is descriptive only. Nothing about acquiring, holding or releasing a lock
// consults it, and a record that is missing, truncated or written by a version
// of gup that spelled it differently costs nothing but detail in one error
// message.
type Owner struct {
	// PID is the operating-system process ID of the holder.
	PID int `json:"pid"`
	// Host is the machine the holder runs on, so a lock file on a shared home
	// directory says which machine the process it names lives on.
	Host string `json:"host"`
	// Command is the gup subcommand that took the lock ("update", "import", ...),
	// so the error message can say what is running rather than only that
	// something is.
	Command string `json:"command"`
	// Acquired is when the lock was taken, in RFC 3339.
	Acquired time.Time `json:"acquired"`
}

// BusyError reports that another gup process holds the lock. It is a distinct
// type so a caller can tell "someone else is running" apart from "the lock file
// could not be opened", which need different responses from a user.
type BusyError struct {
	// Path is the lock file that could not be acquired.
	Path string
	// Owner is what the lock file said. A record that could not be read leaves
	// this zero, and the message degrades to naming only the path - the refusal
	// itself came from the kernel, not from the file.
	Owner Owner
}

// Error renders the message a user sees when two gup commands overlap. It names
// the other process concretely, because the alternative - "resource temporarily
// unavailable" - tells a user nothing about what to do.
//
// It deliberately does NOT suggest deleting the lock file, and says why in one
// clause: the lock is the operating system's, so it is already gone if the
// process is, and deleting the file while that process still runs buys the user
// exactly the concurrent run the lock exists to prevent.
func (e *BusyError) Error() string {
	var b strings.Builder
	b.WriteString("another gup process is already running")
	if e.Owner.PID > 0 {
		fmt.Fprintf(&b, " (pid %d", e.Owner.PID)
		if e.Owner.Host != "" {
			fmt.Fprintf(&b, " on %s", e.Owner.Host)
		}
		if e.Owner.Command != "" {
			fmt.Fprintf(&b, ", running %q", "gup "+e.Owner.Command)
		}
		if !e.Owner.Acquired.IsZero() {
			fmt.Fprintf(&b, ", since %s", e.Owner.Acquired.Format(time.RFC3339))
		}
		b.WriteString(")")
	}
	b.WriteString(". gup serializes commands that change your $GOBIN or gup.json," +
		" so wait for it to finish and run this command again")
	fmt.Fprintf(&b, ". The lock is held by the operating system, not by %s,"+
		" so it is released the moment that process ends and there is never a file to delete by hand", e.Path)
	return b.String()
}

// Lock is an acquired lock. Release returns it; it is safe to call Release more
// than once, so a caller can defer it and still release early.
type Lock struct {
	path string
	// file holds the kernel lock. While this descriptor is open no other
	// descriptor, in this process or any other, can take the same lock; when it
	// closes - deliberately, or because the process died - the kernel drops it.
	file          *os.File
	freeInProcess func()
	releaseOnce   sync.Once
	releaseErr    error
}

// Path returns the lock file this lock holds.
func (l *Lock) Path() string { return l.path }

// Acquire takes the lock at path, creating its parent directory if needed, and
// returns a Lock the caller must Release.
//
// command names the gup subcommand for the error another process would see. A
// lock held by another process is reported as *BusyError after roughly the wait
// timeout. ctx cancellation aborts the wait; canceling it after acquisition is
// not enough on its own to release the lock - the caller's Release, or the
// process ending, is what does that.
func Acquire(ctx context.Context, path, command string) (*Lock, error) {
	path, err := normalizePath(path)
	if err != nil {
		return nil, err
	}
	if fileutil.IsDir(path) {
		return nil, fmt.Errorf("lock path %s is a directory, not a file", path)
	}
	//nolint:gosec // G703: the path names the resource being guarded, which is
	// exactly what a caller must be able to choose; normalizePath has cleaned it.
	if err := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); err != nil {
		return nil, fmt.Errorf("can not create directory for the gup lock file: %w", err)
	}

	// Serialize within this process first. The kernel lock is per descriptor, so
	// two goroutines in one process racing on it would make one of them report
	// "another gup process is already running" about itself, which is both wrong
	// and untestable.
	freeInProcess, err := acquireInProcess(ctx, path)
	if err != nil {
		return nil, err
	}

	lock, err := acquireOnDisk(ctx, path, command)
	if err != nil {
		freeInProcess()
		return nil, err
	}
	lock.freeInProcess = freeInProcess
	rememberHeldLock(lock)
	return lock, nil
}

// MultiLock holds several locks taken together, in a deterministic order.
type MultiLock struct {
	locks []*Lock
}

// Paths returns the lock files held, in acquisition order.
func (m *MultiLock) Paths() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.locks))
	for _, l := range m.locks {
		out = append(out, l.path)
	}
	return out
}

// AcquireAll takes every lock in paths and returns them as one unit.
//
// The paths are deduplicated and sorted before anything is taken, which is what
// makes deadlock impossible: two processes asking for the same set in different
// orders still acquire it in the same order. If any lock cannot be taken, the
// ones already held are released before returning, so a failed acquisition never
// leaves a partial hold behind. An empty path list is not an error - a command
// that writes nothing needs no lock, and expressing that as "no resources" keeps
// the decision with the command instead of duplicating it here.
func AcquireAll(ctx context.Context, command string, paths ...string) (*MultiLock, error) {
	// Normalize BEFORE sorting and deduplicating, or two spellings of one path
	// survive as two entries and the second acquisition waits out the timeout
	// against the first - a deadlock with a stopwatch on it.
	ordered := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized, err := normalizePath(path)
		if err != nil {
			return nil, err
		}
		ordered = append(ordered, normalized)
	}
	slices.Sort(ordered)
	ordered = slices.Compact(ordered)

	held := &MultiLock{}
	for _, path := range ordered {
		lock, err := Acquire(ctx, path, command)
		if err != nil {
			// A rollback that could not release a lock matters as much as the
			// acquisition that failed. Reporting only the first error hides that.
			return nil, errors.Join(err, held.Release())
		}
		held.locks = append(held.locks, lock)
	}
	return held, nil
}

// Release returns every held lock, in reverse acquisition order, and reports
// every problem rather than the first.
func (m *MultiLock) Release() error {
	if m == nil {
		return nil
	}
	var errs []error
	for i := len(m.locks) - 1; i >= 0; i-- {
		errs = append(errs, m.locks[i].Release())
	}
	return errors.Join(errs...)
}

// acquireOnDisk is the cross-process half of Acquire: open the lock file and ask
// the kernel for it, retrying until the deadline passes.
//
// Every iteration checks the context and the deadline before it can loop again,
// so neither a permanently held lock nor a cancellation can leave this spinning.
// The wait is a poll rather than a blocking lock request on purpose: a blocking
// request waits in the kernel, where neither the deadline nor ctx could reach it.
func acquireOnDisk(ctx context.Context, path, command string) (*Lock, error) {
	deadline := time.Now().Add(waitTimeout())
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		//nolint:gosec // G304: path is gup's own lock file, derived from the resource
		// being protected rather than from user input.
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, lockFileMode)
		if err != nil {
			return nil, fmt.Errorf("can not open the gup lock file %s: %w", path, err)
		}

		locked, err := tryLockFile(file)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("can not lock %s: %w", path, err)
		}
		if locked {
			// The lock is held from here on. Nothing below can lose it, and a
			// failure to describe the holder is not a reason to give it up.
			writeOwner(file, command)
			return &Lock{path: path, file: file}, nil
		}

		// Someone else holds it. The record is read only to name them; an
		// unreadable one leaves the message thinner, never the verdict wrong.
		owner := readOwner(file)
		_ = file.Close()

		if !time.Now().Before(deadline) {
			return nil, &BusyError{Path: path, Owner: owner}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}

// writeOwner records the holder in the lock file it has just taken.
//
// Every failure is ignored, and that is the point: the lock is already held by
// the kernel, so refusing to proceed because a descriptive record could not be
// written would turn a cosmetic problem into a failed command. The worst outcome
// is a waiter that reports the path without naming the process.
//
// The file is truncated first because it may still carry a previous holder's
// longer record, and a half-overwritten one would parse as neither.
func writeOwner(file *os.File, command string) {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	data, err := json.Marshal(Owner{
		PID:      os.Getpid(),
		Host:     host,
		Command:  command,
		Acquired: time.Now(),
	})
	if err != nil {
		return
	}
	if err := file.Truncate(0); err != nil {
		return
	}
	_, _ = file.WriteAt(append(data, '\n'), 0)
}

// readOwner parses the holder's record out of an open lock file. A file that is
// empty, truncated or written by some other version of gup yields the zero
// Owner, which the message renders as "another gup process is already running"
// with no parenthesis.
func readOwner(file *os.File) Owner {
	raw := make([]byte, ownerRecordLimit)
	n, err := file.ReadAt(raw, 0)
	if n == 0 && err != nil {
		return Owner{}
	}
	var owner Owner
	if json.Unmarshal(raw[:n], &owner) != nil {
		return Owner{}
	}
	return owner
}

// ownerRecordLimit bounds the read above. The record is a fixed set of short
// fields, so anything longer is not one - and reading a bounded amount means a
// lock file somebody filled with garbage costs a fixed read rather than its own
// size in memory.
const ownerRecordLimit = 4096

// Release drops the kernel lock and closes the descriptor holding it. Calling it
// twice is safe and returns the first call's result, so a defer plus an early
// release do not fight.
//
// The lock FILE is left where it is. Deleting it is the one thing that could
// still break exclusion: a waiter has the file open already, and unlinking it
// would let the next process create a different file at the same name and lock
// that instead, leaving two processes each holding a lock nobody else can see.
// An empty 0600 file next to the resource is a much smaller cost than that, and
// the same file is reused by every command that follows.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	l.releaseOnce.Do(func() {
		l.releaseErr = l.releaseFile()
		if l.freeInProcess != nil {
			l.freeInProcess()
		}
		forgetHeldLock(l)
	})
	return l.releaseErr
}

// releaseFile clears the owner record, drops the kernel lock and closes the
// descriptor.
//
// The record is cleared while the lock is still held, so no waiter can ever read
// a record naming a process that has already let go. Failing to clear it is
// ignored for the reason writing it is: it describes the holder, it does not
// make the lock.
func (l *Lock) releaseFile() error {
	if l.file == nil {
		return nil
	}
	_ = l.file.Truncate(0)

	var errs []error
	if err := unlockFile(l.file); err != nil {
		errs = append(errs, fmt.Errorf("can not unlock the gup lock file %s: %w", l.path, err))
	}
	// Closing releases the lock even when the explicit unlock above failed, so it
	// happens either way.
	if err := l.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("can not close the gup lock file %s: %w", l.path, err))
	}
	l.file = nil
	return errors.Join(errs...)
}

// A word on interruption, since the absence of a signal handler here is a
// decision rather than an omission.
//
// It is tempting to catch SIGINT and release the lock on the way out. That is
// wrong, and subtly so: releasing does not stop the work. The command that holds
// the lock is still installing binaries and rewriting gup.json in another
// goroutine, and a second gup started in the moment between the release and the
// process actually dying walks into exactly the overlap the lock exists to
// prevent - with no error anywhere, because both processes believe they hold it.
//
// So the lock is held until the process is gone, and nothing gets to shorten
// that. gup's long-running commands already cancel their work on a signal (see
// cmd's signal-canceling context): the run unwinds, the command returns, and the
// deferred Release drops the lock - in that order, which is the order that is
// safe. A command killed outright never returns at all, and the kernel drops its
// lock as it reaps the process. Neither path can leave two gups running.

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
var inProcessLocks = struct { //nolint:gochecknoglobals // process-wide by definition
	mu   sync.Mutex
	held map[string]chan struct{}
}{held: map[string]chan struct{}{}}

// acquireInProcess claims path's in-process slot and returns the function that
// hands it back. The wait is bounded by the same timeout the cross-process wait
// uses: an unbounded wait here would turn a caller that forgot to release into a
// hang with no diagnosis, which is worse than a clear timeout.
func acquireInProcess(ctx context.Context, path string) (func(), error) {
	timeout := time.NewTimer(waitTimeout())
	defer timeout.Stop()

	for {
		inProcessLocks.mu.Lock()
		if released, held := inProcessLocks.held[path]; held {
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
		inProcessLocks.held[path] = released
		inProcessLocks.mu.Unlock()

		var once sync.Once
		return func() {
			once.Do(func() {
				inProcessLocks.mu.Lock()
				delete(inProcessLocks.held, path)
				inProcessLocks.mu.Unlock()
				close(released)
			})
		}, nil
	}
}
