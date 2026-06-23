//nolint:paralleltest
package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

func Test_migrate_missingArgs(t *testing.T) {
	cmd := newMigrateCmd()
	cmd.SetArgs([]string{"only-one-path"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("migrate with fewer than two paths should fail")
	}
	got := err.Error()
	for _, want := range []string{"requires BEFORE_PATH and AFTER_PATH", "gup migrate /old/gobin /new/gobin"} {
		if !strings.Contains(got, want) {
			t.Errorf("error should contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Usage:") {
		t.Errorf("error should be concise, not full help, got:\n%s", got)
	}
}

const (
	testImportPathXY   = "github.com/x/y"
	testImportPathTool = "github.com/example/tool"
	testBinPosixer     = "posixer"
	testShellBash      = "bash"
	testCmdCompletion  = "completion"

	testOldModule         = "github.com/cosmtrek/air"
	testNewModule         = "github.com/air-verse/air"
	testParserJobsErr     = "parser --jobs argument error"
	testGobinEmpty        = "$GOBIN is empty"
	testNoExistDir        = "no_exist_dir"
	testBinAir            = "air"
	testGoVersion1224     = "go1.22.4"
	testGoVersionNoDwarf5 = "go1.26.0-X:nodwarf5"
	testFlagInstall       = "--install"
	testCmdGup            = "gup"
	testCmdList           = "list"
	testCmdExport         = "export"
	testCmdImport         = "import"
	testShellFish         = "fish"
	testBinMytool         = "mytool"
	testBinMytoolExe      = "mytool.exe"
	testPkg1              = "pkg1"
	testPkg2              = "pkg2"
	testPkg3              = "pkg3"
	testPkg2Exe           = "pkg2.exe"
	testMissing           = "missing"
	testKeptTool          = "kept-tool"
	testNewTool           = "new-tool"
	testDeleteCancel      = "delete cancel"
	testNoConfigDir       = "can not make .config directory"
	testVersion123        = "v1.2.3"
	testVersionTwo        = "v2.0.0"
	testBinTool           = "tool"
	testBinPosixerExe     = "posixer.exe"
	testBinToolA          = "tool-a"
	testCmdVersion        = "version"
	testCmdUpdate         = "update"
	testCmdRemove         = "remove"
	testShellZsh          = "zsh"
	testShellPowershell   = "powershell"
	testNameSuccess       = "success"
	testNameTest2         = "test2"
	testUnknown           = "unknown"
)

// captureMigrateOutput redirects print output into a buffer for the duration of fn.
func captureMigrateOutput(t *testing.T, fn func()) string {
	t.Helper()
	orgStdout := print.Stdout
	orgStderr := print.Stderr
	buf := &bytes.Buffer{}
	print.Stdout = buf
	print.Stderr = buf
	defer func() {
		print.Stdout = orgStdout
		print.Stderr = orgStderr
	}()
	fn()
	return buf.String()
}

func Test_validateMigratePaths(t *testing.T) {
	t.Run("BEFORE_PATH does not exist", func(t *testing.T) {
		after := t.TempDir()
		err := validateMigratePaths(filepath.Join(t.TempDir(), "missing"), after, false)
		if err == nil {
			t.Fatal("expected error for missing BEFORE_PATH")
		}
	})

	t.Run("BEFORE_PATH is a file", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := validateMigratePaths(file, t.TempDir(), false); err == nil {
			t.Fatal("expected error when BEFORE_PATH is a file")
		}
	})

	t.Run("same directory is rejected", func(t *testing.T) {
		dir := t.TempDir()
		if err := validateMigratePaths(dir, dir, false); err == nil {
			t.Fatal("expected error when BEFORE_PATH == AFTER_PATH")
		}
	})

	t.Run("same directory via relative path is rejected", func(t *testing.T) {
		dir := t.TempDir()
		rel := filepath.Join(dir, "sub", "..")
		if err := validateMigratePaths(dir, rel, false); err == nil {
			t.Fatal("expected error when paths resolve to the same directory")
		}
	})

	t.Run("same-directory failure does not create intermediate dirs", func(t *testing.T) {
		before := t.TempDir()
		parent := filepath.Dir(before)
		base := filepath.Base(before)
		midName := "should-not-be-created"
		// Build an uncleaned path that resolves (via filepath.Abs) to BEFORE,
		// e.g. /tmp/before/../before, going through a non-existent segment.
		after := strings.Join([]string{parent, midName, "..", base}, string(os.PathSeparator))

		if err := validateMigratePaths(before, after, false); err == nil {
			t.Fatal("expected error when AFTER_PATH resolves to BEFORE_PATH")
		}
		if _, err := os.Stat(filepath.Join(parent, midName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("validation failure must not create %q", filepath.Join(parent, midName))
		}
	})

	t.Run("AFTER_PATH is a file", func(t *testing.T) {
		before := t.TempDir()
		afterFile := filepath.Join(t.TempDir(), "after")
		if err := os.WriteFile(afterFile, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := validateMigratePaths(before, afterFile, false); err == nil {
			t.Fatal("expected error when AFTER_PATH is a file")
		}
	})

	t.Run("AFTER_PATH is auto-created", func(t *testing.T) {
		before := t.TempDir()
		after := filepath.Join(t.TempDir(), "new", "gobin")
		if err := validateMigratePaths(before, after, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		info, err := os.Stat(after)
		if err != nil {
			t.Fatalf("AFTER_PATH was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("AFTER_PATH is not a directory")
		}
	})

	t.Run("AFTER_PATH is not created in dry-run", func(t *testing.T) {
		before := t.TempDir()
		after := filepath.Join(t.TempDir(), "dryrun", "gobin")
		if err := validateMigratePaths(before, after, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(after); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("AFTER_PATH should not be created during dry-run")
		}
	})
}

func Test_resolveMigrateVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pkg         goutil.Package
		wantVersion string
		wantSkip    bool
	}{
		{
			name:        "regular version",
			pkg:         goutil.Package{ImportPath: testImportPathXY, Version: &goutil.Version{Current: testVersion123}},
			wantVersion: testVersion123,
			wantSkip:    false,
		},
		{
			name:     "empty import path",
			pkg:      goutil.Package{ImportPath: "", Version: &goutil.Version{Current: testVersionOne}},
			wantSkip: true,
		},
		{
			name:     commandLineArguments,
			pkg:      goutil.Package{ImportPath: commandLineArguments, Version: &goutil.Version{Current: testVersionOne}},
			wantSkip: true,
		},
		{
			name:     "empty version",
			pkg:      goutil.Package{ImportPath: testImportPathXY, Version: &goutil.Version{Current: "  "}},
			wantSkip: true,
		},
		{
			name:     "nil version pointer",
			pkg:      goutil.Package{ImportPath: testImportPathXY},
			wantSkip: true,
		},
		{
			name:     develVersion,
			pkg:      goutil.Package{ImportPath: testImportPathXY, Version: &goutil.Version{Current: develVersion}},
			wantSkip: true,
		},
		{
			name:     "devel with parentheses",
			pkg:      goutil.Package{ImportPath: testImportPathXY, Version: &goutil.Version{Current: develVersionParen}},
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotVersion, gotSkip, _ := resolveMigrateVersion(tt.pkg)
			if gotSkip != tt.wantSkip {
				t.Fatalf("resolveMigrateVersion() skip = %v, want %v", gotSkip, tt.wantSkip)
			}
			if !tt.wantSkip && gotVersion != tt.wantVersion {
				t.Fatalf("resolveMigrateVersion() version = %q, want %q", gotVersion, tt.wantVersion)
			}
		})
	}
}

func Test_migratePackages_install(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	type call struct {
		importPath string
		version    string
	}
	var calls []call
	installByVersionMigrateCtx = func(_ context.Context, importPath, version string) error {
		calls = append(calls, call{importPath, version})
		return nil
	}

	pkgs := []goutil.Package{
		{Name: testBinTool, ImportPath: testImportPathTool, Version: &goutil.Version{Current: testVersion123}},
	}

	out := captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, false, false, 1, false, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})

	if len(calls) != 1 {
		t.Fatalf("installer called %d times, want 1 (output: %s)", len(calls), out)
	}
	if calls[0].importPath != testImportPathTool || calls[0].version != testVersion123 {
		t.Fatalf("installed %q@%q, want exact reinstall", calls[0].importPath, calls[0].version)
	}
}

func Test_migratePackages_addOnlySkip(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())
	// Simulate a binary that already exists in AFTER_PATH. Use the same name
	// migrate would produce (with the platform-specific suffix on Windows).
	existing := binaryNameFromImportPath(testImportPathTool)
	if err := os.WriteFile(filepath.Join(after, existing), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	called := false
	installByVersionMigrateCtx = func(context.Context, string, string) error {
		called = true
		return nil
	}

	pkgs := []goutil.Package{
		{Name: testBinTool, ImportPath: testImportPathTool, Version: &goutil.Version{Current: testVersion123}},
	}

	out := captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, false, false, 1, false, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})

	if called {
		t.Fatal("installer should not run for an existing binary without --force")
	}
	if !strings.Contains(out, "skip") {
		t.Fatalf("expected skip message, got: %s", out)
	}
}

