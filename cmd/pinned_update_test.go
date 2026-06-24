package cmd

import (
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
)

const pinnedTestImport = "example.com/tool"

// pinnedTestPkg builds an installed package for "example.com/tool" with a
// resolved pin (channel + PinnedVersion), as configstate would hand to
// updateWithChannels.
func pinnedTestPkg(installed, pinned string) goutil.Package {
	name := pinnedTestImport
	if i := strings.LastIndex(pinnedTestImport, "/"); i >= 0 {
		name = pinnedTestImport[i+1:]
	}
	return goutil.Package{
		Name:          name,
		ImportPath:    pinnedTestImport,
		ModulePath:    pinnedTestImport,
		Version:       &goutil.Version{Current: installed},
		GoVersion:     &goutil.Version{Current: "go1.22.4", Latest: "go1.22.4"},
		UpdateChannel: goutil.UpdateChannelPinned,
		PinnedVersion: pinned,
	}
}

//nolint:paralleltest // swaps package-level install stubs; must not run in parallel
func TestUpdatePinned_reinstallsAtPinWhenDifferent(t *testing.T) {
	var gotImport, gotVersion string
	called := false
	installByVersionUpd = func(importPath, version string) error {
		called = true
		gotImport, gotVersion = importPath, version
		return nil
	}
	getLatestVer = func(string) (string, error) {
		t.Fatal("pinned update must not resolve @latest")
		return "", nil
	}
	installLatest = func(string) error { t.Fatal("pinned update must not install @latest"); return nil }
	defer func() {
		installByVersionUpd = goutil.Install
		getLatestVer = goutil.GetLatestVer
		installLatest = goutil.InstallLatest
	}()

	p := pinnedTestPkg("v1.1.0", "v1.0.0")
	res := updatePinned(t.Context(), p)
	if !called {
		t.Fatal("expected go install <path>@<pinned> to be called")
	}
	if gotImport != "example.com/tool" || gotVersion != "v1.0.0" {
		t.Errorf("installed %s@%s, want example.com/tool@v1.0.0", gotImport, gotVersion)
	}
	if !res.updated || res.status != statusUpdated {
		t.Errorf("result updated=%v status=%q, want updated/updated", res.updated, res.status)
	}
}

//nolint:paralleltest // swaps package-level install stubs; must not run in parallel
func TestUpdatePinned_skipsWhenAlreadyAtPin(t *testing.T) {
	installByVersionUpd = func(string, string) error {
		t.Fatal("must not reinstall when already at the pinned version")
		return nil
	}
	defer func() { installByVersionUpd = goutil.Install }()

	p := pinnedTestPkg("v1.0.0", "v1.0.0")
	res := updatePinned(t.Context(), p)
	if res.updated {
		t.Error("updated = true, want false when already at the pin")
	}
	if res.status != statusPinned {
		t.Errorf("status = %q, want pinned", res.status)
	}
	if !res.skipped {
		t.Error("skipped = false, want true")
	}
}

//nolint:paralleltest // swaps package-level install stubs; must not run in parallel
func TestUpdatePinned_emptyPinIsError(t *testing.T) {
	installByVersionUpd = func(string, string) error {
		t.Fatal("must not install when the pin target is missing")
		return nil
	}
	defer func() { installByVersionUpd = goutil.Install }()

	p := pinnedTestPkg("v1.0.0", "")
	res := updatePinned(t.Context(), p)
	if res.err == nil {
		t.Fatal("expected an error for a pin with no recorded version, never a silent @latest")
	}
}

func TestCheckPinned_status(t *testing.T) {
	t.Parallel()
	match := checkPinned(pinnedTestPkg("v1.0.0", "v1.0.0"))
	if match.status != statusPinned {
		t.Errorf("matching pin status = %q, want pinned", match.status)
	}
	mismatch := checkPinned(pinnedTestPkg("v1.1.0", "v1.0.0"))
	if mismatch.status != statusPinMismatch {
		t.Errorf("differing pin status = %q, want pin-mismatch", mismatch.status)
	}
}

func TestPinnedJSONRecord(t *testing.T) {
	t.Parallel()
	p := pinnedTestPkg("v1.1.0", "v1.0.0")
	rec := newJSONPackage(p, statusPinMismatch, nil)
	if rec.Channel != "pinned" {
		t.Errorf("channel = %q, want pinned", rec.Channel)
	}
	if rec.PinnedVersion != "v1.0.0" {
		t.Errorf("pinned_version = %q, want v1.0.0", rec.PinnedVersion)
	}
	if rec.CurrentVersion != "v1.1.0" {
		t.Errorf("current_version = %q, want v1.1.0", rec.CurrentVersion)
	}
	// gup never queries @latest for a pin, so latest_version must stay empty.
	if rec.LatestVersion != "" {
		t.Errorf("latest_version = %q, want empty for a pinned package", rec.LatestVersion)
	}
	if rec.Status != statusPinMismatch {
		t.Errorf("status = %q, want pin-mismatch", rec.Status)
	}
}
