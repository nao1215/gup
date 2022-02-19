package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

func init() {
	if err := os.MkdirAll(DirPath(), 0775); err != nil {
		print.Err(fmt.Errorf("%s: %w", "can not make config directory", err))
	}
}

// FilePath return configuration-file path.
func FilePath() string {
	return filepath.Join(DirPath(), "gup.conf")
}

// DirPath return directory path that store configuration-file.
func DirPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", cmdinfo.Name())
}

// ReadConfFile return contents of configuration-file (package information)
func ReadConfFile() ([]goutil.Package, error) {
	contents, err := file.ReadFileToList(FilePath())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't read gui.conf", err)
	}

	pkgs := []goutil.Package{}
	for _, v := range contents {
		pkg := goutil.Package{}

		v = deleteComment(v)
		if isBlank(v) {
			continue
		}
		equalIdx := strings.Index(v, "=")
		pkg.Name = strings.TrimSpace(v[:equalIdx])
		pkg.ImportPath = strings.TrimSpace(v[equalIdx:])
	}

	return pkgs, nil
}

// WriteConfFile write package information at configuration-file.
func WriteConfFile(pkgs []goutil.Package) error {
	file, err := os.Create(FilePath())
	if err != nil {
		return fmt.Errorf("%s %s: %w", "can't update", FilePath(), err)
	}
	defer file.Close()

	text := ""
	for _, v := range pkgs {
		text = text + fmt.Sprintf("%s = %s\n", v.Name, v.ImportPath)
	}

	_, err = file.Write(([]byte)(text))
	if err != nil {
		return fmt.Errorf("%s %s: %w", "can't update", FilePath(), err)
	}
	return nil
}

func isBlank(line string) bool {
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, "\n", "")
	return len(line) == 0
}

func deleteComment(line string) string {
	r := regexp.MustCompile(`#./*`)
	return r.ReplaceAllString(line, "")
}
