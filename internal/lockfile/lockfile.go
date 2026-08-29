// Package lockfile provides the cross-platform advisory lock that keeps two gup
// processes from mutating the same state at the same time.
//
// gup's state is $GOBIN plus gup.json. A command that rewrites gup.json already
// writes it atomically (temp file + rename), so a reader never sees a half
// written file - but atomicity is not mutual exclusion. Two `gup update` runs
// interleaved still lose one of the two results: both read the same gup.json,
// both install, and whichever finishes last writes a file describing only what
// it did. `gup remove` deleting a binary that a concurrent `gup update` is
// reinstalling is the same class of race with a worse outcome. This package is
// what makes the mutating commands take turns.
//
// The implementation is deliberately plain: an exclusively created file holding
// the owner's identity. That works identically on Linux, macOS and Windows,
// needs no CGO, and needs no flock/LockFileEx build tags, because O_CREATE|
// O_EXCL is atomic on every filesystem gup targets. What such a lock normally
// gets wrong is the crash case - a lock file left behind by a killed process
// wedges the tool forever - so a lock here is reclaimable in two independent
// ways: the owner records its PID and host, and the holder touches the file
// while it works. A lock whose owning PID is gone on this host, or whose
// heartbeat stopped long enough ago, is stale and gets taken over.
package lockfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nao1215/gup/internal/fileutil"
)

const (
	// DefaultWait is how long Acquire keeps retrying before reporting that
	// another gup process holds the lock. It is short on purpose: waiting is only
	// there to absorb the moment between one command releasing and the next
	// starting, not to queue behind a `gup update` that may run for minutes. A
	// user who ran two gup commands at once wants to be told so, not to watch a
	// second terminal hang.
	DefaultWait = 3 * time.Second

	// retryInterval is the gap between acquisition attempts while waiting.
	retryInterval = 50 * time.Millisecond

	// heartbeatInterval is how often the holder touches the lock file to say it
	// is still working. `gup update` on a large toolset runs far longer than any
	// fixed timeout would tolerate, so liveness is proven continuously rather
	// than assumed from the acquisition time.
	heartbeatInterval = 5 * time.Second

	// staleAfter is how long a lock file may go untouched before another process
	// may take it over. It is an order of magnitude above heartbeatInterval, so a
	// loaded machine that delays a few heartbeats does not lose its lock, while a
	// process killed with SIGKILL (which runs no cleanup) blocks the next command
	// for at most this long. The PID check below usually reclaims such a lock
	// immediately; this bound is the fallback for the cases the PID check cannot
	// answer, such as a lock file written by another machine on a shared home.
	staleAfter = 60 * time.Second

	// lockFileMode is the permission of a newly created lock file. It carries the
	// invoking user's PID and command line, so it is owner-readable only, like
	// every other file gup writes.
	lockFileMode fs.FileMode = 0600
)

// Owner records who holds a lock. It is written into the lock file as JSON so a
// waiting process - or a human looking at a leftover file - can tell which
// process is responsible and whether it still exists.
type Owner struct {
	// PID is the operating-system process ID of the holder.
	PID int `json:"pid"`
	// Host is the machine the holder runs on. A home directory shared over NFS
	// can carry a lock file from a different machine, where the PID means nothing;
	// recording the host is what stops gup from reading a stranger's PID as its
	// own liveness signal.
	Host string `json:"host"`
	// Command is the gup subcommand that took the lock ("update", "import", ...),
	// so the error message can say what is running rather than only that
	// something is.
	Command string `json:"command"`
	// Acquired is when the lock was taken, in RFC 3339. It is informational: the
	// staleness decision uses the file's modification time, which the heartbeat
	// keeps current.
	Acquired time.Time `json:"acquired"`
}

// BusyError reports that another live gup process holds the lock. It is a
// distinct type so a caller can tell "someone else is running" apart from "the
// lock file could not be written", which need different responses from a user.
type BusyError struct {
	// Path is the lock file that could not be acquired.
	Path string
	// Owner is what the lock file said. A lock file that could not be read leaves
	// this zero, and the message degrades to naming only the path.
	Owner Owner
}

// Error renders the message a user sees when two gup commands overlap. It names
// the other process concretely and says what to do, because the alternative -
// "resource temporarily unavailable" - leaves a user with a lock file they do
// not know they may delete.
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
	fmt.Fprintf(&b, "; if that process is gone, delete %s", e.Path)
	return b.String()
}

