package completion

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nao1215/gorky/file"
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// DeployShellCompletionFileIfNeeded creates the shell completion file.
// If the file with the same contents already exists, it is not created.
func DeployShellCompletionFileIfNeeded(cmd *cobra.Command) {
	if !IsWindows() {
		makeBashCompletionFileIfNeeded(cmd)
		makeFishCompletionFileIfNeeded(cmd)
		makeZshCompletionFileIfNeeded(cmd)
	}
}

// IsWindows check whether runtime is windosw or not.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func makeBashCompletionFileIfNeeded(cmd *cobra.Command) {
	if existSameBashCompletionFile(cmd) {
		return
	}

	path := bashCompletionFilePath()
	bashCompletion := new(bytes.Buffer)
	if err := cmd.GenBashCompletionV2(bashCompletion, false); err != nil {
		print.Err(fmt.Errorf("can not generate bash completion content: %w", err))
		return
	}

	if !file.IsDir(path) {
		if err := os.MkdirAll(filepath.Dir(path), file.FileModeCreatingDir); err != nil {
			print.Err(fmt.Errorf("can not create bash-completion file: %w", err))
			return
		}
	}
	fp, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE, file.FileModeCreatingFile)
	if err != nil {
		print.Err(fmt.Errorf("can not open .bash_completion: %w", err))
		return
	}

	if _, err := fp.WriteString(bashCompletion.String()); err != nil {
		print.Err(fmt.Errorf("can not write .bash_completion %w", err))
		return
	}

	if err := fp.Close(); err != nil {
		print.Err(fmt.Errorf("can not close .bash_completion %w", err))
	}
}

func makeFishCompletionFileIfNeeded(cmd *cobra.Command) {
	if isSameFishCompletionFile(cmd) {
		return
	}

	path := fishCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), file.FileModeCreatingDir); err != nil {
		print.Err(fmt.Errorf("can not create fish-completion file: %w", err))
		return
	}

	if err := cmd.GenFishCompletionFile(path, false); err != nil {
		print.Err(fmt.Errorf("can not create fish-completion file: %w", err))
		return
	}
}

func makeZshCompletionFileIfNeeded(cmd *cobra.Command) {
	if isSameZshCompletionFile(cmd) {
		return
	}

	path := zshCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), file.FileModeCreatingDir); err != nil {
		print.Err(fmt.Errorf("can not create zsh-completion file: %w", err))
		return
	}

	if err := cmd.GenZshCompletionFile(path); err != nil {
		print.Err(fmt.Errorf("can not create zsh-completion file: %w", err))
		return
	}
	appendFpathAtZshrcIfNeeded()
}

func appendFpathAtZshrcIfNeeded() {
	const zshFpath = `
# setting for gup command (auto generate)
fpath=(~/.zsh/completion $fpath)
autoload -Uz compinit && compinit -i
`
	zshrcPath := zshrcPath()
	if !file.IsFile(zshrcPath) {
		fp, err := os.OpenFile(filepath.Clean(zshrcPath), os.O_RDWR|os.O_CREATE, file.FileModeCreatingFile)
		if err != nil {
			print.Err(fmt.Errorf("can not open .zshrc: %w", err).Error())
			return
		}

		if _, err := fp.WriteString(zshFpath); err != nil {
			print.Err(fmt.Errorf("can not write zsh $fpath in .zshrc: %w", err).Error())
			return
		}

		if err := fp.Close(); err != nil {
			print.Err(fmt.Errorf("can not close .zshrc: %w", err).Error())
			return
		}
		return
	}

	zshrc, err := os.ReadFile(filepath.Clean(zshrcPath))
	if err != nil {
		print.Err(fmt.Errorf("can not read .zshrc: %w", err).Error())
		return
	}

	if strings.Contains(string(zshrc), zshFpath) {
		return
	}

	fp, err := os.OpenFile(filepath.Clean(zshrcPath), os.O_RDWR|os.O_APPEND, file.FileModeCreatingFile)
	if err != nil {
		print.Err(fmt.Errorf("can not open .zshrc: %w", err).Error())
		return
	}

	if _, err := fp.WriteString(zshFpath); err != nil {
		print.Err(fmt.Errorf("can not write zsh $fpath in .zshrc: %w", err).Error())
		return
	}

	if err := fp.Close(); err != nil {
		print.Err(fmt.Errorf("can not close .zshrc: %w", err).Error())
		return
	}
}

func existSameBashCompletionFile(cmd *cobra.Command) bool {
	if !file.IsFile(bashCompletionFilePath()) {
		return false
	}
	return hasSameBashCompletionContent(cmd)
}

func hasSameBashCompletionContent(cmd *cobra.Command) bool {
	bashCompletionFileInLocal, err := os.ReadFile(bashCompletionFilePath())
	if err != nil {
		print.Err(fmt.Errorf("can not read .bash_completion: %w", err).Error())
		return false
	}

	currentBashCompletion := new(bytes.Buffer)
	if err := cmd.GenBashCompletionV2(currentBashCompletion, false); err != nil {
		return false
	}
	if !strings.Contains(string(bashCompletionFileInLocal), currentBashCompletion.String()) {
		return false
	}
	return true
}

func isSameFishCompletionFile(cmd *cobra.Command) bool {
	path := fishCompletionFilePath()
	if !file.IsFile(path) {
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
	if !file.IsFile(path) {
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
