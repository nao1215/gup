// Package lockfile provides the cross-platform advisory lock that keeps two gup
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
// The implementation is deliberately plain: an exclusively created file holding
// the owner's identity. That works identically on Linux, macOS and Windows,
// needs no CGO, and needs no flock/LockFileEx build tags, because O_CREATE|
// O_EXCL is atomic on every filesystem gup targets. What such a lock normally
// gets wrong is the crash case - a lock file left behind by a killed process
// wedges the tool forever - so a lock here is reclaimable two independent ways:
// the owner records its PID and host, and the holder touches the file while it
// works. A lock whose owning PID is gone on this host is reclaimed at once; one
// whose origin cannot be checked that way (another host, an unreadable file) is
// reclaimed when its heartbeat has stopped long enough.
package lockfile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
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

	// defaultHeartbeat is how often the holder touches the lock file to say it is
	// still working. `gup update` on a large toolset runs far longer than any
	// fixed timeout would tolerate, so liveness is proven continuously rather
	// than assumed from the acquisition time.
	defaultHeartbeat = 5 * time.Second

	// defaultStaleAfter is how long a lock file may go untouched before another
	// process may take it over. It is an order of magnitude above the heartbeat
	// interval, so a loaded machine that delays a few beats does not lose its
	// lock. It applies ONLY where the owning process cannot be checked directly
	// (see inspect): on this host a live PID keeps its lock no matter how long
	// the process has been stopped.
	defaultStaleAfter = 60 * time.Second

	// lockFileMode is the permission of a newly created lock file. It carries the
	// invoking user's PID and command line, so it is owner-readable only, like
	// every other file gup writes.
	lockFileMode fs.FileMode = 0600

	// dirLockName is the lock file guarding a binary directory. The leading dot
	// keeps it out of gup's own $GOBIN listing, which skips dot-prefixed entries
	// (see goutil.BinaryPathList), so the lock cannot be mistaken for a tool.
	dirLockName = ".gup.lock"
)

// Environment variables that override the timings above. They exist so the
// end-to-end suite can exercise staleness and waiting in seconds instead of
// minutes: behavior that can only be tested by waiting a minute is behavior that
// does not get tested. They are deliberately undocumented for users - the
// defaults are the contract.
const (
	envWait      = "GUP_LOCK_WAIT"
	envHeartbeat = "GUP_LOCK_HEARTBEAT"
	envStale     = "GUP_LOCK_STALE_AFTER"
)

// durationFromEnv returns the duration named by env, or fallback when it is
// unset, unparseable, or not positive. A malformed value falls back rather than
// failing: this is a test knob, and a typo in it must not break the lock.
func durationFromEnv(env string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func waitTimeout() time.Duration { return durationFromEnv(envWait, defaultWait) }

func heartbeatInterval() time.Duration { return durationFromEnv(envHeartbeat, defaultHeartbeat) }

func staleAfter() time.Duration { return durationFromEnv(envStale, defaultStaleAfter) }

// PathForDir returns the lock file guarding a directory whose contents gup
// mutates, such as a $GOBIN or a migrate destination.
func PathForDir(dir string) string { return filepath.Join(dir, dirLockName) }

// PathForFile returns the lock file guarding a single file gup rewrites, such as
// a gup.json. It sits beside the file so the lock travels with the resource,
// which is what makes two processes with different configuration directories but
// the same --file contend for the same lock.
func PathForFile(file string) string { return file + ".lock" }

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
	// Nonce identifies this particular acquisition. PID and host are not enough:
	// after a lock is reclaimed, the original holder must not delete or refresh
	// the file its successor created, and only a value unique per acquisition can
	// distinguish "my lock" from "a lock that happens to be at my path".
	Nonce string `json:"nonce"`
}

