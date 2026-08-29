// Package cmd define subcommands provided by the gup command
//
//nolint:paralleltest
package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func helper_CopyFile(t *testing.T, src, dst string) {
	t.Helper()

	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close() //nolint:errcheck // ignore close error in test

	out, err := os.Create(filepath.Clean(dst))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close() //nolint:errcheck // ignore close error in test

	_, err = io.Copy(out, in)
	if err != nil {
		t.Fatal(err)
	}
}

func helper_setupFakeGoBin(t *testing.T) {
	t.Helper()

	absGobin, err := filepath.Abs(filepath.Join("testdata", "gobin_tmp"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOBIN", absGobin)

	// failsafe to ensure fake GOBIN has been set
	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(gobin, "_tmp") {
		t.Fatal("SHOULD NOT HAPPEN: GOBIN is not set to fake path")
	}
}

// helper_runUpdateWithDeps builds the update command, applies the given flag
// args (e.g. "--dry-run", "--notify"), runs gup() with injected dependencies
// while capturing print output, and returns the output split by newline. It
// replaces dispatching update through Execute() with globally-stubbed operations
// so integration-style update tests inject their fakes explicitly.
func helper_runUpdateWithDeps(t *testing.T, deps dependencies, flagArgs ...string) []string {
	t.Helper()
	cmd := newUpdateCmd()
	if err := cmd.ParseFlags(flagArgs); err != nil {
		t.Fatalf("failed to parse update flags %v: %v", flagArgs, err)
	}
	out := captureCheckOutput(t, func(p *print.Printer) int {
		return gup(deps, p, cmd, cmd.Flags().Args())
	})
	return strings.Split(out, "\n")
}

// helper_runCheckWithDeps is the check counterpart of helper_runUpdateWithDeps:
// it runs check() with injected dependencies (so version lookups never hit the
// network) while capturing output, instead of dispatching through Execute() and
// depending on live module resolution.
func helper_runCheckWithDeps(t *testing.T, deps dependencies, flagArgs ...string) []string {
	t.Helper()
	cmd := newCheckCmd()
	if err := cmd.ParseFlags(flagArgs); err != nil {
		t.Fatalf("failed to parse check flags %v: %v", flagArgs, err)
	}
	out := captureCheckOutput(t, func(p *print.Printer) int {
		return check(deps, p, cmd, cmd.Flags().Args())
	})
	return strings.Split(out, "\n")
}

func helper_stubImportInstaller(t *testing.T) {
	t.Helper()

	orgInstallByVersion := installByVersionCtx
	installByVersionCtx = func(context.Context, string, string) error {
		return nil
	}
	t.Cleanup(func() {
		installByVersionCtx = orgInstallByVersion
	})
}

// helper_runGup runs a gup command and returns its output split by \n. It builds
// the root command and wires a buffer onto it via cobra's SetOut/SetErr (which
// the per-command Printer reads through cmd.OutOrStdout/ErrOrStderr), so output
// is captured without redirecting the process-wide os.Stdout/os.Stderr.
func helper_runGup(t *testing.T, args []string) ([]string, error) {
	t.Helper()

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	buf, err := runRootWithBuffer(args)
	if err != nil {
		return nil, err
	}
	return strings.Split(buf, "\n"), nil
}

// helper_runGupCaptureAllOutput runs a gup command and returns all of its output
// (stdout and stderr merged), captured via the root command's SetOut/SetErr.
func helper_runGupCaptureAllOutput(t *testing.T, args []string) (string, error) {
	t.Helper()

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	return runRootWithBuffer(args)
}

// runRootWithBuffer executes the root command with args (the first element is the
// "gup" program name, like os.Args), capturing stdout and stderr into one buffer
// via cobra's configurable writers. Returning the buffer and the Execute error
// keeps the helpers free of any process-global stream redirection.
func runRootWithBuffer(args []string) (string, error) {
	buf := &bytes.Buffer{}
	rootCmd := newRootCmd()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args[1:])
	err := rootCmd.Execute()
	return buf.String(), err
}

func setupXDGBase(t *testing.T) {
	t.Helper()

	origHome := os.Getenv("HOME")
	origConfig := xdg.ConfigHome
	origData := xdg.DataHome
	origCache := xdg.CacheHome

	// Use os.MkdirTemp instead of t.TempDir() to avoid flaky
	// "directory not empty" failures on macOS caused by Spotlight
	// indexing race conditions during t.TempDir() cleanup.
	base, err := os.MkdirTemp("", "gup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	telemetryDir, err := os.MkdirTemp("", "gup-test-telemetry-*")
	if err != nil {
		t.Fatalf("failed to create telemetry temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Unsetenv("HOME")
		if origHome != "" {
			_ = os.Setenv("HOME", origHome)
		}
		xdg.ConfigHome = origConfig
		xdg.DataHome = origData
		xdg.CacheHome = origCache
		_ = os.RemoveAll(base)
		_ = os.RemoveAll(telemetryDir)
	})

	t.Setenv("HOME", base)
	t.Setenv("GOTELEMETRY", "off")
	t.Setenv("GOTELEMETRYDIR", telemetryDir)
	xdg.ConfigHome = filepath.Join(base, "config")
	xdg.DataHome = filepath.Join(base, "data")
	xdg.CacheHome = filepath.Join(base, "cache")

	for _, dir := range []string{xdg.ConfigHome, xdg.DataHome, xdg.CacheHome} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create XDG directory %s: %v", dir, err)
		}
	}
}

