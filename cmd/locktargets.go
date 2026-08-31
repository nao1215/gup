// Which resources each subcommand locks.
//
// This is the per-command half of the lock policy: given a command's flags and
// arguments, what does it actually change? The answers live beside each other
// rather than beside the commands themselves so that the whole picture can be
// read at once - a lock guarding the wrong resource is indistinguishable from a
// working one until two processes disagree about which file they share, and the
// only way to catch that is to see all the answers together.
//
// What is NOT here is the decision to lock at all (commandLockPolicy, in
// statelock.go), how a lock is taken (internal/lockfile), or where a gup.json
// comes from (configpath.go).

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/lockfile"
	"github.com/spf13/cobra"
)

// binDirPerm is the permission gup uses when it creates a directory it installs
// binaries into: a migrate AFTER_PATH, or a $GOBIN that does not exist yet. The
// lock and the commands share it so taking the lock first cannot change the
// permissions a user ends up with.
const binDirPerm = 0o755

// binDirLockTargets locks the $GOBIN whose contents the command changes. It is
// the whole policy for `remove`, and part of it for `update` and `import`.
func binDirLockTargets(_ *cobra.Command, _ []string) ([]string, error) {
	gobin, err := goutil.GoBin()
	if err != nil {
		return nil, err
	}
	return dirLockTarget(gobin)
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
// Three outcomes come out of here, and keeping them apart is the whole job.
//
// A lock path is the ordinary one. No lock and no error means the path CANNOT be
// a directory - a regular file sits there, or one of its parents is a file - and
// the command about to run says so far better than a lock error would ("$GOBIN
// is not a directory", "AFTER_PATH is not a directory"). That case is safe to
// leave unlocked because it writes nothing: the command is going to fail on the
// same thing a moment later.
//
// An error is everything else: a parent that refuses the mkdir, a read-only
// filesystem, a full disk, a name the filesystem will not take. Those used to
// return no lock as well, which read as "nothing to guard" and was not - the
// command ran to completion with no lock at all, because a $GOBIN whose creation
// was refused for one gup can be created a moment later by another, or exist
// already for a user whose permissions differ from the directory gup could not
// make. Being unable to work out what to lock is not the same as having nothing
// to lock, and the second is the only one that may proceed.
func dirLockTarget(dir string) ([]string, error) {
	if !fileutil.IsDir(dir) {
		if _, err := os.Lstat(dir); err == nil {
			// The path exists but is not a directory: the command says so better.
			return nil, nil
		}
		if err := os.MkdirAll(dir, binDirPerm); err != nil {
			if hasNonDirectoryAncestor(dir) {
				// A file sits somewhere above the target, so this path can never be a
				// directory. That is the command's diagnosis to give, not the lock's.
				return nil, nil
			}
			return nil, fmt.Errorf("can not prepare the lock for %s: %w", dir, err)
		}
	}
	return []string{lockfile.PathForDir(dir)}, nil
}

// hasNonDirectoryAncestor reports whether some existing component of dir is not
// a directory, which is the one mkdir failure that means "this path can never be
// a directory" rather than "gup could not create it".
//
// It walks the path rather than reading an errno because the errno is not the
// same everywhere: ENOTDIR on Unix, and on Windows a create through a file
// component surfaces as ERROR_PATH_NOT_FOUND or ERROR_DIRECTORY depending on
// where the file sits. Asking the filesystem what is actually there answers the
// question the same way on every platform.
func hasNonDirectoryAncestor(dir string) bool {
	for path := filepath.Clean(dir); ; {
		info, err := os.Lstat(path)
		if err == nil {
			return !info.IsDir()
		}
		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
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
	lockPath, err := lockfile.PathForFile(resolved.writeTarget)
	if err != nil {
		return nil, err
	}
	return []string{lockPath}, nil
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
	binLock, err := dirLockTarget(gobin)
	if err != nil {
		return nil, err
	}
	confLock, err := configFileLockTargets(cmd, nil)
	if err != nil {
		return nil, err
	}
	return append(binLock, confLock...), nil
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
	binLock, err := installedBinDirLockTarget()
	if err != nil {
		return nil, err
	}
	confLock, err := lockfile.PathForFile(resolved.writeTarget)
	if err != nil {
		return nil, err
	}
	return append(binLock, confLock), nil
}

// pinLockTargets locks the gup.json `gup pin` rewrites and the $GOBIN it
// resolves the pin target against, for the reason export locks it: pinning a
// binary a concurrent `gup remove` is deleting writes a pin for a tool that is
// not installed. `gup unpin` needs no such lock - it names an entry in gup.json
// and never looks at $GOBIN.
func pinLockTargets(cmd *cobra.Command, args []string) ([]string, error) {
	binLock, err := installedBinDirLockTarget()
	if err != nil {
		return nil, err
	}
	confLock, err := configFileLockTargets(cmd, args)
	if err != nil {
		return nil, err
	}
	return append(binLock, confLock...), nil
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
func installedBinDirLockTarget() ([]string, error) {
	gobin, err := goutil.GoBin()
	if err != nil {
		// Not knowing where $GOBIN is means not knowing what to lock. The command
		// would fail on the same question a moment later, so reporting it here
		// costs nothing and keeps the alternative - running unlocked because the
		// resource could not be named - off the table.
		return nil, err
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
	after, err := dirLockTarget(args[1])
	if err != nil {
		// AFTER_PATH could not be resolved to something lockable at all, which is
		// not the same as its being unusable: another gup may create or already
		// have the directory this one could not. Nothing runs without a lock.
		return nil, err
	}
	if after == nil {
		// AFTER_PATH cannot be a directory: a regular file sits there, or a file
		// sits somewhere above it. The command reports that better than any lock
		// error would, and it writes nothing, so neither directory needs guarding.
		return nil, nil
	}
	// BEFORE_PATH exists - the check above made sure - so this locks it without
	// creating anything.
	before, err := dirLockTarget(args[0])
	if err != nil {
		return nil, err
	}
	return append(before, after...), nil
}