func Test_migratePackages_force(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())
	existing := binaryNameFromImportPath(testImportPathTool)
	if err := os.WriteFile(filepath.Join(after, existing), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	called := false
	installByVersionMigrateCtx = func(context.Context, string, string) error {
		called = true
		return nil
	}

	pkgs := []goutil.Package{
		{Name: testBinTool, ImportPath: testImportPathTool, Version: &goutil.Version{Current: testVersion123}},
	}

	captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, false, false, 1, true, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})

	if !called {
		t.Fatal("installer should run for an existing binary when --force is set")
	}
}

func Test_migratePackages_dryRun(t *testing.T) {
	after := t.TempDir()

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	called := false
	installByVersionMigrateCtx = func(context.Context, string, string) error {
		called = true
		return nil
	}

	pkgs := []goutil.Package{
		{Name: testBinTool, ImportPath: testImportPathTool, Version: &goutil.Version{Current: testVersion123}},
	}

	captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, true, false, 1, false, 0); got != 0 {
			t.Fatalf("migratePackages() dry-run = %d, want 0", got)
		}
	})

	if called {
		t.Fatal("installer must not run during dry-run")
	}
}

func Test_migratePackages_skipDevelAndUnknown(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	called := 0
	installByVersionMigrateCtx = func(context.Context, string, string) error {
		called++
		return nil
	}

	pkgs := []goutil.Package{
		{Name: "devbin", ImportPath: "github.com/example/dev", Version: &goutil.Version{Current: develVersionParen}},
		{Name: "noimport", ImportPath: "", Version: &goutil.Version{Current: testVersionOne}},
		{Name: "good", ImportPath: "github.com/example/good", Version: &goutil.Version{Current: testVersionOne}},
	}

	captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, false, false, 2, false, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})

	if called != 1 {
		t.Fatalf("installer called %d times, want 1 (only the good package)", called)
	}
}

