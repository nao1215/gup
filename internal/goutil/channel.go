package goutil

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// UpdateChannel is the update source channel for go install.
type UpdateChannel string

const (
	// UpdateChannelLatest updates by @latest.
	UpdateChannelLatest UpdateChannel = "latest"
	// UpdateChannelMain updates by @main (and fallback to @master if main is missing).
	UpdateChannelMain UpdateChannel = "main"
	// UpdateChannelMaster updates by @master.
	UpdateChannelMaster UpdateChannel = "master"
	// UpdateChannelPinned keeps the binary at a concrete recorded version; gup
	// installs that exact version and never resolves @latest/@main/@master.
	UpdateChannelPinned UpdateChannel = "pinned"
)

// NormalizeUpdateChannel normalizes a user/config value into a valid channel.
// Unknown or blank values are treated as "latest". This is the lenient,
// CLI-convenience normalization used once a channel value is already trusted
// (e.g. internal re-normalization of a value that ReadConfFile already
// validated). Parsing an untrusted config value must instead go through
// ParseConfigChannel, which rejects unknown values rather than silently
// degrading them to @latest.
func NormalizeUpdateChannel(channel string) UpdateChannel {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case string(UpdateChannelMain):
		return UpdateChannelMain
	case string(UpdateChannelMaster):
		return UpdateChannelMaster
	case string(UpdateChannelPinned):
		return UpdateChannelPinned
	case string(UpdateChannelLatest):
		return UpdateChannelLatest
	default:
		return UpdateChannelLatest
	}
}

// ParseConfigChannel parses a channel value read from gup.json strictly. A blank
// channel defaults to @latest (the historical config default), but an unknown
// value is an error rather than being silently treated as @latest: a config that
// names a channel gup does not understand is ambiguous, and degrading it to
// @latest could update a binary from the wrong source (the exact failure pinning
// must prevent). The returned channel is one of latest/main/master/pinned.
func ParseConfigChannel(channel string) (UpdateChannel, error) {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "":
		return UpdateChannelLatest, nil
	case string(UpdateChannelLatest):
		return UpdateChannelLatest, nil
	case string(UpdateChannelMain):
		return UpdateChannelMain, nil
	case string(UpdateChannelMaster):
		return UpdateChannelMaster, nil
	case string(UpdateChannelPinned):
		return UpdateChannelPinned, nil
	default:
		return "", fmt.Errorf("unknown channel %q (must be one of latest, main, master, pinned)", channel)
	}
}

// IsReservedChannelKeyword reports whether v is a channel keyword and therefore
// not a valid concrete pinned version. A pinned package must record a real,
// installable version, never "latest"/"main"/"master"/"pinned".
func IsReservedChannelKeyword(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(UpdateChannelLatest), string(UpdateChannelMain), string(UpdateChannelMaster), string(UpdateChannelPinned):
		return true
	default:
		return false
	}
}

// PinnedVersionForms describes the version strings a pin accepts. It is the
// shared remedy appended to every pin validation error, so the pin command and a
// rejected gup.json entry point the user at the same fix.
const PinnedVersionForms = "pin a full version such as v1.2.3 (prereleases such as v1.2.3-rc.1 and " +
	"v2.0.0+incompatible are allowed), or a pseudo-version such as " +
	"v0.0.0-20240102150405-abcdef123456 to pin a commit"

// incompatibleBuild is the only build-metadata suffix Go module versions allow.
const incompatibleBuild = "+incompatible"

// pseudoVersionRevLen is the length of the revision in a pseudo-version: the go
// command always records a 12-character commit hash prefix.
const pseudoVersionRevLen = 12

// ValidatePinnedVersion validates that version names one fixed Go module
// version: a canonical semantic version (vMAJOR.MINOR.PATCH, optionally with a
// prerelease, or +incompatible for a v2+ module without a /vN path) or a
// well-formed pseudo-version. Anything the go command resolves when it runs - a
// branch, a commit hash, an abbreviated version such as v1 or v1.2, or a version
// query such as >=v1.2.0 - is rejected, because installing it later could pick up
// different code than the pin was recorded for. It is the single rule shared by
// config parsing, config writing, and the pin command so an unsafe pin can never
// be accepted or persisted. The check is purely syntactic: it needs neither the
// network nor the go command, and it does not confirm that the version exists.
func ValidatePinnedVersion(version string) error {
	v := strings.TrimSpace(version)
	if v == "" {
		return errors.New("pinned version is empty: " + PinnedVersionForms)
	}
	if IsReservedChannelKeyword(v) {
		return fmt.Errorf("pinned version %q must be a concrete version, not a channel keyword: %s", v, PinnedVersionForms)
	}
	if problem := pinnedVersionProblem(v); problem != "" {
		return fmt.Errorf("pinned version %q %s: %s", v, problem, PinnedVersionForms)
	}
	return nil
}

// pinnedVersionProblem returns why v is not a fixed Go module version, or "" when
// it is one.
func pinnedVersionProblem(v string) string {
	if !semver.IsValid(v) {
		switch {
		case isCommitHash(v):
			return "looks like a commit hash, which is not a version"
		case strings.IndexAny(v, "<>=!") == 0 || isVersionQueryKeyword(v):
			return "is a version query, which resolves to a different version over time"
		default:
			return "is not a Go module version (branch names and other references that can move are not allowed)"
		}
	}

	build := semver.Build(v)
	if build != "" && build != incompatibleBuild {
		return fmt.Sprintf("has build metadata %q, but Go module versions only allow %q", build, incompatibleBuild)
	}
	// semver accepts v1 and v1.2 as shorthands for v1.0.0 and v1.2.0, but the go
	// command treats them as queries for the newest matching release, so only the
	// canonical spelling pins a version.
	base := strings.TrimSuffix(v, build)
	if semver.Canonical(base) != base {
		return "is an abbreviated version, which the go command resolves to the newest matching release"
	}
	if build == incompatibleBuild {
		if major := semver.Major(v); major == "v0" || major == "v1" {
			return fmt.Sprintf("uses %q with major version %s, but only v2 and later can be incompatible", incompatibleBuild, major)
		}
	}
	if module.IsPseudoVersion(v) {
		if _, err := module.PseudoVersionBase(v); err != nil {
			return "is not a valid pseudo-version"
		}
		if _, err := module.PseudoVersionTime(v); err != nil {
			return "is not a valid pseudo-version (malformed timestamp)"
		}
		if rev, err := module.PseudoVersionRev(v); err != nil || len(rev) != pseudoVersionRevLen || !isHex(rev) {
			return fmt.Sprintf("is not a valid pseudo-version (the revision must be the %d-character lowercase commit hash prefix)", pseudoVersionRevLen)
		}
	}
	return ""
}

// isCommitHash reports whether v looks like an abbreviated or full commit hash:
// 7 to 64 hexadecimal digits.
func isCommitHash(v string) bool {
	const minAbbrevHash, maxHash = 7, 64
	return len(v) >= minAbbrevHash && len(v) <= maxHash && isHex(strings.ToLower(v))
}

// isVersionQueryKeyword reports whether v is one of the go command's module
// query keywords other than the channel keywords (latest is handled as a channel).
func isVersionQueryKeyword(v string) bool {
	switch strings.ToLower(v) {
	case "upgrade", "patch", "none":
		return true
	default:
		return false
	}
}

// isHex reports whether s consists only of lowercase hexadecimal digits.
func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
