// Package configstate centralizes the shared config/state logic that the gup
// subcommands (update, check, list, export) need to agree on: where gup.json is
// read from and written to, how a saved-channel config is read and validated,
// how per-binary update channels are resolved and merged, and which version is
// persisted back. Keeping this logic in one package - rather than spread ad hoc
// across the cmd/ command files - guarantees that every command resolves the
// config identically and that the documented behavior around config ambiguity,
// empty environments, channel precedence and config persistence stays
// consistent.
package configstate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nao1215/gup/internal/binname"
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

// ValidateExplicitFile validates an explicitly provided --file path so that
// 'check', 'update' and 'list' honor it even on an empty environment, where the
// normal config read is otherwise skipped (#368). An empty confFile means no
// --file was given, so there is nothing to validate. A malformed file, an
// unsupported schema version, or a directory path is reported as an error; a
// non-existent path is left as "no config", matching the non-empty-environment
// behavior.
func ValidateExplicitFile(confFile string) error {
	if strings.TrimSpace(confFile) == "" {
		return nil
	}
	path, err := config.ResolveImportFilePath(confFile)
	if err != nil {
		return err
	}
	_, err = ReadFileIfExists(path)
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

// identityPathPrefix and identityNamePrefix namespace the two kinds of package
// identity keys so an import_path can never collide with a binary name that
// happens to be spelled the same.
const (
	identityPathPrefix = "path\x00"
	identityNamePrefix = "name\x00"
	// maxIdentityKeys is the most keys identityKeys can return (import_path + name).
	maxIdentityKeys = 2
)

// nameIdentityKey returns the name-based identity key for a binary name, applying
// the shared cross-OS normalization (trim + case-insensitive ".exe" via
// normalizeBinName). A bare binary name - such as a renamed-from entry that
// carries no import_path - can only be identified by this key.
func nameIdentityKey(name string) string {
	return identityNamePrefix + normalizeBinName(name)
}

// identityKeys returns the keys that identify p as a logical package, strongest
// first: its import_path when present, then its cross-OS normalized name (via
// nameIdentityKey). This is the single identity rule shared by
// ApplySavedChannels, ResolveChannels and MergePackages; looking the keys up in
// order yields "prefer import_path, fall back to name", so the same gup.json
// entry is interpreted identically on every command path (#341).
func identityKeys(p goutil.Package) []string {
	keys := make([]string, 0, maxIdentityKeys)
	if ip := strings.TrimSpace(p.ImportPath); ip != "" {
		keys = append(keys, identityPathPrefix+ip)
	}
	keys = append(keys, nameIdentityKey(p.Name))
	return keys
}

// savedEntry is the per-package state recovered from gup.json: the update
// channel and, for a pinned package, the concrete target version.
type savedEntry struct {
	channel       goutil.UpdateChannel
	pinnedVersion string
}

// channelIndex maps saved packages to their saved state under the shared
// identity rule. Each saved entry is registered under all of its identity keys
// so a lookup can prefer import_path and fall back to name.
type channelIndex map[string]savedEntry

// savedPinnedVersion extracts the concrete pinned target from a saved package,
// preferring the dedicated PinnedVersion field and falling back to the recorded
// version. It returns "" for any non-pinned channel.
func savedPinnedVersion(p goutil.Package) string {
	if goutil.NormalizeUpdateChannel(string(p.UpdateChannel)) != goutil.UpdateChannelPinned {
		return ""
	}
	if v := strings.TrimSpace(p.PinnedVersion); v != "" {
		return v
	}
	if p.Version != nil {
		return strings.TrimSpace(p.Version.Current)
	}
	return ""
}

// indexSavedChannels builds a channelIndex from a gup.json package list.
func indexSavedChannels(confPkgs []goutil.Package) channelIndex {
	idx := make(channelIndex, len(confPkgs)*maxIdentityKeys)
	for _, p := range confPkgs {
		entry := savedEntry{
			channel:       goutil.NormalizeUpdateChannel(string(p.UpdateChannel)),
			pinnedVersion: savedPinnedVersion(p),
		}
		for _, k := range identityKeys(p) {
			idx[k] = entry
		}
	}
	return idx
}

// entryFor returns the saved state for p, matching by import_path first and then
// by normalized name.
func (idx channelIndex) entryFor(p goutil.Package) (savedEntry, bool) {
	for _, k := range identityKeys(p) {
		if entry, ok := idx[k]; ok {
			return entry, true
		}
	}
	return savedEntry{}, false
}

// ApplySavedChannels copies each package's saved update channel (and pinned
// target version, when pinned) from confPkgs, matching by the shared package
// identity (import_path first, then cross-OS normalized name), so a channel is
// not silently reset to @latest across binary renames, hand-edited configs, or
// cross-OS name differences (#341). Packages with no saved entry default to
// @latest.
func ApplySavedChannels(pkgs, confPkgs []goutil.Package) []goutil.Package {
	saved := indexSavedChannels(confPkgs)
	result := make([]goutil.Package, 0, len(pkgs))
	for _, p := range pkgs {
		p.UpdateChannel = goutil.UpdateChannelLatest
		p.PinnedVersion = ""
		if entry, ok := saved.entryFor(p); ok {
			p.UpdateChannel = entry.channel
			p.PinnedVersion = entry.pinnedVersion
		}
		result = append(result, p)
	}
	return result
}

// ResolveChannels computes the effective update channel for every installed
// package. The precedence, lowest to highest, is:
//  1. @latest by default,
//  2. the channel saved in gup.json (confPkgs),
//  3. the channel selected by the --main/--master/--latest flags.
//
// The saved channel is matched by the shared package identity (import_path
// first, then cross-OS normalized name), so a saved entry is honored even when
// the installed binary was renamed, as long as the import_path still identifies
// it. Flag names are matched with the host-OS normalization used for
// exclude/target matching so a Windows "foo.exe" still matches "foo".
//
// A binary named in two conflicting channel flags is an error. A flag naming a
// binary that is not an update target is reported through warn (a non-fatal
// notice) and otherwise ignored. warn may be nil.
//
// reportedMissing holds target names the caller has already reported as
// "not found" (e.g. missing positional targets). A flag naming one of those is
// silently skipped instead of triggering a second, redundant notice for the
// same name.
//
// The second return value maps each pinned package's name to its concrete pin
// target version, so the caller installs that exact version instead of @latest.
// A package moved off the pinned channel by an explicit flag is dropped from the
// map, so the resolved channel and the pin target can never disagree.
func ResolveChannels(
	pkgs []goutil.Package,
	confPkgs []goutil.Package,
	mainPkgNames []string,
	masterPkgNames []string,
	latestPkgNames []string,
	reportedMissing []string,
	warn func(string),
) (map[string]goutil.UpdateChannel, map[string]string, error) {
	saved := indexSavedChannels(confPkgs)
	channelMap := make(map[string]goutil.UpdateChannel, len(pkgs))
	pinnedMap := make(map[string]string, len(pkgs))
	normalizedToActual := make(map[string]string, len(pkgs))
	for _, p := range pkgs {
		channel := goutil.UpdateChannelLatest
		if entry, ok := saved.entryFor(p); ok {
			channel = entry.channel
			if entry.pinnedVersion != "" {
				pinnedMap[p.Name] = entry.pinnedVersion
			}
		}
		channelMap[p.Name] = channel
		normalizedToActual[binname.NormalizeForMatch(p.Name)] = p.Name
	}

	alreadyReported := make(map[string]struct{}, len(reportedMissing))
	for _, name := range reportedMissing {
		if normalized := binname.NormalizeForMatch(name); normalized != "" {
			alreadyReported[normalized] = struct{}{}
		}
	}

	assignedByFlag := map[string]string{}
	apply := func(flag string, names []string, channel goutil.UpdateChannel) error {
		for _, raw := range names {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			normalized := binname.NormalizeForMatch(name)
			if prevFlag, ok := assignedByFlag[normalized]; ok && prevFlag != flag {
				return fmt.Errorf("same binary (%s) is specified in both --%s and --%s", name, prevFlag, flag)
			}
			assignedByFlag[normalized] = flag

			actual, ok := normalizedToActual[normalized]
			if !ok {
				if _, reported := alreadyReported[normalized]; !reported && warn != nil {
					warn("not found '" + name + "' package in update target")
				}
				continue
			}
			channelMap[actual] = channel
		}
		return nil
	}

	if err := apply("main", mainPkgNames, goutil.UpdateChannelMain); err != nil {
		return nil, nil, err
	}
	if err := apply("master", masterPkgNames, goutil.UpdateChannelMaster); err != nil {
		return nil, nil, err
	}
	if err := apply(latestKeyword, latestPkgNames, goutil.UpdateChannelLatest); err != nil {
		return nil, nil, err
	}
	// A binary moved off the pinned channel by an explicit flag is no longer
	// pinned: drop its pin target so the resolved channel and the pin target can
	// never disagree.
	for name, channel := range channelMap {
		if channel != goutil.UpdateChannelPinned {
			delete(pinnedMap, name)
		}
	}
	return channelMap, pinnedMap, nil
}

// PackageChannel returns the resolved channel for the named package, preferring
// an explicit entry in channelMap and falling back to the package's own
// channel. Both are normalized so an empty/unknown value becomes @latest.
func PackageChannel(name string, fallback goutil.UpdateChannel, channelMap map[string]goutil.UpdateChannel) goutil.UpdateChannel {
	if channel, ok := channelMap[name]; ok {
		return goutil.NormalizeUpdateChannel(string(channel))
	}
	return goutil.NormalizeUpdateChannel(string(fallback))
}

// MergePackages merges the previously-saved config (confPkgs) with the packages
// that were successfully processed in this run, returning the package list to
// persist back to gup.json. Entries are keyed by the shared package identity
// (import_path first, then cross-OS normalized name), so a single logical
// package never produces duplicate entries that differ only by a stale name, a
// renamed binary, or a Windows ".exe" suffix. Successful packages overwrite
// their saved entry with the freshly resolved channel and version; the stale
// pre-rename entry is dropped; every retained entry's channel is re-normalized.
// The result is sorted by name so the written file is stable across runs.
func MergePackages(confPkgs []goutil.Package, succeededPkgs []goutil.Package, channelMap map[string]goutil.UpdateChannel, renamedPkgs map[string]string) []goutil.Package {
	byKey := map[string]goutil.Package{}
	aliasToKey := map[string]string{}

	// upsert stores p under its canonical identity key, reusing the key of any
	// already-stored entry that shares one of p's identity keys so logically
	// identical packages collapse into one.
	upsert := func(p goutil.Package) {
		keys := identityKeys(p)
		canonical := keys[0]
		// Fold together every existing canonical this input bridges. An input can
		// match more than one (its name maps to an old entry while its import_path
		// maps to a new one); without folding, the same logical package would stay
		// split across two canonicals.
		folded := map[string]bool{}
		for _, k := range keys {
			existing, ok := aliasToKey[k]
			if !ok || folded[existing] {
				continue
			}
			if len(folded) == 0 {
				canonical = existing
			} else {
				delete(byKey, existing)
				for ak, av := range aliasToKey {
					if av == existing {
						aliasToKey[ak] = canonical
					}
				}
			}
			folded[existing] = true
		}
		byKey[canonical] = p
		for _, k := range keys {
			aliasToKey[k] = canonical
		}
	}

	for _, p := range confPkgs {
		upsert(SanitizePackage(p))
	}
	for _, p := range succeededPkgs {
		if p.Name == "" || p.ImportPath == "" {
			continue
		}
		channel := PackageChannel(p.Name, p.UpdateChannel, channelMap)
		pinnedVersion := ""
		if channel == goutil.UpdateChannelPinned {
			pinnedVersion = savedPinnedVersion(goutil.Package{UpdateChannel: channel, PinnedVersion: p.PinnedVersion, Version: p.Version})
		}
		upsert(goutil.Package{
			Name:          p.Name,
			ImportPath:    p.ImportPath,
			Version:       &goutil.Version{Current: PersistedVersion(p)},
			UpdateChannel: channel,
			PinnedVersion: pinnedVersion,
		})
	}

	// Drop the stale pre-rename entry so a module-path rename (where import_path
	// changes) does not leave a duplicate behind. The renamed-from name is matched
	// by the same cross-OS name identity used throughout this package, not by raw
	// string equality, so a hand-edited or cross-OS spelling of the old name (e.g.
	// "foo.EXE" vs "foo.exe", or an untrimmed name) is still cleaned up.
	for oldName, newName := range renamedPkgs {
		canonical, ok := aliasToKey[nameIdentityKey(oldName)]
		if !ok {
			continue
		}
		// When the rename did not change import_path, upsert already collapsed the
		// old and new entries into this one canonical, so the surviving entry is
		// the fresh package - deleting it would drop the package entirely. Only
		// remove a genuinely stale pre-rename entry.
		if kept, ok := byKey[canonical]; ok && nameIdentityKey(kept.Name) == nameIdentityKey(newName) {
			continue
		}
		delete(byKey, canonical)
	}

	for key, p := range byKey {
		if channel, ok := channelMap[p.Name]; ok {
			p.UpdateChannel = goutil.NormalizeUpdateChannel(string(channel))
			// A package moved off the pinned channel keeps no pin target.
			if p.UpdateChannel != goutil.UpdateChannelPinned {
				p.PinnedVersion = ""
			}
			byKey[key] = SanitizePackage(p)
		}
	}

	merged := make([]goutil.Package, 0, len(byKey))
	for _, p := range byKey {
		merged = append(merged, p)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name < merged[j].Name
	})
	return merged
}

