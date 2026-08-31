// The lock policy: which subcommands take a lock, and the wrapper that holds one
// while a command runs.
//
// The table below is the decision - mutating or not - for every subcommand gup
// registers. What each mutating one locks lives in locktargets.go, and how a
// lock is taken in internal/lockfile.

package cmd

import (
	"context"
	"fmt"

	"github.com/nao1215/gup/internal/lockfile"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// acquireStateLock is lockfile.AcquireAll, indirected so tests can observe what
// a subcommand asked to lock without arranging real concurrent processes.
var acquireStateLock = lockfile.AcquireAll //nolint:gochecknoglobals // test seam

// lockTargets computes the lock files a subcommand needs, from its flags and
// arguments. Returning an empty list means the command changes nothing and
// should not contend for anything.
type lockTargets func(cmd *cobra.Command, args []string) ([]string, error)

// commandLockPolicy classifies EVERY subcommand gup registers: a mutating one
// maps to the resources it writes, a read-only one maps to nil.
//
// It is a declaration rather than a convention because forgetting to lock a new
// mutating command is invisible - the command works perfectly until the day two
// of them run at once on someone else's machine. withStateLock refuses to run a
// command that is not listed here, and a test walks the registered commands to
// make sure the list has not fallen behind.
//
// The nil entries are a decision, not an omission. Every write gup performs
// lands through an atomic rename, so a reader sees either the previous complete
// gup.json or the next one, never a partial file; making readers block would
// trade a race that cannot happen for a `gup list` that hangs behind a long
// `gup update`.
//
// Two of those nil entries do write files, which is worth saying out loud rather
// than leaving to be discovered: `completion --install` rewrites shell profiles
// and completion files, and `man` writes man pages. They are unlocked on the
// merits, not by oversight. Both write through the same atomic replace gup.json
// gets, so no reader sees a partial file; both are deterministic, so two
// concurrent runs write byte-identical content and a lost update loses nothing;
// and the resources are the user's own dotfiles, where the cost of a lock is a
// .zshrc.lock left in their home directory whenever one is interrupted. The
// writer a lock could not exclude anyway - the user's editor - is the one that
// would actually lose work.
var commandLockPolicy = map[string]lockTargets{ //nolint:gochecknoglobals // the policy table itself

	cmdNameUpdate:     updateLockTargets,
	cmdNameImport:     importLockTargets,
	cmdNameExport:     exportLockTargets,
	cmdNameRemove:     binDirLockTargets,
	cmdNameMigrate:    migrateLockTargets,
	cmdNamePin:        pinLockTargets,
	cmdNameUnpin:      configFileLockTargets,
	cmdNameCheck:      nil,
	cmdNameList:       nil,
	cmdNameVersion:    nil,
	cmdNameCompletion: nil,
	cmdNameMan:        nil,
	cmdNameBugReport:  nil,
}

// withStateLock runs a mutating subcommand while holding a lock on each resource
// it writes, so two gup processes cannot change the same $GOBIN or the same
// gup.json at the same time.
//
// The locks are scoped to the RESOURCES, not to gup's configuration directory.
// That distinction is the whole point: $GOBIN and gup.json move independently,
// so a user with a per-project XDG_CONFIG_HOME still shares one $GOBIN across
// projects, and two commands given the same `--file` may be started from
// different configuration directories entirely. A single config-directory lock
// would serialize neither of those.
func withStateLock(p *print.Printer, cmd *cobra.Command, args []string, name string, run func() int) int {
	targets, ok := commandLockPolicy[name]
	if !ok {
		// Reaching this means a subcommand was added without deciding whether it
		// mutates state. Failing is the only safe answer: silently running unlocked
		// is how the guarantee erodes.
		p.Err(fmt.Errorf("internal error: %q is not classified in commandLockPolicy", name))
		return 1
	}

	// cobra fills the context in ExecuteC, so the production path always has one;
	// a command whose Run is invoked directly (a test, or a future caller that
	// builds the command itself) has a nil one, and passing that through would
	// panic on the first ctx.Err() rather than fail the command. The lock is a
	// safety mechanism, so it must not be the thing that crashes gup.
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var paths []string
	if targets != nil {
		var err error
		if paths, err = targets(cmd, args); err != nil {
			p.Err(err)
			return 1
		}
	}

	lock, err := acquireStateLock(ctx, name, paths...)
	if err != nil {
		p.Err(err)
		return 1
	}
	defer func() {
		// Releasing is dropping a kernel lock and closing a descriptor, so the only
		// way it fails is a filesystem problem on the way out. The work is already
		// done and correct - nothing overlapped it, because the lock was held for
		// every moment of it - so the failure is reported without changing the exit
		// status, and the next command takes the same lock file as if nothing had
		// happened.
		if releaseErr := lock.Release(); releaseErr != nil {
			p.Err(releaseErr)
		}
	}()
	return run()
}