// BusyError reports that another gup process holds the lock and is alive. It is
// a distinct type so a caller can tell "someone else is running" apart from "the
// lock file could not be written", which need different responses from a user.
type BusyError struct {
	// Path is the lock file that could not be acquired.
	Path string
	// Owner is what the lock file said. A lock file that could not be read leaves
	// this zero, and the message degrades to naming only the path.
	Owner Owner
}

// Error renders the message a user sees when two gup commands overlap. It names
// the other process concretely, because the alternative - "resource temporarily
// unavailable" - tells a user nothing about what to do.
//
// It deliberately does NOT suggest deleting the lock file. This error is only
// returned for a lock gup believes is held by a live process, and a user who
// deletes it gets exactly the concurrent run the lock exists to prevent. A lock
// whose owner is gone is reclaimed without anyone being asked.
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
	fmt.Fprintf(&b, ". If that process is gone, gup reclaims %s by itself", e.Path)
	return b.String()
}

// ReclaimError reports a lock file that gup judged abandoned but could not
// remove - almost always a permissions problem on the file or its directory.
// Retrying cannot fix that, so it is reported instead of waited out, naming the
// path so the user can act on it. This is the one case where deleting the file
// by hand is the right advice.
type ReclaimError struct {
	// Path is the abandoned lock file.
	Path string
	// Err is why it could not be removed; nil when the take-over kept losing a
	// race rather than failing outright.
	Err error
}

func (e *ReclaimError) Error() string {
	reason := "another process kept re-creating it"
	if e.Err != nil {
		reason = e.Err.Error()
	}
	return fmt.Sprintf("the abandoned gup lock file %s could not be removed: %s."+
		" Delete it by hand (no gup process owns it) and run this command again", e.Path, reason)
}

// Unwrap exposes the underlying filesystem error so callers can test for
// fs.ErrPermission.
func (e *ReclaimError) Unwrap() error { return e.Err }

// TakenOverError reports that this process's lock had already been reclaimed by
// another gup by the time it was released. The work is done either way, but the
// two runs overlapped, so it is surfaced rather than swallowed.
type TakenOverError struct {
	// Path is the lock file that now belongs to someone else.
	Path string
}

func (e *TakenOverError) Error() string {
	return fmt.Sprintf("the gup lock %s was taken over by another process while this command was running;"+
		" the two runs may have overlapped", e.Path)
}

// Lock is an acquired advisory lock. Release returns it; it is safe to call
// Release more than once, so a caller can defer it and still release early.
type Lock struct {
	path string
	// nonce is what makes this lock identifiable. Release and the heartbeat both
	// verify it before touching the file, so a reclaimed lock is never deleted or
	// refreshed by its previous owner.
	nonce string
	// freeInProcess hands the in-process slot back (see acquireInProcess).
	freeInProcess func()
	// stopHeartbeat ends the heartbeat goroutine.
	stopHeartbeat chan struct{}
	// heartbeatDone closes once that goroutine has returned, so Release cannot
	// race a final os.Chtimes against its own os.Remove.
	heartbeatDone chan struct{}
	releaseOnce   sync.Once
	releaseErr    error
}

// Path returns the lock file this lock holds.
func (l *Lock) Path() string { return l.path }

// exitFunc is os.Exit, indirected so the signal path can be tested without
// killing the test binary. It is read from the signal goroutine and written by
// tests, so both go through exitMu: a seam that trips the race detector makes
// `go test -race` unusable for everything else in the package.
var (
	exitMu   sync.Mutex //nolint:gochecknoglobals // guards exitFunc
	exitFunc = os.Exit  //nolint:gochecknoglobals // test seam
)

// exitProcess ends the process through the current exit function.
func exitProcess(code int) {
	exitMu.Lock()
	fn := exitFunc
	exitMu.Unlock()
	fn(code)
}

// nowFunc is time.Now, indirected so staleness can be tested without sleeping.
var nowFunc = time.Now //nolint:gochecknoglobals // test seam

