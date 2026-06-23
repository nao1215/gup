package completion

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		return fmt.Errorf("HOME environment variable is not set; cannot determine where to install shell completion files")
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

func makeBashCompletionFileIfNeeded(cmd *cobra.Command) (err error) {
	if existSameBashCompletionFile(cmd) {
		return nil
	}

	path := bashCompletionFilePath()
	bashCompletion := new(bytes.Buffer)
	if genErr := cmd.GenBashCompletionV2(bashCompletion, false); genErr != nil {
		return fmt.Errorf("can not generate bash completion content: %w", genErr)
	}

	if !fileutil.IsDir(filepath.Dir(path)) {
		if mkErr := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); mkErr != nil {
			return fmt.Errorf("can not create bash-completion file: %w", mkErr)
		}
	}
	fp, openErr := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileutil.FileModeCreatingFile)
	if openErr != nil {
		return fmt.Errorf("can not open .bash_completion: %w", openErr)
	}
	defer func() {
		if closeErr := fp.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("can not close .bash_completion: %w", closeErr))
		}
	}()

	if _, writeErr := fp.WriteString(bashCompletion.String()); writeErr != nil {
		return fmt.Errorf("can not write .bash_completion: %w", writeErr)
	}
	return err
}

func makeFishCompletionFileIfNeeded(cmd *cobra.Command) error {
	if isSameFishCompletionFile(cmd) {
		return nil
	}

	path := fishCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); err != nil {
		return fmt.Errorf("can not create fish-completion file: %w", err)
	}

	if err := cmd.GenFishCompletionFile(path, false); err != nil {
		return fmt.Errorf("can not create fish-completion file: %w", err)
	}
	return nil
}

func makeZshCompletionFileIfNeeded(cmd *cobra.Command) error {
	if isSameZshCompletionFile(cmd) {
		return nil
	}

	path := zshCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); err != nil {
		return fmt.Errorf("can not create zsh-completion file: %w", err)
	}

	if err := cmd.GenZshCompletionFile(path); err != nil {
		return fmt.Errorf("can not create zsh-completion file: %w", err)
	}
	return appendFpathAtZshrcIfNeeded()
}

// zshFpathMarker is the comment line that uniquely identifies gup's fpath block
// in .zshrc. Idempotency keys on this marker rather than the full snippet text so
// a re-run never appends a second block even if the referenced completion
// directory differs between runs (e.g. ZDOTDIR toggled while .zshrc resolves to
// the same file).
const zshFpathMarker = "# setting for gup command (auto generate)"

// zshFpathSnippet returns the fpath block written to .zshrc. The completion
// directory it references tracks zshCompletionFilePath: under ZDOTDIR it expands
// via ${ZDOTDIR} (zsh expands ~ to $HOME, not $ZDOTDIR), otherwise it uses the
// portable ~/.zsh/completion form (#366).
func zshFpathSnippet() string {
	completionDir := "~/.zsh/completion"
	if strings.TrimSpace(os.Getenv("ZDOTDIR")) != "" {
		completionDir = "${ZDOTDIR}/.zsh/completion"
	}
	return fmt.Sprintf(`
%s
fpath=(%s $fpath)
autoload -Uz compinit && compinit -i
`, zshFpathMarker, completionDir)
}

func appendFpathAtZshrcIfNeeded() (err error) {
	zshFpath := zshFpathSnippet()
	zshrcPath := zshrcPath()
	if !fileutil.IsFile(zshrcPath) {
		fp, openErr := os.OpenFile(filepath.Clean(zshrcPath), os.O_RDWR|os.O_CREATE, fileutil.FileModeCreatingFile)
		if openErr != nil {
			return fmt.Errorf("can not open .zshrc: %w", openErr)
		}
		defer func() {
			if closeErr := fp.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("can not close .zshrc: %w", closeErr))
			}
		}()

		if _, writeErr := fp.WriteString(zshFpath); writeErr != nil {
			return fmt.Errorf("can not write zsh $fpath in .zshrc: %w", writeErr)
		}
		return err
	}

	zshrc, readErr := os.ReadFile(filepath.Clean(zshrcPath))
	if readErr != nil {
		return fmt.Errorf("can not read .zshrc: %w", readErr)
	}

	if strings.Contains(string(zshrc), zshFpathMarker) {
		return nil
	}

	fp, openErr := os.OpenFile(filepath.Clean(zshrcPath), os.O_RDWR|os.O_APPEND, fileutil.FileModeCreatingFile)
	if openErr != nil {
		return fmt.Errorf("can not open .zshrc: %w", openErr)
	}
	defer func() {
		if closeErr := fp.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("can not close .zshrc: %w", closeErr))
		}
	}()

	if _, writeErr := fp.WriteString(zshFpath); writeErr != nil {
		return fmt.Errorf("can not write zsh $fpath in .zshrc: %w", writeErr)
	}
	return err
}

func existSameBashCompletionFile(cmd *cobra.Command) bool {
	if !fileutil.IsFile(bashCompletionFilePath()) {
		return false
	}
	return hasSameBashCompletionContent(cmd)
}

func hasSameBashCompletionContent(cmd *cobra.Command) bool {
	bashCompletionFileInLocal, err := os.ReadFile(bashCompletionFilePath())
	if err != nil {
		// The caller only reaches here when the file exists; treat a read error
		// as "not the same" so the completion file is regenerated.
		return false
	}

	currentBashCompletion := new(bytes.Buffer)
	if err := cmd.GenBashCompletionV2(currentBashCompletion, false); err != nil {
		return false
	}
	if !bytes.Equal(currentBashCompletion.Bytes(), bashCompletionFileInLocal) {
		return false
	}
	return true
}

func isSameFishCompletionFile(cmd *cobra.Command) bool {
	path := fishCompletionFilePath()
	if !fileutil.IsFile(path) {
		return false
	}

	currentFishCompletion := new(bytes.Buffer)
	if err := cmd.GenFishCompletion(currentFishCompletion, false); err != nil {
		return false
	}

	fishCompletionInLocal, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return false
	}

	if !bytes.Equal(currentFishCompletion.Bytes(), fishCompletionInLocal) {
		return false
	}
	return true
}

func isSameZshCompletionFile(cmd *cobra.Command) bool {
	path := zshCompletionFilePath()
	if !fileutil.IsFile(path) {
		return false
	}

	currentZshCompletion := new(bytes.Buffer)
	if err := cmd.GenZshCompletion(currentZshCompletion); err != nil {
		return false
	}

	zshCompletionInLocal, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return false
	}

	if !bytes.Equal(currentZshCompletion.Bytes(), zshCompletionInLocal) {
		return false
	}
	return true
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
