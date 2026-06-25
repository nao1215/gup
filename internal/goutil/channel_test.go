package goutil

import (
	"testing"
)

const (
	cvV100 = "v1.0.0"
	cvV110 = "v1.1.0"
)

func TestNormalizeUpdateChannel_pinned(t *testing.T) {
	t.Parallel()
	if got := NormalizeUpdateChannel("pinned"); got != UpdateChannelPinned {
		t.Errorf("NormalizeUpdateChannel(pinned) = %q, want pinned", got)
	}
	if got := NormalizeUpdateChannel("PINNED"); got != UpdateChannelPinned {
		t.Errorf("NormalizeUpdateChannel(PINNED) = %q, want pinned", got)
	}
	// Unknown values stay lenient (CLI convenience): degrade to latest.
	if got := NormalizeUpdateChannel("stable"); got != UpdateChannelLatest {
		t.Errorf("NormalizeUpdateChannel(stable) = %q, want latest", got)
	}
}

func TestParseConfigChannel(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		in      string
		want    UpdateChannel
		wantErr bool
	}{
		{name: "blank defaults to latest", in: "", want: UpdateChannelLatest},
		{name: "whitespace defaults to latest", in: "   ", want: UpdateChannelLatest},
		{name: "latest channel", in: string(UpdateChannelLatest), want: UpdateChannelLatest},
		{name: "main channel", in: string(UpdateChannelMain), want: UpdateChannelMain},
		{name: "master channel", in: string(UpdateChannelMaster), want: UpdateChannelMaster},
		{name: "pinned channel", in: string(UpdateChannelPinned), want: UpdateChannelPinned},
		{name: "uppercase accepted", in: "Latest", want: UpdateChannelLatest},
		{name: "unknown is an error, not latest", in: "stable", wantErr: true},
		{name: "typo is an error", in: "lates", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseConfigChannel(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseConfigChannel(%q) expected error, got nil (=%q)", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConfigChannel(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseConfigChannel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidatePinnedVersion(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "semver ok", in: "v1.62.0"},
		{name: "pseudo version ok", in: "v0.0.0-20240101000000-000000000000"},
		{name: "empty rejected", in: "", wantErr: true},
		{name: "whitespace rejected", in: "   ", wantErr: true},
		{name: "latest keyword rejected", in: string(UpdateChannelLatest), wantErr: true},
		{name: "main keyword rejected", in: string(UpdateChannelMain), wantErr: true},
		{name: "master keyword rejected", in: string(UpdateChannelMaster), wantErr: true},
		{name: "pinned keyword rejected", in: string(UpdateChannelPinned), wantErr: true},
		{name: "devel rejected", in: develVersionParen, wantErr: true},
		{name: "unknown rejected", in: unknown, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePinnedVersion(tt.in)
			if tt.wantErr && err == nil {
				t.Errorf("ValidatePinnedVersion(%q) expected error, got nil", tt.in)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePinnedVersion(%q) unexpected error: %v", tt.in, err)
			}
		})
	}
}

func TestPackagePinHelpers(t *testing.T) {
	t.Parallel()

	pinnedMatch := Package{UpdateChannel: UpdateChannelPinned, PinnedVersion: cvV100, Version: &Version{Current: cvV100}}
	if !pinnedMatch.IsPinned() {
		t.Error("IsPinned() = false, want true")
	}
	if !pinnedMatch.PinSatisfied() {
		t.Error("PinSatisfied() = false, want true for matching version")
	}

	pinnedMismatch := Package{UpdateChannel: UpdateChannelPinned, PinnedVersion: cvV100, Version: &Version{Current: cvV110}}
	if pinnedMismatch.PinSatisfied() {
		t.Error("PinSatisfied() = true, want false for differing version (incl. downgrade)")
	}

	notPinned := Package{UpdateChannel: UpdateChannelLatest, Version: &Version{Current: cvV100}}
	if notPinned.IsPinned() || notPinned.PinSatisfied() {
		t.Error("a latest-channel package must not report as pinned/satisfied")
	}

	// An empty pin target is never satisfied, even if Current is also empty.
	emptyPin := Package{UpdateChannel: UpdateChannelPinned, PinnedVersion: "  ", Version: &Version{Current: ""}}
	if emptyPin.PinSatisfied() {
		t.Error("PinSatisfied() = true for an empty pin target, want false")
	}

	// A nil Version is never satisfied.
	nilVer := Package{UpdateChannel: UpdateChannelPinned, PinnedVersion: cvV100}
	if nilVer.PinSatisfied() {
		t.Error("PinSatisfied() = true with nil Version, want false")
	}
}
