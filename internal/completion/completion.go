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

// DeployShellCompletionFileIfNeeded creates the shell completion files.
// If a file with the same contents already exists, it is not recreated.
//
// It returns a non-nil error when HOME is unset/empty (so completion files are
// never written into relative paths under the current directory) or when any
// required completion file cannot be written. Errors from individual shells are
// aggregated so a single run reports every failure (#343).
func DeployShellCompletionFileIfNeeded(cmd *cobra.Command) error {
	if IsWindows() {
		return nil
	}
	if strings.TrimSpace(os.Getenv("HOME")) == "" {
		return errors.New("HOME environment variable is not set; cannot determine where to install shell completion files")
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
// it. The parent directory is created as needed.
func (c completionFile) sync(cmd *cobra.Command) (err error) {
	want, genErr := c.content(cmd)
	if genErr != nil {
		return genErr
	}

	path := c.path()
	if got, readErr := os.ReadFile(filepath.Clean(path)); readErr == nil && bytes.Equal(got, want) {
		return nil
	}

	if mkErr := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); mkErr != nil {
		return fmt.Errorf("can not create %s-completion file: %w", c.shell, mkErr)
	}
	fp, openErr := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileutil.FileModeCreatingFile)
	if openErr != nil {
		return fmt.Errorf("can not open %s-completion file: %w", c.shell, openErr)
	}
	defer func() {
		if closeErr := fp.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("can not close %s-completion file: %w", c.shell, closeErr))
		}
	}()

	if _, writeErr := fp.Write(want); writeErr != nil {
		return fmt.Errorf("can not write %s-completion file: %w", c.shell, writeErr)
	}
	return err
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

func appendFpathAtZshrcIfNeeded() (err error) {
	zshrcPath := zshrcPath()

	// New .zshrc: write the snippet into a freshly created file.
	if !fileutil.IsFile(zshrcPath) {
		return writeZshrcString(zshrcPath, zshFpathSnippet(), os.O_RDWR|os.O_CREATE)
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
		return writeZshrcString(zshrcPath, updated, os.O_RDWR|os.O_TRUNC)
	}

	// No gup block yet: append a fresh one without disturbing existing content.
	return writeZshrcString(zshrcPath, zshFpathSnippet(), os.O_RDWR|os.O_APPEND)
}

// writeZshrcString writes data to .zshrc using the given open flags, joining any
// close error so a failed flush is surfaced.
func writeZshrcString(path, data string, flag int) (err error) {
	fp, openErr := os.OpenFile(filepath.Clean(path), flag, fileutil.FileModeCreatingFile)
	if openErr != nil {
		return fmt.Errorf("can not open .zshrc: %w", openErr)
	}
	defer func() {
		if closeErr := fp.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("can not close .zshrc: %w", closeErr))
		}
	}()

	if _, writeErr := fp.WriteString(data); writeErr != nil {
		return fmt.Errorf("can not write zsh $fpath in .zshrc: %w", writeErr)
	}
	return err
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