func Test_completeNCPUs(t *testing.T) {
	got, directive := completeNCPUs(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	n := runtime.NumCPU()
	if len(got) != n {
		t.Errorf("len(completeNCPUs) = %d, want %d", len(got), n)
	}
	if got[0] != "1" {
		t.Errorf("first element = %q, want 1", got[0])
	}
}

func TestExecute(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    testNameSuccess,
			args:    []string{""},
			wantErr: false,
		},
		{
			name:    "fail",
			args:    []string{"no-exist-subcommand", "--no-exist-option"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			err := Execute()
			gotErr := err != nil
			if tt.wantErr != gotErr {
				t.Errorf("expected error return %v, got %v: %v", tt.wantErr, gotErr, err)
			}
		})
	}
}

func TestExecute_Check(t *testing.T) {
	setupXDGBase(t)

	if runtime.GOOS == goosWindows {
		gobinDir := filepath.Join("testdata", "check_success_for_windows")
		t.Setenv("GOBIN", gobinDir)
	} else {
		gobinDir := filepath.Join("testdata", "check_success")
		t.Setenv("GOBIN", gobinDir)
	}

	// Inject stubbed version lookups (latest == v9.9.9) so every installed binary
	// is reported as needing an update, deterministically and without resolving
	// modules over the network.
	got := helper_runCheckWithDeps(t, stubUpdateDeps())

	if !strings.Contains(got[len(got)-2], "posixer") {
		t.Errorf("posixer package is not included in the update target: %s", got[len(got)-2])
	}
	if !strings.Contains(got[len(got)-2], "gal") {
		t.Errorf("gal package is not included in the update target: %s", got[len(got)-2])
	}
}

func TestExecute_Version(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name   string
		args   []string
		stdout []string
	}{
		{
			name:   testNameSuccess,
			args:   []string{testCmdGup, testCmdVersion},
			stdout: []string{"gup version (devel) (under Apache License version 2.0)", ""},
		},
	}
	for _, tt := range tests {
		got, err := helper_runGup(t, tt.args)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(tt.stdout, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	}
}

// TestExecute_RootVersionFlag verifies the top-level --version / -V flag prints
// the same version information as the "gup version" subcommand (issue #325).
func TestExecute_RootVersionFlag(t *testing.T) {
	for _, arg := range []string{"--version", "-V"} {
		t.Run(arg, func(t *testing.T) {
			b := bytes.NewBufferString("")
			c := newRootCmd()
			c.SetOut(b)
			c.SetArgs([]string{arg})
			if err := c.Execute(); err != nil {
				t.Fatalf("Execute(%s) error = %v", arg, err)
			}
			got := strings.TrimRight(b.String(), "\n")
			if got != cmdinfo.GetVersion() {
				t.Errorf("gup %s = %q, want %q", arg, got, cmdinfo.GetVersion())
			}
		})
	}
}

