package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/lockfile"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// acquireStateLock is lockfile.AcquireAll, indirected so tests can observe what
// a subcommand asked to lock without arranging real concurrent processes.
var acquireStateLock = lockfile.AcquireAll //nolint:gochecknoglobals // test seam

// binDirPerm is the permission gup uses when it creates a directory it installs
// binaries into: a migrate AFTER_PATH, or a $GOBIN that does not exist yet. The
// lock and the commands share it so taking the lock first cannot change the
// permissions a user ends up with.
const binDirPerm = 0o755

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
		// A failure to remove a lock file does not invalidate the work that was
		// just done, so it is reported without changing the exit status: the next
		// gup run reclaims the file through the staleness check either way.
		if releaseErr := lock.Release(); releaseErr != nil {
			p.Err(releaseErr)
		}
	}()
	return run()
}

// binDirLockTargets locks the $GOBIN whose contents the command changes. It is
// the whole policy for `remove`, and part of it for `update` and `import`.
func binDirLockTargets(_ *cobra.Command, _ []string) ([]string, error) {
	gobin, err := goutil.GoBin()
	if err != nil {
		return nil, err
	}
	return dirLockTarget(gobin), nil
}

// dirLockTarget returns the lock guarding a directory whose contents a command
// is about to change, creating the directory when it does not exist yet.
//
// The directory is created here, with the same permission the commands use, for
// a reason worth stating: a target that does not exist yet is exactly when two
// processes are most likely to collide. Two `gup import` runs pointed at a new
// $GOBIN, or two migrations into a new AFTER_PATH, would otherwise both find
// nothing to lock and install into the same directory at once. Skipping the lock
// there would leave the first run of every command - the one nobody has tested
// on their machine yet - as the only unprotected one.
//
// What is NOT done here is forcing the issue when the path cannot be a
// directory. A regular file at the target, or a parent that rejects the mkdir,
// yields no lock so the command produces its own diagnosis ("AFTER_PATH is not a
// directory") instead of a lock-file error about the same problem. Nothing is
// written in that case anyway, because the command is about to fail on it.
func dirLockTarget(dir string) []string {
	if !fileutil.IsDir(dir) {
		if _, err := os.Lstat(dir); err == nil {
			// The path exists but is not a directory: the command says so better.
			return nil
		}
		if err := os.MkdirAll(dir, binDirPerm); err != nil {
			return nil
		}
	}
	return []string{lockfile.PathForDir(dir)}
}

// configFileLockTargets locks the gup.json the command rewrites, resolved
// exactly the way the command resolves it - including an explicit `--file`, so
// two processes writing one shared config contend even when their configuration
// directories differ.
func configFileLockTargets(cmd *cobra.Command, _ []string) ([]string, error) {
	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		return nil, err
	}
	resolved, err := resolveConfigPaths(cmd, confFile)
	if err != nil {
		return nil, err
	}
	return []string{lockfile.PathForFile(resolved.target)}, nil
}

// resolvedConfigKey keys the config resolution on a command's context.
type resolvedConfigKey struct{}

// resolvedConfig is the set of gup.json paths a command run works with.
type resolvedConfig struct {
	// rule names the resolution that produced these, because export resolves a
	// path by different rules than the commands that also read the file.
	rule string
	// confFile is the --file value they were resolved from, so a caller asking
	// about a different one is never answered from this.
	confFile string
	// read is the file the command reads its current state from, and write the
	// path it was asked to write, as the user spelled it - which is what messages
	// name, so a user sees the path they typed.
	read  string
	write string
	// target is write with symlinks resolved: the file the write actually lands
	// on, and therefore the only thing worth locking.
	//
	// It is carried rather than recomputed for the same reason the pair above is:
	// resolving a link twice is asking the filesystem a question twice, and a link
	// repointed between the lock and the write would send the write to a file the
	// command holds no lock on. writeConfigFile is handed this path, so its own
	// resolution has nothing left to follow.
	target string
}

