//nolint:paralleltest,errcheck,gosec
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

const testVersionZero = "v0.0.0"
const testVersionOne = "v1.0.0"
const testVersionNine = "v9.9.9"

//nolint:gochecknoglobals // legacy stubs used by tests via init bridge.
var (
	getLatestVer        = goutil.GetLatestVer
	installLatest       = goutil.InstallLatest
	installMainOrMaster = goutil.InstallMainOrMaster
	installByVersionUpd = goutil.Install
)

//nolint:gochecknoinits
func init() {
	// Keep existing tests that stub legacy function variables working
	// after adding context-aware update/lookup paths.
	getLatestVerCtx = func(_ context.Context, modulePath string) (string, error) {
		return getLatestVer(modulePath)
	}
	installLatestCtx = func(_ context.Context, importPath string) error {
		return installLatest(importPath)
	}
	installMainOrMasterCtx = func(_ context.Context, importPath string) error {
		return installMainOrMaster(importPath)
	}
	installByVersionUpdCtx = func(_ context.Context, importPath, version string) error {
		return installByVersionUpd(importPath, version)
	}
}

func Test_gup(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	tests := []struct {
		name   string
		args   args
		want   int
		stderr []string
	}{
		{
			name: "parser --dry-run argument error",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			want: 1,
			stderr: []string{
				"gup:ERROR: can not parse command line argument (--dry-run): flag accessed but not defined: dry-run",
				"",
			},
		},
		{
			name: "parser --notify argument error",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			want: 1,
			stderr: []string{
				"gup:ERROR: can not parse command line argument (--notify): flag accessed but not defined: notify",
				"",
			},
		},
		{
			name: testParserJobsErr,
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			want: 1,
			stderr: []string{
				"gup:ERROR: can not parse command line argument (--jobs): flag accessed but not defined: jobs",
				"",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "parser --dry-run argument error":
				tt.args.cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
				tt.args.cmd.Flags().BoolP("jobs", "j", false, "Specify the number of CPU cores to use")
			case "parser --notify argument error":
				tt.args.cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
				tt.args.cmd.Flags().BoolP("jobs", "j", false, "Specify the number of CPU cores to use")
			case testParserJobsErr:
				tt.args.cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
				tt.args.cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
			}

			OsExit = func(code int) {}
			defer func() {
				OsExit = os.Exit
			}()

			orgStdout := print.Stdout
			orgStderr := print.Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			print.Stdout = pw
			print.Stderr = pw

			if got := gup(tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("gup() = %v, want %v", got, tt.want)
			}
			if err := pw.Close(); err != nil {
				t.Fatal(err)
			}
			print.Stdout = orgStdout
			print.Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			defer pr.Close()
			got := strings.Split(buf.String(), "\n")

			if diff := cmp.Diff(tt.stderr, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_completePathBinaries_prefix(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Setenv("GOBIN", filepath.Join("testdata", "check_success_for_windows"))
	} else {
		t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))
	}

	got, _ := completePathBinaries(nil, nil, "ga")
	if len(got) == 0 {
		t.Fatalf("completion should return at least one candidate")
	}

	for _, name := range got {
		if !strings.HasPrefix(strings.ToLower(name), "ga") {
			t.Fatalf("unexpected completion candidate for prefix ga: %s", name)
		}
	}
}

func Test_catchSignal(t *testing.T) {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})

	go catchSignal(signals, func() {
		close(done)
	})
	signals <- syscall.SIGINT

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("catchSignal should call cancel function")
	}
}

func Test_gup_ignoreGoUpdateFlag(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("ignore-go-update", "true"); err != nil {
		t.Fatalf("failed to set ignore-go-update flag: %v", err)
	}

	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	getLatestVer = func(string) (string, error) { return testVersionZero, nil }
	installLatest = func(string) error { return nil }
	installMainOrMaster = func(string) error { return nil }
	installByVersionUpd = func(string, string) error { return nil }
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	if got := gup(cmd, []string{}); got != 0 {
		t.Fatalf("gup() = %v, want %v", got, 0)
	}
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	var buf bytes.Buffer
	if _, err = io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = pr.Close()
	}()

	output := strings.Split(buf.String(), "\n")
	if len(output) == 0 || !strings.Contains(output[0], "update binary under") {
		t.Fatalf("unexpected output: %v", output)
	}
}