func TestExecute_RootHelpIncludesProjectLinks(t *testing.T) {
	got, err := helper_runGupCaptureAllOutput(t, []string{testCmdGup, "--help"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Documentation: https://nao1215.github.io/gup/",
		"GitHub Sponsors: https://github.com/sponsors/nao1215",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("gup --help output does not contain %q:\n%s", want, got)
		}
	}
}

// TestSubcommandExamples verifies the root command and every subcommand ship
// copy-paste-friendly Example help mentioning the command (issue #326).
func TestSubcommandExamples(t *testing.T) {
	root := newRootCmd()
	if root.Example == "" {
		t.Error("root command should define an Example")
	}

	subs := root.Commands()
	if len(subs) == 0 {
		t.Fatal("expected the root command to have subcommands")
	}
	for _, sub := range subs {
		want := "gup " + sub.Name()
		if !strings.Contains(sub.Example, want) {
			t.Errorf("%q Example = %q, want it to contain %q", sub.Name(), sub.Example, want)
		}
	}
}

func TestExecute_List(t *testing.T) {
	setupXDGBase(t)

	type test struct {
		name   string
		gobin  string
		args   []string
		stdout []string
		want   int
	}
	tests := []test{}

	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name:  "check success in windows",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{testCmdGup, testCmdList},
			stdout: []string{
				"github.com/nao1215/gal/cmd/gal",
				"github.com/nao1215/posixer",
			},
			want: 2,
		})
	} else {
		tests = append(tests, test{
			name:  "check success in nix family",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{testCmdGup, testCmdList},
			stdout: []string{
				"    gal: github.com/nao1215/gal/cmd/gal@v1.1.1",
				"posixer: github.com/nao1215/posixer@v0.1.0",
				" subaru: github.com/nao1215/subaru@v1.0.0",
			},
			want: 3,
		})
	}
	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		got, err := helper_runGup(t, tt.args)
		if err != nil {
			t.Fatal(err)
		}

		count := 0
		for _, g := range got {
			for _, w := range tt.stdout {
				if strings.Contains(g, w) {
					count++
				}
			}
		}
		if count != tt.want {
			t.Errorf("value is mismatch. want=%d got=%d", tt.want, count)
		}
	}
}

func TestExecute_Remove_Force(t *testing.T) {
	setupXDGBase(t)

	type test struct {
		name   string
		gobin  string
		args   []string
		stdout []string
	}
	tests := []test{}

	var src, dest string
	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name:   testNameSuccess,
			gobin:  filepath.Join("testdata", "delete_force"),
			args:   []string{testCmdGup, testCmdRemove, "-f", testBinPosixerExe},
			stdout: []string{},
		})
		if err := os.MkdirAll(filepath.Join("testdata", "delete_force"), 0750); err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(filepath.Join("testdata", "delete_force"))
			if err != nil {
				t.Fatal(err)
			}
		}()
		src = filepath.Join("testdata", "check_success_for_windows", testBinPosixerExe)
		dest = filepath.Join("testdata", "delete_force", testBinPosixerExe)
	} else {
		tests = append(tests, test{
			name:   testNameSuccess,
			gobin:  filepath.Join("testdata", "delete"),
			args:   []string{testCmdGup, testCmdRemove, "-f", "posixer"},
			stdout: []string{},
		})
		if err := os.MkdirAll(filepath.Join("testdata", "delete"), 0750); err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(filepath.Join("testdata", "delete"))
			if err != nil {
				t.Fatal(err)
			}
		}()
		src = filepath.Join("testdata", "check_success", "posixer")
		dest = filepath.Join("testdata", "delete", "posixer")
	}

	helper_CopyFile(t, src, dest)

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			if err := Execute(); err != nil {
				t.Error(err)
			}
		})

		if fileutil.IsFile(dest) {
			t.Errorf("failed to remove posixer command")
		}
	}
}

