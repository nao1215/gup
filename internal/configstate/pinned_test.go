package configstate

import (
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
)

const (
	pinTestImport = "example.com/tool"
	pinTestOther  = "other"
	pinTestV110   = "v1.1.0"
)

// pinnedConf returns the saved-config entry for the shared test tool pinned to
// testVer100.
func pinnedConf() goutil.Package {
	name := pinTestImport
	if i := strings.LastIndex(pinTestImport, "/"); i >= 0 {
		name = pinTestImport[i+1:]
	}
	return goutil.Package{
		Name:          name,
		ImportPath:    pinTestImport,
		Version:       &goutil.Version{Current: testVer100},
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: testVer100,
	}
}

func TestApplySavedChannels_setsPinnedVersion(t *testing.T) {
	t.Parallel()
	conf := []goutil.Package{pinnedConf()}
	installed := []goutil.Package{{Name: testToolName, ImportPath: pinTestImport, Version: &goutil.Version{Current: pinTestV110}}}

	got := ApplySavedChannels(installed, conf)
	if got[0].UpdateChannel != goutil.UpdateChannelPinned {
		t.Errorf("channel = %q, want pinned", got[0].UpdateChannel)
	}
	if got[0].PinnedVersion != testVer100 {
		t.Errorf("PinnedVersion = %q, want v1.0.0", got[0].PinnedVersion)
	}
	// The installed (current) version must stay untouched so a pin can downgrade.
	if got[0].Version.Current != pinTestV110 {
		t.Errorf("Version.Current = %q, want v1.1.0 (installed, not pin target)", got[0].Version.Current)
	}
}

func TestResolveChannels_returnsPinnedMap(t *testing.T) {
	t.Parallel()
	conf := []goutil.Package{pinnedConf()}
	installed := []goutil.Package{{Name: testToolName, ImportPath: pinTestImport, Version: &goutil.Version{Current: pinTestV110}}}

	channelMap, pinnedMap, err := ResolveChannels(installed, conf, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ResolveChannels() error: %v", err)
	}
	if channelMap[testToolName] != goutil.UpdateChannelPinned {
		t.Errorf("channel = %q, want pinned", channelMap[testToolName])
	}
	if pinnedMap[testToolName] != testVer100 {
		t.Errorf("pinnedMap[tool] = %q, want v1.0.0", pinnedMap[testToolName])
	}
}

func TestResolveChannels_latestFlagClearsPin(t *testing.T) {
	t.Parallel()
	conf := []goutil.Package{pinnedConf()}
	installed := []goutil.Package{{Name: testToolName, ImportPath: pinTestImport, Version: &goutil.Version{Current: pinTestV110}}}

	channelMap, pinnedMap, err := ResolveChannels(installed, conf, nil, nil, []string{testToolName}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveChannels() error: %v", err)
	}
	if channelMap[testToolName] != goutil.UpdateChannelLatest {
		t.Errorf("--latest should override pin: channel = %q, want latest", channelMap[testToolName])
	}
	if _, ok := pinnedMap[testToolName]; ok {
		t.Errorf("pin target must be dropped when --latest unpins the package, got %q", pinnedMap[testToolName])
	}
}

func TestMergePackages_preservesPin(t *testing.T) {
	t.Parallel()
	conf := []goutil.Package{pinnedConf()}
	// A bystander package updates (forcing a config write) while "tool" stays pinned.
	succeeded := []goutil.Package{
		{Name: testToolName, ImportPath: pinTestImport, Version: &goutil.Version{Current: testVer100, Latest: testVer100}, UpdateChannel: goutil.UpdateChannelPinned, PinnedVersion: testVer100},
		{Name: pinTestOther, ImportPath: "example.com/" + pinTestOther, Version: &goutil.Version{Current: testVer200, Latest: testVer200}, UpdateChannel: goutil.UpdateChannelLatest},
	}
	channelMap := map[string]goutil.UpdateChannel{testToolName: goutil.UpdateChannelPinned, pinTestOther: goutil.UpdateChannelLatest}

	merged := MergePackages(conf, succeeded, channelMap, nil)
	var tool *goutil.Package
	for i := range merged {
		if merged[i].Name == testToolName {
			tool = &merged[i]
		}
	}
	if tool == nil {
		t.Fatal("tool missing from merged result")
	}
	if tool.UpdateChannel != goutil.UpdateChannelPinned {
		t.Errorf("merged tool channel = %q, want pinned", tool.UpdateChannel)
	}
	if tool.PinnedVersion != testVer100 || tool.Version.Current != testVer100 {
		t.Errorf("merged pin lost: PinnedVersion=%q Version.Current=%q, want v1.0.0", tool.PinnedVersion, tool.Version.Current)
	}
}