// Acquire takes the lock at path, creating its parent directory if needed, and
// returns a Lock the caller must Release.
//
// command names the gup subcommand for the error another process would see. A
// lock held by a live process is reported as *BusyError after roughly the wait
// timeout; a lock left behind by a process that no longer exists, or whose
// heartbeat stopped, is taken over instead of reported. ctx cancellation aborts
// the wait, and canceling it after acquisition is not enough on its own to
// release the lock - the caller's Release is what does that.
func Acquire(ctx context.Context, path, command string) (*Lock, error) {
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		// Without an absolute path the in-process registry key is ambiguous and
		// two callers naming the same file could both proceed, so this is an error
		// rather than a silent fallback to the relative path.
		return nil, fmt.Errorf("can not resolve the gup lock path: %w", err)
	}
	if fileutil.IsDir(path) {
		return nil, fmt.Errorf("lock path %s is a directory, not a file", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); err != nil {
		return nil, fmt.Errorf("can not create directory for the gup lock file: %w", err)
	}

	// Serialize within this process first. Two goroutines in one process racing
	// on O_EXCL would make one of them report "another gup process is already
	// running" about itself, which is both wrong and untestable.
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
	registerForSignals(lock)
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
	ordered := slices.Clone(paths)
	slices.Sort(ordered)
	ordered = slices.Compact(ordered)

	held := &MultiLock{}
	for _, path := range ordered {
		lock, err := Acquire(ctx, path, command)
		if err != nil {
			_ = held.Release()
			return nil, err
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

// acquireOnDisk is the cross-process half of Acquire: create the lock file
// exclusively, taking over an abandoned one, until the deadline passes.
//
// Every iteration checks the context and the deadline before it can loop again,
// including the take-over path. That ordering is the whole point: an earlier
// version returned to the top of the loop straight after a failed take-over,
// which turned "a stale lock file in a directory I cannot write" into a tight
// loop that no timeout or cancellation could end.
func acquireOnDisk(ctx context.Context, path, command string) (*Lock, error) {
	deadline := nowFunc().Add(waitTimeout())
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		created, err := createLockFile(path, command)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("can not create the gup lock file %s: %w", path, err)
		}

		owner, stale := inspect(path)
		var reclaimErr error
		if stale {
			reclaimErr = reclaim(path)
			// A permission problem will not resolve by waiting, and waiting would
			// only replace a clear diagnosis with a timeout.
			if reclaimErr != nil && errors.Is(reclaimErr, fs.ErrPermission) {
				return nil, &ReclaimError{Path: path, Err: reclaimErr}
			}
		}

		if !nowFunc().Before(deadline) {
			if stale {
				return nil, &ReclaimError{Path: path, Err: reclaimErr}
			}
			return nil, &BusyError{Path: path, Owner: owner}
		}
		if stale && reclaimErr == nil {
			// The file was ours to take: retry immediately rather than sleeping.
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}

// reclaim removes an abandoned lock file by renaming it aside first. Only one
// process can rename a given file, so two processes that both judged it
// abandoned cannot both go on to create it; the loser sees the winner's fresh
// lock on its next attempt. A file that has already vanished counts as reclaimed.
func reclaim(path string) error {
	aside := fmt.Sprintf("%s.stale-%d-%d", path, os.Getpid(), nowFunc().UnixNano())
	if err := os.Rename(path, aside); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	_ = os.Remove(aside)
	return nil
}

// createLockFile creates path exclusively and writes the owner record into it.
// It returns an error satisfying errors.Is(err, fs.ErrExist) when the lock is
// already held, which is the signal acquireOnDisk loops on.
func createLockFile(path, command string) (*Lock, error) {
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}

	//nolint:gosec // G304: path is gup's own lock file, derived from the resource
	// being protected rather than from user input, and O_EXCL is the point of the call.
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
		Nonce:    nonce,
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

	return &Lock{path: path, nonce: nonce}, nil
}

// newNonce returns a random identifier for one acquisition.
func newNonce() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("can not generate a gup lock identifier: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

// inspect reads the lock file and decides whether the lock may be taken over.
//
// The order matters, and is the opposite of what it might look like it could be.
// When the file was written by a live process ON THIS HOST, that is a definitive
// answer and nothing else is consulted - however long ago the process last
// touched the file. A `gup update` suspended with Ctrl-Z, a laptop resumed from
// sleep, or a container throttled for a minute all stop the heartbeat without
// stopping the process, and treating those as abandoned would hand the lock to a
// second gup while the first is still working.
//
// The heartbeat age is the fallback for everything the PID check cannot answer:
// a lock file from another machine on a shared home directory, one whose owner
// record is unreadable or truncated, or one naming no usable PID. A lock file
// that vanished between the failed create and this read reports abandoned so the
// caller retries immediately.
func inspect(path string) (owner Owner, stale bool) {
	info, err := os.Stat(path)
	if err != nil {
		return Owner{}, true
	}

	owner, readErr := readOwner(path)
	if readErr == nil && owner.PID > 0 && ownedByThisHost(owner) {
		return owner, !processAlive(owner.PID)
	}

	// Not attributable to a live local process: fall back to the heartbeat. A
	// file that is still being touched is held by someone; one that is not has
	// been abandoned. A freshly created file whose owner record has not been
	// written yet lands here too, and is correctly treated as held.
	if nowFunc().Sub(info.ModTime()) > staleAfter() {
		return owner, true
	}
	return owner, false
}

// ownedByThisHost reports whether the owner record was written on this machine,
// which is the precondition for its PID meaning anything here.
func ownedByThisHost(owner Owner) bool {
	host, err := os.Hostname()
	if err != nil {
		return false
	}
	// An empty recorded host means the writer could not determine its own name;
	// its PID is then unattributable, so it is not treated as local.
	return owner.Host != "" && owner.Host == host
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

// stillOurs reports whether the file at the lock's path is the one this lock
// created. Everything destructive - removing the file, refreshing its timestamp
// - goes through this check, so a lock that was reclaimed while this process was
// stopped can neither be deleted nor kept alive by its former owner.
func (l *Lock) stillOurs() bool {
	owner, err := readOwner(l.path)
	if err != nil {
		return false
	}
	return owner.Nonce != "" && owner.Nonce == l.nonce
}

// startHeartbeat begins touching the lock file so a long-running command is not
// mistaken for a crashed one.
func (l *Lock) startHeartbeat() {
	l.stopHeartbeat = make(chan struct{})
	l.heartbeatDone = make(chan struct{})

	go func() {
		defer close(l.heartbeatDone)
		ticker := time.NewTicker(heartbeatInterval())
		defer ticker.Stop()
		for {
			select {
			case <-l.stopHeartbeat:
				return
			case <-ticker.C:
				// Refresh only while the file is still this lock's. Touching a
				// successor's lock would keep IT alive on this process's behalf,
				// hiding the successor's own death from the next waiter.
				if !l.stillOurs() {
					continue
				}
				now := nowFunc()
				// A failure here is not worth interrupting the user's command over:
				// the worst case is that this lock becomes reclaimable early, and the
				// caller is about to release it anyway.
				_ = os.Chtimes(l.path, now, now)
			}
		}
	}()
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
		unregisterFromSignals(l)
		l.releaseErr = l.releaseFile()
		if l.freeInProcess != nil {
			l.freeInProcess()
		}
	})
	return l.releaseErr
}

// releaseFile removes the lock file, but only while it is still this lock's.
//
// The identity check is not a nicety. A lock reclaimed as abandoned - because
// this process was stopped past the staleness bound on a shared filesystem, or
// because an operator removed the file - is replaced by a successor's lock at
// the same path. Removing it unconditionally would delete a lock another gup is
// actively relying on, and the process after that would walk straight in. A file
// that is already gone is not an error: the postcondition "this process no
// longer holds the lock" is satisfied either way.
func (l *Lock) releaseFile() error {
	owner, err := readOwner(l.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		// Present but unreadable: it is not provably ours, so it is left alone.
		return &TakenOverError{Path: l.path}
	}
	if owner.Nonce != l.nonce {
		return &TakenOverError{Path: l.path}
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("can not remove the gup lock file %s: %w", l.path, err)
	}
	return nil
}

// signalGuard releases every held lock when the process is interrupted.
//
// gup's subcommands exit through os.Exit, so a Ctrl-C during `gup update` would
// otherwise terminate the process with no deferred Release having run, leaving
// lock files that block the next command until they age out. One guard serves
// every lock: with several held at once, handling the signal per lock would race
// several goroutines into os.Exit and release only whichever won.
var signalGuard = struct { //nolint:gochecknoglobals // process-wide by definition
	mu    sync.Mutex
	locks []*Lock
	ch    chan os.Signal
	stop  chan struct{}
}{}

// handledSignals are the termination signals a process can act on. SIGKILL is
// absent because it cannot be caught; that path is covered by the PID check,
// which reclaims a dead owner's lock immediately. SIGHUP is present because
// closing a terminal is a routine way to end a long update.
var handledSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} //nolint:gochecknoglobals // a fixed list

// registerForSignals arranges for lock to be released if the process is
// interrupted, starting the watcher on the first registration.
func registerForSignals(lock *Lock) {
	signalGuard.mu.Lock()
	defer signalGuard.mu.Unlock()

	signalGuard.locks = append(signalGuard.locks, lock)
	if signalGuard.ch != nil {
		return
	}
	signalGuard.ch = make(chan os.Signal, 1)
	signalGuard.stop = make(chan struct{})
	signal.Notify(signalGuard.ch, handledSignals...)

	ch, stop := signalGuard.ch, signalGuard.stop
	go func() {
		select {
		case <-stop:
			return
		case sig := <-ch:
			releaseAllForSignal()
			exitProcess(exitStatusFor(sig))
		}
	}()
}

// unregisterFromSignals drops lock from the guard, stopping the watcher once
// nothing is held. Leaving signal.Notify installed after the last release would
// keep gup's altered SIGINT disposition in place for the rest of the process.
func unregisterFromSignals(lock *Lock) {
	signalGuard.mu.Lock()
	defer signalGuard.mu.Unlock()

	signalGuard.locks = slices.DeleteFunc(signalGuard.locks, func(l *Lock) bool { return l == lock })
	if len(signalGuard.locks) > 0 || signalGuard.ch == nil {
		return
	}
	signal.Stop(signalGuard.ch)
	close(signalGuard.stop)
	signalGuard.ch = nil
	signalGuard.stop = nil
}

// releaseAllForSignal removes the lock files of every held lock. It touches the
// files only, not the heartbeat goroutines or the in-process registry, because
// the process is about to exit and the only state that outlives it is on disk.
func releaseAllForSignal() {
	signalGuard.mu.Lock()
	locks := slices.Clone(signalGuard.locks)
	signalGuard.mu.Unlock()

	for _, l := range locks {
		_ = l.releaseFile()
	}
}

// exitStatusFor maps a signal to the exit status a shell expects from a process
// killed by it (128 + signal number), so scripts see the same status they would
// have seen without gup's handler installed.
func exitStatusFor(sig os.Signal) int {
	const signalExitBase = 128
	if s, ok := sig.(syscall.Signal); ok && s > 0 {
		return signalExitBase + int(s)
	}
	return 1
}

// inProcessLocks serializes acquisitions of the same path inside one process.
// The on-disk lock alone cannot do this: O_EXCL does not distinguish "held by
// another process" from "held by this one", so a second acquisition in the same
// process would be reported as a foreign conflict.
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