func TestExecute_Export(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name  string
		gobin string
		args  []string
		want  string
	}{}

	if runtime.GOOS == goosWindows {
		tests = append(tests, struct {
			name  string
			gobin string
			args  []string
			want  string
		}{
			name:  testNameSuccess,
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{testCmdGup, testCmdExport},
			want: `{
  "schema_version": 1,
  "packages": [
    {
      "name": "gal.exe",
      "import_path": "github.com/nao1215/gal/cmd/gal",
      "version": "latest",
      "channel": "latest"
    },
    {
      "name": "posixer.exe",
      "import_path": "github.com/nao1215/posixer",
      "version": "latest",
      "channel": "latest"
    }
  ]
}
`,
		})
	} else {
		tests = append(tests, struct {
			name  string
			gobin string
			args  []string
			want  string
		}{
			name:  testNameSuccess,
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{testCmdGup, testCmdExport},
			want: `{
  "schema_version": 1,
  "packages": [
    {
      "name": "gal",
      "import_path": "github.com/nao1215/gal/cmd/gal",
      "version": "v1.1.1",
      "channel": "latest"
    },
    {
      "name": "posixer",
      "import_path": "github.com/nao1215/posixer",
      "version": "v0.1.0",
      "channel": "latest"
    },
    {
      "name": "subaru",
      "import_path": "github.com/nao1215/subaru",
      "version": "v1.0.0",
      "channel": "latest"
    }
  ]
}
`,
		})
	}

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		doBackup := false
		if fileutil.IsFile(config.FilePath()) {
			if err := os.Rename(config.FilePath(), config.FilePath()+".backup"); err != nil {
				t.Fatal(err)
			}
			doBackup = true
		}
		defer func() {
			if doBackup {
				if err := os.Rename(config.FilePath()+".backup", config.FilePath()); err != nil {
					t.Fatal(err)
				}
			}
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			if err := Execute(); err != nil {
				t.Error(err)
			}
		})

		if !fileutil.IsFile(config.FilePath()) {
			t.Error(config.FilePath() + " does not exist. failed to generate")
			continue
		}

		got, err := os.ReadFile(config.FilePath())
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != tt.want {
			t.Errorf("mismatch: want=%s, got=%s", tt.want, string(got))
		}
	}
}

func TestExecute_Export_WithOutputOption(t *testing.T) {
	setupXDGBase(t)

	type test struct {
		name  string
		gobin string
		args  []string
		want  []string
	}

	tests := []test{}
	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name:  testNameSuccess,
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{testCmdGup, testCmdExport, "--output"},
			want: []string{
				"{",
				`  "schema_version": 1,`,
				`  "packages": [`,
				`      "name": "gal.exe",`,
				`      "name": "posixer.exe",`,
				`      "version": "latest",`,
				""},
		})
	} else {
		tests = append(tests, test{
			name:  testNameSuccess,
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{testCmdGup, testCmdExport, "--output"},
			want: []string{
				"{",
				`  "schema_version": 1,`,
				`  "packages": [`,
				`      "name": "gal",`,
				`      "name": "posixer",`,
				`      "name": "subaru",`,
				`      "version": "v1.1.1",`,
				`      "version": "v0.1.0",`,
				`      "version": "v1.0.0",`,
				""},
		})
	}

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		got, err := helper_runGup(t, tt.args)
		if err != nil {
			t.Fatal(err)
		}

		for _, want := range tt.want {
			if want == "" {
				continue
			}
			contains := false
			for _, g := range got {
				if strings.Contains(g, want) {
					contains = true
					break
				}
			}
			if !contains {
				t.Errorf("value is mismatch. output does not contain %q", want)
			}
		}
	}
}

