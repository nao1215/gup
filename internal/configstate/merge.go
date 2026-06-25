package configstate

import (
	"sort"
	"strings"

	"github.com/nao1215/gup/internal/goutil"
)

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
		// Persist the version for the *effective* channel, not the package's own
		// (possibly stale) UpdateChannel: a package moved off pinned by an explicit
		// flag must not keep persisting its old pin target as the version, which
		// would leave a non-pinned entry frozen at the pin (a channel/pin
		// disagreement).
		persistSource := p
		persistSource.UpdateChannel = channel
		persistSource.PinnedVersion = pinnedVersion
		upsert(goutil.Package{
			Name:          p.Name,
			ImportPath:    p.ImportPath,
			Version:       &goutil.Version{Current: PersistedVersion(persistSource)},
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