// Test_gup_emptyEnv_validatesExplicitMalformedFile verifies the #368 contract
// for update: an explicit --file is validated even when no binaries are
// installed, so a malformed config fails instead of being silently ignored.
func Test_gup_emptyEnv_validatesExplicitMalformedFile(t *testing.T) {
	setupXDGBase(t)
	t.Setenv("GOBIN", t.TempDir()) // empty environment

	badJSON := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badJSON, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("file", badJSON); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}

	var got int
	out := captureCheckOutput(t, func() int {
		got = gup(cmd, []string{})
		return got
	})
	if got != 1 {
		t.Fatalf("gup() = %d, want 1 for a malformed --file on an empty environment", got)
	}
	if !strings.Contains(out, badJSON) {
		t.Errorf("error should name the failing --file %q, got: %s", badJSON, out)
	}
}

// Test_gup_emptyEnv_validatesExplicitDirectoryFile verifies #368 for update when
// --file points to a directory.
func Test_gup_emptyEnv_validatesExplicitDirectoryFile(t *testing.T) {
	setupXDGBase(t)
	t.Setenv("GOBIN", t.TempDir()) // empty environment

	dir := filepath.Join(t.TempDir(), "config-dir")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("file", dir); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}

	got := 0
	_ = captureCheckOutput(t, func() int {
		got = gup(cmd, []string{})
		return got
	})
	if got != 1 {
		t.Fatalf("gup() = %d, want 1 for a directory --file on an empty environment", got)
	}
}

// Test_gup_emptyEnv_succeedsWithoutExplicitConfigProblem is the #368 regression
// guard: an empty environment with no explicit config problem still exits 0.
func Test_gup_emptyEnv_succeedsWithoutExplicitConfigProblem(t *testing.T) {
	setupXDGBase(t)
	chdirToTemp(t)
	t.Setenv("GOBIN", t.TempDir()) // empty environment

	got := 0
	_ = captureCheckOutput(t, func() int {
		got = gup(newUpdateCmd(), []string{})
		return got
	})
	if got != 0 {
		t.Fatalf("gup() = %d, want 0 on an empty environment with no config problem", got)
	}
}

