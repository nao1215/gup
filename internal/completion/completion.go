package completion

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/spf13/cobra"
)

// renameFunc is os.Rename, indirected so tests can simulate a rename failure at
// the atomic-commit step and assert the original file is left intact.
var renameFunc = os.Rename //nolint:gochecknoglobals // swapped in tests

// DeployShellCompletionFileIfNeeded creates the shell completion files.
// If a file with the same contents already exists, it is not recreated.
//
// It returns a non-nil error when HOME (or an XDG_DATA_HOME/XDG_CONFIG_HOME/
// ZDOTDIR override) is unset or relative (so completion files are never written
// into relative paths under the current directory) or when any required
// completion file cannot be written. Errors from individual shells are
// aggregated so a single run reports every failure (#343).
func DeployShellCompletionFileIfNeeded(cmd *cobra.Command) error {
	if IsWindows() {
		return nil
	}
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return errors.New("HOME environment variable is not set; cannot determine where to install shell completion files")
	}
	// A relative HOME is rejected for the same reason as a relative XDG/ZDOTDIR
	// value: with the XDG variables unset, the install paths fall back to HOME, so
	// a relative HOME would write completion files under the current working
	// directory. gup never implicitly absolutizes it.
	if !filepath.IsAbs(home) {
		return fmt.Errorf("HOME must be an absolute path to install shell completion files, but is a relative path: %q", home)
	}
	if err := validateCompletionPathEnv(); err != nil {
		return err
	}

	return errors.Join(
		makeBashCompletionFileIfNeeded(cmd),
		makeFishCompletionFileIfNeeded(cmd),
		makeZshCompletionFileIfNeeded(cmd),
	)
}

// IsWindows check whether runtime is windosw or not.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// completionFile describes a shell completion file gup manages: which shell it
// is for (used in diagnostics), where it is installed (a func because the path
// depends on environment variables), and how its current contents are generated
// from cmd. The three shells differ only in these three values, so the
// "generate, compare, write if changed" flow lives once on this type rather than
// being copied per shell.
type completionFile struct {
	shell    string
	path     func() string
	generate func(cmd *cobra.Command, w io.Writer) error
}

// bashCompletionSpec, fishCompletionSpec and zshCompletionSpec describe each
// shell's managed completion file. They are constructors rather than package
// variables so the package keeps no mutable global state.
func bashCompletionSpec() completionFile {
	return completionFile{
		shell: "bash",
		path:  bashCompletionFilePath,
		generate: func(cmd *cobra.Command, w io.Writer) error {
			return cmd.GenBashCompletionV2(w, false)
		},
	}
}

func fishCompletionSpec() completionFile {
	return completionFile{
		shell: "fish",
		path:  fishCompletionFilePath,
		generate: func(cmd *cobra.Command, w io.Writer) error {
			return cmd.GenFishCompletion(w, false)
		},
	}
}

func zshCompletionSpec() completionFile {
	return completionFile{
		shell: "zsh",
		path:  zshCompletionFilePath,
		generate: func(cmd *cobra.Command, w io.Writer) error {
			return cmd.GenZshCompletion(w)
		},
	}
}

// content renders the completion file gup would install for this shell.
func (c completionFile) content(cmd *cobra.Command) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := c.generate(cmd, buf); err != nil {
		return nil, fmt.Errorf("can not generate %s completion content: %w", c.shell, err)
	}
	return buf.Bytes(), nil
}

// upToDate reports whether the installed file already holds exactly the contents
// cmd would generate, so a re-install can skip rewriting it. A generation error
// or a missing/unreadable file is treated as "not up to date" so the file is
// regenerated.
func (c completionFile) upToDate(cmd *cobra.Command) bool {
	want, err := c.content(cmd)
	if err != nil {
		return false
	}
	got, err := os.ReadFile(filepath.Clean(c.path()))
	if err != nil {
		return false
	}
	return bytes.Equal(got, want)
}