func TestExecute_Import_WithInputOption(t *testing.T) {
	setupXDGBase(t)
	helper_stubImportInstaller(t)

	gobinDir := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobinDir)
	if err := os.MkdirAll(gobinDir, 0o750); err != nil {
		t.Fatal(err)
	}

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	confFile := "testdata/gup_config/nix.json"
	if runtime.GOOS == goosWindows {
		confFile = "testdata/gup_config/windows.json"
	}

	got, err := helper_runGup(t, []string{testCmdGup, testCmdImport, "-f", confFile})
	if err != nil {
		t.Fatal(err)
	}

	contain := false
	for _, v := range got {
		if strings.Contains(v, "[1/1] github.com/nao1215/gup") {
			contain = true
		}
	}

	if !contain {
		t.Error("failed import")
	}
}

func TestExecute_Import_WithBadInputFile(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name      string
		inputFile string
		want      []string
	}{
		{
			name:      "specify not exist file",
			inputFile: "not_exist_file_path",
			want: []string{
				"gup:ERROR: not_exist_file_path is not found",
				"",
			},
		},
		{
			name:      "specify empty file",
			inputFile: "testdata/gup_config/empty.json",
			want: []string{
				"gup:ERROR: unable to import package: no package information",
				"",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			OsExit = func(code int) {}
			defer func() {
				OsExit = os.Exit
			}()

			got, err := helper_runGup(t, []string{testCmdGup, testCmdImport, "-f", tt.inputFile})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExecute_Update(t *testing.T) {
	setupXDGBase(t)
	deps := stubUpdateDeps()
	helper_setupFakeGoBin(t)
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(gobin); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(gobin, 0750); err != nil {
		t.Fatal(err)
	}

	var targetPath, binName string
	if runtime.GOOS == goosWindows {
		binName = "gal.exe"
		targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
	} else {
		binName = "gal"
		targetPath = filepath.Join("testdata", "check_success", binName)
	}

	helper_CopyFile(t, targetPath, filepath.Join(gobin, binName))

	if err = os.Chmod(filepath.Join(gobin, binName), 0600); err != nil {
		t.Fatal(err)
	}

	got := helper_runUpdateWithDeps(t, deps)

	contain := false
	for _, v := range got {
		if strings.Contains(v, "[1/1] github.com/nao1215/gal/cmd/gal") {
			contain = true
		}
	}
	if !contain {
		t.Errorf("failed to update gal command")
	}
}

func TestExecute_Update_DryRunAndNotify(t *testing.T) {
	setupXDGBase(t)
	deps := stubUpdateDeps()
	helper_setupFakeGoBin(t)
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(gobin); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(gobin, 0750); err != nil {
		t.Fatal(err)
	}

	var targetPath, binName string
	if runtime.GOOS == goosWindows {
		binName = testBinPosixerExe
		targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
	} else {
		binName = "posixer"
		targetPath = filepath.Join("testdata", "check_success", binName)
	}

	helper_CopyFile(t, targetPath, filepath.Join(gobin, binName))

	if err = os.Chmod(filepath.Join(gobin, binName), 0600); err != nil {
		t.Fatal(err)
	}

	got := helper_runUpdateWithDeps(t, deps, testFlagDryRun, testFlagNotify)

	contain := false
	for _, v := range got {
		if strings.Contains(v, "[1/1] github.com/nao1215/posixer") {
			contain = true
		}
	}
	if !contain {
		t.Errorf("failed to update posixer command")
	}
}

// assetsDirForTest returns the directory where notification icons are deployed.
func assetsDirForTest() string {
	return filepath.Join(config.DirPath(), "assets")
}

func TestExecute_NoAssetsForReadOnlyCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: testCmdVersion, args: []string{testCmdGup, testCmdVersion}},
		{name: testCmdHelp, args: []string{testCmdGup, testCmdHelp}},
		{name: "completion bash", args: []string{testCmdGup, testCmdCompletion, testShellBash}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupXDGBase(t)

			if _, err := helper_runGupCaptureAllOutput(t, tt.args); err != nil {
				t.Fatal(err)
			}

			if fileutil.IsDir(assetsDirForTest()) {
				t.Errorf("read-only command %q created assets directory %s", tt.name, assetsDirForTest())
			}
		})
	}
}

