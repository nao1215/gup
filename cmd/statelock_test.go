// Package cmd's state-lock tests set process-wide environment variables and
// swap package-level seams, so they do not run in parallel.
//
//nolint:paralleltest // see above
package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/lockfile"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// flagTrue is the string form of a boolean flag value, as cobra's Flags().Set
// takes it.
const flagTrue = "true"

// newLockTestCommand returns a cobra command wired to buf, which is what
// withStateLock's printer writes to.
func newLockTestCommand(buf *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{Use: testCmdGup}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())
	return cmd
}

// stubLockTargets makes name lock exactly paths, so a test can drive
// withStateLock without depending on how a real subcommand resolves its
// resources.
func stubLockTargets(t *testing.T, name string, paths ...string) {
	t.Helper()
	original, existed := commandLockPolicy[name]
	commandLockPolicy[name] = func(*cobra.Command, []string) ([]string, error) { return paths, nil }
	t.Cleanup(func() {
		if existed {
			commandLockPolicy[name] = original
			return
		}
		delete(commandLockPolicy, name)
	})
}

// Test_withStateLock_runsAndReleases covers the ordinary path: every resource
// the command declared is locked while it runs, its exit status is passed
// through unchanged, and the locks are gone afterwards.
func Test_withStateLock_runsAndReleases(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.lock")
	second := filepath.Join(dir, "second.lock")
	stubLockTargets(t, testCmdUpdate, first, second)

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	heldDuringRun := false
	got := withStateLock(print.New(buf, buf), cmd, nil, testCmdUpdate, func() int {
		heldDuringRun = fileExists(t, first) && fileExists(t, second)
		return 7
	})

	if got != 7 {
		t.Errorf("withStateLock() = %d, want the subcommand's status 7", got)
	}
	if !heldDuringRun {
		t.Error("the subcommand ran without every declared resource being locked")
	}
	for _, path := range []string{first, second} {
		if fileExists(t, path) {
			t.Errorf("%s was not released after the subcommand returned", path)
		}
	}
	if buf.Len() != 0 {
		t.Errorf("withStateLock() wrote %q on the success path, want nothing", buf.String())
	}
}

// Test_withStateLock_locksNothingWhenTheCommandWritesNothing covers `--dry-run`
// and `export --output`: an operation that changes no state must not queue
// behind a real update, let alone fail after waiting for one.
func Test_withStateLock_locksNothingWhenTheCommandWritesNothing(t *testing.T) {
	stubLockTargets(t, testCmdUpdate)

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	var lockedPaths []string
	original := acquireStateLock
	acquireStateLock = func(ctx context.Context, command string, paths ...string) (*lockfile.MultiLock, error) {
		lockedPaths = paths
		return lockfile.AcquireAll(ctx, command, paths...)
	}
	t.Cleanup(func() { acquireStateLock = original })

	if got := withStateLock(print.New(buf, buf), cmd, nil, testCmdUpdate, func() int { return 0 }); got != 0 {
		t.Errorf("withStateLock() = %d, want 0", got)
	}
	if len(lockedPaths) != 0 {
		t.Errorf("a command that writes nothing locked %v", lockedPaths)
	}
}

