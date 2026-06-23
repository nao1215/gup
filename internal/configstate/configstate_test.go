package configstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
)

const (
	testImportPathPosixer = "github.com/nao1215/posixer"
	validConf             = `{"schema_version":1,"packages":[{"name":"posixer","import_path":"` + testImportPathPosixer + `","version":"v0.1.0","channel":"main"}]}` + "\n"

	testPosixer  = "posixer"
	testToolA    = "tool-a"
	testToolB    = "tool-b"
	testToolC    = "tool-c"
	testKeptTool = "kept-tool"
	testNewTool  = "new-tool"
	testVer100   = "v1.0.0"
	testVer200   = "v2.0.0"

	testOldName  = "old-name"
	testNewName  = "new-name"
	testFooPath  = "example.com/foo/cmd/foo"
	testToolName = "tool"
	testToolPath = "example.com/tool/cmd/tool"

	testFoo       = "foo"
	testBar       = "bar"
	testStalePath = "example.com/stale/foo"
	testFreshPath = "example.com/fresh/bar"

	testOldFooPath = "example.com/old/foo"
	testNewFooPath = "example.com/new/foo"
)

// setupConfigHome points xdg.ConfigHome at a fresh temp dir so config.FilePath()
// and config.DirPath() resolve under the test sandbox.
func setupConfigHome(t *testing.T) {
	t.Helper()
	orig := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = orig })
	xdg.ConfigHome = t.TempDir()
}

func TestReadFileIfExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(valid, []byte(validConf), 0o600); err != nil {
		t.Fatal(err)
	}
	malformed := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(malformed, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	t.Run("missing path yields empty slice", func(t *testing.T) {
		t.Parallel()
		got, err := ReadFileIfExists(filepath.Join(dir, "missing.json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d packages, want 0", len(got))
		}
	})

	t.Run("directory is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := ReadFileIfExists(sub)
		if err == nil || !strings.Contains(err.Error(), "is a directory") {
			t.Fatalf("err = %v, want 'is a directory'", err)
		}
	})

	t.Run("valid file parsed with channel", func(t *testing.T) {
		t.Parallel()
		got, err := ReadFileIfExists(valid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].UpdateChannel != goutil.UpdateChannelMain {
			t.Fatalf("got %+v, want one package on @main", got)
		}
	})

	t.Run("malformed file fails fast", func(t *testing.T) {
		t.Parallel()
		if _, err := ReadFileIfExists(malformed); err == nil {
			t.Fatal("expected error for malformed config")
		}
	})
}

func TestValidateExplicitFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(valid, []byte(validConf), 0o600); err != nil {
		t.Fatal(err)
	}
	malformed := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(malformed, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		confFile string
		wantErr  bool
	}{
		{name: "empty is no-op", confFile: "", wantErr: false},
		{name: "missing path is no config", confFile: filepath.Join(dir, "missing.json"), wantErr: false},
		{name: "valid passes", confFile: valid, wantErr: false},
		{name: "malformed fails", confFile: malformed, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateExplicitFile(tt.confFile); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateExplicitFile(%q) err = %v, wantErr %v", tt.confFile, err, tt.wantErr)
			}
		})
	}
}

//nolint:paralleltest // mutates xdg.ConfigHome via setupConfigHome
func TestResolveWritePath(t *testing.T) {
	setupConfigHome(t)

	existing := filepath.Join(t.TempDir(), "existing.json")
	if err := os.WriteFile(existing, []byte(validConf), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.json")

	tests := []struct {
		name         string
		confFile     string
		confReadPath string
		want         string
	}{
		{name: "explicit missing path honored", confFile: missing, confReadPath: missing, want: missing},
		{name: "explicit existing path honored", confFile: existing, confReadPath: existing, want: existing},
		{name: "no flag reuses existing auto-detected", confFile: "", confReadPath: existing, want: existing},
		{name: "no flag, none existing, falls back to user config", confFile: "", confReadPath: missing, want: config.FilePath()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveWritePath(tt.confFile, tt.confReadPath); got != tt.want {
				t.Errorf("ResolveWritePath(%q, %q) = %q, want %q", tt.confFile, tt.confReadPath, got, tt.want)
			}
		})
	}
}

