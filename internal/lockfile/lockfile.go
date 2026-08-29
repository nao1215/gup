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
//
// Every step that could act on the WRONG file - taking a lock over, refreshing
// it, releasing it - is made to act on a file it holds rather than on a path it
// looked at a moment ago. Taking over and releasing rename the file aside first,
// because only one process can rename a given file, and then check what they
// took; the heartbeat verifies and refreshes through a single descriptor. A path
// checked and then acted on is two operations on what may be two different
// files, which is how a lock ends up refreshing, or deleting, its successor's.
package lockfile

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// pidTrustMultiple bounds how long a live local PID is taken as proof that
	// the lock is still held, expressed in multiples of the staleness bound.
	//
	// The PID check has to be bounded or it becomes unfalsifiable. A gup killed
	// with SIGKILL leaves its PID in the lock file, and once the operating system
	// recycles that number onto an unrelated process - which macOS does within
	// tens of thousands of spawns, and a container with a small pid_max does in
	// hours - processAlive answers "yes" forever. The lock would then never be
	// reclaimed by anything, and gup would keep telling the user it reclaims
	// abandoned locks by itself while doing no such thing.
	//
	// An hour is chosen to be far longer than any pause the heartbeat is expected
	// to survive (a suspended process, a throttled container, a laptop asleep for
	// a while) and far shorter than "forever".
	pidTrustMultiple = 60

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

