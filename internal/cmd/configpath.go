// Where a command's gup.json is, resolved once per run.
//
// Resolution consults the filesystem: with no --file, whether ./gup.json exists
// at that moment decides both which config is read and where the write lands.
// Asking that question twice is a race no lock can cover, because the lock is
// taken for the first answer and the write goes to the second. So the answer is
// settled before the lock is taken, remembered on the command's context, and
// read back by the command body - which is what makes this a concern of its own
// rather than a detail of either the lock policy or the commands.

package cmd

import (
	"context"
	"fmt"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/configstate"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/spf13/cobra"
)

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
	// readTarget and writeTarget are those two with symlinks resolved: the files
	// the command actually reads and actually writes, and therefore the only
	// things worth locking or handing to an opener.
	//
	// They are carried rather than recomputed for the same reason the pair above
	// is. Resolving a link twice is asking the filesystem a question twice, and a
	// dotfile manager repointing the link between the two answers is enough to
	// send the write to a file the command holds no lock on - or, just as bad, to
	// merge the contents of a file it never locked into the one it did.
	//
	// The two resolve to the same file in every resolution these rules can
	// produce: an explicit --file is both, and auto-detection picks one path for
	// both. They are kept apart because the rules say so, not because a case is
	// known where they diverge.
	readTarget  string
	writeTarget string
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
	readTarget, err := fileutil.ResolveSymlinkTarget(read)
	if err != nil {
		return nil, fmt.Errorf("can not resolve config path %s: %w", read, err)
	}
	writeTarget, err := fileutil.ResolveSymlinkTarget(write)
	if err != nil {
		return nil, fmt.Errorf("can not resolve config path %s: %w", write, err)
	}
	resolved := &resolvedConfig{
		rule: rule, confFile: confFile,
		read: read, write: write,
		readTarget: readTarget, writeTarget: writeTarget,
	}
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
