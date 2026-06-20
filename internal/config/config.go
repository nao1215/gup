// Package config define gup command setting.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
)

// ptr returns a pointer to v. It replaces the external pointer.Ptr helper,
// whose go1.26 build constraint conflicts with this module's go1.25.0 policy.
func ptr[T any](v T) *T {
	return &v
}

// ConfigFileName is gup command configuration file
const ConfigFileName = "gup.json"
const configSchemaVersion = 1

type configFile struct {
	SchemaVersion int             `json:"schema_version"`
	Packages      []configPackage `json:"packages"`
}

type configPackage struct {
	Name       string `json:"name"`
	ImportPath string `json:"import_path"`
	Version    string `json:"version"`
	Channel    string `json:"channel"`
}

// FilePath return configuration-file path.
func FilePath() string {
	return filepath.Join(DirPath(), ConfigFileName)
}

// LocalFilePath returns the path to gup.json in the current directory.
func LocalFilePath() string {
	return filepath.Join(".", ConfigFileName)
}

// DirPath return directory path that store configuration-file.
// Default path is $HOME/.config/gup.
func DirPath() string {
	return filepath.Join(xdg.ConfigHome, cmdinfo.Name)
}

// ResolveImportFilePath resolves the gup.json path used by import.
// Priority:
//   - an explicit path always wins.
//   - if exactly one auto-detected candidate (the user-level config or
//     ./gup.json) exists, that candidate is used.
//   - if both candidates exist and no explicit path is given, the choice is
//     ambiguous and an error is returned so the caller can ask the user to
//     rerun with --file.
//   - if neither candidate exists, the user-level config path is returned so
//     the caller can report it as "not found".
func ResolveImportFilePath(explicitPath string) (string, error) {
	explicitPath = strings.TrimSpace(explicitPath)
	if explicitPath != "" {
		return explicitPath, nil
	}

	defaultPath := FilePath()
	local := LocalFilePath()
	defaultExists := fileutil.IsFile(defaultPath)
	localExists := fileutil.IsFile(local)

	if defaultExists && localExists {
		return "", fmt.Errorf(
			"multiple gup.json candidates found (%s and %s); rerun with --file to choose one",
			defaultPath, local)
	}
	if localExists {
		return local, nil
	}
	return defaultPath, nil
}

// ResolveExportFilePath resolves config file path for export.
// Priority: explicit path > default config path.
func ResolveExportFilePath(explicitPath string) string {
	explicitPath = strings.TrimSpace(explicitPath)
	if explicitPath != "" {
		return explicitPath
	}
	return FilePath()
}

// ReadConfFile return contents of configuration-file (package information)
func ReadConfFile(path string) ([]goutil.Package, error) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return []goutil.Package{}, nil
	}

	conf := configFile{}
	if err := json.Unmarshal(raw, &conf); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if conf.SchemaVersion != configSchemaVersion {
		return nil, fmt.Errorf("%s has unsupported schema_version: %d", path, conf.SchemaVersion)
	}

	pkgs := make([]goutil.Package, 0, len(conf.Packages))
	for i, v := range conf.Packages {
		name := strings.TrimSpace(v.Name)
		importPath := strings.TrimSpace(v.ImportPath)
		version := strings.TrimSpace(v.Version)
		if name == "" || importPath == "" || version == "" {
			return nil, fmt.Errorf("%s contains invalid package entry at index %d", path, i)
		}

		binVer := goutil.Version{Current: version, Latest: ""}
		goVer := goutil.Version{Current: "<from gup.json>", Latest: ""}
		pkgs = append(pkgs, goutil.Package{
			Name:          name,
			ImportPath:    importPath,
			Version:       ptr(binVer),
			GoVersion:     ptr(goVer),
			UpdateChannel: goutil.NormalizeUpdateChannel(v.Channel),
		})
	}

	return pkgs, nil
}

// WriteConfFile write package information at configuration-file.
func WriteConfFile(file io.Writer, pkgs []goutil.Package) error {
	conf := configFile{
		SchemaVersion: configSchemaVersion,
		Packages:      make([]configPackage, 0, len(pkgs)),
	}

	for _, v := range pkgs {
		version := "latest"
		if v.Version != nil {
			version = normalizeConfVersion(v.Version.Current)
		}
		channel := goutil.NormalizeUpdateChannel(string(v.UpdateChannel))
		conf.Packages = append(conf.Packages, configPackage{
			Name:       v.Name,
			ImportPath: v.ImportPath,
			Version:    version,
			Channel:    string(channel),
		})
	}

	out, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return fmt.Errorf("can't marshal gup.json JSON: %w", err)
	}
	out = append(out, '\n')

	_, err = file.Write(out)
	if err != nil {
		return fmt.Errorf("can't write gup.json: %w", err)
	}
	return nil
}

func normalizeConfVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "(devel)" || version == "devel" {
		return "latest"
	}
	return version
}
