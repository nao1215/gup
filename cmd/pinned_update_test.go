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

	p := pinnedTestPkg("v1.1.0", testVersionOne)
	res := updatePinned(defaultDependencies(), t.Context(), p, false)
	if !called {
		t.Fatal("expected go install <path>@<pinned> to be called")
	}
	if gotImport != pinnedTestImport || gotVersion != testVersionOne {
		t.Errorf("installed %s@%s, want %s@%s", gotImport, gotVersion, pinnedTestImport, testVersionOne)
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

	p := pinnedTestPkg(testVersionOne, testVersionOne)
	res := updatePinned(defaultDependencies(), t.Context(), p, false)
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
	res := updatePinned(defaultDependencies(), t.Context(), p, false)
	if res.err == nil {
		t.Fatal("expected an error for a pin with no recorded version, never a silent @latest")
	}
}

func TestCheckPinned_status(t *testing.T) {
	t.Parallel()
	match := checkPinned(pinnedTestPkg(testVersionOne, testVersionOne), false)
	if match.status != statusPinned {
		t.Errorf("matching pin status = %q, want pinned", match.status)
	}
	mismatch := checkPinned(pinnedTestPkg("v1.1.0", testVersionOne), false)
	if mismatch.status != statusPinMismatch {
		t.Errorf("differing pin status = %q, want pin-mismatch", mismatch.status)
	}
}

// pinnedStaleGo builds a pinned package whose installed version matches the pin
// (so PinSatisfied is true) but which was built with an older Go toolchain than
// the current one, so only a Go-toolchain rebuild is pending.
func pinnedStaleGo() goutil.Package {
	p := pinnedTestPkg(testVersionOne, testVersionOne)
	p.GoVersion = &goutil.Version{Current: "go1.22.0", Latest: "go1.23.0"}
	return p
}

func TestCheckPinned_goToolchainOutdated(t *testing.T) {
	t.Parallel()
	// Version matches the pin but the binary was built with an older Go: the pin
	// locks the module version, not the Go build, so this is a pending rebuild.
	outdated := checkPinned(pinnedStaleGo(), false)
	if outdated.status != statusPinMismatch {
		t.Errorf("go-outdated pin status = %q, want pin-mismatch", outdated.status)
	}
	// --ignore-go-update keeps it pinned despite the Go delta.
	ignored := checkPinned(pinnedStaleGo(), true)
	if ignored.status != statusPinned {
		t.Errorf("ignore-go-update pin status = %q, want pinned", ignored.status)
	}
}

//nolint:paralleltest // swaps package-level install stubs; must not run in parallel
func TestUpdatePinned_reinstallsWhenGoOutdated(t *testing.T) {
	var gotVersion string
	called := false
	installByVersionUpd = func(_ string, version string) error {
		called = true
		gotVersion = version
		return nil
	}
	defer func() { installByVersionUpd = goutil.Install }()

	// Version already matches the pin, only the Go toolchain is newer.
	res := updatePinned(defaultDependencies(), t.Context(), pinnedStaleGo(), false)
	if !called {
		t.Fatal("expected a reinstall at the pinned version when the Go toolchain is newer")
	}
	if gotVersion != testVersionOne {
		t.Errorf("reinstalled at %q, want the pinned v1.0.0 (never @latest)", gotVersion)
	}
	if !res.updated || res.status != statusUpdated {
		t.Errorf("result updated=%v status=%q, want updated/updated", res.updated, res.status)
	}
}

//nolint:paralleltest // swaps package-level install stubs; must not run in parallel
func TestUpdatePinned_ignoreGoUpdateKeepsPin(t *testing.T) {
	installByVersionUpd = func(string, string) error {
		t.Fatal("must not reinstall a satisfied pin when Go updates are ignored")
		return nil
	}
	defer func() { installByVersionUpd = goutil.Install }()

	res := updatePinned(defaultDependencies(), t.Context(), pinnedStaleGo(), true)
	if res.updated || res.status != statusPinned {
		t.Errorf("result updated=%v status=%q, want kept/pinned", res.updated, res.status)
	}
}

func TestPinnedJSONRecord(t *testing.T) {
	t.Parallel()
	p := pinnedTestPkg("v1.1.0", testVersionOne)
	rec := newJSONPackage(p, statusPinMismatch, nil)
	if rec.Channel != "pinned" {
		t.Errorf("channel = %q, want pinned", rec.Channel)
	}
	if rec.PinnedVersion != testVersionOne {
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