// Test_gup_invalidConfigFile verifies the #369 contract: a malformed gup.json is
// not silently treated as "no config" (which would downgrade saved channels to
// @latest); update fails fast and names the failing path instead.
func Test_gup_invalidConfigFile(t *testing.T) {
	setupXDGBase(t)
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))
	helper_stubUpdateOps(t)

	confPath := config.FilePath()
	if err := os.MkdirAll(filepath.Dir(confPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(confPath, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got int
	out := captureCheckOutput(t, func() int {
		got = gup(newUpdateCmd(), []string{})
		return got
	})
	if got != 1 {
		t.Fatalf("gup() = %v, want %v (malformed config must fail fast)", got, 1)
	}
	if !strings.Contains(out, confPath) {
		t.Errorf("error should name the failing config path %q, got: %s", confPath, out)
	}
}

func Test_gup_dryRun(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("failed to set dry-run flag: %v", err)
	}

	var installCalled atomic.Bool
	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	installLatest = func(string) error {
		installCalled.Store(true)
		return nil
	}
	installMainOrMaster = func(string) error {
		installCalled.Store(true)
		return nil
	}
	installByVersionUpd = func(string, string) error {
		installCalled.Store(true)
		return nil
	}
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	if got := gup(cmd, []string{}); got != 0 {
		t.Fatalf("gup() = %v, want %v", got, 0)
	}
	if !installCalled.Load() {
		t.Fatalf("expected installer to be invoked in dry-run mode")
	}
	if gobin := os.Getenv("GOBIN"); !strings.Contains(gobin, "check_success") {
		t.Fatalf("GOBIN should be restored after dry-run, got %s", gobin)
	}
}

func Test_update_not_use_go_cmd(t *testing.T) {
	t.Run("Not found go command", func(t *testing.T) {
		t.Setenv("PATH", "")

		orgStdout := print.Stdout
		orgStderr := print.Stderr
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		print.Stdout = pw
		print.Stderr = pw

		if got := gup(newUpdateCmd(), []string{}); got != 1 {
			t.Errorf("gup() = %v, want %v", got, 1)
		}
		if err := pw.Close(); err != nil {
			t.Fatal(err)
		}
		print.Stdout = orgStdout
		print.Stderr = orgStderr

		buf := bytes.Buffer{}
		_, err = io.Copy(&buf, pr)
		if err != nil {
			t.Error(err)
		}
		defer pr.Close()
		got := strings.Split(buf.String(), "\n")

		want := []string{}
		if runtime.GOOS == goosWindows {
			want = append(want, `gup:ERROR: you didn't install golang: exec: "go": executable file not found in %PATH%`)
			want = append(want, "")
		} else {
			want = append(want, `gup:ERROR: you didn't install golang: exec: "go": executable file not found in $PATH`)
			want = append(want, "")
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	})
}

func Test_desktopNotifyIfNeeded(t *testing.T) {
	type args struct {
		result int
		enable bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Notify update success",
			args: args{
				result: 0,
				enable: true,
			},
		},

		{
			name: "Notify update fail",
			args: args{
				result: 1,
				enable: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desktopNotifyIfNeeded(tt.args.result, tt.args.enable)
		})
	}
}

func Test_gup_jobsClamp(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("jobs", "-1"); err != nil {
		t.Fatalf("failed to set jobs flag: %v", err)
	}

	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	getLatestVer = func(string) (string, error) { return testVersionZero, nil }
	installLatest = func(string) error { return nil }
	installMainOrMaster = func(string) error { return nil }
	installByVersionUpd = func(string, string) error { return nil }
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	_, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	// Should not hang with jobs=-1 (clamped to 1)
	got := gup(cmd, []string{})
	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	if got != 0 {
		t.Errorf("gup() = %v, want 0", got)
	}
}

func Test_replaceImportPathPrefix(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		oldModule  string
		newModule  string
		wantImport string
	}{
		{
			name:       "same as module root",
			importPath: testOldModule,
			oldModule:  testOldModule,
			newModule:  testNewModule,
			wantImport: testNewModule,
		},
		{
			name:       "subdir path",
			importPath: "github.com/cosmtrek/air/cmd/air",
			oldModule:  testOldModule,
			newModule:  testNewModule,
			wantImport: "github.com/air-verse/air/cmd/air",
		},
		{
			name:       "empty import path",
			importPath: "",
			oldModule:  testOldModule,
			newModule:  testNewModule,
			wantImport: testNewModule,
		},
		{
			name:       "no prefix match keeps original import path",
			importPath: testImportPathTool,
			oldModule:  testOldModule,
			newModule:  testNewModule,
			wantImport: testImportPathTool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceImportPathPrefix(tt.importPath, tt.oldModule, tt.newModule)
			if got != tt.wantImport {
				t.Errorf("replaceImportPathPrefix() = %q, want %q", got, tt.wantImport)
			}
		})
	}
}

