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
)

// FilePath return configuration-file path.
func FilePath() string {
	return filepath.Join(DirPath(), "gup.conf")
}

// DirPath return directory path that store configuration-file.
func DirPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// If $HOME is empty, .config directory can be created in the
		// current directory. The .config directory path is displayed
		// when reporting the completion of the export subcommand.
		// So, user notices that the output destination is strange.
		return filepath.Join(os.Getenv("HOME"), ".config", cmdinfo.Name())
	}
	return filepath.Join(home, ".config", cmdinfo.Name())
}

// ReadConfFile return contents of configuration-file (package information)
func ReadConfFile() ([]goutil.Package, error) {
	contents, err := file.ReadFileToList(FilePath())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "can't read gup.conf", err)
	}

	pkgs := []goutil.Package{}
	for _, v := range contents {
		pkg := goutil.Package{}
		ver := goutil.Version{Current: "<from gup.conf>", Latest: ""}

		v = deleteComment(v)
		if isBlank(v) {
			continue
		}
		equalIdx := strings.Index(v, "=")
		pkg.Name = strings.TrimSpace(v[:equalIdx-1])
		pkg.ImportPath = strings.TrimSpace(v[equalIdx+1:])
		pkg.Version = &ver
		pkgs = append(pkgs, pkg)
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
		// lost version information
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
