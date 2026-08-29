package cmd

import (
	"context"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/lockfile"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// acquireStateLock is lockfile.Acquire, indirected so tests can drive the
// failure branch of withStateLock without arranging a real concurrent process.
var acquireStateLock = lockfile.Acquire //nolint:gochecknoglobals // test seam

// stateLockPath is config.LockFilePath, indirected for the same reason.
var stateLockPath = config.LockFilePath //nolint:gochecknoglobals // test seam

// withStateLock runs a mutating subcommand while holding gup's advisory lock, so
// two gup processes cannot change $GOBIN or gup.json at the same time.
//
// The commands that need this are the ones that write: update, import, migrate,
// remove, export, pin and unpin. Their individual gup.json writes are already
// atomic, but atomicity is not exclusion - two concurrent `gup update` runs both
// read the same file, both install, and the one that finishes last persists a
// record of only its own work, silently discarding the other's. A `gup remove`
// racing a `gup update` on the same binary is the same collision with a worse
// result.
//
// The read-only commands (list, check, version, completion, man, bug-report) do
// NOT take the lock, and that is deliberate rather than an omission. Every write
// gup performs lands through an atomic rename, so a reader either sees the
// previous complete gup.json or the next one, never a partial file; making
// readers block would trade a race that cannot happen for a `gup list` that
// hangs behind a long `gup update`. What a reader can still observe is a
// consistent file that is simply out of date by the time it prints - which is
// true of any snapshot of a system still being changed, and is why `gup check`
// reports what it saw rather than promising it is still current.
func withStateLock(p *print.Printer, cmd *cobra.Command, command string, run func() int) int {
	// cobra fills the context in ExecuteC, so the production path always has one;
	// a command whose Run is invoked directly (a test, or a future caller that
	// builds the command itself) has a nil one, and passing that through would
	// panic on the first ctx.Err() rather than fail the command. The lock is a
	// safety mechanism, so it must not be the thing that crashes gup.
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	lock, err := acquireStateLock(ctx, stateLockPath(), command)
	if err != nil {
		p.Err(err)
		return 1
	}
	defer func() {
		// A failure to remove the lock file does not invalidate the work that was
		// just done, so it is reported without changing the exit status: the next
		// gup run reclaims the file through the staleness check either way.
		if releaseErr := lock.Release(); releaseErr != nil {
			p.Err(releaseErr)
		}
	}()
	return run()
}
