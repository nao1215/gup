// The owner record: who holds a lock, and the message a process waiting for one
// sees.
//
// None of this is the lock. The exclusion is the kernel's, decided before the
// file is read; what the holder writes into the lock file is a courtesy, so a
// waiter can name the command it is waiting for instead of reporting an
// anonymous "resource unavailable". A record that is missing, truncated, or
// written by another version of gup costs a message its detail and never its
// verdict.

package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

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