func TestExecute_AssetsDeployedForNotifyCommand(t *testing.T) {
	setupXDGBase(t)
	deps := stubUpdateDeps()
	helper_setupFakeGoBin(t)
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gobin); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gobin, 0750); err != nil {
		t.Fatal(err)
	}

	binName := testBinPosixer
	targetPath := filepath.Join("testdata", "check_success", binName)
	if runtime.GOOS == goosWindows {
		binName = testBinPosixer + ".exe"
		targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
	}
	helper_CopyFile(t, targetPath, filepath.Join(gobin, binName))
	if err = os.Chmod(filepath.Join(gobin, binName), 0600); err != nil {
		t.Fatal(err)
	}

	if fileutil.IsDir(assetsDirForTest()) {
		t.Fatalf("assets directory %s should not exist before notify command runs", assetsDirForTest())
	}

	_ = helper_runUpdateWithDeps(t, deps, testFlagDryRun, testFlagNotify)

	if !fileutil.IsDir(assetsDirForTest()) {
		t.Errorf("notify command did not deploy assets directory %s", assetsDirForTest())
	}
	if !fileutil.IsFile(assets.InfoIconPath()) {
		t.Errorf("notify command did not deploy info icon %s", assets.InfoIconPath())
	}
	if !fileutil.IsFile(assets.WarningIconPath()) {
		t.Errorf("notify command did not deploy warning icon %s", assets.WarningIconPath())
	}
}

// TestExecute_Completion drives 'gup completion --install' through the process
// entrypoint and checks that the files a user would then have actually exist.
// The set differs by platform because the platforms offer different shells:
// bash, fish and zsh on Linux and macOS, PowerShell on Windows, where none of
// those completion directories exist and the completer is dot-sourced from the
// profile instead.
func TestExecute_Completion(t *testing.T) {
	setupXDGBase(t)
	home := os.Getenv("HOME")

	// The Windows install resolves its profile from PROFILE (or USERPROFILE),
	// not from HOME, so both are redirected into the test's temp tree. Without
	// this the test would write a completer and a profile block into the real
	// user's Documents folder on any Windows machine that runs it.
	profile := filepath.Join(home, "Microsoft.PowerShell_profile.ps1")
	t.Setenv("USERPROFILE", home)
	t.Setenv("PROFILE", profile)

	t.Run("generate completion file", func(t *testing.T) {
		os.Args = []string{testCmdGup, testCmdCompletion, testFlagInstall}
		if err := Execute(); err != nil {
			t.Fatal(err)
		}

		posixFiles := []string{
			filepath.Join(home, ".local", "share", "bash-completion", "completions", cmdinfo.Name),
			filepath.Join(home, ".config", testShellFish, "completions", cmdinfo.Name+".fish"),
			filepath.Join(home, ".zsh", testCmdCompletion, "_"+cmdinfo.Name),
		}
		windowsFiles := []string{
			profile,
			filepath.Join(home, "gup.completion.ps1"),
		}

		want, unwanted := posixFiles, windowsFiles
		if runtime.GOOS == goosWindows {
			want, unwanted = windowsFiles, posixFiles
		}
		for _, path := range want {
			if !fileutil.IsFile(path) {
				t.Errorf("completion --install did not generate %s", path)
			}
		}
		// The other platform's layout must stay untouched: installing bash
		// completion on Windows, or a PowerShell profile on Linux, would be
		// writing into a shell the user does not have.
		for _, path := range unwanted {
			if fileutil.IsFile(path) {
				t.Errorf("completion --install generated %s, which does not belong on %s", path, runtime.GOOS)
			}
		}
	})
}