// Test_withStateLock_refusesWhenAnotherProcessHoldsTheLock is the reason this
// wrapper exists: a second gup must not start changing state behind the first
// one's back, and the user must be told which process is in the way.
func Test_withStateLock_refusesWhenAnotherProcessHoldsTheLock(t *testing.T) {
	stubLockTargets(t, testCmdRemove, filepath.Join(t.TempDir(), "gup.lock"))

	original := acquireStateLock
	acquireStateLock = func(_ context.Context, _ string, paths ...string) (*lockfile.MultiLock, error) {
		return nil, &lockfile.BusyError{
			Path:  paths[0],
			Owner: lockfile.Owner{PID: 4242, Host: "workstation", Command: testCmdUpdate},
		}
	}
	t.Cleanup(func() { acquireStateLock = original })

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, nil, testCmdRemove, func() int {
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
// lock that cannot be released cleanly. The work already succeeded, so turning
// that into a failed command would be a lie; the problem is reported and the
// subcommand's status stands.
func Test_withStateLock_reportsAReleaseFailureWithoutDiscardingTheResult(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "gup.lock")
	stubLockTargets(t, testCmdUpdate, lockPath)

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	got := withStateLock(print.New(buf, buf), cmd, nil, testCmdUpdate, func() int {
		// Replace the lock with another process's: releasing must then report the
		// overlap rather than delete a lock it no longer owns.
		if err := os.WriteFile(lockPath, []byte(`{"pid":1,"nonce":"someone-else"}`), 0o600); err != nil {
			t.Fatalf("failed to replace the lock file: %v", err)
		}
		return 0
	})

	if got != 0 {
		t.Errorf("withStateLock() = %d, want the subcommand's own status 0", got)
	}
	if !strings.Contains(buf.String(), "taken over") {
		t.Errorf("withStateLock() did not report the release problem; output = %q", buf.String())
	}
}

// Test_withStateLock_propagatesContextCancellation covers Ctrl-C arriving while
// gup waits its turn: the wait ends with the context's error rather than the
// full timeout, and the subcommand never runs.
func Test_withStateLock_propagatesContextCancellation(t *testing.T) {
	stubLockTargets(t, testCmdImport, filepath.Join(t.TempDir(), "gup.lock"))

	original := acquireStateLock
	acquireStateLock = func(ctx context.Context, _ string, _ ...string) (*lockfile.MultiLock, error) {
		return nil, ctx.Err()
	}
	t.Cleanup(func() { acquireStateLock = original })

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, nil, testCmdImport, func() int {
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

// Test_withStateLock_toleratesACommandWithNoContext covers a command whose Run
// is invoked without going through Execute. cobra only fills the context in
// ExecuteC, so Context() is nil there, and handing that to the lock would panic
// on ctx.Err() -- a safety mechanism that crashes the program it protects is
// worse than none.
func Test_withStateLock_toleratesACommandWithNoContext(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "gup.lock")
	stubLockTargets(t, testCmdUpdate, lockPath)

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: testCmdGup}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Deliberately no SetContext: this is what a directly invoked Run sees.

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, nil, testCmdUpdate, func() int {
		ran = true
		return 0
	})

	if got != 0 || !ran {
		t.Errorf("withStateLock() = %d (ran=%v), want the subcommand to run and return 0; output = %q", got, ran, buf.String())
	}
}

// Test_withStateLock_refusesAnUnclassifiedCommand covers the developer mistake
// this design is built to catch. A subcommand that reaches withStateLock without
// an entry in commandLockPolicy has not been decided about, and running it
// unlocked would erode the guarantee silently.
func Test_withStateLock_refusesAnUnclassifiedCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, nil, "brand-new-command", func() int {
		ran = true
		return 0
	})

	if got != 1 || ran {
		t.Errorf("withStateLock() = %d (ran=%v), want a refusal for an unclassified command", got, ran)
	}
	if !strings.Contains(buf.String(), "commandLockPolicy") {
		t.Errorf("the error %q does not say where to classify the command", buf.String())
	}
}

// Test_withStateLock_reportsATargetResolutionFailure asserts that a resolution
// failure is surfaced instead of being silently treated as "nothing to lock",
// which would run the command unprotected.
func Test_withStateLock_reportsATargetResolutionFailure(t *testing.T) {
	commandLockPolicy["target-failure"] = func(*cobra.Command, []string) ([]string, error) {
		return nil, errors.New("cannot decide what to lock")
	}
	t.Cleanup(func() { delete(commandLockPolicy, "target-failure") })

	buf := new(bytes.Buffer)
	cmd := newLockTestCommand(buf)

	ran := false
	got := withStateLock(print.New(buf, buf), cmd, nil, "target-failure", func() int {
		ran = true
		return 0
	})

	if got != 1 || ran {
		t.Errorf("withStateLock() = %d (ran=%v), want a refusal when the resources cannot be resolved", got, ran)
	}
	if !strings.Contains(buf.String(), "cannot decide what to lock") {
		t.Errorf("the resolution error was not reported; output = %q", buf.String())
	}
}

