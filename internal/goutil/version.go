package goutil

import (
	"strings"

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

// VersionUpToDate reports whether current is at least available (current >=
// available) under semver, treating "unknown" or unparsable versions as not up
// to date. Any prefix such as "v" must already be stripped by the caller. It is
// the exported entry point the presentation layer uses to decide colorization,
// keeping the version-comparison rule owned by goutil while display lives in the
// cmd layer.
func VersionUpToDate(current, available string) bool {
	return versionUpToDate(current, available)
}

// GoVersionUpToDate is VersionUpToDate for Go toolchain versions: it first
// normalizes known non-semver separators (e.g. "go1.26.0-X:nodwarf5") so custom
// toolchains compare correctly. The "go" prefix must already be stripped.
func GoVersionUpToDate(current, available string) bool {
	return goVersionUpToDate(current, available)
}