// SanitizePackage returns a trimmed, channel-normalized copy of p suitable for
// writing to gup.json. A missing/blank version is normalized to "latest". A
// pinned package keeps its concrete pin target in both PinnedVersion and the
// version field so the pin survives the merge/write cycle and never degrades to
// "latest".
func SanitizePackage(p goutil.Package) goutil.Package {
	channel := goutil.NormalizeUpdateChannel(string(p.UpdateChannel))

	version := latestKeyword
	if p.Version != nil {
		v := strings.TrimSpace(p.Version.Current)
		if v != "" {
			version = v
		}
	}

	pinnedVersion := ""
	if channel == goutil.UpdateChannelPinned {
		pinnedVersion = savedPinnedVersion(p)
		if pinnedVersion != "" {
			version = pinnedVersion
		}
	}

	return goutil.Package{
		Name:          strings.TrimSpace(p.Name),
		ImportPath:    strings.TrimSpace(p.ImportPath),
		Version:       &goutil.Version{Current: version},
		UpdateChannel: channel,
		PinnedVersion: pinnedVersion,
	}
}

// PersistedVersion picks the version to persist for p. A pinned package always
// persists its concrete pin target, never @latest and never the installed
// version, so the pin cannot drift. For every other channel it persists the
// freshly resolved latest version when it is a real version, otherwise the
// current version, and "latest" as a last resort. "unknown" is treated as
// not-a-version so it is never written back (which would force a needless
// reinstall; see issue #300).
func PersistedVersion(p goutil.Package) string {
	if goutil.NormalizeUpdateChannel(string(p.UpdateChannel)) == goutil.UpdateChannelPinned {
		if pinned := savedPinnedVersion(p); pinned != "" {
			return pinned
		}
	}
	if p.Version == nil {
		return latestKeyword
	}
	if latest := strings.TrimSpace(p.Version.Latest); latest != "" && latest != "unknown" {
		return latest
	}
	if current := strings.TrimSpace(p.Version.Current); current != "" {
		return current
	}
	return latestKeyword
}