// Lock is an acquired advisory lock. Release returns it; it is safe to call
// Release more than once, so a caller can defer it and still release early.
type Lock struct {
	path string
	// freeInProcess hands the in-process slot back (see acquireInProcess).
	freeInProcess func()
	// stopHeartbeat ends the heartbeat and signal goroutine.
	stopHeartbeat chan struct{}
	// heartbeatDone closes once that goroutine has returned, so Release cannot
	// race a final os.Chtimes against its own os.Remove.
	heartbeatDone chan struct{}
	signals       chan os.Signal
	releaseOnce   sync.Once
	releaseErr    error
}

// Path returns the lock file this lock holds.
func (l *Lock) Path() string { return l.path }

// exitFunc is os.Exit, indirected so the signal path can be tested without
// killing the test binary.
var exitFunc = os.Exit //nolint:gochecknoglobals // test seam

// nowFunc is time.Now, indirected so staleness can be tested without sleeping.
var nowFunc = time.Now //nolint:gochecknoglobals // test seam

// Acquire takes the lock at path, creating its parent directory if needed, and
// returns a Lock the caller must Release.
//
// command names the gup subcommand for the error another process would see. A
// lock held by a live process is reported as *BusyError after roughly
// DefaultWait; a lock left behind by a process that no longer exists, or whose
// heartbeat stopped, is taken over instead of reported. ctx cancellation aborts
// the wait, and cancelling it after acquisition is not enough on its own to
// release the lock - the caller's Release is what does that.
func Acquire(ctx context.Context, path, command string) (*Lock, error) {
	path = filepath.Clean(path)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if fileutil.IsDir(path) {
		return nil, fmt.Errorf("lock path %s is a directory, not a file", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); err != nil {
		return nil, fmt.Errorf("can not create directory for the gup lock file: %w", err)
	}

	// Serialize within this process first. Two goroutines in one process racing
	// on O_EXCL would make one of them report "another gup process is already
	// running" about itself, which is both wrong and untestable; the in-process
	// gate waits instead, bounded only by ctx.
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
	lock.startHeartbeat()
	return lock, nil
}

// acquireOnDisk is the cross-process half of Acquire: create the lock file
// exclusively, taking over a stale one, until the deadline passes.
func acquireOnDisk(ctx context.Context, path, command string) (*Lock, error) {
	deadline := nowFunc().Add(DefaultWait)
	for {
		created, err := createLockFile(path, command)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("can not create the gup lock file %s: %w", path, err)
		}

		owner, stale := inspect(path)
		if stale {
			// Take the stale file over by renaming it aside: only one process can
			// rename a given file, so two processes that both judged it stale cannot
			// both proceed to create it. Losing the race is not an error - the next
			// iteration simply sees the winner's fresh lock.
			steal := fmt.Sprintf("%s.stale-%d-%d", path, os.Getpid(), nowFunc().UnixNano())
			if renameErr := os.Rename(path, steal); renameErr == nil {
				_ = os.Remove(steal)
			}
			continue
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !nowFunc().Before(deadline) {
			return nil, &BusyError{Path: path, Owner: owner}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}

// createLockFile creates path exclusively and writes the owner record into it.
// It returns an error satisfying errors.Is(err, fs.ErrExist) when the lock is
// already held, which is the signal acquireOnDisk loops on.
func createLockFile(path, command string) (*Lock, error) {
	//nolint:gosec // G304: path is gup's own lock file, derived from the config
	// directory rather than from user input, and O_EXCL is the point of the call.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, lockFileMode)
	if err != nil {
		return nil, err
	}

	host, hostErr := os.Hostname()
	if hostErr != nil {
		host = ""
	}
	owner := Owner{
		PID:      os.Getpid(),
		Host:     host,
		Command:  command,
		Acquired: nowFunc(),
	}
	// A marshal failure is impossible for this struct, but writing an empty lock
	// file would leave a lock nobody can attribute; treat any write problem as a
	// failed acquisition and clean up rather than holding an unreadable lock.
	data, err := json.Marshal(owner)
	if err == nil {
		_, err = file.Write(append(data, '\n'))
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("can not write the gup lock file %s: %w", path, err)
	}

	return &Lock{path: path}, nil
}

// inspect reads the lock file and decides whether the lock may be taken over.
//
// Two independent signals make a lock stale, because neither is sufficient
// alone. The PID check is exact but only meaningful for a lock written by this
// same host - a shared home directory can hold another machine's lock file,
// where that PID may well name a live but unrelated local process. The
// heartbeat-age check covers everything the PID check cannot answer: another
// host, an unreadable or truncated file, or a PID that has already been reused.
// A lock file that vanished between the failed create and this read reports
// stale so the caller retries immediately.
func inspect(path string) (owner Owner, stale bool) {
	info, err := os.Stat(path)
	if err != nil {
		return Owner{}, true
	}
	if nowFunc().Sub(info.ModTime()) > staleAfter {
		owner, _ = readOwner(path)
		return owner, true
	}

	owner, err = readOwner(path)
	if err != nil {
		// Unreadable or corrupt, but recently touched: something is actively
		// writing it (the window between O_EXCL create and the owner write), so
		// treat it as held and let the caller wait it out.
		return Owner{}, false
	}
	if host, hostErr := os.Hostname(); hostErr == nil && owner.Host != "" && owner.Host != host {
		// Written by a different machine: only its heartbeat can tell us anything,
		// and it is fresh, so the lock is held.
		return owner, false
	}
	if owner.PID <= 0 {
		return owner, true
	}
	return owner, !processAlive(owner.PID)
}

// readOwner parses the owner record out of a lock file.
func readOwner(path string) (Owner, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Owner{}, err
	}
	var owner Owner
	if err := json.Unmarshal(raw, &owner); err != nil {
		return Owner{}, err
	}
	return owner, nil
}

// startHeartbeat begins touching the lock file so a long-running command is not
// mistaken for a crashed one, and arranges for Ctrl-C to release the lock.
//
// The signal handler is what makes the common interruption case clean: gup's
// subcommands exit through os.Exit, so a SIGINT during `gup update` would
// otherwise terminate the process with no deferred Release having run, leaving a
// lock file that blocks the next command until it ages out. Handling the signal
// here removes the lock and then exits with the conventional 128+signal status,
// which is the same observable outcome as the default disposition.
func (l *Lock) startHeartbeat() {
	l.stopHeartbeat = make(chan struct{})
	l.heartbeatDone = make(chan struct{})
	l.signals = make(chan os.Signal, 1)
	signal.Notify(l.signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer close(l.heartbeatDone)
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-l.stopHeartbeat:
				return
			case <-ticker.C:
				now := nowFunc()
				// A failure here is not worth interrupting the user's command over:
				// the worst case is that this lock becomes reclaimable early, and the
				// caller is about to release it anyway.
				_ = os.Chtimes(l.path, now, now)
			case sig := <-l.signals:
				_ = l.releaseFile()
				exitFunc(exitStatusFor(sig))
				return
			}
		}
	}()
}

