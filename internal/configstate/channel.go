package configstate

import (
	"fmt"
	"strings"

	"github.com/nao1215/gup/internal/binname"
	"github.com/nao1215/gup/internal/goutil"
)

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