// normalizeBinName makes a binary name comparable regardless of the OS the
// config was exported from. It trims surrounding whitespace and strips a
// trailing ".exe" suffix case-insensitively, so a channel saved on Windows
// ("foo.exe", or a hand-edited "foo.EXE") still matches the same binary named
// "foo" on any OS (#341). Unlike binname.NormalizeForMatch this is intentionally
// OS-independent: the saved config may originate from a different OS than the
// one currently running gup. (The host-OS-aware matching used for
// channel/exclude flags lives in internal/binname, the single source of truth.)
func normalizeBinName(name string) string {
	name = strings.TrimSpace(name)
	const exe = ".exe"
	if len(name) >= len(exe) && strings.EqualFold(name[len(name)-len(exe):], exe) {
		return name[:len(name)-len(exe)]
	}
	return name
}

// sameIdentity reports whether a and b name the same logical package under the
// shared identity rule (import_path first, then cross-OS normalized name).
func sameIdentity(a, b goutil.Package) bool {
	for _, ka := range identityKeys(a) {
		for _, kb := range identityKeys(b) {
			if ka == kb {
				return true
			}
		}
	}
	return false
}

// SetPin returns confPkgs with target pinned to version: the matching entry is
// rewritten with channel "pinned" and the concrete version, or a new pinned
// entry is appended when target is not yet present. The version is validated so
// an unsafe pin can never enter the config. The result is sanitized and sorted
// so the written file is stable.
func SetPin(confPkgs []goutil.Package, target goutil.Package, version string) ([]goutil.Package, error) {
	version = strings.TrimSpace(version)
	if err := goutil.ValidatePinnedVersion(version); err != nil {
		return nil, err
	}

	pinned := goutil.Package{
		Name:          strings.TrimSpace(target.Name),
		ImportPath:    strings.TrimSpace(target.ImportPath),
		Version:       &goutil.Version{Current: version},
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: version,
	}

	result := make([]goutil.Package, 0, len(confPkgs)+1)
	replaced := false
	for _, p := range confPkgs {
		if !replaced && sameIdentity(p, pinned) {
			result = append(result, SanitizePackage(pinned))
			replaced = true
			continue
		}
		result = append(result, SanitizePackage(p))
	}
	if !replaced {
		result = append(result, SanitizePackage(pinned))
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// RemovePin returns confPkgs with target's pin cleared (channel reset to
// @latest) and reports whether a pinned entry was actually changed. A target
// that is absent or not pinned leaves the config unchanged and returns false, so
// unpin is idempotent.
func RemovePin(confPkgs []goutil.Package, target goutil.Package) ([]goutil.Package, bool) {
	changed := false
	result := make([]goutil.Package, 0, len(confPkgs))
	for _, p := range confPkgs {
		if sameIdentity(p, target) && goutil.NormalizeUpdateChannel(string(p.UpdateChannel)) == goutil.UpdateChannelPinned {
			p.UpdateChannel = goutil.UpdateChannelLatest
			p.PinnedVersion = ""
			changed = true
		}
		result = append(result, SanitizePackage(p))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, changed
}