// pidTrustWindow is how long a lock file may go untouched before its recorded
// PID stops being believed, however alive that PID looks.
func pidTrustWindow() time.Duration { return pidTrustMultiple * staleAfter() }

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
	path, err := normalizePath(path)
	if err != nil {
		return nil, err
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

		owner, observed, stale := inspect(path)
		var reclaimErr error
		if stale {
			reclaimErr = reclaim(path, observed)
			switch {
			case errors.Is(reclaimErr, errLockChanged):
				// The file is no longer the one that was judged abandoned, so the
				// judgment does not apply to what is there now. Look again rather than
				// report a lock that may well be held.
				stale, reclaimErr = false, nil
			case reclaimErr != nil && errors.Is(reclaimErr, fs.ErrPermission):
				// A permission problem will not resolve by waiting, and waiting would
				// only replace a clear diagnosis with a timeout.
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

// errLockChanged reports that the file acted on was not the one the caller had
// observed, so whatever was decided about it has to be decided again.
var errLockChanged = errors.New("the gup lock file changed while it was being examined")

// reclaim removes an abandoned lock file - but only the exact file that was
// judged abandoned.
//
// Renaming is the atomic step. Only one process can rename a given file, so two
// processes that both judged it abandoned cannot both go on to create it; the
// loser sees the winner's fresh lock on its next attempt. What renaming alone
// does NOT give is a guarantee that the file being taken over is still the one
// the decision was about: between the judgment and the rename, the owner's
// heartbeat may have run, or a faster process may have reclaimed the file and
// created its own. Removing THAT file would hand the lock to a second process
// while the first is still working, which is the one outcome this package
// exists to prevent.
//
// So the file is examined after it is detached, when it can no longer change,
// and put back untouched unless it is byte for byte - and modification time for
// modification time - the file that was judged. A file that has already vanished
// counts as reclaimed: there is nothing left to hold.
func reclaim(path string, observed state) error {
	if !observed.exists {
		// It was already gone when it was looked at, so there is nothing to take
		// over and nothing to prove; the caller simply tries to create it again.
		return nil
	}
	aside, err := detach(path, "stale")
	if err != nil || aside == "" {
		return err
	}
	if !observed.matches(aside) {
		restore(aside, path)
		return errLockChanged
	}
	_ = os.Remove(aside)
	return nil
}

// detach renames the lock file aside so it can be examined and disposed of
// without a file created after the look being mistaken for it. It returns an
// empty name when the file has already gone, which every caller treats as
// success: the postcondition each of them wants is that the observed file is no
// longer at the path.
func detach(path, reason string) (string, error) {
	aside := fmt.Sprintf("%s.%s-%d-%d", path, reason, os.Getpid(), nowFunc().UnixNano())
	if err := os.Rename(path, aside); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return aside, nil
}

// restore puts a detached lock file back after it turns out not to have been the
// caller's to take.
//
// It renames unconditionally, and that is deliberate. Reaching here means a file
// belonging to a live owner was briefly detached, and putting it back is what
// keeps that owner's lock. A third process that created its own lock at the path
// inside that window loses it - but it finds out, because every operation it
// goes on to perform checks the nonce and reports a take-over, whereas the owner
// whose file was dropped would carry on with no lock and no idea.
func restore(aside, path string) {
	if err := os.Rename(aside, path); err != nil {
		// Nothing further can be done: the file cannot go back and must not be
		// left lying around under a name no owner will ever recognize.
		_ = os.Remove(aside)
	}
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

	// Between the exclusive create and the line above, this process holds a lock
	// file that names nobody: an empty file, which no waiter can attribute to a
	// live owner. A process descheduled there for longer than the staleness bound
	// has its file reclaimed, and would otherwise return from here believing it
	// took the lock while its successor was already working. The acquisition is
	// only real if the file at the path is still the one that was created.
	if !ownsFile(path, nonce) {
		return nil, fmt.Errorf("the gup lock file %s was taken over while it was being written: %w", path, fs.ErrExist)
	}

	return &Lock{path: path, nonce: nonce}, nil
}

// ownsFile reports whether the lock file at path carries this nonce.
func ownsFile(path, nonce string) bool {
	owner, err := readOwner(path)
	return err == nil && owner.Nonce != "" && owner.Nonce == nonce
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
// When the file was written by a live process ON THIS HOST, that answer wins
// over the heartbeat: a `gup update` suspended with Ctrl-Z, a laptop resumed
// from sleep, or a container throttled for a minute all stop the heartbeat
// without stopping the process, and treating those as abandoned would hand the
// lock to a second gup while the first is still working.
//
// That trust is bounded, though (pidTrustMultiple). A PID outlives the process
// that owned it: once the operating system recycles the number, an unbounded
// check would answer "still held" forever and no gup would ever reclaim the
// file. Past the window the heartbeat decides again, so a lock is always
// reclaimable in the end - which is what the busy message promises the user.
//
// The heartbeat age is the fallback for everything the PID check cannot answer:
// a lock file from another machine on a shared home directory, one whose owner
// record is unreadable or truncated, or one naming no usable PID. A lock file
// that vanished between the failed create and this read reports abandoned so the
// caller retries immediately.
//
// The returned state is what the verdict rests on. It travels with the verdict
// so the take-over can prove it is removing the file that was judged, and not
// one that replaced it in the meantime.
func inspect(path string) (Owner, state, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return Owner{}, state{}, true
	}
	observed := state{exists: true, mod: info.ModTime()}

	var owner Owner
	attributable := false
	if raw, readErr := os.ReadFile(filepath.Clean(path)); readErr == nil {
		observed.raw = raw
		var parsed Owner
		if json.Unmarshal(raw, &parsed) == nil {
			owner, attributable = parsed, true
		}
	}

	age := nowFunc().Sub(observed.mod)
	if attributable && owner.PID > 0 && ownedByThisHost(owner) && age <= pidTrustWindow() {
		return owner, observed, !processAlive(owner.PID)
	}

	// Not attributable to a live local process - another host, an unreadable
	// record, or a PID too old to still believe - so the heartbeat decides. A
	// file that is still being touched is held by someone; one that is not has
	// been abandoned. A freshly created file whose owner record has not been
	// written yet lands here too, and is correctly treated as held.
	if age > staleAfter() {
		return owner, observed, true
	}
	return owner, observed, false
}

// state is the lock file exactly as it looked when a decision was made about it:
// its content and its modification time. A take-over that cannot show the file
// is unchanged since then is a take-over of something it never examined.
type state struct {
	// exists is false when the file was already gone when it was looked at.
	exists bool
	// raw is the file's content, or nil when it could not be read.
	raw []byte
	// mod is the modification time the staleness verdict was measured against.
	mod time.Time
}

// matches reports whether the file at path is still the one this state
// describes. The modification time is compared as well as the content, because
// it is what the staleness verdict was computed from: a heartbeat that ran in
// the meantime writes the same bytes back and would otherwise pass unnoticed.
func (s state) matches(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(s.mod) {
		return false
	}
	if s.raw == nil {
		// The content could not be read when the verdict was reached, so the
		// modification time is the whole of what the verdict rested on - which is
		// the case for a lock file whose permissions deny reading it.
		return true
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	return err == nil && bytes.Equal(raw, s.raw)
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
				l.refresh()
			}
		}
	}()
}

// refresh moves the lock file's modification time forward to say the owner is
// still working - and only while the file is still this lock's, because
// refreshing a successor's lock would keep IT alive on this process's behalf and
// hide the successor's own death from everyone waiting.
//
// The check and the refresh go through ONE descriptor, and the refresh is a
// rewrite of the bytes that were just read rather than a change of timestamp by
// name. Checking the path and then touching the path is two operations on what
// may by then be two different files: a lock reclaimed in between would be read
// as this one and refreshed for its new owner. A descriptor cannot drift that
// way - it names the file it opened, so the worst case is a write to a file that
// has already been taken away, which nobody sees.
//
// Every failure is silent on purpose. A heartbeat that cannot be written makes
// this lock reclaimable earlier than it should be, which is a far smaller
// problem than interrupting the user's command over it.
func (l *Lock) refresh() {
	// The path is gup's own lock file, derived from the resource it guards.
	file, err := os.OpenFile(l.path, os.O_RDWR, lockFileMode)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	raw, err := io.ReadAll(file)
	if err != nil {
		return
	}
	var owner Owner
	if json.Unmarshal(raw, &owner) != nil || owner.Nonce == "" || owner.Nonce != l.nonce {
		return
	}
	// The same bytes, written back at the same offset: the content is unchanged
	// and the modification time moves, which is the whole of what a heartbeat is.
	_, _ = file.WriteAt(raw, 0)
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
// actively relying on, and the process after that would walk straight in.
//
// The file is therefore detached first and identified afterwards, for the same
// reason a take-over is: reading the nonce at a path and then deleting that path
// are two operations, and a lock reclaimed between them would be deleted on the
// strength of its predecessor's identity. A file that turns out to belong to
// someone else goes back where it was. A file that is already gone is not an
// error: the postcondition "this process no longer holds the lock" is satisfied
// either way.
func (l *Lock) releaseFile() error {
	aside, err := detach(l.path, "release")
	if err != nil {
		return fmt.Errorf("can not remove the gup lock file %s: %w", l.path, err)
	}
	if aside == "" {
		return nil
	}
	owner, err := readOwner(aside)
	if err != nil || owner.Nonce != l.nonce {
		// Not provably ours - a successor's lock, or a file too damaged to attribute
		// - so it is put back exactly as it was found.
		restore(aside, l.path)
		return &TakenOverError{Path: l.path}
	}
	if err := os.Remove(aside); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("can not remove the gup lock file %s: %w", l.path, err)
	}
	return nil
}

// A word on interruption, since the absence of a signal handler here is a
// decision rather than an omission.
//
// It is tempting to catch SIGINT and delete the lock files on the way out. That
// is wrong, and subtly so: deleting the file does not stop the work. The command
// that holds the lock is still installing binaries and rewriting gup.json in
// another goroutine, and a second gup started in the moment between the deletion
// and the process actually dying walks into exactly the overlap the lock exists
// to prevent - with no error anywhere, because both processes believe they hold
// it.
//
// So the lock is held until the process is gone, and nothing gets to shorten
// that. gup's long-running commands already cancel their work on a signal (see
// cmd's signal-canceling context): the run unwinds, the command returns, and
// the deferred Release removes the file - in that order, which is the order that
// is safe. A command that has no such handler is killed outright by the default
// disposition, and its lock file is reclaimed by the next gup the moment it
// notices the owning PID is gone. Neither path can leave two gups running.

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
