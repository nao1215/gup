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

// ConfigFileName is gup command configuration file.
const ConfigFileName = "gup.json"

// Config schema versions. v1 is the original format (latest/main/master
// channels only). v2 adds the "pinned" channel, whose entries carry a concrete
// target version. A gup.json is written as v2 only when it actually contains a
// pinned package, so environments with no pins keep producing the v1 format an
// older gup can still read. Writing "channel": "pinned" into a v1 file would be
// unsafe because an older gup normalizes unknown channels to @latest; emitting
// v2 instead makes an older gup fail fast on the unsupported schema_version
// rather than silently unpin the package.
const (
	configSchemaVersionV1 = 1
	configSchemaVersionV2 = 2
)

// Placeholder version strings that are normalized to "latest" when persisted,
// because none of them names a concrete, installable module version.
const (
	versionLatest      = "latest"
	versionDevel       = "devel"
	versionDevelParen  = "(devel)"
	versionUnknownWord = "unknown"
)

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

// ReadConfFile return contents of configuration-file (package information).
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
	if conf.SchemaVersion != configSchemaVersionV1 && conf.SchemaVersion != configSchemaVersionV2 {
		return nil, fmt.Errorf("%s has unsupported schema_version: %d (supported: %d, %d)",
			path, conf.SchemaVersion, configSchemaVersionV1, configSchemaVersionV2)
	}

	pkgs := make([]goutil.Package, 0, len(conf.Packages))
	for i, v := range conf.Packages {
		name := strings.TrimSpace(v.Name)
		importPath := strings.TrimSpace(v.ImportPath)
		version := strings.TrimSpace(v.Version)
		if name == "" || importPath == "" || version == "" {
			return nil, fmt.Errorf("%s contains invalid package entry at index %d", path, i)
		}

		// Channels are parsed strictly: an unknown channel is an error rather than
		// being silently treated as @latest, so a misread config can never update a
		// binary from the wrong source (#pinning safety rule).
		channel, err := goutil.ParseConfigChannel(v.Channel)
		if err != nil {
			return nil, fmt.Errorf("%s package %q: %w", path, name, err)
		}

		pinnedVersion := ""
		if channel == goutil.UpdateChannelPinned {
			// A pinned channel is only meaningful in schema v2. An older gup writes
			// v1 and normalizes unknown channels to @latest, so a v1 file that claims
			// "pinned" is ambiguous/hand-edited: fail fast instead of guessing.
			if conf.SchemaVersion < configSchemaVersionV2 {
				return nil, fmt.Errorf("%s package %q: channel \"pinned\" requires schema_version %d, but file is schema_version %d",
					path, name, configSchemaVersionV2, conf.SchemaVersion)
			}
			if err := goutil.ValidatePinnedVersion(version); err != nil {
				return nil, fmt.Errorf("%s package %q: %w", path, name, err)
			}
			pinnedVersion = version
		}

		binVer := goutil.Version{Current: version, Latest: ""}
		goVer := goutil.Version{Current: "<from gup.json>", Latest: ""}
		pkgs = append(pkgs, goutil.Package{
			Name:          name,
			ImportPath:    importPath,
			Version:       ptr(binVer),
			GoVersion:     ptr(goVer),
			UpdateChannel: channel,
			PinnedVersion: pinnedVersion,
		})
	}

	return pkgs, nil
}

// WriteConfFile write package information at configuration-file.
func WriteConfFile(file io.Writer, pkgs []goutil.Package) error {
	conf := configFile{
		SchemaVersion: schemaVersionFor(pkgs),
		Packages:      make([]configPackage, 0, len(pkgs)),
	}

	for _, v := range pkgs {
		channel := goutil.NormalizeUpdateChannel(string(v.UpdateChannel))
		version, err := versionForChannel(v, channel)
		if err != nil {
			return fmt.Errorf("can't write package %q: %w", v.Name, err)
		}
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

// normalizeConfVersion is the single place that decides which version string is
// persisted to gup.json. Placeholder versions that are not real, installable
// versions ("", "(devel)"/"devel", and "unknown") are normalized to "latest".
// Persisting "unknown" would make versionUpToDate never match it and force a
// needless reinstall on every run (see issue #300). All gup.json writes go
// through WriteConfFile, which calls this, so the behavior can't diverge by
// path.
func normalizeConfVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == versionDevelParen || version == versionDevel || version == versionUnknownWord {
		return versionLatest
	}
	return version
}

// schemaVersionFor picks the schema version to write: v2 when any package is
// pinned (so the "pinned" channel is only ever emitted under a schema that
// understands it), otherwise v1 so environments without pins keep producing a
// file an older gup can read unchanged.
func schemaVersionFor(pkgs []goutil.Package) int {
	for _, v := range pkgs {
		if goutil.NormalizeUpdateChannel(string(v.UpdateChannel)) == goutil.UpdateChannelPinned {
			return configSchemaVersionV2
		}
	}
	return configSchemaVersionV1
}

// versionForChannel returns the version string to persist for a package. For a
// pinned package the concrete pinned target is written (from PinnedVersion,
// falling back to the recorded current version) and validated so an unsafe pin
// is never written to disk - a pin must never degrade into "latest". For every
// other channel the existing placeholder normalization applies.
func versionForChannel(p goutil.Package, channel goutil.UpdateChannel) (string, error) {
	if channel == goutil.UpdateChannelPinned {
		version := strings.TrimSpace(p.PinnedVersion)
		if version == "" && p.Version != nil {
			version = strings.TrimSpace(p.Version.Current)
		}
		if err := goutil.ValidatePinnedVersion(version); err != nil {
			return "", err
		}
		return version, nil
	}
	if p.Version != nil {
		return normalizeConfVersion(p.Version.Current), nil
	}
	return versionLatest, nil
}
