package goutil

import (
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
)

// Package is package information.
type Package struct {
	// Name is package name
	Name string
	// ImportPath is the package import path used by 'go install'
	ImportPath string
	// ModulePath is path where go.mod is stored.
	// May not be set if module path cannot be determined
	ModulePath string
	// Version store Package version (current and latest).
	Version *Version
	// GoVersion stores version of Go toolchain
	GoVersion *Version
	// UpdateChannel stores preferred update channel.
	UpdateChannel UpdateChannel
	// PinnedVersion is the concrete target version when UpdateChannel is
	// "pinned". It is empty for every other channel. It is kept separate from
	// Version.Current (the installed version) so a pin can downgrade a binary and
	// so check/update compare the installed version against the pin target
	// without consulting @latest.
	PinnedVersion string
}

// IsPinned reports whether the package is pinned to a concrete version.
func (p *Package) IsPinned() bool {
	return p.UpdateChannel == UpdateChannelPinned
}

// PinSatisfied reports whether a pinned package's installed version already
// matches its pinned target. The comparison is exact (not ">="), so a pin that
// asks for an older version than the one installed is reported as unsatisfied
// and will be reinstalled at the pinned version.
func (p *Package) PinSatisfied() bool {
	if !p.IsPinned() || p.Version == nil {
		return false
	}
	pinned := strings.TrimSpace(p.PinnedVersion)
	if pinned == "" {
		// An empty pin target is never "satisfied": treat it as a mismatch so an
		// invalid pinned package is surfaced, not silently reported as up-to-date.
		return false
	}
	return strings.TrimSpace(p.Version.Current) == pinned
}

// Version is package version information.
type Version struct {
	// Current(before update) version
	Current string
	// Latest(after update) version
	Latest string
}

// NewVersion return Version instance.
func NewVersion() *Version {
	return &Version{
		Current: "",
		Latest:  "",
	}
}

// SetLatestVer set package latest version.
func (p *Package) SetLatestVer() {
	p.Version.Latest = GetPackageVersion(p.Name)
}

// CurrentToLatestStr returns string about the current version and the latest version.
func (p *Package) CurrentToLatestStr() string {
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

// PinnedResultStr returns a human-readable description of a pinned package's
// state: kept at the pin, installed at a different version than the pin, or at
// the pinned version but built with an older Go toolchain (a pending rebuild).
// A Go delta is shown only when the caller left GoVersion.Current != Latest;
// check/update zero it out when the delta is being ignored, so this stays quiet.
func (p *Package) PinnedResultStr() string {
	pinned := strings.TrimSpace(p.PinnedVersion)
	current := ""
	if p.Version != nil {
		current = strings.TrimSpace(p.Version.Current)
	}
	if !p.PinSatisfied() {
		installed := current
		if installed == "" {
			installed = unknown
		}
		return "pinned " + color.GreenString(pinned) + ", installed " + color.YellowString(installed)
	}
	if p.GoVersion != nil && !p.IsGoUpToDate() {
		currentGo, latestGo := colorVersionPair(p.GoVersion.Current, p.GoVersion.Latest, "go")
		return "pinned " + color.GreenString(pinned) + " (" + currentGo + " to " + latestGo + ")"
	}
	return "pinned " + color.GreenString(pinned)
}

// VersionCheckResultStr returns string about command version check.
func (p *Package) VersionCheckResultStr() string {
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

func colorVersionPair(current, latest, prefix string) (string, string) {
	upToDate := versionUpToDate
	if prefix == "go" {
		upToDate = goVersionUpToDate
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

// IsPackageUpToDate checks if the Package (set by the package author) version is up to date.
// Returns true if current >= available.
func (p *Package) IsPackageUpToDate() bool {
	return versionUpToDate(
		strings.TrimPrefix(p.Version.Current, "v"),
		strings.TrimPrefix(p.Version.Latest, "v"),
	)
}

// IsGoUpToDate checks if the Golang runtime version is up to date.
// Returns true if current >= available.
func (p *Package) IsGoUpToDate() bool {
	return goVersionUpToDate(
		strings.TrimPrefix(p.GoVersion.Current, "go"),
		strings.TrimPrefix(p.GoVersion.Latest, "go"),
	)
}

// goVersionUpToDate compares Go toolchain versions.
// Some custom toolchains append non-semver separators (e.g. "X:nodwarf5"),
// so normalize known separators before semver comparison.
func goVersionUpToDate(current, available string) bool {
	current = strings.TrimSpace(current)
	available = strings.TrimSpace(available)
	if current == unknown || available == unknown {
		return false
	}
	// If both strings are exactly the same, treat as up-to-date even when
	// custom tags are not strict semver.
	if current == available {
		return true
	}
	return versionUpToDate(
		normalizeGoVersionForCompare(current),
		normalizeGoVersionForCompare(available),
	)
}

func normalizeGoVersionForCompare(ver string) string {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return ver
	}

	var b strings.Builder
	b.Grow(len(ver))
	for _, r := range ver {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '.',
			r == '-',
			r == '+':
			b.WriteRune(r)
		default:
			b.WriteByte('.')
		}
	}
	return b.String()
}

// versionUpToDate parses versions and compares them.
// Returns true if current >= available.
func versionUpToDate(current, available string) bool {
	if current == unknown || available == unknown {
		return false // unknown version is not up to date
	}

	currentVer, err := version.NewVersion(current)
	if err != nil {
		return false // invalid version is not up to date
	}
	availableVer, err := version.NewVersion(available)
	if err != nil {
		return false // invalid version is not up to date
	}

	if currentVer.GreaterThanOrEqual(availableVer) {
		return true
	}
	return false
}