func Test_migratePackages_modulePathMismatchRetry(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	var installedPaths []string
	installByVersionMigrateCtx = func(_ context.Context, importPath, _ string) error {
		installedPaths = append(installedPaths, importPath)
		if importPath == "github.com/old/mod/cmd/tool" {
			return errors.New("go install: module declares its path as: github.com/new/mod\n\tbut was required as: github.com/old/mod")
		}
		return nil
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: "github.com/old/mod/cmd/tool",
			ModulePath: "github.com/old/mod",
			Version:    &goutil.Version{Current: testVersionOne},
		},
	}

	captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, false, false, 1, false, 0); got != 0 {
			t.Fatalf("migratePackages() = %d, want 0", got)
		}
	})

	if len(installedPaths) != 2 {
		t.Fatalf("installer called %d times, want 2 (initial + retry): %v", len(installedPaths), installedPaths)
	}
	if installedPaths[1] != "github.com/new/mod/cmd/tool" {
		t.Fatalf("retry import path = %q, want github.com/new/mod/cmd/tool", installedPaths[1])
	}
}

func Test_migratePackages_installError(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	installByVersionMigrateCtx = func(context.Context, string, string) error {
		return errors.New("install failed")
	}

	pkgs := []goutil.Package{
		{Name: testBinTool, ImportPath: testImportPathTool, Version: &goutil.Version{Current: testVersionOne}},
	}

	captureMigrateOutput(t, func() {
		if got := migratePackages(pkgs, after, false, false, 1, false, 0); got != 1 {
			t.Fatalf("migratePackages() = %d, want 1 on install error", got)
		}
	})
}