// resolveConfigPaths returns the gup.json the command reads and the one it
// writes, resolving them at most once per command run.
//
// Resolving consults the filesystem: with no --file, whether ./gup.json exists
// at that moment decides both which config is read and where the write lands.
// Answering that question twice is a race the lock cannot cover. A command that
// starts with no config anywhere locks the user-level path; if another process
// creates ./gup.json while it works, a second resolution would send the write to
// ./gup.json instead - a file this command holds no lock on, and another process
// may be writing. So the answer is settled before the lock is taken and
// remembered on the command's context, and the command body reads the same
// answer the lock was taken for.
func resolveConfigPaths(cmd *cobra.Command, confFile string) (*resolvedConfig, error) {
	if cached := cachedConfigPaths(cmd, ruleImport, confFile); cached != nil {
		return cached, nil
	}
	read, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		return nil, err
	}
	write := configstate.ResolveWritePath(confFile, read)
	return newResolvedConfig(cmd, ruleImport, confFile, read, write)
}

// resolveExportPath is the same for `gup export`, which resolves where it writes
// by its own rule - an explicit --file, else the user-level config - and reads
// its saved channels back from that same file.
func resolveExportPath(cmd *cobra.Command, confFile string) (*resolvedConfig, error) {
	if cached := cachedConfigPaths(cmd, ruleExport, confFile); cached != nil {
		return cached, nil
	}
	write := config.ResolveExportFilePath(confFile)
	return newResolvedConfig(cmd, ruleExport, confFile, write, write)
}

// The resolution rules a resolvedConfig can come from.
const (
	ruleImport = "import"
	ruleExport = "export"
)

// newResolvedConfig follows the write path to the file it lands on and remembers
// the result for the rest of the command run.
func newResolvedConfig(cmd *cobra.Command, rule, confFile, read, write string) (*resolvedConfig, error) {
	target, err := fileutil.ResolveSymlinkTarget(write)
	if err != nil {
		return nil, fmt.Errorf("can not resolve config path %s: %w", write, err)
	}
	resolved := &resolvedConfig{rule: rule, confFile: confFile, read: read, write: write, target: target}
	rememberConfigPaths(cmd, resolved)
	return resolved, nil
}

// cachedConfigPaths returns the resolution already made for this command, or nil
// when there is none to reuse.
func cachedConfigPaths(cmd *cobra.Command, rule, confFile string) *resolvedConfig {
	if cmd == nil {
		return nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	cached, ok := ctx.Value(resolvedConfigKey{}).(*resolvedConfig)
	if !ok || cached.rule != rule || cached.confFile != confFile {
		return nil
	}
	return cached
}

// rememberConfigPaths stores a resolution for the rest of the command run. A
// command invoked directly, without cobra having given it a context, simply
// resolves again: the memo is an anti-race measure, not a cache for speed.
func rememberConfigPaths(cmd *cobra.Command, resolved *resolvedConfig) {
	if cmd == nil {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, resolvedConfigKey{}, resolved))
}

// updateLockTargets locks both resources `gup update` writes: the $GOBIN it
// installs into and the gup.json it may persist channels to. A --dry-run run
// writes neither, so it locks nothing and never waits behind a real update.
func updateLockTargets(cmd *cobra.Command, _ []string) ([]string, error) {
	dryRun, err := getFlagBool(cmd, "dry-run")
	if err != nil {
		return nil, err
	}
	if dryRun {
		return nil, nil
	}

	gobin, err := goutil.GoBin()
	if err != nil {
		return nil, err
	}
	confLock, err := configFileLockTargets(cmd, nil)
	if err != nil {
		return nil, err
	}
	return append(dirLockTarget(gobin), confLock...), nil
}

// importLockTargets locks the $GOBIN `gup import` installs into. It reads
// gup.json and never writes it, so the config file needs no lock; a --dry-run
// run installs nothing and needs none at all.
func importLockTargets(cmd *cobra.Command, _ []string) ([]string, error) {
	dryRun, err := getFlagBool(cmd, "dry-run")
	if err != nil {
		return nil, err
	}
	if dryRun {
		return nil, nil
	}
	return binDirLockTargets(cmd, nil)
}

