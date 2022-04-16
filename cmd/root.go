package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/completion"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "gup",
	Short: `gup command update binaries installed by 'go install'.
If you update all binaries, just run '$ gup update'`,
}

// Execute run gup process.
func Execute() {
	assets.DeployIconIfNeeded()
	deployShellCompletionFileIfNeeded(rootCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	if err := rootCmd.Execute(); err != nil {
		print.Err(err)
	}
}

// deleteShellCompletionFileIfNeeded creates the shell completion file.
// If the file with the same contents already exists, it is not created.
func deployShellCompletionFileIfNeeded(cmd *cobra.Command) {
	makeBashCompletionFileIfNeeded(cmd)
	makeFishCompletionFileIfNeeded(cmd)
	makeZshCompletionFileIfNeeded(cmd)
}

func makeBashCompletionFileIfNeeded(cmd *cobra.Command) {
	if existSameBashCompletionFile(cmd) {
		return
	}

	path := completion.BashCompletionFilePath()
	bashCompletion := new(bytes.Buffer)
	if err := cmd.GenBashCompletion(bashCompletion); err != nil {
		print.Err(fmt.Errorf("can not generate bash completion content: %w", err))
		return
	}

	if !file.IsFile(path) {
		fp, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0664)
		if err != nil {
			print.Err(fmt.Errorf("can not create .bash_completion: %w", err))
			return
		}
		defer fp.Close()

		if _, err := fp.WriteString(bashCompletion.String()); err != nil {
			print.Err(fmt.Errorf("can not write .bash_completion %w", err))
		}
		print.Info("create bash-completion file: " + path)
		return
	}

	fp, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0664)
	if err != nil {
		print.Err(fmt.Errorf("can not append .bash_completion for gup: %w", err))
		return
	}
	defer fp.Close()

	if _, err := fp.WriteString(bashCompletion.String()); err != nil {
		print.Err(fmt.Errorf("can not append .bash_completion for gup: %w", err))
		return
	}

	print.Info("append bash-completion for gup: " + path)
}

func makeFishCompletionFileIfNeeded(cmd *cobra.Command) {
	if isSameFishCompletionFile(cmd) {
		return
	}

	path := completion.FishCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0775); err != nil {
		print.Err(fmt.Errorf("can not create fish-completion file: %w", err))
		return
	}

	if err := cmd.GenFishCompletionFile(path, false); err != nil {
		print.Err(fmt.Errorf("can not create fish-completion file: %w", err))
		return
	}
	print.Info("create fish-completion file: " + path)
}

func makeZshCompletionFileIfNeeded(cmd *cobra.Command) {
	if isSameZshCompletionFile(cmd) {
		return
	}

	path := completion.ZshCompletionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0775); err != nil {
		print.Err(fmt.Errorf("can not create zsh-completion file: %w", err))
		return
	}

	if err := cmd.GenZshCompletionFile(path); err != nil {
		print.Err(fmt.Errorf("can not create zsh-completion file: %w", err))
		return
	}
	print.Info("create zsh-completion file: " + path)

	const zshFpath = `
# setting for gup command (auto generate)
fpath=(~/.zsh/completion $fpath)
autoload -Uz compinit && compinit -i
`
	zshrcPath := completion.ZshrcPath()
	if !file.IsFile(zshrcPath) {
		fp, err := os.OpenFile(zshrcPath, os.O_RDWR|os.O_CREATE, 0664)
		if err != nil {
			print.Err(fmt.Errorf("can not add zsh $fpath in .zshrc: %w", err))
			return
		}
		defer fp.Close()

		if _, err := fp.WriteString(zshFpath); err != nil {
			print.Err(fmt.Errorf("can not add zsh $fpath in .zshrc: %w", err))
		}
		return
	}

	zshrc, err := ioutil.ReadFile(zshrcPath)
	if err != nil {
		print.Err(fmt.Errorf("can not read .zshrc: %w", err))
		return
	}

	if strings.Contains(string(zshrc), zshFpath) {
		return
	}

	fp, err := os.OpenFile(zshrcPath, os.O_RDWR|os.O_APPEND, 0664)
	if err != nil {
		print.Err(fmt.Errorf("can not add zsh $fpath in .zshrc: %w", err))
		return
	}
	defer fp.Close()

	if _, err := fp.WriteString(zshFpath); err != nil {
		print.Err(fmt.Errorf("can not add zsh $fpath in .zshrc: %w", err))
		return
	}
}

func existSameBashCompletionFile(cmd *cobra.Command) bool {
	if !file.IsFile(completion.BashCompletionFilePath()) {
		return false
	}
	return hasSameBashCompletionContent(cmd)
}

func hasSameBashCompletionContent(cmd *cobra.Command) bool {
	bashCompletionFileInLocal, err := ioutil.ReadFile(completion.BashCompletionFilePath())
	if err != nil {
		print.Err(fmt.Errorf("can not read .bash_completion: %w", err))
		return false
	}

	currentBashCompletion := new(bytes.Buffer)
	if err := cmd.GenBashCompletion(currentBashCompletion); err != nil {
		return false
	}
	if !strings.Contains(string(bashCompletionFileInLocal), currentBashCompletion.String()) {
		return false
	}
	return true
}

func isSameFishCompletionFile(cmd *cobra.Command) bool {
	path := completion.FishCompletionFilePath()
	if !file.IsFile(path) {
		return false
	}

	currentFishCompletion := new(bytes.Buffer)
	if err := cmd.GenFishCompletion(currentFishCompletion, false); err != nil {
		return false
	}

	fishCompletionInLocal, err := ioutil.ReadFile(path)
	if err != nil {
		return false
	}

	if bytes.Compare(currentFishCompletion.Bytes(), fishCompletionInLocal) != 0 {
		return false
	}
	return true
}

func isSameZshCompletionFile(cmd *cobra.Command) bool {
	path := completion.ZshCompletionFilePath()
	if !file.IsFile(path) {
		return false
	}

	currentZshCompletion := new(bytes.Buffer)
	if err := cmd.GenZshCompletion(currentZshCompletion); err != nil {
		return false
	}

	zshCompletionInLocal, err := ioutil.ReadFile(path)
	if err != nil {
		return false
	}

	if bytes.Compare(currentZshCompletion.Bytes(), zshCompletionInLocal) != 0 {
		return false
	}
	return true
}
