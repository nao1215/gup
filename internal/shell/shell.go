package shell

import (
	"os"
	"path/filepath"
)

// AllShellHistoryFilePath return *_history file path.
func AllShellHistoryFilePath() []string {
	paths := []string{}
	paths = append(paths, BashHistoryFilePath())
	paths = append(paths, ZshHistoryFilePath())
	paths = append(paths, FishHistoryFilePath())
	return paths
}

// BashHistoryFilePath return absolute path of .bash_history.
func BashHistoryFilePath() string {
	histFile := os.Getenv("HISTFILE")
	if histFile != "" {
		return histFile
	}
	return filepath.Join(os.Getenv("HOME"), ".bash_history")
}

// ZshHistoryFilePath return absolute path of .zsh_history.
func ZshHistoryFilePath() string {
	histFile := os.Getenv("HISTFILE")
	if histFile != "" {
		return histFile
	}
	return filepath.Join(os.Getenv("HOME"), ".zsh_history")
}

// FishHistoryFilePath return absolute path of fish_history.
func FishHistoryFilePath() string {
	home := os.Getenv("XDG_DATA_HOME")
	if home != "" {
		return filepath.Join(home, "fish/fish_history")
	}
	return filepath.Join(os.Getenv("HOME"), ".local/share/fish/fish_history")
}
