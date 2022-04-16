package completion

import (
	"os"
	"path/filepath"

	"github.com/nao1215/gup/internal/cmdinfo"
)

// BashCompletionFilePath return bash-completion file path.
func BashCompletionFilePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "bash_completion.d", cmdinfo.Name())
}

// FishCompletionFilePath return fish-completion file path.
func FishCompletionFilePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "fish", "completions", cmdinfo.Name()+".fish")
}

// ZshCompletionFilePath return zsh-completion file path.
func ZshCompletionFilePath() string {
	return filepath.Join(os.Getenv("HOME"), ".zsh", "completion", "_"+cmdinfo.Name())
}

// ZshrcPath return .zshrc path.
func ZshrcPath() string {
	return filepath.Join(os.Getenv("HOME"), ".zshrc")
}
