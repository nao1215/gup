package configstate

import (
	"sort"
	"strings"

	"github.com/nao1215/gup/internal/goutil"
)

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
		// Drop every entry that matches the pinned package's identity, not just the
		// first: a hand-edited gup.json with duplicate entries for one package would
		// otherwise leave a stale duplicate behind that overwrites the new pin on the
		// next config read.
		if sameIdentity(p, pinned) {
			if !replaced {
				result = append(result, SanitizePackage(pinned))
				replaced = true
			}
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