func Test_migratePackages_jobsBoundary(t *testing.T) {
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	installByVersionMigrateCtx = func(context.Context, string, string) error { return nil }

	pkgs := []goutil.Package{
		{Name: "a", ImportPath: "github.com/example/a", Version: &goutil.Version{Current: testVersionOne}},
		{Name: "b", ImportPath: "github.com/example/b", Version: &goutil.Version{Current: testVersionOne}},
	}

	for _, jobs := range []int{-1, 0, 1, 100} {
		captureMigrateOutput(t, func() {
			if got := migratePackages(pkgs, after, false, false, jobs, false, 0); got != 0 {
				t.Fatalf("migratePackages(jobs=%d) = %d, want 0", jobs, got)
			}
		})
	}
}

func Test_runMigrate_filterByBinary(t *testing.T) {
	before := filepath.Join("testdata", "check_success")
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	var installed []string
	installByVersionMigrateCtx = func(_ context.Context, importPath, _ string) error {
		installed = append(installed, importPath)
		return nil
	}

	cmd := newMigrateCmd()
	captureMigrateOutput(t, func() {
		if got := runMigrate(cmd, []string{before, after, testBinPosixer}); got != 0 {
			t.Fatalf("runMigrate() = %d, want 0", got)
		}
	})

	if len(installed) != 1 {
		t.Fatalf("installed %d packages, want only posixer: %v", len(installed), installed)
	}
	if installed[0] != "github.com/nao1215/posixer" {
		t.Fatalf("installed %q, want github.com/nao1215/posixer", installed[0])
	}
}

func Test_runMigrate_missingTargetWarning(t *testing.T) {
	before := filepath.Join("testdata", "check_success")
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	original := installByVersionMigrateCtx
	t.Cleanup(func() { installByVersionMigrateCtx = original })

	var installed []string
	installByVersionMigrateCtx = func(_ context.Context, importPath, _ string) error {
		installed = append(installed, importPath)
		return nil
	}

	cmd := newMigrateCmd()
	out := captureMigrateOutput(t, func() {
		// "posixer" exists in testdata; "doesnotexist" does not.
		if got := runMigrate(cmd, []string{before, after, testBinPosixer, "doesnotexist"}); got != 0 {
			t.Fatalf("runMigrate() = %d, want 0", got)
		}
	})

	if len(installed) != 1 {
		t.Fatalf("installed %d packages, want only posixer: %v", len(installed), installed)
	}
	if !strings.Contains(out, "doesnotexist") {
		t.Fatalf("expected a missing-target warning for 'doesnotexist', got: %s", out)
	}
}

func Test_runMigrate_allTargetsMissingWarns(t *testing.T) {
	before := filepath.Join("testdata", "check_success")
	after := t.TempDir()
	t.Setenv("GOBIN", t.TempDir())

	cmd := newMigrateCmd()
	out := captureMigrateOutput(t, func() {
		if got := runMigrate(cmd, []string{before, after, "nope1", "nope2"}); got != 1 {
			t.Fatalf("runMigrate() = %d, want 1 when no requested binary exists", got)
		}
	})

	if !strings.Contains(out, "nope1") || !strings.Contains(out, "nope2") {
		t.Fatalf("expected warnings for each missing target, got: %s", out)
	}
}

func Test_runMigrate_sameDirError(t *testing.T) {
	dir := t.TempDir()
	cmd := newMigrateCmd()
	out := captureMigrateOutput(t, func() {
		if got := runMigrate(cmd, []string{dir, dir}); got != 1 {
			t.Fatalf("runMigrate() = %d, want 1 for same directory", got)
		}
	})
	if !strings.Contains(out, "same directory") {
		t.Fatalf("expected 'same directory' error, got: %s", out)
	}
}

func Test_runMigrate_beforeNotFound(t *testing.T) {
	cmd := newMigrateCmd()
	captureMigrateOutput(t, func() {
		if got := runMigrate(cmd, []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()}); got != 1 {
			t.Fatalf("runMigrate() = %d, want 1 for missing BEFORE_PATH", got)
		}
	})
}

func Test_binaryExistsInDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exists"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !binaryExistsInDir(dir, "exists") {
		t.Fatal("expected existing binary to be detected")
	}
	if binaryExistsInDir(dir, "missing") {
		t.Fatal("missing binary should not be detected")
	}
	if binaryExistsInDir(dir, "") {
		t.Fatal("empty name should not be detected")
	}
}
