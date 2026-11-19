package cmd

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

const (
	floorTestImport = "github.com/example/floortool"
	floorTestName   = "floortool"
	// floorGoLocal is the local go command; floorGoOld and floorGoNew are Go
	// versions newer than it that an installed binary may have been built with.
	floorGoLocal = "go1.26.4"
	floorGoOld   = "go1.26.6"
	floorGoNew   = "go1.26.8"
)

// floorTestPkg is a binary at v1.0.0 built with goBefore, while the local go
// command is go1.26.4.
func floorTestPkg(goBefore string) goutil.Package {
	return goutil.Package{
		Name:       floorTestName,
		ImportPath: floorTestImport,
		ModulePath: floorTestImport,
		Version:    &goutil.Version{Current: testVersionOne},
		GoVersion:  &goutil.Version{Current: goBefore, Latest: floorGoLocal},
	}
}

// runFloorUpdate updates pkg to v9.9.9 on @latest. The fake install records the
// toolchain floor it received and the rebuilt binary reports goAfter.
func runFloorUpdate(t *testing.T, pkg goutil.Package, goAfter string, jsonOut bool) (floor, out string, results goutil.Package) {
	t.Helper()
	var mu sync.Mutex
	deps := stubUpdateDeps()
	deps.installLatest = func(ctx context.Context, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		floor = goutil.MinGoToolchain(ctx)
		return nil
	}
	deps.installedGoVersion = func(string) (string, error) { return goAfter, nil }

	var succeeded []goutil.Package
	out = captureCheckOutput(t, func(p *print.Printer) int {
		var code int
		code, succeeded, _ = updateWithChannels(deps, p, []goutil.Package{pkg}, false, false, 1, false,
			map[string]goutil.UpdateChannel{pkg.Name: goutil.UpdateChannelLatest}, nil, 0, jsonOut, false)
		if code != 0 {
			t.Fatalf("updateWithChannels() = %d, want 0", code)
		}
		return code
	})
	if len(succeeded) != 1 {
		t.Fatalf("want one updated package, got %d (output: %s)", len(succeeded), out)
	}
	return floor, out, succeeded[0]
}

func TestUpdate_asksForAtLeastThePreviousGo(t *testing.T) {
	t.Parallel()
	floor, _, _ := runFloorUpdate(t, floorTestPkg(floorGoOld), floorGoOld, false)
	if floor != floorGoOld {
		t.Errorf("install got toolchain floor %q, want the Go the binary was built with (go1.26.6)", floor)
	}
}

func TestUpdate_reportsTheGoActuallyUsed(t *testing.T) {
	t.Parallel()
	// The go command switched to go1.26.8 because the module requires it, so the
	// line must not claim the binary went down to the local go1.26.4.
	_, out, pkg := runFloorUpdate(t, floorTestPkg(floorGoNew), floorGoNew, false)
	if pkg.GoVersion.Latest != floorGoNew {
		t.Errorf("installed Go = %q, want go1.26.8", pkg.GoVersion.Latest)
	}
	if strings.Contains(out, floorGoLocal) {
		t.Errorf("output must not mention the local go1.26.4 the binary was not built with:\n%s", out)
	}
	if strings.Contains(out, "WARN") {
		t.Errorf("no warning is due when the Go did not go down:\n%s", out)
	}
}

func TestUpdate_jsonReportsTheGoActuallyUsed(t *testing.T) {
	t.Parallel()
	_, out, _ := runFloorUpdate(t, floorTestPkg(floorGoNew), floorGoNew, true)
	if !strings.Contains(out, `"installed_go_version": "`+floorGoNew+`"`) {
		t.Errorf("installed_go_version must be the Go the binary was built with:\n%s", out)
	}
}

func TestUpdate_warnsWhenGoWentDown(t *testing.T) {
	t.Parallel()
	// GOTOOLCHAIN=local kept the go command from downloading go1.26.6.
	_, out, _ := runFloorUpdate(t, floorTestPkg(floorGoOld), floorGoLocal, false)
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "rebuilt with go1.26.4, older than the go1.26.6") {
		t.Errorf("want a warning that the Go went down:\n%s", out)
	}
	if !strings.Contains(out, "GOTOOLCHAIN=auto") {
		t.Errorf("the warning must say how to avoid it:\n%s", out)
	}
}

