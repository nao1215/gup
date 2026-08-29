package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/lockfile"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// newLockTestCommand returns a cobra command wired to buf, which is what
// withStateLock's printer writes to.
func newLockTestCommand(buf *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{Use: "gup"}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())
	return cmd
}

// Test_withStateLock_runsAndReleases covers the ordinary path: the subcommand
// runs while the lock file exists, its exit status is passed through unchanged,
// and the lock is gone afterwards so the next gup command can take it.
func Test_withStateLock_runsAndReleases(t *testing.T) { //nolint:paralleltest // swaps package-level test seams
	lockPath := filepath.Join(t.TempDir(), "gup.lock")
	restore := useLockPath(t, lockPath)
	defer restore()

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	ranWithLockHeld := false
	got := withStateLock(print.New(buf, buf), cmd, testCmdUpdate, func() int {
		ranWithLockHeld = fileExists(t, lockPath)
		return 7
	})

	if got != 7 {
		t.Errorf("withStateLock() = %d, want the subcommand's status 7", got)
	}
	if !ranWithLockHeld {
		t.Error("the subcommand ran without the lock file being held")
	}
	if fileExists(t, lockPath) {
		t.Error("the lock file was not released after the subcommand returned")
	}
	if buf.Len() != 0 {
		t.Errorf("withStateLock() wrote %q on the success path, want nothing", buf.String())
	}
}

// Test_withStateLock_refusesWhenAnotherProcessHoldsTheLock is the reason this
// wrapper exists: a second gup must not start updating $GOBIN behind the first
// one's back, and the user must be told which process is in the way.
func Test_withStateLock_refusesWhenAnotherProcessHoldsTheLock(t *testing.T) { //nolint:paralleltest // swaps package-level test seams
	original := acquireStateLock
	acquireStateLock = func(_ context.Context, path, _ string) (*lockfile.Lock, error) {
		return nil, &lockfile.BusyError{
			Path:  path,
			Owner: lockfile.Owner{PID: 4242, Host: "workstation", Command: testCmdUpdate},
		}
	}
	t.Cleanup(func() { acquireStateLock = original })

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, testCmdRemove, func() int {
		ran = true
		return 0
	})

	if got != 1 {
		t.Errorf("withStateLock() = %d, want 1 when the lock is held", got)
	}
	if ran {
		t.Error("the subcommand ran even though the lock could not be acquired")
	}
	for _, want := range []string{"another gup process is already running", "4242", "gup update"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("withStateLock() error output %q does not mention %q", buf.String(), want)
		}
	}
}

// Test_withStateLock_reportsAReleaseFailureWithoutDiscardingTheResult covers a
// lock file that cannot be removed (a read-only config directory, say). The work
// already succeeded, so turning that into a failed command would be a lie; the
// problem is reported and the subcommand's status stands.
func Test_withStateLock_reportsAReleaseFailureWithoutDiscardingTheResult(t *testing.T) { //nolint:paralleltest // swaps package-level test seams
	lockPath := filepath.Join(t.TempDir(), "gup.lock")
	restore := useLockPath(t, lockPath)
	defer restore()

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	got := withStateLock(print.New(buf, buf), cmd, testCmdUpdate, func() int {
		// Replace the lock file with a directory: os.Remove then fails with
		// ENOTEMPTY rather than the "already gone" case Release tolerates.
		if err := os.Remove(lockPath); err != nil {
			t.Fatalf("failed to remove the lock file: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(lockPath, "child"), 0o750); err != nil {
			t.Fatalf("failed to create the blocking directory: %v", err)
		}
		return 0
	})

	if got != 0 {
		t.Errorf("withStateLock() = %d, want the subcommand's own status 0", got)
	}
	if !strings.Contains(buf.String(), "lock file") {
		t.Errorf("withStateLock() did not report the release failure; output = %q", buf.String())
	}
}

// Test_withStateLock_propagatesContextCancellation covers Ctrl-C arriving while
// gup waits its turn: the wait ends with the context's error rather than the
// full timeout, and the subcommand never runs.
func Test_withStateLock_propagatesContextCancellation(t *testing.T) { //nolint:paralleltest // swaps package-level test seams
	original := acquireStateLock
	acquireStateLock = func(ctx context.Context, _, _ string) (*lockfile.Lock, error) {
		return nil, ctx.Err()
	}
	t.Cleanup(func() { acquireStateLock = original })

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, testCmdImport, func() int {
		ran = true
		return 0
	})

	if got != 1 {
		t.Errorf("withStateLock() = %d, want 1 when the context is already canceled", got)
	}
	if ran {
		t.Error("the subcommand ran despite a canceled context")
	}
	if !strings.Contains(buf.String(), context.Canceled.Error()) {
		t.Errorf("withStateLock() output %q does not report the cancellation", buf.String())
	}
}

