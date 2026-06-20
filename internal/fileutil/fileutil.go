// Package fileutil provides simple file system helper functions.
package fileutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// FileModeCreatingDir is the permission used for creating directories.
	FileModeCreatingDir fs.FileMode = 0750
	// FileModeCreatingFile is the permission used for creating files.
	FileModeCreatingFile fs.FileMode = 0600
)

// IsFile reports whether the path exists and is a file.
func IsFile(path string) bool {
	stat, err := os.Stat(path)
	return (err == nil) && (!stat.IsDir())
}

// IsDir reports whether the path exists and is a directory.
func IsDir(path string) bool {
	stat, err := os.Stat(path)
	return (err == nil) && (stat.IsDir())
}

// IsHiddenFile reports whether the path is a hidden file (name starts with ".").
func IsHiddenFile(filePath string) bool {
	_, name := filepath.Split(filePath)
	return IsFile(filePath) && strings.HasPrefix(name, ".")
}
