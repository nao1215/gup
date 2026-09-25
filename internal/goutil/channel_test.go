package goutil

import (
	"strconv"
	"strings"
	"testing"
)

const (
	cvV100 = "v1.0.0"
	cvV110 = "v1.1.0"

	// Pins that name a moving reference rather than one fixed version.
	cvBranch     = "release"
	cvShortHash  = "abc1234"
	cvMajorMinor = "v1.2"
	cvQuery      = ">=v1.2.0"
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
		// Accepted: every form names one fixed version.
		{name: "semver ok", in: "v1.62.0"},
		{name: "zero version ok", in: "v0.0.0"},
		{name: "surrounding whitespace ok", in: " v1.2.3 "},
		{name: "large numbers ok", in: "v10.200.3000"},
		{name: "prerelease ok", in: "v1.2.3-rc.1"},
		{name: "prerelease alnum ok", in: "v1.2.3-beta"},
		{name: "prerelease numeric zero ok", in: "v1.2.3-0"},
		{name: "incompatible ok", in: "v2.0.0+incompatible"},
		{name: "incompatible prerelease ok", in: "v3.1.0-rc.1+incompatible"},
		{name: "pseudo version ok", in: "v0.0.0-20240101000000-000000000000"},
		{name: "pseudo version after tag ok", in: "v1.2.4-0.20240102150405-abcdef123456"},
		{name: "pseudo version after prerelease ok", in: "v1.2.3-rc.1.0.20240102150405-abcdef123456"},
		{name: "pseudo version incompatible ok", in: "v2.0.1-0.20240102150405-abcdef123456+incompatible"},

		// Rejected: empty, keywords, and placeholders.
		{name: "empty rejected", in: "", wantErr: true},
		{name: "whitespace rejected", in: "   ", wantErr: true},
		{name: "latest keyword rejected", in: string(UpdateChannelLatest), wantErr: true},
		{name: "main keyword rejected", in: string(UpdateChannelMain), wantErr: true},
		{name: "master keyword rejected", in: string(UpdateChannelMaster), wantErr: true},
		{name: "pinned keyword rejected", in: string(UpdateChannelPinned), wantErr: true},
		{name: "devel rejected", in: develVersionParen, wantErr: true},
		{name: "unknown rejected", in: unknown, wantErr: true},

		// Rejected: references the go command resolves at install time.
		{name: "branch rejected", in: cvBranch, wantErr: true},
		{name: "branch with slash rejected", in: "feature/pin", wantErr: true},
		{name: "short hash rejected", in: cvShortHash, wantErr: true},
		{name: "full hash rejected", in: "0123456789abcdef0123456789abcdef01234567", wantErr: true},
		{name: "major only rejected", in: "v1", wantErr: true},
		{name: "major minor rejected", in: cvMajorMinor, wantErr: true},
		{name: "abbreviated incompatible rejected", in: "v2+incompatible", wantErr: true},
		{name: "query rejected", in: cvQuery, wantErr: true},
		{name: "upgrade query rejected", in: "upgrade", wantErr: true},
		{name: "none query rejected", in: "none", wantErr: true},

		// Rejected: malformed or non-canonical versions.
		{name: "missing v rejected", in: "1.2.3", wantErr: true},
		{name: "uppercase V rejected", in: "V1.2.3", wantErr: true},
		{name: "leading zero rejected", in: "v01.2.3", wantErr: true},
		{name: "four components rejected", in: "v1.2.3.4", wantErr: true},
		{name: "prerelease leading zero rejected", in: "v1.2.3-01", wantErr: true},
		{name: "empty prerelease rejected", in: "v1.2.3-", wantErr: true},
		{name: "inner space rejected", in: "v1.2.3 rc", wantErr: true},
		{name: "wildcard rejected", in: "v1.2.x", wantErr: true},
		{name: "build metadata rejected", in: "v1.2.3+meta", wantErr: true},
		{name: "incompatible v1 rejected", in: "v1.2.3+incompatible", wantErr: true},
		{name: "incompatible v0 rejected", in: "v0.1.0+incompatible", wantErr: true},
		{name: "pseudo bad month rejected", in: "v0.0.0-20241301000000-abcdef123456", wantErr: true},
		{name: "pseudo short rev rejected", in: "v0.0.0-20240102150405-abcdef1", wantErr: true},
		{name: "pseudo uppercase rev rejected", in: "v0.0.0-20240102150405-ABCDEF123456", wantErr: true},
		{name: "pseudo without base but incompatible rejected", in: "v2.0.0-20240102150405-abcdef123456+incompatible", wantErr: true},
		{name: "pseudo negative patch rejected", in: "v1.0.0-0.20240102150405-abcdef123456", wantErr: true},
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

// TestValidatePinnedVersionRejectsMovingReferences is the regression test for
// pins that name something the go command resolves at install time rather than
// a fixed version: a branch, a commit hash, an abbreviated version, or a
// version query. Each of these was once accepted and persisted, so a later
// 'gup update' could install different code under the same "pin".
func TestValidatePinnedVersionRejectsMovingReferences(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		cvBranch,    // branch name
		"develop",   // branch name
		cvShortHash, // short commit hash
		"0123456789abcdef0123456789abcdef01234567", // full commit hash
		"v1",         // major-only query: resolves to the newest v1.x.y
		cvMajorMinor, // minor-only query: resolves to the newest v1.2.x
		cvQuery,      // version query
		"<v2",        // version query
		"upgrade",    // version query keyword
		"patch",      // version query keyword
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if err := ValidatePinnedVersion(in); err == nil {
				t.Errorf("ValidatePinnedVersion(%q) = nil, want an error: it does not name a fixed version", in)
			}
		})
	}
}

// TestValidatePinnedVersionErrorGuidesTheFix checks that a rejection names the
// offending value, says why it is not a fixed version, and tells the user which
// forms are accepted - including the pseudo-version to use for a commit.
func TestValidatePinnedVersionErrorGuidesTheFix(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in     string
		reason string
	}{
		{in: cvBranch, reason: "branch names"},
		{in: cvShortHash, reason: "commit hash"},
		{in: "v1", reason: "abbreviated version"},
		{in: cvMajorMinor, reason: "abbreviated version"},
		{in: cvQuery, reason: "version query"},
		{in: "v1.2.3+incompatible", reason: "only v2 and later"},
		{in: string(UpdateChannelLatest), reason: "channel keyword"},
	} {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			err := ValidatePinnedVersion(tt.in)
			if err == nil {
				t.Fatalf("ValidatePinnedVersion(%q) = nil, want an error", tt.in)
			}
			msg := err.Error()
			for _, want := range []string{strconv.Quote(tt.in), tt.reason, "full version such as v1.2.3", "pseudo-version"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not contain %q", msg, want)
				}
			}
		})
	}
}
