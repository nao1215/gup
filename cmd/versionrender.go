package cmd

import (
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/goutil"
)

// unknownVersion mirrors goutil's sentinel for an undeterminable version. It is
// only used for display here (showing "unknown" when a pinned package has no
// recorded installed version).
const unknownVersion = "unknown"

// hideIgnoredGoDelta collapses an ignored Go-toolchain delta on a package that
// is otherwise up to date, so the human-readable line reads consistently with the
// "no update" decision instead of printing a phantom "goX to goY" the command
// will not act on. It mirrors what checkPinned and updatePinned already do for
// pinned packages.
//
// The collapse happens only when ignoreGoUpdate is true. Callers set that flag
// when --ignore-go-update is passed or when the installed Go version can't be
// detected (ignoreGoUpdate = opts.ignoreGoUpdate || !goVersionAvailable), so both
// reach here through the same single condition. It is a no-op in --json mode so
// the machine-readable installed_go_version stays the real installed toolchain (a
// documented, factual field), keeping that contract unchanged.
func hideIgnoredGoDelta(p *goutil.Package, ignoreGoUpdate, jsonOut bool) {
	if jsonOut || !ignoreGoUpdate || p.GoVersion == nil {
		return
	}
	p.GoVersion.Latest = p.GoVersion.Current
}

// currentToLatestStr renders the current->latest transition for an update,
// coloring each version by whether it is the up-to-date side. The version
// comparison rule itself is owned by goutil; this layer only decides how to
// present it.
func currentToLatestStr(p goutil.Package) string {
	if p.IsPackageUpToDate() && p.IsGoUpToDate() {
		return "Already up-to-date: " + color.GreenString(p.Version.Current) + " / " + color.GreenString(p.GoVersion.Current)
	}
	var ret string
	if p.Version.Current != p.Version.Latest {
		currentVer, latestVer := colorVersionPair(p.Version.Current, p.Version.Latest, "v")
		ret += currentVer + " to " + latestVer
	}
	if p.GoVersion.Current != p.GoVersion.Latest {
		if len(ret) != 0 {
			ret += ", "
		}
		currentGo, latestGo := colorVersionPair(p.GoVersion.Current, p.GoVersion.Latest, "go")
		ret += currentGo + " to " + latestGo
	}
	return ret
}

// pinnedResultStr renders a human-readable description of a pinned package's
// state: kept at the pin, installed at a different version than the pin, or at
// the pinned version but built with an older Go toolchain (a pending rebuild).
// A Go delta is shown only when the caller left GoVersion.Current != Latest;
// check/update zero it out when the delta is being ignored, so this stays quiet.
func pinnedResultStr(p goutil.Package) string {
	pinned := strings.TrimSpace(p.PinnedVersion)
	current := ""
	if p.Version != nil {
		current = strings.TrimSpace(p.Version.Current)
	}
	if !p.PinSatisfied() {
		installed := current
		if installed == "" {
			installed = unknownVersion
		}
		return "pinned " + color.GreenString(pinned) + ", installed " + color.YellowString(installed)
	}
	if p.GoVersion != nil && !p.IsGoUpToDate() {
		currentGo, latestGo := colorVersionPair(p.GoVersion.Current, p.GoVersion.Latest, "go")
		return "pinned " + color.GreenString(pinned) + " (" + currentGo + " to " + latestGo + ")"
	}
	return "pinned " + color.GreenString(pinned)
}

// versionCheckResultStr renders the version/Go-toolchain check result for a
// package, coloring each side by whether it is up to date.
func versionCheckResultStr(p goutil.Package) string {
	if p.IsPackageUpToDate() && p.IsGoUpToDate() {
		return "Already up-to-date: " + color.GreenString(p.Version.Current) + " / " + color.GreenString(p.GoVersion.Current)
	}
	var ret string
	currentVer, latestVer := colorVersionPair(p.Version.Current, p.Version.Latest, "v")
	if p.Version.Current == p.Version.Latest {
		ret += currentVer
	} else {
		ret += "current: " + currentVer + ", latest: " + latestVer
	}
	ret += " / "
	currentGo, latestGo := colorVersionPair(p.GoVersion.Current, p.GoVersion.Latest, "go")
	if p.GoVersion.Current == p.GoVersion.Latest {
		ret += currentGo
	} else {
		ret += "current: " + currentGo + ", installed: " + latestGo
	}
	return ret
}

// colorVersionPair colors a (current, latest) version pair: the up-to-date side
// is green, the out-of-date side yellow. prefix is the version prefix ("v" for
// package versions, "go" for the toolchain) that selects the comparison rule and
// is stripped before comparing. The comparison itself defers to goutil.
func colorVersionPair(current, latest, prefix string) (string, string) {
	upToDate := goutil.VersionUpToDate
	if prefix == "go" {
		upToDate = goutil.GoVersionUpToDate
	}

	currentNoPrefix := strings.TrimPrefix(current, prefix)
	latestNoPrefix := strings.TrimPrefix(latest, prefix)
	currentUpToDate := upToDate(currentNoPrefix, latestNoPrefix)
	latestUpToDate := upToDate(latestNoPrefix, currentNoPrefix)

	switch {
	case currentUpToDate && latestUpToDate:
		return color.GreenString(current), color.GreenString(latest)
	case currentUpToDate:
		return color.GreenString(current), color.YellowString(latest)
	case latestUpToDate:
		return color.YellowString(current), color.GreenString(latest)
	default:
		return color.YellowString(current), color.YellowString(latest)
	}
}
