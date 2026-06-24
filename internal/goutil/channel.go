package goutil

import (
	"errors"
	"fmt"
	"strings"
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

// ValidatePinnedVersion validates that version is a usable concrete version for a
// pinned package: non-empty, not a channel keyword, and not a placeholder that
// does not name a real version. It is the single rule shared by config parsing,
// config writing, and the pin command so an unsafe pin can never be accepted or
// persisted.
func ValidatePinnedVersion(version string) error {
	v := strings.TrimSpace(version)
	if v == "" {
		return errors.New("pinned version is empty")
	}
	if IsReservedChannelKeyword(v) {
		return fmt.Errorf("pinned version %q must be a concrete version, not a channel keyword", v)
	}
	switch strings.ToLower(v) {
	case develVersionParen, develVersion, unknown:
		return fmt.Errorf("pinned version %q is not a concrete installable version", v)
	}
	return nil
}