// Test_commandLockPolicy_classifiesEveryRegisteredCommand is what keeps the
// policy from falling behind the CLI. Adding a mutating subcommand and
// forgetting to lock it is invisible: the command works until two of them run at
// once on a user's machine. Every registered command must be classified, and
// every classification must name a command that exists.
func Test_commandLockPolicy_classifiesEveryRegisteredCommand(t *testing.T) {
	registered := map[string]bool{}
	for _, sub := range newRootCmd().Commands() {
		// cobra adds `help` itself during Execute; it runs no gup code.
		if sub.Name() == testCmdHelp {
			continue
		}
		registered[sub.Name()] = true
	}
	if len(registered) == 0 {
		t.Fatal("no subcommands were found; the enumeration is broken, not the policy")
	}

	for name := range registered {
		if _, ok := commandLockPolicy[name]; !ok {
			t.Errorf("%q is registered but not classified in commandLockPolicy: decide whether it mutates state", name)
		}
	}
	for name := range commandLockPolicy {
		// `man` is registered only on platforms that have man(1), so its policy
		// entry legitimately outlives its registration on Windows.
		if name == "man" {
			continue
		}
		if !registered[name] {
			t.Errorf("commandLockPolicy classifies %q, which is no longer a subcommand", name)
		}
	}
}