func TestShouldPersistChannels(t *testing.T) {
	t.Parallel()
	if ShouldPersistChannels(nil, nil, nil) {
		t.Error("all empty should return false")
	}
	if !ShouldPersistChannels([]string{"a"}, nil, nil) {
		t.Error("main non-empty should return true")
	}
	if !ShouldPersistChannels(nil, []string{"b"}, nil) {
		t.Error("master non-empty should return true")
	}
	if !ShouldPersistChannels(nil, nil, []string{"c"}) {
		t.Error("latest non-empty should return true")
	}
}

func TestApplySavedChannels(t *testing.T) {
	t.Parallel()

	t.Run("prefers import path", func(t *testing.T) {
		t.Parallel()
		confPkgs := []goutil.Package{{Name: "old", ImportPath: "example.com/foo/cmd/foo", UpdateChannel: goutil.UpdateChannelMain}}
		pkgs := []goutil.Package{{Name: "new", ImportPath: "example.com/foo/cmd/foo"}}
		got := ApplySavedChannels(pkgs, confPkgs)
		if len(got) != 1 || got[0].UpdateChannel != goutil.UpdateChannelMain {
			t.Fatalf("channel should match by import_path; got %+v", got)
		}
	})

	t.Run("name fallback ignores exe suffix and case", func(t *testing.T) {
		t.Parallel()
		confPkgs := []goutil.Package{{Name: " foo.EXE ", UpdateChannel: goutil.UpdateChannelMaster}}
		pkgs := []goutil.Package{{Name: "foo", ImportPath: "example.com/foo"}}
		got := ApplySavedChannels(pkgs, confPkgs)
		if len(got) != 1 || got[0].UpdateChannel != goutil.UpdateChannelMaster {
			t.Fatalf("channel should match across .EXE difference; got %+v", got)
		}
	})

	t.Run("unmatched defaults to latest", func(t *testing.T) {
		t.Parallel()
		pkgs := []goutil.Package{{Name: "bar", ImportPath: "example.com/bar"}}
		got := ApplySavedChannels(pkgs, nil)
		if len(got) != 1 || got[0].UpdateChannel != goutil.UpdateChannelLatest {
			t.Fatalf("unmatched package should default to latest; got %+v", got)
		}
	})
}