// exportLockTargets locks the gup.json `gup export` writes and the $GOBIN it
// describes. The $GOBIN lock is not there because export writes to it - it does
// not - but because the file it writes is a snapshot of it: a `gup remove`
// deleting a binary halfway through the walk yields a gup.json listing a tool
// that is no longer installed, and a later `gup import` reinstalls it.
//
// With --output it prints to standard output and touches no file, so it locks
// nothing: making a command people pipe into other tools queue behind an update
// would be a regression, and the worst a torn read can do there is print a line
// the user can see.
func exportLockTargets(cmd *cobra.Command, _ []string) ([]string, error) {
	output, err := getFlagBool(cmd, "output")
	if err != nil {
		return nil, err
	}
	if output {
		return nil, nil
	}
	explicit, err := getFlagString(cmd, "file")
	if err != nil {
		return nil, err
	}
	resolved, err := resolveExportPath(cmd, explicit)
	if err != nil {
		return nil, err
	}
	return append(installedBinDirLockTarget(), lockfile.PathForFile(resolved.target)), nil
}

// pinLockTargets locks the gup.json `gup pin` rewrites and the $GOBIN it
// resolves the pin target against, for the reason export locks it: pinning a
// binary a concurrent `gup remove` is deleting writes a pin for a tool that is
// not installed. `gup unpin` needs no such lock - it names an entry in gup.json
// and never looks at $GOBIN.
func pinLockTargets(cmd *cobra.Command, args []string) ([]string, error) {
	confLock, err := configFileLockTargets(cmd, args)
	if err != nil {
		return nil, err
	}
	return append(installedBinDirLockTarget(), confLock...), nil
}

// installedBinDirLockTarget returns the lock guarding the $GOBIN a command
// READS, creating the directory when it is not there yet.
//
// Skipping the lock for a $GOBIN that does not exist looks safe - nothing is
// installed, so there is nothing to read - and is not. Whether it exists is
// precisely what another gup can change: an `import` racing an `export` creates
// $GOBIN and fills it, and the export, holding no lock because the directory was
// missing when it looked, reads a directory mid-install and writes what it found
// over a gup.json that described a complete tool set. An empty environment is a
// normal first run for export, which writes an empty configuration rather than
// failing, so that overwrite is silent.
//
// The directory is therefore created and locked, as it is for the commands that
// install into it. That leaves an empty $GOBIN behind on a machine that had
// none, which is the same thing `gup update` and `gup remove` already do, and a
// far smaller surprise than a gup.json rewritten from a half-populated read.
func installedBinDirLockTarget() []string {
	gobin, err := goutil.GoBin()
	if err != nil {
		return nil
	}
	return dirLockTarget(gobin)
}

// migrateLockTargets locks both directories `gup migrate` depends on: AFTER_PATH,
// which it installs into, and BEFORE_PATH, which it reads the versions out of.
// Neither is necessarily $GOBIN - migrate takes both as arguments and may touch
// neither the current $GOBIN nor gup.json - so locking $GOBIN instead would
// guard a resource it does not use and leave both of these unprotected.
//
// BEFORE_PATH is locked for the reason export locks $GOBIN: what migrate writes
// into AFTER_PATH is derived from what it read there, so a `gup remove` (or
// another migration installing into it) changing it mid-scan produces a
// migration of a tool set that never existed. Deadlock is not a concern even
// when two migrations name the directories the other way round, because the
// locks are taken in sorted order rather than the order they are given in.
func migrateLockTargets(cmd *cobra.Command, args []string) ([]string, error) {
	dryRun, err := getFlagBool(cmd, "dry-run")
	if err != nil {
		return nil, err
	}
	if dryRun || len(args) < migrateMinArgs {
		// Too few arguments is a usage error the command reports itself with a
		// better message than a lock failure would give.
		return nil, nil
	}
	// Locking AFTER_PATH creates it when it does not exist yet, which must not
	// happen for a migration that is going to be rejected: `gup migrate /nope
	// /tmp/new` would leave /tmp/new behind after failing. A BEFORE_PATH that is
	// not a directory is exactly that case, and runMigrate reports it with a
	// better message than any lock error could. Nothing is written in that case,
	// so nothing needs guarding.
	if !fileutil.IsDir(args[0]) {
		return nil, nil
	}
	after := dirLockTarget(args[1])
	if after == nil {
		// AFTER_PATH cannot be a directory (a regular file sits there, or its
		// parent refuses the mkdir). The command reports that better than any lock
		// error would, and it writes nothing, so neither directory needs guarding.
		return nil, nil
	}
	// BEFORE_PATH exists - the check above made sure - so this locks it without
	// creating anything.
	return append(dirLockTarget(args[0]), after...), nil
}