// Test_lockTargets covers what each mutating command declares it will change.
// These are the assertions that catch a lock guarding the wrong resource, which
// is indistinguishable from a working lock until two processes disagree about
// which file they share.
func Test_lockTargets(t *testing.T) {
	gobin := t.TempDir()
	configHome := t.TempDir()
	after := t.TempDir()
	explicit := filepath.Join(t.TempDir(), "shared-gup.json")

	t.Setenv("GOBIN", gobin)
	// Run from a directory with no ./gup.json so resolution lands on the
	// user-level config, which is what the default cases assert.
	t.Chdir(t.TempDir())
	originalConfigHome := xdg.ConfigHome
	xdg.ConfigHome = configHome
	t.Cleanup(func() { xdg.ConfigHome = originalConfigHome })

	defaultConfig := filepath.Join(configHome, "gup", "gup.json")

	tests := []struct {
		name  string
		args  []string
		flags map[string]string
		want  []string
	}{
		{
			name: testCmdUpdate,
			want: []string{lockfile.PathForDir(gobin), lockfile.PathForFile(defaultConfig)},
		},
		{
			name:  testCmdUpdate + " --dry-run",
			flags: map[string]string{fnDryRun: flagTrue},
			want:  nil,
		},
		{
			// The lock must follow --file, or two processes writing one shared
			// config from different XDG_CONFIG_HOME values would not contend.
			name:  testCmdUpdate + " --file",
			flags: map[string]string{"file": explicit},
			want:  []string{lockfile.PathForDir(gobin), lockfile.PathForFile(explicit)},
		},
		{
			name: testCmdImport,
			want: []string{lockfile.PathForDir(gobin)},
		},
		{
			name:  testCmdImport + " --dry-run",
			flags: map[string]string{fnDryRun: flagTrue},
			want:  nil,
		},
		{
			// export writes gup.json and its content is a snapshot of $GOBIN, so a
			// concurrent remove must not delete a binary halfway through the walk.
			name: testCmdExport,
			want: []string{lockfile.PathForDir(gobin), lockfile.PathForFile(defaultConfig)},
		},
		{
			name:  testCmdExport + " --output",
			flags: map[string]string{"output": flagTrue},
			want:  nil,
		},
		{
			name:  testCmdExport + " --file",
			flags: map[string]string{"file": explicit},
			want:  []string{lockfile.PathForDir(gobin), lockfile.PathForFile(explicit)},
		},
		{
			name: testCmdRemove,
			want: []string{lockfile.PathForDir(gobin)},
		},
		{
			// migrate writes into AFTER_PATH, which is neither $GOBIN nor gup.json.
			name: testCmdMigrate,
			args: []string{t.TempDir(), after},
			want: []string{lockfile.PathForDir(after)},
		},
		{
			name:  testCmdMigrate + " --dry-run",
			args:  []string{t.TempDir(), after},
			flags: map[string]string{fnDryRun: flagTrue},
			want:  nil,
		},
		{
			// pin resolves its target against the installed binaries, so it locks
			// $GOBIN for the same reason export does.
			name: testCmdPin,
			want: []string{lockfile.PathForDir(gobin), lockfile.PathForFile(defaultConfig)},
		},
		{
			// unpin names an entry in gup.json and never looks at $GOBIN.
			name: testCmdUnpin,
			want: []string{lockfile.PathForFile(defaultConfig)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := strings.Fields(tt.name)[0]
			targets, ok := commandLockPolicy[command]
			if !ok || targets == nil {
				t.Fatalf("%q has no lock targets", command)
			}

			cmd := findSubcommand(t, command)
			for flag, value := range tt.flags {
				if err := cmd.Flags().Set(flag, value); err != nil {
					t.Fatalf("failed to set --%s: %v", flag, err)
				}
			}

			got, err := targets(cmd, tt.args)
			if err != nil {
				t.Fatalf("lock targets error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("lock targets = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_lockTargets_binDirIsIndependentOfTheConfigDirectory is the property that
// a config-directory-derived lock could not provide: two gup processes started
// with different XDG_CONFIG_HOME values but the same $GOBIN must contend for the
// same lock, because they install into and delete from the same directory.
func Test_lockTargets_binDirIsIndependentOfTheConfigDirectory(t *testing.T) {
	gobin := t.TempDir()
	t.Setenv("GOBIN", gobin)
	t.Chdir(t.TempDir())

	originalConfigHome := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = originalConfigHome })

	seen := make([]string, 0, 2)
	for range 2 {
		xdg.ConfigHome = t.TempDir()
		got, err := binDirLockTargets(findSubcommand(t, testCmdRemove), nil)
		if err != nil {
			t.Fatalf("binDirLockTargets() error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("binDirLockTargets() = %v, want exactly one lock", got)
		}
		seen = append(seen, got[0])
	}

	if seen[0] != seen[1] {
		t.Errorf("two configuration directories produced different $GOBIN locks (%q vs %q); processes sharing a $GOBIN would not contend",
			seen[0], seen[1])
	}
	if !strings.HasPrefix(seen[0], gobin) {
		t.Errorf("the $GOBIN lock %q does not live in $GOBIN %q", seen[0], gobin)
	}
}

// findSubcommand returns a freshly built subcommand by name, so each case starts
// from default flag values.
func findSubcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, sub := range newRootCmd().Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	t.Fatalf("no %q subcommand is registered", name)
	return nil
}

// Test_dirLockTarget_onlyLocksAnExistingDirectory covers why the lock does not
// create the directory it guards. Creating it would take the resource into
// existence before the command has looked at it: a path that is really a regular
// file would stop producing the command's own diagnosis and start producing a
// lock-file error, and a directory the command creates with its own permissions
// would get the lock's instead.
func Test_dirLockTarget_onlyLocksAnExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "a-file")
	if err := os.WriteFile(regular, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	if got := dirLockTarget(dir); len(got) != 1 || got[0] != lockfile.PathForDir(dir) {
		t.Errorf("dirLockTarget(existing directory) = %v, want the directory's lock", got)
	}
	if got := dirLockTarget(regular); got != nil {
		t.Errorf("dirLockTarget(regular file) = %v, want no lock so the command reports the problem itself", got)
	}
}

// Test_dirLockTarget_locksATargetThatDoesNotExistYet is the first-run case, and
// the one most likely to collide: two `gup import` runs pointed at a new $GOBIN,
// or two migrations into a new AFTER_PATH, would otherwise both find nothing to
// lock and install into the same directory at once.
func Test_dirLockTarget_locksATargetThatDoesNotExistYet(t *testing.T) {
	target := filepath.Join(t.TempDir(), "not-created-yet")

	got := dirLockTarget(target)
	if len(got) != 1 || got[0] != lockfile.PathForDir(target) {
		t.Fatalf("dirLockTarget(missing directory) = %v, want it created and locked", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("the directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s was created but is not a directory", target)
	}
	// The permission must be the one the commands use, or taking the lock first
	// would silently change what a user ends up with. Only POSIX applies the mode
	// passed to MkdirAll; Windows derives directory permissions from ACLs and
	// reports 0777 whatever was asked for, so there is nothing to compare there.
	if runtime.GOOS != goosWindows {
		if perm := info.Mode().Perm(); perm != binDirPerm {
			t.Errorf("created the directory with mode %o, want %o", perm, binDirPerm)
		}
	}
}

// Test_dirLockTarget_leavesAnUncreatableTargetToTheCommand covers a parent that
// rejects the mkdir: no lock, so the command reports the real problem instead of
// a lock-file error about the same one.
func Test_dirLockTarget_leavesAnUncreatableTargetToTheCommand(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "a-file")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	if got := dirLockTarget(filepath.Join(blocker, "under-a-file")); got != nil {
		t.Errorf("dirLockTarget(uncreatable path) = %v, want no lock", got)
	}
}

// Test_migrateLockTargets_leavesAnInvalidAfterPathToTheCommand is the regression
// the end-to-end suite caught: locking AFTER_PATH created it, so `gup migrate`
// pointed at a regular file reported "can not create directory for the gup lock
// file" instead of "AFTER_PATH is not a directory".
func Test_migrateLockTargets_leavesAnInvalidAfterPathToTheCommand(t *testing.T) {
	dir := t.TempDir()
	afterFile := filepath.Join(dir, "after-is-a-file")
	if err := os.WriteFile(afterFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	got, err := migrateLockTargets(findSubcommand(t, testCmdMigrate), []string{dir, afterFile})
	if err != nil {
		t.Fatalf("migrateLockTargets() error: %v", err)
	}
	if got != nil {
		t.Errorf("migrateLockTargets() = %v, want no lock so migrate reports the bad AFTER_PATH itself", got)
	}
}

// Test_migrateLockTargets_doesNotCreateAfterPathForAnInvalidBeforePath covers
// the other half of that rule, and a side effect worth avoiding: resolving the
// lock CREATES AFTER_PATH, and it ran before migrate validated anything. A
// migration that cannot run - a BEFORE_PATH that does not exist - would fail and
// still leave a new directory behind on the user's disk.
func Test_migrateLockTargets_doesNotCreateAfterPathForAnInvalidBeforePath(t *testing.T) {
	dir := t.TempDir()
	missingBefore := filepath.Join(dir, "no-such-gobin")
	after := filepath.Join(dir, "new-gobin")

	got, err := migrateLockTargets(findSubcommand(t, testCmdMigrate), []string{missingBefore, after})
	if err != nil {
		t.Fatalf("migrateLockTargets() error: %v", err)
	}
	if got != nil {
		t.Errorf("migrateLockTargets() = %v, want no lock so migrate reports the bad BEFORE_PATH itself", got)
	}
	if _, err := os.Stat(after); !os.IsNotExist(err) {
		t.Errorf("AFTER_PATH was created for a migration that cannot run: %v", err)
	}
}

// Test_configFileLockTargets_locksTheFileASymlinkPointsAt covers the config that
// a dotfile manager (stow, chezmoi, yadm) linked into place. Writing follows the
// link and rewrites its target, so a lock beside the link would guard a file
// nobody writes: `--file link/gup.json` and `--file real/gup.json` would take
// two different locks on one file and never contend.
func Test_configFileLockTargets_locksTheFileASymlinkPointsAt(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("symlink creation needs privileges on Windows; the rule is POSIX-specific here")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real", "gup.json")
	if err := os.MkdirAll(filepath.Dir(real), 0o750); err != nil {
		t.Fatalf("failed to create the target directory: %v", err)
	}
	if err := os.WriteFile(real, []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}
	link := filepath.Join(dir, "link-gup.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("failed to link the config: %v", err)
	}

	cmd := findSubcommand(t, testCmdPin)
	if err := cmd.Flags().Set("file", link); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}
	got, err := configFileLockTargets(cmd, nil)
	if err != nil {
		t.Fatalf("configFileLockTargets() error: %v", err)
	}
	if want := []string{lockfile.PathForFile(real)}; !slices.Equal(got, want) {
		t.Errorf("configFileLockTargets() = %v, want %v (the file the write lands on)", got, want)
	}
}

// Test_resolveConfigPaths_answersOncePerCommand is the anti-race rule. Where the
// write lands depends on what exists on disk when the question is asked, so
// asking twice can produce two answers: the lock is taken for the first and the
// write goes to the second. Here another process creates ./gup.json between the
// two questions, which is exactly the case that would move an unlocked write.
func Test_resolveConfigPaths_answersOncePerCommand(t *testing.T) {
	configHome := t.TempDir()
	originalConfigHome := xdg.ConfigHome
	xdg.ConfigHome = configHome
	t.Cleanup(func() { xdg.ConfigHome = originalConfigHome })
	t.Chdir(t.TempDir())

	cmd := newLockTestCommand(&bytes.Buffer{})
	read, write, err := resolveConfigPaths(cmd, "")
	if err != nil {
		t.Fatalf("resolveConfigPaths() error: %v", err)
	}
	if want := filepath.Join(configHome, "gup", "gup.json"); write != want {
		t.Fatalf("write path = %q, want the user-level config %q", write, want)
	}

	// Another gup, started in this directory, creates the local config.
	if err := os.WriteFile("gup.json", []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write ./gup.json: %v", err)
	}

	gotRead, gotWrite, err := resolveConfigPaths(cmd, "")
	if err != nil {
		t.Fatalf("resolveConfigPaths() error: %v", err)
	}
	if gotRead != read || gotWrite != write {
		t.Errorf("resolveConfigPaths() = (%q, %q) the second time, want the answer the lock was taken for (%q, %q)",
			gotRead, gotWrite, read, write)
	}
}

// Test_resolveConfigPaths_isPerFileValue covers the memo's key: a different
// --file is a different question, and answering it from the previous answer
// would lock one file and write another.
func Test_resolveConfigPaths_isPerFileValue(t *testing.T) {
	t.Chdir(t.TempDir())
	cmd := newLockTestCommand(&bytes.Buffer{})
	if _, _, err := resolveConfigPaths(cmd, ""); err != nil {
		t.Fatalf("resolveConfigPaths() error: %v", err)
	}

	explicit := filepath.Join(t.TempDir(), "other-gup.json")
	_, write, err := resolveConfigPaths(cmd, explicit)
	if err != nil {
		t.Fatalf("resolveConfigPaths() error: %v", err)
	}
	if write != explicit {
		t.Errorf("write path = %q, want %q", write, explicit)
	}
}

// Test_resolveConfigPaths_withoutACommandContext covers a command whose Run is
// invoked directly, with no context for cobra to have filled: the resolution
// still has to work, it simply cannot be remembered.
func Test_resolveConfigPaths_withoutACommandContext(t *testing.T) {
	t.Chdir(t.TempDir())
	explicit := filepath.Join(t.TempDir(), "explicit-gup.json")

	for _, cmd := range []*cobra.Command{nil, {Use: testCmdGup}} {
		_, write, err := resolveConfigPaths(cmd, explicit)
		if err != nil {
			t.Fatalf("resolveConfigPaths() error: %v", err)
		}
		if write != explicit {
			t.Errorf("write path = %q, want %q", write, explicit)
		}
	}
}

// Test_updateLockTargets_locksTheFileASymlinkPointsAt is the same rule
// configFileLockTargets follows, applied to the command that was still missing
// it. `gup update --file link/gup.json` writes through the link, so a lock
// beside the link would let it run alongside a `gup pin --file real/gup.json`
// rewriting the very same file.
func Test_updateLockTargets_locksTheFileASymlinkPointsAt(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("symlink creation needs privileges on Windows; the rule is POSIX-specific here")
	}
	gobin := t.TempDir()
	t.Setenv("GOBIN", gobin)

	dir := t.TempDir()
	real := filepath.Join(dir, "real-gup.json")
	if err := os.WriteFile(real, []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}
	link := filepath.Join(dir, "link-gup.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("failed to link the config: %v", err)
	}

	cmd := findSubcommand(t, testCmdUpdate)
	if err := cmd.Flags().Set("file", link); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}
	got, err := updateLockTargets(cmd, nil)
	if err != nil {
		t.Fatalf("updateLockTargets() error: %v", err)
	}
	want := []string{lockfile.PathForDir(gobin), lockfile.PathForFile(real)}
	if !slices.Equal(got, want) {
		t.Errorf("updateLockTargets() = %v, want %v (the file the write lands on)", got, want)
	}
}

// Test_installedBinDirLockTarget_doesNotCreateTheDirectory covers the difference
// between locking a directory a command WRITES and one it only reads. The
// writers create it, because two first runs would otherwise install into a
// directory neither had locked; a reader has nothing to collide over, and
// `gup export` creating $GOBIN as a side effect would be a surprise.
func Test_installedBinDirLockTarget_doesNotCreateTheDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-gobin")
	t.Setenv("GOBIN", missing)

	if got := installedBinDirLockTarget(); got != nil {
		t.Errorf("installedBinDirLockTarget() = %v, want no lock for a $GOBIN that does not exist", got)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("$GOBIN was created by a command that only reads it: %v", err)
	}
}