func TestPersistedVersion_pinnedUsesPinTarget(t *testing.T) {
	t.Parallel()
	// Even with a different installed/current version, a pinned package persists
	// the pin target, never the installed version.
	p := goutil.Package{
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: testVer100,
		Version:       &goutil.Version{Current: pinTestV110, Latest: pinTestV110},
	}
	if got := PersistedVersion(p); got != testVer100 {
		t.Errorf("PersistedVersion(pinned) = %q, want v1.0.0", got)
	}
}

func TestSetPin_addsAndReplaces(t *testing.T) {
	t.Parallel()
	conf := []goutil.Package{
		{Name: pinTestOther, ImportPath: "example.com/" + pinTestOther, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelLatest},
	}
	target := goutil.Package{Name: testToolName, ImportPath: pinTestImport}

	got, err := SetPin(conf, target, "v1.2.3")
	if err != nil {
		t.Fatalf("SetPin() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	var tool *goutil.Package
	for i := range got {
		if got[i].Name == testToolName {
			tool = &got[i]
		}
	}
	if tool == nil || tool.UpdateChannel != goutil.UpdateChannelPinned || tool.PinnedVersion != "v1.2.3" {
		t.Fatalf("SetPin did not pin tool: %+v", tool)
	}

	// Re-pinning replaces in place (no duplicate).
	got2, err := SetPin(got, target, testVer200)
	if err != nil {
		t.Fatalf("SetPin() re-pin error: %v", err)
	}
	count := 0
	for _, p := range got2 {
		if p.Name == testToolName {
			count++
			if p.PinnedVersion != testVer200 {
				t.Errorf("re-pin version = %q, want v2.0.0", p.PinnedVersion)
			}
		}
	}
	if count != 1 {
		t.Errorf("re-pin produced %d tool entries, want 1", count)
	}
}

func TestSetPin_rejectsBadVersion(t *testing.T) {
	t.Parallel()
	target := goutil.Package{Name: testToolName, ImportPath: pinTestImport}
	for _, v := range []string{"", "latest", "main", "master"} {
		if _, err := SetPin(nil, target, v); err == nil {
			t.Errorf("SetPin(version=%q) expected error, got nil", v)
		}
	}
}

func TestRemovePin(t *testing.T) {
	t.Parallel()
	conf := []goutil.Package{pinnedConf()}

	got, changed := RemovePin(conf, goutil.Package{Name: testToolName})
	if !changed {
		t.Fatal("RemovePin should report changed=true for a pinned package")
	}
	if got[0].UpdateChannel != goutil.UpdateChannelLatest || got[0].PinnedVersion != "" {
		t.Errorf("after unpin channel=%q pinned=%q, want latest and empty", got[0].UpdateChannel, got[0].PinnedVersion)
	}

	// Idempotent: unpinning a non-pinned package changes nothing.
	_, changed2 := RemovePin(got, goutil.Package{Name: testToolName})
	if changed2 {
		t.Error("RemovePin on a non-pinned package should report changed=false")
	}

	// Unknown target is a no-op.
	_, changed3 := RemovePin(conf, goutil.Package{Name: "missing"})
	if changed3 {
		t.Error("RemovePin on an unknown target should report changed=false")
	}
}
