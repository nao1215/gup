package file

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// FileModeCreatingDir is used for creating directory
	FileModeCreatingDir fs.FileMode = 0750
	// FileModeCreatingFile is used for creating directory
	FileModeCreatingFile fs.FileMode = 0600
)

// ReadFileToList convert file content to string list.
func ReadFileToList(path string) ([]string, error) {
	var strList []string
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			// TODO: If use go 1.20, rewrite like this.
			// err = errors.Join(err, closeErr)
			err = closeErr // overwrite error
		}
	}()

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF && len(line) == 0 {
			break
		}
		strList = append(strList, line)
	}
	return strList, nil
}

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
