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
// The read-only entries are a decision, not an omission. Every write gup
// performs lands through an atomic rename, so a reader sees either the previous
// complete gup.json or the next one, never a partial file; making readers block
// would trade a race that cannot happen for a `gup list` that hangs behind a
// long `gup update`.
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
	writePath, err := resolveConfigWritePath(cmd)
	if err != nil {
		return nil, err
	}
	return configFileLock(writePath)
}

// configFileLock returns the lock guarding the gup.json a command writes.
//
// It follows a symlink to the file the write actually lands on, because that is
// what the write does: writeConfigFile resolves the link and rewrites its
// target, so that a dotfile manager's link survives the update. A lock placed
// beside the LINK would leave `--file link/gup.json` and
// `--file real/gup.json` taking two different locks on one file, which is the
// case a lock scoped to the resource exists to catch.
func configFileLock(writePath string) ([]string, error) {
	resolved, err := fileutil.ResolveSymlinkTarget(writePath)
	if err != nil {
		return nil, fmt.Errorf("can not resolve config path %s: %w", writePath, err)
	}
	return []string{lockfile.PathForFile(resolved)}, nil
}

// resolveConfigWritePath returns where the command writes gup.json, using the
// one resolution the command is allowed to make (see resolveConfigPaths).
func resolveConfigWritePath(cmd *cobra.Command) (string, error) {
	confFile, err := getFlagString(cmd, "file")
	if err != nil {
		return "", err
	}
	_, writePath, err := resolveConfigPaths(cmd, confFile)
	return writePath, err
}

// resolvedConfigKey keys the config resolution on a command's context.
type resolvedConfigKey struct{}

// resolvedConfig is the pair of gup.json paths a command run works with: the one
// it reads and the one it writes.
type resolvedConfig struct {
	// confFile is the --file value the pair was resolved from, so a caller asking
	// about a different one is never answered from this.
	confFile string
	read     string
	write    string
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
func resolveConfigPaths(cmd *cobra.Command, confFile string) (read, write string, err error) {
	if cached := cachedConfigPaths(cmd, confFile); cached != nil {
		return cached.read, cached.write, nil
	}
	read, err = config.ResolveImportFilePath(confFile)
	if err != nil {
		return "", "", err
	}
	write = configstate.ResolveWritePath(confFile, read)
	rememberConfigPaths(cmd, &resolvedConfig{confFile: confFile, read: read, write: write})
	return read, write, nil
}

// cachedConfigPaths returns the resolution already made for this command, or nil
// when there is none to reuse.
func cachedConfigPaths(cmd *cobra.Command, confFile string) *resolvedConfig {
	if cmd == nil {
		return nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	cached, ok := ctx.Value(resolvedConfigKey{}).(*resolvedConfig)
	if !ok || cached.confFile != confFile {
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
	confLock, err := configFileLock(config.ResolveExportFilePath(explicit))
	if err != nil {
		return nil, err
	}
	return append(installedBinDirLockTarget(), confLock...), nil
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
// READS, and nothing when it cannot be resolved or does not exist yet.
//
// It deliberately does not create the directory the way dirLockTarget does. That
// creation exists for the commands that install into $GOBIN, where two first
// runs would otherwise collide in a directory neither had locked; a command that
// only reads $GOBIN has nothing to collide over, and creating a directory as a
// side effect of `gup export` would be a surprise. A $GOBIN that is not there
// holds no binaries either, so there is nothing to keep still.
func installedBinDirLockTarget() []string {
	gobin, err := goutil.GoBin()
	if err != nil || !fileutil.IsDir(gobin) {
		return nil
	}
	return []string{lockfile.PathForDir(gobin)}
}

// migrateLockTargets locks AFTER_PATH, the directory `gup migrate` installs
// into. That is deliberately NOT $GOBIN: migrate takes both directories as
// arguments and may touch neither the current $GOBIN nor gup.json, so locking
// $GOBIN would guard a resource it does not write while leaving the one it does
// unprotected.
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
	return dirLockTarget(args[1]), nil
}