func TestUpdate_fallsBackToLocalGoWhenBinaryUnreadable(t *testing.T) {
	t.Parallel()
	deps := stubUpdateDeps()
	deps.installedGoVersion = func(string) (string, error) { return "", errors.New("unreadable") }
	var succeeded []goutil.Package
	captureCheckOutput(t, func(p *print.Printer) int {
		code, s, _ := updateWithChannels(deps, p, []goutil.Package{floorTestPkg("go1.26.2")}, false, false, 1, false,
			map[string]goutil.UpdateChannel{floorTestName: goutil.UpdateChannelLatest}, nil, 0, false, false)
		succeeded = s
		return code
	})
	if len(succeeded) != 1 || succeeded[0].GoVersion.Latest != floorGoLocal {
		t.Errorf("want the local go1.26.4 as a fallback, got %+v", succeeded)
	}
}

func TestUpdatePinned_recordsTheGoActuallyUsed(t *testing.T) {
	t.Parallel()
	var floor string
	deps := testDeps()
	deps.installByVersion = func(ctx context.Context, _, _ string) error {
		floor = goutil.MinGoToolchain(ctx)
		return nil
	}
	deps.installedGoVersion = func(string) (string, error) { return floorGoNew, nil }

	p := pinnedTestPkg("v1.1.0", testVersionOne)
	p.GoVersion = &goutil.Version{Current: floorGoNew, Latest: floorGoLocal}
	var succeeded []goutil.Package
	out := captureCheckOutput(t, func(pr *print.Printer) int {
		code, s, _ := updateWithChannels(deps, pr, []goutil.Package{p}, false, false, 1, false,
			map[string]goutil.UpdateChannel{p.Name: goutil.UpdateChannelPinned},
			map[string]string{p.Name: testVersionOne}, 0, false, false)
		succeeded = s
		return code
	})
	if floor != floorGoNew {
		t.Errorf("pinned install got toolchain floor %q, want go1.26.8", floor)
	}
	if len(succeeded) != 1 || succeeded[0].GoVersion.Current != floorGoNew {
		t.Errorf("pinned Go must be the Go the binary was built with (go1.26.8), got %+v", succeeded)
	}
	if strings.Contains(out, floorGoLocal) || strings.Contains(out, "WARN") {
		t.Errorf("pinned line must not claim a Go downgrade:\n%s", out)
	}
}

func TestUpdatePinned_warnsWhenGoWentDown(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	deps.installedGoVersion = func(string) (string, error) { return floorGoLocal, nil }

	p := pinnedTestPkg("v1.1.0", testVersionOne)
	p.GoVersion = &goutil.Version{Current: floorGoOld, Latest: floorGoLocal}
	out := captureCheckOutput(t, func(pr *print.Printer) int {
		code, _, _ := updateWithChannels(deps, pr, []goutil.Package{p}, false, false, 1, false,
			map[string]goutil.UpdateChannel{p.Name: goutil.UpdateChannelPinned},
			map[string]string{p.Name: testVersionOne}, 0, false, false)
		return code
	})
	if !strings.Contains(out, "rebuilt with go1.26.4, older than the go1.26.6") {
		t.Errorf("want a warning that the Go went down:\n%s", out)
	}
}

func Test_migratePackages_asksForAtLeastThePreviousGo(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })
	var floor string
	installByVersionMigrateCtx = func(ctx context.Context, _, _ string) error {
		floor = goutil.MinGoToolchain(ctx)
		return nil
	}

	pkgs := []goutil.Package{{
		Name:       testBinTool,
		ImportPath: testImportPathTool,
		Version:    &goutil.Version{Current: testVersion123},
		GoVersion:  &goutil.Version{Current: floorGoOld, Latest: "unknown"},
	}}
	captureMigrateOutput(t, func(p *print.Printer) {
		if got := migratePackages(p, pkgs, after, false, false, 1, false, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})
	if floor != floorGoOld {
		t.Errorf("migrate install got toolchain floor %q, want go1.26.6", floor)
	}
}

func Test_migratePackages_warnsWhenGoWentDown(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	origInstall, origGo := installByVersionMigrateCtx, installedGoVersionMigrate
	t.Cleanup(func() { installByVersionMigrateCtx, installedGoVersionMigrate = origInstall, origGo })
	installByVersionMigrateCtx = func(context.Context, string, string) error { return nil }
	installedGoVersionMigrate = func(string) (string, error) { return floorGoLocal, nil }

	pkgs := []goutil.Package{{
		Name:       testBinTool,
		ImportPath: testImportPathTool,
		Version:    &goutil.Version{Current: testVersion123},
		GoVersion:  &goutil.Version{Current: floorGoOld, Latest: "unknown"},
	}}
	out := captureMigrateOutput(t, func(p *print.Printer) {
		if got := migratePackages(p, pkgs, after, false, false, 1, false, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})
	if !strings.Contains(out, "rebuilt with "+floorGoLocal+", older than the "+floorGoOld) {
		t.Errorf("want a warning that the Go went down:\n%s", out)
	}
}