// Test_stateLockPath_isBesideTheConfigFile pins the lock's location to gup's own
// config directory. A lock in a shared temp directory would be global to the
// machine rather than scoped to the gup.json it protects, so a user who
// relocates XDG_CONFIG_HOME per project would be serialized across projects.
func Test_stateLockPath_isBesideTheConfigFile(t *testing.T) { //nolint:paralleltest // reads process-wide config state
	got := stateLockPath()
	if want := filepath.Join(config.DirPath(), config.LockFileName); got != want {
		t.Errorf("stateLockPath() = %q, want %q", got, want)
	}
}

// Test_mutatingCommands_takeTheStateLock asserts the wiring itself: every
// command that changes $GOBIN or gup.json must go through withStateLock, and the
// read-only ones must not. Wiring is easy to forget on a newly added command, and
// the omission is invisible until two runs collide on a user's machine.
func Test_mutatingCommands_takeTheStateLock(t *testing.T) { //nolint:paralleltest // swaps package-level test seams
	mutating := []struct {
		name string
		args []string
	}{
		{name: testCmdUpdate, args: []string{testCmdUpdate, testFlagDryRun}},
		{name: testCmdImport, args: []string{testCmdImport, testFlagFile, "no-such-gup.json"}},
		{name: testCmdExport, args: []string{testCmdExport, testFlagFile, filepath.Join(t.TempDir(), "gup.json")}},
		{name: testCmdRemove, args: []string{testCmdRemove, testBinNoSuch}},
		{name: testCmdPin, args: []string{testCmdPin, testBinNoSuch, "v1.0.0"}},
		{name: testCmdUnpin, args: []string{testCmdUnpin, testBinNoSuch}},
		{name: testCmdMigrate, args: []string{testCmdMigrate, t.TempDir(), t.TempDir()}},
	}
	readOnly := []struct {
		name string
		args []string
	}{
		{name: testCmdList, args: []string{testCmdList}},
		{name: testCmdCheck, args: []string{testCmdCheck}},
		{name: testCmdVersion, args: []string{testCmdVersion}},
	}

	for _, tt := range mutating {
		t.Run(tt.name, func(t *testing.T) {
			if got := lockedCommandName(t, tt.args); got != tt.name {
				t.Errorf("%q acquired the state lock as %q, want %q (a mutating command must take the lock)",
					strings.Join(tt.args, " "), got, tt.name)
			}
		})
	}
	for _, tt := range readOnly { //nolint:paralleltest // each subtest swaps the same package-level seams
		t.Run(tt.name, func(t *testing.T) {
			if got := lockedCommandName(t, tt.args); got != "" {
				t.Errorf("%q acquired the state lock as %q; read-only commands must not block behind a running update",
					strings.Join(tt.args, " "), got)
			}
		})
	}
}

// lockedCommandName runs args through the real root command with the lock
// acquisition stubbed out, and reports the command name the lock was taken
// under, or "" when no lock was taken. The subcommand's own outcome is
// irrelevant here - the arguments deliberately name things that do not exist, so
// each command fails fast after the lock decision has already been made.
func lockedCommandName(t *testing.T, args []string) string {
	t.Helper()

	// Point $GOBIN at an empty directory so `update --dry-run` and `check` have
	// nothing to scan: this test is about which commands take the lock, and
	// letting them walk the developer's real $GOBIN would make it slow and
	// dependent on what happens to be installed.
	t.Setenv("GOBIN", t.TempDir())

	locked := ""
	originalAcquire := acquireStateLock
	acquireStateLock = func(ctx context.Context, path, command string) (*lockfile.Lock, error) {
		locked = command
		return lockfile.Acquire(ctx, path, command)
	}
	t.Cleanup(func() { acquireStateLock = originalAcquire })

	restore := useLockPath(t, filepath.Join(t.TempDir(), "gup.lock"))
	defer restore()

	originalExit := OsExit
	OsExit = func(int) {}
	t.Cleanup(func() { OsExit = originalExit })

	buf := new(bytes.Buffer)
	root := newRootCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil && !errors.Is(err, context.Canceled) {
		// A failing subcommand is expected; the lock decision happened first.
		t.Logf("%v returned %v", args, err)
	}
	return locked
}

// useLockPath points the state lock at path for the duration of a test.
func useLockPath(t *testing.T, path string) func() {
	t.Helper()
	original := stateLockPath
	stateLockPath = func() string { return path }
	return func() { stateLockPath = original }
}
