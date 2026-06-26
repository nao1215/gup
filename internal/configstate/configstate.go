// Package configstate centralizes the shared config/state logic that the gup
// subcommands (update, check, list, export) need to agree on: where gup.json is
// read from and written to, how a saved-channel config is read and validated,
// how per-binary update channels are resolved and merged, and which version is
// persisted back. Keeping this logic in one package - rather than spread ad hoc
// across the cmd/ command files - guarantees that every command resolves the
// config identically and that the documented behavior around config ambiguity,
// empty environments, channel precedence and config persistence stays
// consistent.
//
// The package is split across focused files that share one identity rule:
//   - identity.go: the package-identity keys (import_path first, then cross-OS
//     normalized name) shared by every lookup.
//   - channel.go:  reading the saved channel index and resolving the effective
//     per-package update channel from config + flags.
//   - merge.go:    merging resolved packages back into the list persisted to
//     gup.json, and normalizing each persisted entry/version.
//   - pin.go:      adding and removing concrete version pins.
//   - configstate.go (this file): the read/validate/resolve entry points the
//     cmd/ layer calls.
package configstate

import (
	"fmt"
	"strings"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
)

// latestKeyword is the version/channel literal persisted when no concrete
// version is known.
const latestKeyword = "latest"

// ReadFileIfExists returns the packages stored in the gup.json at path. A
// non-existent path is not an error: it means "no saved config" and yields an
// empty slice so callers keep their default behavior. A directory passed where
// a gup.json file is expected is a user mistake, not "no config": it is
// rejected so check/update/export fail fast instead of silently ignoring the
// path (#367, #368). A malformed or unsupported-schema file is surfaced as an
// error so callers fail fast instead of falling back to @latest (#369).
func ReadFileIfExists(path string) ([]goutil.Package, error) {
	if fileutil.IsDir(path) {
		return nil, fmt.Errorf("%s is a directory, not a gup.json file", path)
	}
	if !fileutil.IsFile(path) {
		return []goutil.Package{}, nil
	}
	pkgs, err := config.ReadConfFile(path)
	if err != nil {
		return nil, err
	}
	return pkgs, nil
}

// ValidateResolvedConfig validates the gup.json that this run would read,
// whether it is named explicitly with --file or auto-detected, so that an empty
// environment fails fast on the same config problems a non-empty environment
// would, instead of silently succeeding just because zero binaries are installed
// (#368). An empty confFile means auto-detection: when both the user-level
// config and ./gup.json exist the choice is ambiguous and an error is returned,
// matching the non-empty path. A directory where a file is expected, a malformed
// file, an unsupported schema version, or invalid channel/pin data are all
// reported as errors; a non-existent resolved path is left as "no config".
//
// This mirrors the resolution ResolveAndApplyChannels performs (minus the
// channel application, which is a no-op on an empty package list), so empty and
// non-empty environments validate identically.
func ValidateResolvedConfig(confFile string) error {
	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		return err
	}
	_, err = ReadFileIfExists(confReadPath)
	return err
}

// ResolveWritePath decides where update persists channel and rename data.
// An explicit --file always wins, even when the file does not exist yet:
// otherwise "gup update --main x --file new.json" would silently save channels
// to the user-level config instead of the path the user named (the file is
// created on first write). Without --file, an already-existing auto-detected
// config (confReadPath) is reused so updates round-trip, and as a last resort
// the user-level config path is used.
func ResolveWritePath(confFile, confReadPath string) string {
	if strings.TrimSpace(confFile) != "" {
		return confReadPath
	}
	if fileutil.IsFile(confReadPath) {
		return confReadPath
	}
	return config.FilePath()
}

// ShouldPersistChannels reports whether any channel-selecting flag was provided
// and therefore the resolved channels should be written back to gup.json.
func ShouldPersistChannels(mainPkgNames, masterPkgNames, latestPkgNames []string) bool {
	return len(mainPkgNames) > 0 || len(masterPkgNames) > 0 || len(latestPkgNames) > 0
}

// ResolveAndApplyChannels resolves the saved per-package update channel from
// gup.json so that 'check' and 'list --json' compare each binary against the
// same source 'gup update' would install from. The config is located with the
// same resolution rules used by import/update; when both the user-level config
// and ./gup.json exist and no --file is given, the choice is ambiguous and an
// error is returned so the caller fails fast instead of silently picking one
// (#342, #364). A malformed or unreadable config also fails fast (#369). When
// no config exists every package keeps the default @latest behavior.
func ResolveAndApplyChannels(pkgs []goutil.Package, confFile string) ([]goutil.Package, error) {
	confReadPath, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		return nil, err
	}

	confPkgs, err := ReadFileIfExists(confReadPath)
	if err != nil {
		return nil, err
	}

	return ApplySavedChannels(pkgs, confPkgs), nil
}