// exitStatusFor maps a signal to the exit status a shell expects from a process
// killed by it (128 + signal number), so scripts see the same status they would
// have seen without gup's handler.
func exitStatusFor(sig os.Signal) int {
	const signalExitBase = 128
	if s, ok := sig.(syscall.Signal); ok && s > 0 {
		return signalExitBase + int(s)
	}
	return 1
}

// Release removes the lock file and stops the heartbeat. Calling it twice is
// safe and returns the first call's result, so a defer plus an early release do
// not fight.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	l.releaseOnce.Do(func() {
		if l.stopHeartbeat != nil {
			close(l.stopHeartbeat)
			<-l.heartbeatDone
		}
		if l.signals != nil {
			signal.Stop(l.signals)
		}
		l.releaseErr = l.releaseFile()
		if l.freeInProcess != nil {
			l.freeInProcess()
		}
	})
	return l.releaseErr
}

// releaseFile removes the lock file. A file that is already gone (an operator
// deleted it, or a stale-takeover claimed it) is not an error: the postcondition
// "this process no longer holds the lock" is satisfied either way.
func (l *Lock) releaseFile() error {
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("can not remove the gup lock file %s: %w", l.path, err)
	}
	return nil
}

// inProcessLocks serializes acquisitions of the same path inside one process.
// The on-disk lock alone cannot do this: O_EXCL does not distinguish "held by
// another process" from "held by this one", so a second acquisition in the same
// process would be reported as a foreign conflict. Waiting here is unbounded
// except by ctx, because two gup goroutines contending for the lock are
// cooperating parts of one program - unlike two processes, where a bounded wait
// and a clear message are the right answer.
var inProcessLocks = struct { //nolint:gochecknoglobals // process-wide by definition
	mu   sync.Mutex
	held map[string]chan struct{}
}{held: map[string]chan struct{}{}}

// acquireInProcess claims path's in-process slot and returns the function that
// hands it back.
func acquireInProcess(ctx context.Context, path string) (func(), error) {
	for {
		inProcessLocks.mu.Lock()
		if released, held := inProcessLocks.held[path]; held {
			inProcessLocks.mu.Unlock()
			select {
			case <-released:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
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
