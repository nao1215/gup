package goutil

import "strings"

// UpdateChannel is the update source channel for go install.
type UpdateChannel string

const (
	// UpdateChannelLatest updates by @latest.
	UpdateChannelLatest UpdateChannel = "latest"
	// UpdateChannelMain updates by @main (and fallback to @master if main is missing).
	UpdateChannelMain UpdateChannel = "main"
	// UpdateChannelMaster updates by @master.
	UpdateChannelMaster UpdateChannel = "master"
)

// NormalizeUpdateChannel normalizes a user/config value into a valid channel.
// Unknown or blank values are treated as "latest".
func NormalizeUpdateChannel(channel string) UpdateChannel {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case string(UpdateChannelMain):
		return UpdateChannelMain
	case string(UpdateChannelMaster):
		return UpdateChannelMaster
	case string(UpdateChannelLatest):
		return UpdateChannelLatest
	default:
		return UpdateChannelLatest
	}
}