func TestExecute_CompletionForShell(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		shell      string
		wantOutput bool
		wantErr    bool
		wantHeader string
	}{
		{
			shell:      testShellBash,
			wantOutput: true,
			wantErr:    false,
		},
		{
			shell:      testShellFish,
			wantOutput: true,
			wantErr:    false,
		},
		{
			shell:      testShellZsh,
			wantOutput: true,
			wantErr:    false,
		},
		{
			shell:      testShellPowershell,
			wantOutput: true,
			wantErr:    false,
			wantHeader: "# powershell completion for gup",
		},
		{
			shell:      "unknown-shell",
			wantOutput: false,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got, err := helper_runGupCaptureAllOutput(t, []string{testCmdGup, testCmdCompletion, tt.shell})

			gotErr := err != nil
			if tt.wantErr != gotErr {
				t.Errorf("expected error return %v, got %v", tt.wantErr, gotErr)
			}

			gotOutput := strings.TrimSpace(got) != ""
			if tt.wantOutput != gotOutput {
				t.Errorf("expected output %v, got %v", tt.wantOutput, gotOutput)
			}
			if tt.wantHeader != "" && !strings.Contains(got, tt.wantHeader) {
				t.Errorf("completion output does not include %q", tt.wantHeader)
			}
		})
	}
}

// helper_shellCompletion drives cobra's hidden "__complete" command through the
// real root command, the same way a shell does. It returns the candidate names
// and the directive line so the wiring - not just the helper functions - is
// covered.
func helper_shellCompletion(t *testing.T, args ...string) ([]string, cobra.ShellCompDirective) {
	t.Helper()

	rootCmd := newRootCmd()
	stdout := &bytes.Buffer{}
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs(append([]string{cobra.ShellCompRequestCmd}, args...))

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("__complete %v: %v", args, err)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) == 0 {
		t.Fatalf("__complete %v printed nothing", args)
	}
	directiveLine := lines[len(lines)-1]
	if !strings.HasPrefix(directiveLine, ":") {
		t.Fatalf("__complete %v: last line %q is not a directive", args, directiveLine)
	}
	directive, err := strconv.Atoi(strings.TrimPrefix(directiveLine, ":"))
	if err != nil {
		t.Fatalf("__complete %v: can't parse directive %q: %v", args, directiveLine, err)
	}

	candidates := []string{}
	for _, line := range lines[:len(lines)-1] {
		if line == "" {
			continue
		}
		// cobra separates a candidate from its description with a tab.
		candidates = append(candidates, strings.SplitN(line, "\t", 2)[0])
	}
	return candidates, cobra.ShellCompDirective(directive)
}

// Test_shellCompletion_arguments runs the completions a shell actually requests,
// through the assembled command tree. It is the regression net for the argument
// completion contracts: migrate reads its own BEFORE_PATH, pin/unpin only accept
// one binary, and --timeout never completes file names.
func Test_shellCompletion_arguments(t *testing.T) {
	gobin := helper_makeBinaries(t, testBinGobinOnly)
	t.Setenv("GOBIN", gobin)
	before := helper_makeBinaries(t, testBinBeforeOnly)
	after := t.TempDir()

	tests := []struct {
		name          string
		args          []string
		want          []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			name:          "update completes $GOBIN binaries",
			args:          []string{testCmdUpdate, ""},
			want:          []string{testBinGobinOnly},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "migrate completes directories for BEFORE_PATH",
			args:          []string{testCmdMigrate, ""},
			want:          []string{},
			wantDirective: cobra.ShellCompDirectiveFilterDirs,
		},
		{
			name:          "migrate completes BINARY from BEFORE_PATH, not $GOBIN",
			args:          []string{testCmdMigrate, before, after, ""},
			want:          []string{testBinBeforeOnly},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "pin completes a binary for its first argument",
			args:          []string{testCmdPin, ""},
			want:          []string{testBinGobinOnly},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "pin does not complete a binary for VERSION",
			args:          []string{testCmdPin, testBinGobinOnly, ""},
			want:          []string{},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "unpin does not complete past its single argument",
			args:          []string{"unpin", testBinGobinOnly, ""},
			want:          []string{},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "--timeout does not complete file names",
			args:          []string{testCmdUpdate, testFlagTimeout, ""},
			want:          []string{},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, directive := helper_shellCompletion(t, tt.args...)
			if directive != tt.wantDirective {
				t.Errorf("directive = %v, want %v", directive, tt.wantDirective)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("completions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