// sync writes the completion file when it is missing or stale and leaves an
// already up-to-date file untouched, so repeated --install runs do not rewrite
// it. The write is atomic (see atomicWriteFile), so a failure mid-install never
// truncates or corrupts an existing completion file. The parent directory is
// created as needed.
func (c completionFile) sync(cmd *cobra.Command) error {
	want, genErr := c.content(cmd)
	if genErr != nil {
		return genErr
	}

	path := c.path()
	if got, readErr := os.ReadFile(filepath.Clean(path)); readErr == nil && bytes.Equal(got, want) {
		return nil
	}

	return atomicWriteFile(path, want, c.shell+"-completion file")
}

// atomicWriteFile writes data to path atomically: it writes to a temp file in
// the same directory, fsyncs and closes it, then renames it over path. A rename
// on the same filesystem replaces the destination in one step, so a reader never
// sees a half-written file and an interrupted install never truncates or
// corrupts the existing one. On any failure before the rename the temp file is
// removed and path keeps its previous contents. The parent directory is created
// as needed. When path already exists its permissions are preserved; a new file
// uses the restrictive 0600 mode os.CreateTemp applies. This mirrors the atomic
// gup.json write in cmd/config_file.go. (Completion install is POSIX-only - the
// caller returns early on Windows - so the plain os.Rename replace is atomic.)
func atomicWriteFile(path string, data []byte, what string) (err error) {
	path = filepath.Clean(path)
	// Reject an existing directory before staging any temp file, so a mistaken
	// path cannot turn into a confusing rename failure (mirrors the #367 guard in
	// cmd/config_file.go).
	if fileutil.IsDir(path) {
		return fmt.Errorf("%s path %s is a directory, not a file", what, path)
	}
	// Resolve symlinks at the destination so the rename rewrites the link's target
	// rather than replacing the link itself with a regular file. Dotfile managers
	// (stow, chezmoi, yadm) commonly symlink .zshrc and completion files into place;
	// the previous in-place write followed the link, and this preserves that
	// behavior - including for a dangling link whose target does not exist yet, the
	// state right after a dotfile manager links a file before its first write.
	resolvedPath, resolveErr := resolveSymlinkTarget(path)
	if resolveErr != nil {
		return fmt.Errorf("can not resolve %s path %s: %w", what, path, resolveErr)
	}
	path = resolvedPath
	dir := filepath.Dir(path)
	if mkErr := os.MkdirAll(dir, fileutil.FileModeCreatingDir); mkErr != nil {
		return fmt.Errorf("can not create directory for %s: %w", what, mkErr)
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("can not create temp file for %s: %w", what, err)
	}
	tmpPath := file.Name()
	defer func() {
		if file != nil {
			if closeErr := file.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = file.Write(data); err != nil {
		return fmt.Errorf("can not write %s: %w", what, err)
	}
	// Preserve the permissions of the file being replaced; a new file keeps the
	// 0600 mode os.CreateTemp already applied.
	if info, statErr := os.Stat(path); statErr == nil {
		if chmodErr := os.Chmod(tmpPath, info.Mode().Perm()); chmodErr != nil {
			return fmt.Errorf("can not set permissions for %s: %w", what, chmodErr)
		}
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("can not sync %s: %w", what, err)
	}
	if err = file.Close(); err != nil {
		file = nil
		return fmt.Errorf("can not close %s: %w", what, err)
	}
	file = nil

	if err = renameFunc(tmpPath, path); err != nil {
		return fmt.Errorf("can not finalize %s: %w", what, err)
	}
	return nil
}

// resolveSymlinkTarget follows the chain of symlinks at path and returns the
// final target, even when that target does not exist yet (a dangling symlink -
// e.g. a dotfile manager that linked ~/.zshrc before its target file was ever
// written). This lets atomicWriteFile create or replace the link's target rather
// than the link itself, preserving the symlink. filepath.EvalSymlinks cannot be
// used because it requires the whole path to exist and so fails on a dangling
// link. A non-symlink path (including a missing plain file) is returned
// unchanged; intermediate directory symlinks need no handling because the OS
// resolves them transparently during create/rename. Exhausting the hop limit
// means a symlink cycle: that is reported as an error so the caller aborts,
// rather than returning a still-symlink path that the atomic rename would then
// clobber into a regular file.
func resolveSymlinkTarget(path string) (string, error) {
	for range 255 {
		info, err := os.Lstat(path)
		if err != nil {
			// path does not exist yet (e.g. a dangling link's final target): it is
			// the destination to create, so stop here with no error.
			return path, nil //nolint:nilerr // a missing target is the intended write path
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		dest, err := os.Readlink(path)
		if err != nil {
			// The link is unreadable; fall back to treating path as the destination
			// so the caller surfaces a concrete write error rather than this one.
			return path, nil //nolint:nilerr // fall back to writing at path
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		path = filepath.Clean(dest)
	}
	return "", fmt.Errorf("symlink chain too deep (possible cycle) at %s", path)
}

func makeBashCompletionFileIfNeeded(cmd *cobra.Command) error {
	return bashCompletionSpec().sync(cmd)
}

func makeFishCompletionFileIfNeeded(cmd *cobra.Command) error {
	return fishCompletionSpec().sync(cmd)
}

func makeZshCompletionFileIfNeeded(cmd *cobra.Command) error {
	// Regenerate the completion file only when it is missing or stale, but always
	// reconcile the .zshrc fpath block afterwards. Skipping the reconcile when
	// _gup is already up to date would leave a deleted or broken block unrepaired,
	// so a re-run of --install could exit successfully without actually fixing
	// completion (self-healing install).
	if err := zshCompletionSpec().sync(cmd); err != nil {
		return err
	}
	return appendFpathAtZshrcIfNeeded()
}

// zshFpathMarker is the comment line that uniquely identifies gup's fpath block
// in .zshrc. The reconcile logic keys on this marker so a re-run replaces an
// existing block in place rather than appending a second one, even if the
// referenced completion directory differs between runs (e.g. ZDOTDIR toggled
// while .zshrc resolves to the same file).
const zshFpathMarker = "# setting for gup command (auto generate)"

// zshFpathBlockRE matches gup's managed fpath block: the marker line plus the
// fpath and autoload lines it writes. The fpath/autoload lines are optional so a
// hand-broken block (e.g. with those lines deleted) is still recognized and
// repaired during reconcile. The pattern deliberately does NOT consume the
// newline that precedes the marker, so replacing it can never merge the previous
// line into the block's neighbor.
var zshFpathBlockRE = regexp.MustCompile(
	`[ \t]*` + regexp.QuoteMeta(zshFpathMarker) + `[ \t]*\n` +
		`(?:[ \t]*fpath=\([^\n]*\)[ \t]*\n)?` +
		`(?:[ \t]*autoload -Uz compinit[^\n]*\n?)?`)

// zshFpathBlockBody returns gup's managed block (marker + fpath + autoload),
// without a leading blank line. The completion directory it references tracks
// zshCompletionFilePath: under ZDOTDIR it expands via ${ZDOTDIR} (zsh expands ~
// to $HOME, not $ZDOTDIR), otherwise it uses the portable ~/.zsh/completion form
// (#366).
func zshFpathBlockBody() string {
	completionDir := "~/.zsh/completion"
	if strings.TrimSpace(os.Getenv("ZDOTDIR")) != "" {
		completionDir = "${ZDOTDIR}/.zsh/completion"
	}
	return fmt.Sprintf("%s\nfpath=(%s $fpath)\nautoload -Uz compinit && compinit -i\n", zshFpathMarker, completionDir)
}

// zshFpathSnippet is the block as appended to a new or block-less .zshrc, with a
// leading blank line so it is visually separated from any preceding content.
func zshFpathSnippet() string {
	return "\n" + zshFpathBlockBody()
}

// appendFpathAtZshrcIfNeeded reconciles gup's fpath block in .zshrc. Every final
// write goes through atomicWriteFile so the user's .zshrc is never truncated or
// left half-written if the install fails: the full intended content is staged in
// a temp file and renamed into place in one step. The append case is therefore a
// read-modify-write (read current content, concatenate the snippet, atomic write)
// rather than an in-place O_APPEND.
func appendFpathAtZshrcIfNeeded() error {
	zshrcPath := zshrcPath()

	// New .zshrc: the whole file is just the snippet.
	if !fileutil.IsFile(zshrcPath) {
		return atomicWriteFile(zshrcPath, []byte(zshFpathSnippet()), ".zshrc")
	}

	raw, readErr := os.ReadFile(filepath.Clean(zshrcPath))
	if readErr != nil {
		return fmt.Errorf("can not read .zshrc: %w", readErr)
	}
	content := string(raw)

	if zshFpathBlockRE.MatchString(content) {
		// A gup block already exists (current, stale, or hand-broken). Replace it
		// in place so the user's surrounding lines keep their position, and repair
		// an outdated or partial block. ReplaceAllLiteralString avoids treating the
		// literal "$fpath" in the body as a capture-group reference. When the block
		// is already correct the content is unchanged and the file is not rewritten.
		updated := zshFpathBlockRE.ReplaceAllLiteralString(content, zshFpathBlockBody())
		if updated == content {
			return nil
		}
		return atomicWriteFile(zshrcPath, []byte(updated), ".zshrc")
	}

	// No gup block yet: append a fresh one without disturbing existing content.
	return atomicWriteFile(zshrcPath, []byte(content+zshFpathSnippet()), ".zshrc")
}

func isSameBashCompletionFile(cmd *cobra.Command) bool {
	return bashCompletionSpec().upToDate(cmd)
}

func isSameFishCompletionFile(cmd *cobra.Command) bool {
	return fishCompletionSpec().upToDate(cmd)
}

func isSameZshCompletionFile(cmd *cobra.Command) bool {
	return zshCompletionSpec().upToDate(cmd)
}

// validateCompletionPathEnv rejects a relative XDG_DATA_HOME, XDG_CONFIG_HOME or
// ZDOTDIR before any completion file is written. These variables are otherwise
// used verbatim by xdgDataHome/xdgConfigHome/zshDotDir to build the install
// paths, so a relative value would silently write completion files and the
// .zshrc fpath block under the current working directory instead of the user's
// home. gup never implicitly absolutizes them: a relative value is a
// misconfiguration, so fail fast naming the offending variable, consistent with
// the fail-fast HOME check in DeployShellCompletionFileIfNeeded. An unset
// variable is fine - it falls back to a HOME-based absolute path.
func validateCompletionPathEnv() error {
	for _, name := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "ZDOTDIR"} {
		value := strings.TrimSpace(os.Getenv(name))
		if value != "" && !filepath.IsAbs(value) {
			return fmt.Errorf(
				"%s must be an absolute path to install shell completion files, but is a relative path: %q",
				name, value)
		}
	}
	return nil
}

// xdgDataHome returns the base directory for user data files, honoring
// XDG_DATA_HOME when set and falling back to $HOME/.local/share otherwise (#366).
func xdgDataHome() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

// xdgConfigHome returns the base directory for user config files, honoring
// XDG_CONFIG_HOME when set and falling back to $HOME/.config otherwise (#366).
func xdgConfigHome() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

// zshDotDir returns the directory zsh reads its startup files from, honoring
// ZDOTDIR when set and falling back to $HOME otherwise (#366). Both the zsh
// completion files and .zshrc are resolved relative to this directory so the
// install matches the user's active zsh configuration layout.
func zshDotDir() string {
	if dir := strings.TrimSpace(os.Getenv("ZDOTDIR")); dir != "" {
		return dir
	}
	return os.Getenv("HOME")
}

// bashCompletionFilePath return bash-completion file path.
func bashCompletionFilePath() string {
	return filepath.Join(xdgDataHome(), "bash-completion", "completions", cmdinfo.Name)
}

// fishCompletionFilePath return fish-completion file path.
func fishCompletionFilePath() string {
	return filepath.Join(xdgConfigHome(), "fish", "completions", cmdinfo.Name+".fish")
}

// zshCompletionFilePath return zsh-completion file path.
func zshCompletionFilePath() string {
	return filepath.Join(zshDotDir(), ".zsh", "completion", "_"+cmdinfo.Name)
}

// zshrcPath return .zshrc path.
func zshrcPath() string {
	return filepath.Join(zshDotDir(), ".zshrc")
}