func TestResolveChannels(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{{Name: testToolA}, {Name: testToolB}, {Name: testToolC}}

	t.Run("assigns channels from flags", func(t *testing.T) {
		t.Parallel()
		got, err := ResolveChannels(pkgs, nil, []string{testToolA}, []string{testToolB}, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[testToolA] != goutil.UpdateChannelMain {
			t.Errorf("tool-a = %q, want main", got[testToolA])
		}
		if got[testToolB] != goutil.UpdateChannelMaster {
			t.Errorf("tool-b = %q, want master", got[testToolB])
		}
		if got[testToolC] != goutil.UpdateChannelLatest {
			t.Errorf("tool-c = %q, want latest", got[testToolC])
		}
	})

	t.Run("config channel is the default below flags", func(t *testing.T) {
		t.Parallel()
		confPkgs := []goutil.Package{{Name: testToolC, UpdateChannel: goutil.UpdateChannelMain}}
		got, err := ResolveChannels(pkgs, confPkgs, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[testToolC] != goutil.UpdateChannelMain {
			t.Errorf("tool-c = %q, want main (from config)", got[testToolC])
		}
	})

	t.Run("flag overrides config channel", func(t *testing.T) {
		t.Parallel()
		confPkgs := []goutil.Package{{Name: testToolC, UpdateChannel: goutil.UpdateChannelMain}}
		got, err := ResolveChannels(pkgs, confPkgs, nil, nil, []string{testToolC}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[testToolC] != goutil.UpdateChannelLatest {
			t.Errorf("tool-c = %q, want latest (flag overrides config)", got[testToolC])
		}
	})

	t.Run("conflicting flags error", func(t *testing.T) {
		t.Parallel()
		_, err := ResolveChannels(pkgs, nil, []string{testToolA}, []string{testToolA}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "same binary") {
			t.Fatalf("err = %v, want 'same binary'", err)
		}
	})

	t.Run("unknown flag name is warned and skipped", func(t *testing.T) {
		t.Parallel()
		var warned []string
		got, err := ResolveChannels(pkgs, nil, []string{"nope"}, nil, nil, func(m string) { warned = append(warned, m) })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warned) != 1 || !strings.Contains(warned[0], "nope") {
			t.Fatalf("warn = %v, want a notice mentioning 'nope'", warned)
		}
		if _, ok := got["nope"]; ok {
			t.Fatal("unknown binary should not be added to the channel map")
		}
	})

	t.Run("blank flag name is skipped", func(t *testing.T) {
		t.Parallel()
		got, err := ResolveChannels(pkgs, nil, []string{" "}, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, p := range pkgs {
			if got[p.Name] != goutil.UpdateChannelLatest {
				t.Errorf("%s = %q, want latest", p.Name, got[p.Name])
			}
		}
	})
}

func TestPackageChannel(t *testing.T) {
	t.Parallel()
	channelMap := map[string]goutil.UpdateChannel{testToolA: goutil.UpdateChannelMain}
	if got := PackageChannel(testToolA, goutil.UpdateChannelLatest, channelMap); got != goutil.UpdateChannelMain {
		t.Errorf("found in map: got %q, want main", got)
	}
	if got := PackageChannel(testToolB, goutil.UpdateChannelMaster, channelMap); got != goutil.UpdateChannelMaster {
		t.Errorf("not in map: got %q, want master", got)
	}
}

func TestMergePackages(t *testing.T) {
	t.Parallel()

	confPkgs := []goutil.Package{
		{Name: "old-tool", ImportPath: "github.com/example/old-tool", Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelLatest},
		{Name: testKeptTool, ImportPath: "github.com/example/kept-tool", Version: &goutil.Version{Current: "v0.5.0"}, UpdateChannel: goutil.UpdateChannelLatest},
	}
	succeededPkgs := []goutil.Package{
		{Name: testNewTool, ImportPath: "github.com/example/new-tool", Version: &goutil.Version{Current: testVer100, Latest: testVer200}},
		{Name: "", ImportPath: ""}, // skipped
	}
	channelMap := map[string]goutil.UpdateChannel{
		testNewTool:  goutil.UpdateChannelMain,
		testKeptTool: goutil.UpdateChannelLatest,
	}
	renamedPkgs := map[string]string{"old-tool": testNewTool}

	got := MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
	if len(got) != 2 {
		t.Fatalf("MergePackages() returned %d packages, want 2", len(got))
	}
	if got[0].Name != testKeptTool {
		t.Errorf("first = %q, want kept-tool (sorted)", got[0].Name)
	}
	if got[1].Name != testNewTool {
		t.Errorf("second = %q, want new-tool", got[1].Name)
	}
	if got[1].UpdateChannel != goutil.UpdateChannelMain {
		t.Errorf("new-tool channel = %q, want main", got[1].UpdateChannel)
	}
	if got[1].Version.Current != testVer200 {
		t.Errorf("new-tool version = %q, want v2.0.0 (latest preferred)", got[1].Version.Current)
	}
}

func TestSanitizePackage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pkg     goutil.Package
		wantVer string
	}{
		{name: "nil version -> latest", pkg: goutil.Package{Name: "tool", ImportPath: "github.com/example/tool"}, wantVer: latestKeyword},
		{name: "blank version -> latest", pkg: goutil.Package{Name: "tool", ImportPath: "github.com/example/tool", Version: &goutil.Version{Current: "  "}}, wantVer: latestKeyword},
		{name: "valid version preserved + trimmed", pkg: goutil.Package{Name: " tool ", ImportPath: " github.com/example/tool ", Version: &goutil.Version{Current: " v1.2.3 "}}, wantVer: "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizePackage(tt.pkg)
			if got.Version.Current != tt.wantVer {
				t.Errorf("version = %q, want %q", got.Version.Current, tt.wantVer)
			}
			if got.Name != strings.TrimSpace(tt.pkg.Name) {
				t.Errorf("name = %q, want %q", got.Name, strings.TrimSpace(tt.pkg.Name))
			}
		})
	}
}

func TestPersistedVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pkg  goutil.Package
		want string
	}{
		{name: "nil version", pkg: goutil.Package{}, want: latestKeyword},
		{name: "prefers latest", pkg: goutil.Package{Version: &goutil.Version{Current: testVer100, Latest: testVer200}}, want: testVer200},
		{name: "falls back to current on unknown latest", pkg: goutil.Package{Version: &goutil.Version{Current: testVer100, Latest: "unknown"}}, want: "v1.0.0"},
		{name: "falls back to current on empty latest", pkg: goutil.Package{Version: &goutil.Version{Current: testVer100, Latest: ""}}, want: "v1.0.0"},
		{name: "both empty -> latest", pkg: goutil.Package{Version: &goutil.Version{Current: "", Latest: ""}}, want: latestKeyword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PersistedVersion(tt.pkg); got != tt.want {
				t.Errorf("PersistedVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

//nolint:paralleltest // mutates xdg.ConfigHome via setupConfigHome
func TestResolveAndApplyChannels_appliesCanonicalChannels(t *testing.T) {
	setupConfigHome(t)

	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.FilePath(), []byte(validConf), 0o600); err != nil {
		t.Fatal(err)
	}

	pkgs := []goutil.Package{
		{Name: testPosixer, ImportPath: testImportPathPosixer},
		{Name: "other", ImportPath: "github.com/example/other"},
	}
	got, err := ResolveAndApplyChannels(pkgs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].UpdateChannel != goutil.UpdateChannelMain {
		t.Errorf("posixer channel = %q, want main (from canonical config)", got[0].UpdateChannel)
	}
	if got[1].UpdateChannel != goutil.UpdateChannelLatest {
		t.Errorf("other channel = %q, want latest (no saved entry)", got[1].UpdateChannel)
	}
}

//nolint:paralleltest // mutates xdg.ConfigHome via setupConfigHome
func TestResolveAndApplyChannels_malformedConfigFailsFast(t *testing.T) {
	setupConfigHome(t)

	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.FilePath(), []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	pkgs := []goutil.Package{{Name: testPosixer, ImportPath: testImportPathPosixer}}
	if _, err := ResolveAndApplyChannels(pkgs, ""); err == nil {
		t.Fatal("expected fail-fast error on malformed canonical config")
	}
}

//nolint:paralleltest // mutates xdg.ConfigHome via setupConfigHome
func TestResolveAndApplyChannels_ambiguousConfig(t *testing.T) {
	setupConfigHome(t)

	// Seed the user-level config.
	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.FilePath(), []byte(validConf), 0o600); err != nil {
		t.Fatal(err)
	}

	// Seed ./gup.json by switching to a temp working dir.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	localDir := t.TempDir()
	if err := os.Chdir(localDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LocalFilePath(), []byte(validConf), 0o600); err != nil {
		t.Fatal(err)
	}

	pkgs := []goutil.Package{{Name: testPosixer, ImportPath: testImportPathPosixer}}
	_, err = ResolveAndApplyChannels(pkgs, "")
	if err == nil || !strings.Contains(err.Error(), "multiple gup.json candidates") {
		t.Fatalf("err = %v, want ambiguity error", err)
	}
}

// --- Package-identity consistency (one identity rule shared by
// ApplySavedChannels / ResolveChannels / MergePackages) ---

// TestResolveChannels_honorsSavedChannelByImportPath proves ResolveChannels uses
// import_path identity, so a saved channel survives a binary rename. The saved
// entry's name differs from the installed binary, but the import_path is the
// same, so the resolved channel must come from config, not @latest.
func TestResolveChannels_honorsSavedChannelByImportPath(t *testing.T) {
	t.Parallel()

	confPkgs := []goutil.Package{{Name: testOldName, ImportPath: testFooPath, UpdateChannel: goutil.UpdateChannelMain}}
	pkgs := []goutil.Package{{Name: testNewName, ImportPath: testFooPath}}

	got, err := ResolveChannels(pkgs, confPkgs, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[testNewName] != goutil.UpdateChannelMain {
		t.Errorf("channel = %q, want main (resolved by import_path despite a rename)", got[testNewName])
	}
}

// TestResolveChannels_savedChannelOverridableByFlag proves the import_path-based
// saved channel stays a default that an explicit flag can still override.
func TestResolveChannels_savedChannelOverridableByFlag(t *testing.T) {
	t.Parallel()

	confPkgs := []goutil.Package{{Name: testOldName, ImportPath: testFooPath, UpdateChannel: goutil.UpdateChannelMain}}
	pkgs := []goutil.Package{{Name: testNewName, ImportPath: testFooPath}}

	// Without flags the saved @main (matched by import_path) wins over @latest.
	base, err := ResolveChannels(pkgs, confPkgs, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base[testNewName] != goutil.UpdateChannelMain {
		t.Errorf("without flags channel = %q, want main", base[testNewName])
	}

	// --latest for the installed binary still overrides the saved channel.
	got, err := ResolveChannels(pkgs, confPkgs, nil, nil, []string{testNewName}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[testNewName] != goutil.UpdateChannelLatest {
		t.Errorf("with --latest channel = %q, want latest (flag overrides saved channel)", got[testNewName])
	}
}

// TestMergePackages_dedupesByImportPath proves a renamed-but-same-import_path
// package collapses to a single persisted entry even when no rename mapping is
// supplied (the import_path identity alone must deduplicate it).
func TestMergePackages_dedupesByImportPath(t *testing.T) {
	t.Parallel()

	confPkgs := []goutil.Package{
		{Name: testOldName, ImportPath: testFooPath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMain},
	}
	succeededPkgs := []goutil.Package{
		{Name: testNewName, ImportPath: testFooPath, Version: &goutil.Version{Current: testVer100, Latest: testVer200}},
	}
	channelMap := map[string]goutil.UpdateChannel{testNewName: goutil.UpdateChannelMain}

	got := MergePackages(confPkgs, succeededPkgs, channelMap, nil)
	if len(got) != 1 {
		t.Fatalf("MergePackages() returned %d entries, want 1 (same import_path)", len(got))
	}
	if got[0].Name != testNewName {
		t.Errorf("merged entry name = %q, want %q", got[0].Name, testNewName)
	}
}

// TestMergePackages_dedupesCrossOSNameVariant proves a saved ".exe" name variant
// and the current bare name collapse to a single logical entry.
func TestMergePackages_dedupesCrossOSNameVariant(t *testing.T) {
	t.Parallel()

	confPkgs := []goutil.Package{
		{Name: "tool.EXE", ImportPath: testToolPath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMaster},
	}
	succeededPkgs := []goutil.Package{
		{Name: testToolName, ImportPath: testToolPath, Version: &goutil.Version{Current: testVer100, Latest: testVer200}},
	}
	channelMap := map[string]goutil.UpdateChannel{testToolName: goutil.UpdateChannelMaster}

	got := MergePackages(confPkgs, succeededPkgs, channelMap, nil)
	if len(got) != 1 {
		t.Fatalf("MergePackages() returned %d entries, want 1 (cross-OS .exe variant)", len(got))
	}
	if got[0].Name != testToolName {
		t.Errorf("merged entry name = %q, want %q", got[0].Name, testToolName)
	}
}

// TestIdentity_roundTripConsistency proves the same saved package/channel is
// interpreted identically by every command path: ApplySavedChannels (export),
// ResolveChannels (update), and MergePackages (persistence). A wrong-by-name but
// right-by-import_path entry must resolve to @main everywhere and persist as one
// entry.
func TestIdentity_roundTripConsistency(t *testing.T) {
	t.Parallel()

	confPkgs := []goutil.Package{
		{Name: testOldName, ImportPath: testToolPath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMain},
	}
	installed := goutil.Package{Name: testToolName, ImportPath: testToolPath}

	// ApplySavedChannels (check / list --json / export).
	applied := ApplySavedChannels([]goutil.Package{installed}, confPkgs)
	if len(applied) != 1 || applied[0].UpdateChannel != goutil.UpdateChannelMain {
		t.Fatalf("ApplySavedChannels channel = %+v, want main", applied)
	}

	// ResolveChannels (update).
	channelMap, err := ResolveChannels([]goutil.Package{installed}, confPkgs, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channelMap[testToolName] != goutil.UpdateChannelMain {
		t.Errorf("ResolveChannels channel = %q, want main", channelMap[testToolName])
	}

	// MergePackages (persistence) using the channel map resolved above.
	succeeded := []goutil.Package{{Name: testToolName, ImportPath: testToolPath, Version: &goutil.Version{Current: testVer100, Latest: testVer200}}}
	merged := MergePackages(confPkgs, succeeded, channelMap, nil)
	if len(merged) != 1 {
		t.Fatalf("MergePackages returned %d entries, want 1 logical package", len(merged))
	}
	if merged[0].UpdateChannel != goutil.UpdateChannelMain {
		t.Errorf("MergePackages channel = %q, want main (consistent with the other paths)", merged[0].UpdateChannel)
	}
}

// --- Stale renamed-entry cleanup must use logical identity, not raw name ---

// mergeRenameInputs builds the standard rename-flow inputs for the stale-cleanup
// tests: a saved (old) entry with the given name, a fresh successful package
// under a new name/import_path, and the rename mapping keyed by oldRenamed.
func mergeRenameInputs(savedName, oldRenamed string) ([]goutil.Package, []goutil.Package, map[string]goutil.UpdateChannel, map[string]string) {
	confPkgs := []goutil.Package{
		{Name: savedName, ImportPath: testStalePath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMain},
	}
	succeededPkgs := []goutil.Package{
		{Name: testBar, ImportPath: testFreshPath, Version: &goutil.Version{Current: testVer100, Latest: testVer200}},
	}
	channelMap := map[string]goutil.UpdateChannel{testBar: goutil.UpdateChannelMain}
	renamedPkgs := map[string]string{oldRenamed: testBar}
	return confPkgs, succeededPkgs, channelMap, renamedPkgs
}

// TestMergePackages_removesStaleEntryAcrossExeVariant proves the renamed-entry
// cleanup matches the saved config by logical name identity, not raw string
// equality: a saved "foo.EXE" must be removed when renamedPkgs records the
// host-style "foo.exe" for the same logical package.
func TestMergePackages_removesStaleEntryAcrossExeVariant(t *testing.T) {
	t.Parallel()
	confPkgs, succeededPkgs, channelMap, renamedPkgs := mergeRenameInputs("foo.EXE", "foo.exe")

	got := MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
	if len(got) != 1 {
		t.Fatalf("MergePackages() returned %d entries, want 1 (stale .exe variant must be removed)", len(got))
	}
	if got[0].Name != testBar {
		t.Errorf("remaining entry = %q, want %q", got[0].Name, testBar)
	}
}

// TestMergePackages_removesStaleEntryAcrossWhitespace proves the cleanup ignores
// whitespace differences between the saved name and the renamed-from name,
// consistent with the shared cross-OS name identity.
func TestMergePackages_removesStaleEntryAcrossWhitespace(t *testing.T) {
	t.Parallel()
	confPkgs, succeededPkgs, channelMap, renamedPkgs := mergeRenameInputs(" foo ", testFoo)

	got := MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
	if len(got) != 1 {
		t.Fatalf("MergePackages() returned %d entries, want 1 (stale untrimmed entry must be removed)", len(got))
	}
	if got[0].Name != testBar {
		t.Errorf("remaining entry = %q, want %q", got[0].Name, testBar)
	}
}

// TestMergePackages_removesStaleEntryExactName is a regression guard: the normal
// exact-name rename cleanup still removes the old entry.
func TestMergePackages_removesStaleEntryExactName(t *testing.T) {
	t.Parallel()
	confPkgs, succeededPkgs, channelMap, renamedPkgs := mergeRenameInputs(testFoo, testFoo)

	got := MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
	if len(got) != 1 {
		t.Fatalf("MergePackages() returned %d entries, want 1 (exact-name cleanup regression)", len(got))
	}
	if got[0].Name != testBar {
		t.Errorf("remaining entry = %q, want %q", got[0].Name, testBar)
	}
}

// TestMergePackages_noLogicalDuplicatesAfterRename verifies that after a rename
// the merged result holds exactly one entry per logical package: the stale
// pre-rename spelling (whose name differs from the rename map only by cross-OS
// ".exe" normalization) is gone, an unrelated saved package survives, and no
// duplicate logical entries remain.
func TestMergePackages_noLogicalDuplicatesAfterRename(t *testing.T) {
	t.Parallel()
	confPkgs := []goutil.Package{
		{Name: "foo.EXE", ImportPath: testStalePath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMain},
		{Name: testKeptTool, ImportPath: "example.com/keep/kept", Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelLatest},
	}
	succeededPkgs := []goutil.Package{
		{Name: testBar, ImportPath: testFreshPath, Version: &goutil.Version{Current: testVer100, Latest: testVer200}},
	}
	channelMap := map[string]goutil.UpdateChannel{
		testBar:      goutil.UpdateChannelMain,
		testKeptTool: goutil.UpdateChannelLatest,
	}
	renamedPkgs := map[string]string{"foo.exe": testBar}

	got := MergePackages(confPkgs, succeededPkgs, channelMap, renamedPkgs)
	if len(got) != 2 {
		t.Fatalf("MergePackages() returned %d entries, want 2 (kept + renamed, no stale duplicate)", len(got))
	}
	names := map[string]int{}
	for _, p := range got {
		names[p.Name]++
	}
	if names[testBar] != 1 || names[testKeptTool] != 1 {
		t.Errorf("want exactly one %q and one %q; got %v", testBar, testKeptTool, names)
	}
	if _, ok := names["foo.EXE"]; ok {
		t.Errorf("stale pre-rename entry %q should be removed; got %v", "foo.EXE", names)
	}
}

// TestMergePackages_bridgesSplitCanonicals reproduces the bridge case: two saved
// entries are stored under separate canonicals (one by import_path "old/foo",
// one by import_path "new/foo" under a different name). A successful package then
// arrives whose name matches the first entry and whose import_path matches the
// second, bridging both canonicals. The two must collapse into a single logical
// entry instead of leaving the stale "old/foo" entry behind.
func TestMergePackages_bridgesSplitCanonicals(t *testing.T) {
	t.Parallel()
	confPkgs := []goutil.Package{
		{Name: testFoo, ImportPath: testOldFooPath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMain},
		{Name: testBar, ImportPath: testNewFooPath, Version: &goutil.Version{Current: testVer100}, UpdateChannel: goutil.UpdateChannelMain},
	}
	succeededPkgs := []goutil.Package{
		{Name: testFoo, ImportPath: testNewFooPath, Version: &goutil.Version{Current: testVer100, Latest: testVer200}},
	}
	channelMap := map[string]goutil.UpdateChannel{testFoo: goutil.UpdateChannelMain}

	got := MergePackages(confPkgs, succeededPkgs, channelMap, nil)
	if len(got) != 1 {
		t.Fatalf("MergePackages() returned %d entries, want 1 (bridge input must collapse split canonicals); got %+v", len(got), got)
	}
	if got[0].ImportPath != testNewFooPath {
		t.Errorf("merged import_path = %q, want %q", got[0].ImportPath, testNewFooPath)
	}
}