func Test_removeOldBinaryIfRenamed(t *testing.T) {
	gobin := t.TempDir()
	t.Setenv("GOBIN", gobin)

	oldBinaryPath := filepath.Join(gobin, "old-tool")
	if err := os.WriteFile(oldBinaryPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := removeOldBinaryIfRenamed("old-tool", testNewTool); err != nil {
		t.Fatalf("removeOldBinaryIfRenamed() error = %v", err)
	}
	if _, err := os.Stat(oldBinaryPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old binary should be removed. err = %v", err)
	}

	sameBinaryPath := filepath.Join(gobin, "same-tool")
	if err := os.WriteFile(sameBinaryPath, []byte("dummy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeOldBinaryIfRenamed("same-tool", "same-tool"); err != nil {
		t.Fatalf("removeOldBinaryIfRenamed() should not fail for same name: %v", err)
	}
	if _, err := os.Stat(sameBinaryPath); err != nil {
		t.Fatalf("binary should remain when names are same. err = %v", err)
	}
}

func Test_update_modulePathChangedOnGetLatest(t *testing.T) {
	const (
		oldModule = testOldModule
		newModule = testNewModule
		oldImport = "github.com/cosmtrek/air/cmd/air"
		newImport = "github.com/air-verse/air/cmd/air"
	)

	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()

	var latestCalls []string
	getLatestVer = func(modulePath string) (string, error) {
		latestCalls = append(latestCalls, modulePath)
		if modulePath == oldModule {
			return "", modulePathMismatchErr(oldModule, newModule)
		}
		if modulePath == newModule {
			return testVersion123, nil
		}
		return "", errors.New("unexpected module path")
	}

	var installCalls []string
	installLatest = func(importPath string) error {
		installCalls = append(installCalls, importPath)
		return nil
	}
	installMainOrMaster = func(string) error {
		t.Fatal("installMainOrMaster should not be called")
		return nil
	}
	installByVersionUpd = func(string, string) error {
		t.Fatal("installByVersionUpd should not be called")
		return nil
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinAir,
			ImportPath: oldImport,
			ModulePath: oldModule,
			Version: &goutil.Version{
				Current: testVersion123,
			},
			GoVersion: &goutil.Version{
				Current: testGoVersion1224,
				Latest:  testGoVersion1224,
			},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinAir: goutil.UpdateChannelLatest}
	if got, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false); got != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", got)
	}
	if diff := cmp.Diff([]string{oldModule, newModule}, latestCalls); diff != "" {
		t.Errorf("latest module path calls mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{newImport}, installCalls); diff != "" {
		t.Errorf("install import path calls mismatch (-want +got):\n%s", diff)
	}
}

func Test_update_modulePathChangedOnInstall(t *testing.T) {
	const (
		oldModule = testOldModule
		newModule = testNewModule
		oldImport = testOldModule
		newImport = testNewModule
	)

	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()

	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	installMainOrMaster = func(string) error {
		t.Fatal("installMainOrMaster should not be called")
		return nil
	}
	installByVersionUpd = func(string, string) error {
		t.Fatal("installByVersionUpd should not be called")
		return nil
	}

	var installCalls []string
	installLatest = func(importPath string) error {
		installCalls = append(installCalls, importPath)
		switch len(installCalls) {
		case 1:
			return modulePathMismatchErr(oldModule, newModule)
		case 2:
			return nil
		default:
			return errors.New("unexpected install call")
		}
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinAir,
			ImportPath: oldImport,
			ModulePath: newModule,
			Version: &goutil.Version{
				Current: testVersionOne,
			},
			GoVersion: &goutil.Version{
				Current: testGoVersion1224,
				Latest:  testGoVersion1224,
			},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinAir: goutil.UpdateChannelLatest}
	if got, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false); got != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", got)
	}
	if diff := cmp.Diff([]string{oldImport, newImport}, installCalls); diff != "" {
		t.Errorf("install import path calls mismatch (-want +got):\n%s", diff)
	}
}

func modulePathMismatchErr(requiredPath, declaredPath string) error {
	return errors.New("version constraints conflict:\n" +
		"module declares its path as: " + declaredPath + "\n" +
		"but was required as: " + requiredPath)
}

