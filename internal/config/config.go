// Package config define gup command setting.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/shogo82148/pointer"
)

// ConfigFileName is gup command configuration file
var ConfigFileName = "gup.conf"

// FilePath return configuration-file path.
func FilePath() string {
	return filepath.Join(DirPath(), ConfigFileName)
}

// DirPath return directory path that store configuration-file.
// Default path is $HOME/.config/gup.
func DirPath() string {
	return filepath.Join(xdg.ConfigHome, cmdinfo.Name)
}

// ReadConfFile return contents of configuration-file (package information)
func ReadConfFile(path string) ([]goutil.Package, error) {
	contents, err := readFileToList(path)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", path, err)
	}

	pkgs := []goutil.Package{}
	for _, v := range contents {
		pkg := goutil.Package{}
		binVer := goutil.Version{Current: "<from gup.conf>", Latest: ""}
		goVer := goutil.Version{Current: "<from gup.conf>", Latest: ""}

		v = deleteComment(v)
		if isBlank(v) {
			continue
		}

		// Check if the package name and package path are included
		if len(strings.Split(v, "=")) != 2 {
			return nil, errors.New(path + " is not gup.conf file")
		}

		equalIdx := strings.Index(v, "=")
		pkg.Name = strings.TrimSpace(v[:equalIdx-1])
		pkg.ImportPath = strings.TrimSpace(v[equalIdx+1:])
		pkg.Version = pointer.Ptr(binVer)
		pkg.GoVersion = pointer.Ptr(goVer)
		pkgs = append(pkgs, pkg)
	}

	return pkgs, nil
}

// WriteConfFile write package information at configuration-file.
func WriteConfFile(file *os.File, pkgs []goutil.Package) error {
	text := ""
	for _, v := range pkgs {
		// lost version information
		text = text + fmt.Sprintf("%s = %s\n", v.Name, v.ImportPath)
	}

	_, err := file.Write(([]byte)(text))
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

// readFileToList convert file content to string list.
func readFileToList(path string) ([]string, error) {
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
