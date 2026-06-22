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

func makeBashCompletionFileIfNeeded(cmd *cobra.Command) error {
	if existSameBashCompletionFile(cmd) {
		return nil
	}

	path := bashCompletionFilePath()
	bashCompletion := new(bytes.Buffer)
	if err := cmd.GenBashCompletionV2(bashCompletion, false); err != nil {
		return fmt.Errorf("can not generate bash completion content: %w", err)
	}

	if !fileutil.IsDir(filepath.Dir(path)) {
		if err := os.MkdirAll(filepath.Dir(path), fileutil.FileModeCreatingDir); err != nil {
			return fmt.Errorf("can not create bash-completion file: %w", err)
		}
	}
	fp, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileutil.FileModeCreatingFile)
	if err != nil {
		return fmt.Errorf("can not open .bash_completion: %w", err)
	}
	defer func() {
		if closeErr := fp.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("can not close .bash_completion: %w", closeErr))
		}
	}()

	if _, err := fp.WriteString(bashCompletion.String()); err != nil {
		return fmt.Errorf("can not write .bash_completion: %w", err)
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

func appendFpathAtZshrcIfNeeded() (err error) {
	const zshFpath = `
# setting for gup command (auto generate)
fpath=(~/.zsh/completion $fpath)
autoload -Uz compinit && compinit -i
`
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

	if strings.Contains(string(zshrc), zshFpath) {
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

// bashCompletionFilePath return bash-completion file path.
func bashCompletionFilePath() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "bash-completion", "completions", cmdinfo.Name)
}

// fishCompletionFilePath return fish-completion file path.
func fishCompletionFilePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "fish", "completions", cmdinfo.Name+".fish")
}

// zshCompletionFilePath return zsh-completion file path.
func zshCompletionFilePath() string {
	return filepath.Join(os.Getenv("HOME"), ".zsh", "completion", "_"+cmdinfo.Name)
}

// zshrcPath return .zshrc path.
func zshrcPath() string {
	return filepath.Join(os.Getenv("HOME"), ".zshrc")
}