func Test_installWithSelectedVersion(t *testing.T) {
	t.Parallel()

	// Inject the install operations directly instead of mutating package
	// globals, so this test owns its dependencies and runs in parallel.
	var called string
	deps := dependencies{
		installLatest:       func(context.Context, string) error { called = latestKeyword; return nil },
		installMainOrMaster: func(context.Context, string) error { called = "main"; return nil },
		installByVersion:    func(_ context.Context, _, v string) error { called = "version:" + v; return nil },
	}

	tests := []struct {
		channel goutil.UpdateChannel
		want    string
	}{
		{goutil.UpdateChannelLatest, latestKeyword},
		{goutil.UpdateChannelMain, "main"},
		{goutil.UpdateChannelMaster, "version:master"},
		{testUnknown, latestKeyword}, // default case
	}
	for _, tt := range tests {
		called = ""
		if err := installWithSelectedVersion(deps, context.Background(), "example.com/tool", tt.channel); err != nil {
			t.Errorf("channel=%q: unexpected error: %v", tt.channel, err)
		}
		if called != tt.want {
			t.Errorf("channel=%q: called = %q, want %q", tt.channel, called, tt.want)
		}
	}
}

func Test_installWithSelectedVersion_contextCanceled(t *testing.T) {
	origInstallLatestCtx := installLatestCtx
	defer func() {
		installLatestCtx = origInstallLatestCtx
	}()

	installLatestCtx = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return fmt.Errorf("can't install %s:\n%w", "example.com/tool", ctx.Err())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := installWithSelectedVersion(defaultDependencies(), ctx, "example.com/tool", goutil.UpdateChannelLatest)
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("installWithSelectedVersion() error = %v, want cancellation to be surfaced", err)
	}
}

func Test_binaryNameFromImportPath(t *testing.T) {
	t.Setenv("GOEXE", "")

	got := binaryNameFromImportPath("github.com/example/tool/cmd/mytool")
	want := testBinMytool
	if runtime.GOOS == goosWindows {
		want = testBinMytoolExe
	}
	if got != want {
		t.Errorf("binaryNameFromImportPath() = %q, want %q", got, want)
	}
}

func Test_binaryNameFromImportPathWith(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		goos       string
		goexe      string
		want       string
	}{
		{
			name:       "non-windows keeps base name",
			importPath: "github.com/example/tool/cmd/mytool",
			goos:       "linux",
			goexe:      "",
			want:       testBinMytool,
		},
		{
			name:       "windows adds .exe when GOEXE is empty",
			importPath: testNewModule,
			goos:       goosWindows,
			goexe:      "",
			want:       "air.exe",
		},
		{
			name:       "windows keeps suffix when already .exe",
			importPath: "github.com/example/tool/cmd/mytool.exe",
			goos:       goosWindows,
			goexe:      "",
			want:       testBinMytoolExe,
		},
		{
			name:       "windows respects GOEXE when provided",
			importPath: "github.com/example/tool/cmd/mytool",
			goos:       goosWindows,
			goexe:      ".EXE",
			want:       "mytool.EXE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := binaryNameFromImportPathWith(tt.importPath, tt.goos, tt.goexe)
			if got != tt.want {
				t.Errorf("binaryNameFromImportPathWith(%q, %q, %q) = %q, want %q",
					tt.importPath, tt.goos, tt.goexe, got, tt.want)
			}
		})
	}
}

func Test_removeOldBinaryIfRenamed_unsafeName(t *testing.T) {
	err := removeOldBinaryIfRenamed("../escape", testNewTool)
	if err == nil {
		t.Fatal("expected error for unsafe binary name")
	}
	if !strings.Contains(err.Error(), "unsafe name") {
		t.Errorf("error = %q, want 'unsafe name'", err.Error())
	}
}

func Test_removeOldBinaryIfRenamed_emptyNames(t *testing.T) {
	if err := removeOldBinaryIfRenamed("", testNewTool); err != nil {
		t.Errorf("empty oldName should return nil, got: %v", err)
	}
	if err := removeOldBinaryIfRenamed("old-tool", ""); err != nil {
		t.Errorf("empty newName should return nil, got: %v", err)
	}
}

func Test_updateWithChannels_emptyImportPath(t *testing.T) {
	origGetLatest := getLatestVer
	defer func() { getLatestVer = origGetLatest }()
	getLatestVer = func(string) (string, error) { return testVersionNine, nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: "",
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 1 {
		t.Fatalf("updateWithChannels() = %d, want 1 (empty import path)", result)
	}
}

func Test_updateWithChannels_alreadyUpToDate(t *testing.T) {
	origGetLatest := getLatestVer
	defer func() { getLatestVer = origGetLatest }()
	getLatestVer = func(string) (string, error) { return testVersionOne, nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, succeeded, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	// Already up-to-date packages should still be in succeeded list
	if len(succeeded) != 1 {
		t.Fatalf("succeeded = %d, want 1", len(succeeded))
	}
}

func Test_updateWithChannels_alreadyUpToDate_customGoBuildTag(t *testing.T) {
	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()
	getLatestVer = func(string) (string, error) { return testVersionOne, nil }

	var installCalled atomic.Bool
	installLatest = func(string) error {
		installCalled.Store(true)
		return nil
	}
	installMainOrMaster = func(string) error {
		t.Fatal("installMainOrMaster should not be called")
		return nil
	}
	installByVersionUpd = func(string, string) error {
		t.Fatal("installByVersionUpd should not be called")
		return nil
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersionNoDwarf5, Latest: testGoVersionNoDwarf5},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, succeeded, _ := updateWithChannels(pkgs, false, false, 1, false, channelMap, nil, 0, false, false)

	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	_ = pr.Close()

	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	if len(succeeded) != 1 {
		t.Fatalf("succeeded = %d, want 1", len(succeeded))
	}
	if installCalled.Load() {
		t.Fatal("installLatest should not be called for already-up-to-date package")
	}
	if strings.Contains(buf.String(), "()") {
		t.Fatalf("unexpected empty diff output, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "Already up-to-date") {
		t.Fatalf("expected 'Already up-to-date' output, got:\n%s", buf.String())
	}
}

func Test_updateWithChannels_customGoBuildTag_goVersionDiffColor(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()
	getLatestVer = func(string) (string, error) { return testVersionOne, nil }
	installLatest = func(string) error { return nil }
	installMainOrMaster = func(string) error {
		t.Fatal("installMainOrMaster should not be called")
		return nil
	}
	installByVersionUpd = func(string, string) error {
		t.Fatal("installByVersionUpd should not be called")
		return nil
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: "go1.25.0-X:nodwarf5", Latest: testGoVersionNoDwarf5},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, false, channelMap, nil, 0, false, false)
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	_ = pr.Close()

	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	if !strings.Contains(buf.String(), color.YellowString("go1.25.0-X:nodwarf5")) {
		t.Fatalf("expected current go version in yellow, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), color.GreenString(testGoVersionNoDwarf5)) {
		t.Fatalf("expected latest go version in green, got:\n%s", buf.String())
	}
}

func Test_updateWithChannels_emptyModulePath(t *testing.T) {
	origInstallLatest := installLatest
	defer func() { installLatest = origInstallLatest }()
	installLatest = func(string) error { return nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: "",
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
}

func Test_updateWithChannels_getLatestVerError(t *testing.T) {
	origGetLatest := getLatestVer
	defer func() { getLatestVer = origGetLatest }()
	getLatestVer = func(string) (string, error) {
		return "", errors.New("network error")
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 1 {
		t.Fatalf("updateWithChannels() = %d, want 1", result)
	}
}

func Test_updateWithChannels_masterChannel(t *testing.T) {
	origGetLatest := getLatestVer
	origRef := getVerByRefCtx
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		getVerByRefCtx = origRef
		installByVersionUpd = origInstallByVersionUpd
	}()

	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	// The skip/update decision resolves the version via the @master ref.
	getVerByRefCtx = func(_ context.Context, _ string, _ string) (string, error) { return testVersionNine, nil }
	var calledVersion string
	installByVersionUpd = func(_, ver string) error { calledVersion = ver; return nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelMaster}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	if calledVersion != "master" {
		t.Fatalf("install called with version = %q, want master", calledVersion)
	}
}

// Test_updateWithChannels_masterChannel_skipDecisionUsesChannel verifies that
// the skip/update decision for a package tracked on @master is derived from the
// @master ref, not from @latest. Here @latest still equals the installed
// version (so an @latest-based decision would wrongly skip the package), while
// @master has moved forward and must trigger an update.
func Test_updateWithChannels_masterChannel_skipDecisionUsesChannel(t *testing.T) {
	origGetLatest := getLatestVer
	origRef := getVerByRefCtx
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		getVerByRefCtx = origRef
		installByVersionUpd = origInstallByVersionUpd
	}()

	getLatestVer = func(string) (string, error) { return testVersionOne, nil }
	getVerByRefCtx = func(_ context.Context, _ string, ref string) (string, error) {
		if ref == string(goutil.UpdateChannelMaster) {
			return testVersionNine, nil
		}
		return "", fmt.Errorf("unexpected ref %q", ref)
	}

	var installCalled atomic.Bool
	var calledVersion string
	installByVersionUpd = func(_, ver string) error {
		installCalled.Store(true)
		calledVersion = ver
		return nil
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelMaster}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	if !installCalled.Load() {
		t.Fatal("expected @master install to run, but the package was skipped using the @latest version")
	}
	if calledVersion != "master" {
		t.Fatalf("install called with version = %q, want master", calledVersion)
	}
}

// Test_updateWithChannels_masterChannel_latestMovedButMasterSame verifies the
// reverse failure mode: @latest has moved forward (which would wrongly trigger
// an update) but the @master ref still matches the installed version, so the
// package must be skipped.
func Test_updateWithChannels_masterChannel_latestMovedButMasterSame(t *testing.T) {
	origGetLatest := getLatestVer
	origRef := getVerByRefCtx
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		getVerByRefCtx = origRef
		installByVersionUpd = origInstallByVersionUpd
	}()

	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	getVerByRefCtx = func(_ context.Context, _ string, ref string) (string, error) {
		if ref == string(goutil.UpdateChannelMaster) {
			return testVersionOne, nil
		}
		return "", fmt.Errorf("unexpected ref %q", ref)
	}

	installByVersionUpd = func(_, _ string) error {
		t.Fatal("install must not run: @master is unchanged")
		return nil
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelMaster}
	result, succeeded, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	if len(succeeded) != 1 {
		t.Fatalf("succeeded = %d, want 1 (up-to-date package is still a success)", len(succeeded))
	}
}

// Test_updateWithChannels_mainChannel_skipDecisionUsesChannel verifies the same
// channel-aware skip decision for the @main channel: @latest equals the
// installed version, but @main has moved and must trigger an update.
func Test_updateWithChannels_mainChannel_skipDecisionUsesChannel(t *testing.T) {
	origGetLatest := getLatestVer
	origRef := getVerByRefCtx
	origInstallMain := installMainOrMaster
	defer func() {
		getLatestVer = origGetLatest
		getVerByRefCtx = origRef
		installMainOrMaster = origInstallMain
	}()

	getLatestVer = func(string) (string, error) { return testVersionOne, nil }
	getVerByRefCtx = func(_ context.Context, _ string, ref string) (string, error) {
		if ref == string(goutil.UpdateChannelMain) {
			return testVersionNine, nil
		}
		return "", fmt.Errorf("unexpected ref %q", ref)
	}

	var installCalled atomic.Bool
	installMainOrMaster = func(string) error {
		installCalled.Store(true)
		return nil
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelMain}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
	if !installCalled.Load() {
		t.Fatal("expected @main install to run, but the package was skipped using the @latest version")
	}
}

func Test_gup_excludeFlag(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newUpdateCmd()
	if err := cmd.Flags().Set("exclude", "gal,posixer,subaru"); err != nil {
		t.Fatalf("failed to set exclude flag: %v", err)
	}

	helper_stubUpdateOps(t)

	OsExit = func(code int) {}
	defer func() { OsExit = os.Exit }()

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	_, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	got := gup(cmd, []string{})
	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	if got != 1 {
		t.Errorf("gup() with all excluded = %v, want 1", got)
	}
}

func Test_removeOldBinaryIfRenamed_notExist(t *testing.T) {
	gobin := t.TempDir()
	t.Setenv("GOBIN", gobin)

	// old binary doesn't exist — should succeed silently
	if err := removeOldBinaryIfRenamed("nonexistent-tool", testNewTool); err != nil {
		t.Fatalf("removeOldBinaryIfRenamed() should succeed for non-existent: %v", err)
	}
}

func Test_updateWithChannels_notify(t *testing.T) {
	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
	}()

	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	installLatest = func(string) error { return nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, _, _ := updateWithChannels(pkgs, false, true, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() with notify = %d, want 0", result)
	}
}

func Test_updateWithChannels_installError(t *testing.T) {
	origGetLatest := getLatestVer
	origInstallLatest := installLatest
	defer func() {
		getLatestVer = origGetLatest
		installLatest = origInstallLatest
	}()

	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	installLatest = func(string) error { return errors.New("install failed") }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelLatest}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 1 {
		t.Fatalf("updateWithChannels() = %d, want 1", result)
	}
}

func Test_updateWithChannels_mainChannel(t *testing.T) {
	origGetLatest := getLatestVer
	origRef := getVerByRefCtx
	origInstallLatest := installLatest
	origInstallMain := installMainOrMaster
	origInstallByVersionUpd := installByVersionUpd
	defer func() {
		getLatestVer = origGetLatest
		getVerByRefCtx = origRef
		installLatest = origInstallLatest
		installMainOrMaster = origInstallMain
		installByVersionUpd = origInstallByVersionUpd
	}()

	getLatestVer = func(string) (string, error) { return testVersionNine, nil }
	// The skip/update decision resolves the version via the @main ref.
	getVerByRefCtx = func(_ context.Context, _ string, _ string) (string, error) { return testVersionNine, nil }
	installLatest = func(string) error {
		t.Fatal("installLatest should not be called for main channel")
		return nil
	}
	installMainOrMaster = func(string) error { return nil }
	installByVersionUpd = func(string, string) error { return nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: testImportPathTool,
			ModulePath: testImportPathTool,
			Version:    &goutil.Version{Current: testVersionOne},
			GoVersion:  &goutil.Version{Current: testGoVersion1224, Latest: testGoVersion1224},
		},
	}

	channelMap := map[string]goutil.UpdateChannel{testBinTool: goutil.UpdateChannelMain}
	result, _, _ := updateWithChannels(pkgs, false, false, 1, true, channelMap, nil, 0, false, false)
	if result != 0 {
		t.Fatalf("updateWithChannels() = %d, want 0", result)
	}
}

// Test_update_ambiguousConfigFailsFast verifies the #342 contract for update:
// when both the user-level config and ./gup.json exist and no --file is given,
// update fails fast with the ambiguity error instead of silently choosing one
// config (mirroring import). It must not reach the install path.
func Test_update_ambiguousConfigFailsFast(t *testing.T) {
	gobin, err := filepath.Abs(filepath.Join("testdata", "check_success"))
	if err != nil {
		t.Fatal(err)
	}
	setupXDGBase(t)
	chdirToTemp(t)
	t.Setenv("GOBIN", gobin)

	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.FilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LocalFilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	got := gup(newUpdateCmd(), []string{})

	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	buf := bytes.Buffer{}
	io.Copy(&buf, pr)
	pr.Close()

	if got != 1 {
		t.Errorf("update() = %d, want 1", got)
	}
	out := buf.String()
	if !strings.Contains(out, "multiple gup.json") || !strings.Contains(out, "--file") {
		t.Errorf("expected ambiguity error mentioning --file, got: %s", out)
	}
}
