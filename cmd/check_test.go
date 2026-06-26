//nolint:paralleltest,errcheck,gosec
package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func Test_check(t *testing.T) {
	tests := []struct {
		name  string
		gobin string
		args  []string
		want  int
	}{
		{
			// testdata/check_fail holds a binary not installed by 'go install',
			// so gup finds zero manageable binaries. Per #350 this empty-but-valid
			// state is treated as a friendly first-run success (the unmanageable
			// binary is still surfaced via a warning).
			name:  "not go install command in $GOBIN",
			gobin: filepath.Join("testdata", "check_fail"),
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOBIN", tt.gobin)

			orgStdout := print.Stdout
			orgStderr := print.Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			print.Stdout = pw
			print.Stderr = pw

			if got := check(defaultDependencies(), newCheckCmd(), tt.args); got != tt.want {
				t.Errorf("check() = %v, want %v", got, tt.want)
			}
			pw.Close()
			print.Stdout = orgStdout
			print.Stderr = orgStderr

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			defer pr.Close()

			// An environment with no manageable binaries reports the friendly
			// first-run note and exits 0 (#350).
			if !strings.Contains(buf.String(), "no binaries are installed") {
				t.Errorf("expected empty-environment note, got: %s", buf.String())
			}
		})
	}
}

// Test_check_namedTargetNotInstalled verifies the #350 boundary: an empty
// result caused by the user naming a binary that is not installed is a usage
// error (exit 1), distinct from a genuinely empty environment (exit 0).
func Test_check_namedTargetNotInstalled(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	var got int
	out := captureCheckOutput(t, func() int {
		got = check(defaultDependencies(), newCheckCmd(), []string{"doesnotexist"})
		return got
	})
	if got != 1 {
		t.Fatalf("check(non-existent target) = %d, want 1", got)
	}
	if !strings.Contains(out, "no package information") {
		t.Errorf("expected a no-package-information error, got: %s", out)
	}
}

func Test_CheckOption(t *testing.T) {
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

			if got := check(defaultDependencies(), tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("check() = %v, want %v", got, tt.want)
			}
			pw.Close()
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

func Test_check_not_use_go_cmd(t *testing.T) {
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

		if got := check(defaultDependencies(), &cobra.Command{}, []string{}); got != 1 {
			t.Errorf("check() = %v, want %v", got, 1)
		}
		pw.Close()
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

func Test_check_gobin_is_empty(t *testing.T) {
	type args struct {
		args []string
	}
	tests := []struct {
		name   string
		gobin  string
		args   args
		want   int
		stderr []string
	}{
		{
			// An empty-but-valid environment is a normal first-run condition,
			// not an error (#350): check succeeds with an informational note.
			name:  "gobin is empty",
			gobin: filepath.Join("testdata", "empty_dir"),
			args:  args{},
			want:  0,
			stderr: []string{
				"no binaries are installed under $GOPATH/bin or $GOBIN",
				"",
			},
		},
	}

	if runtime.GOOS == goosWindows {
		tests = append(tests, struct {
			name   string
			gobin  string
			args   args
			want   int
			stderr []string
		}{
			name:  testGobinEmpty,
			gobin: testNoExistDir,
			args:  args{},
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get package info: can't get binary-paths installed by 'go install': open no_exist_dir: The system cannot find the file specified.",
				"",
			},
		})
	} else {
		tests = append(tests, struct {
			name   string
			gobin  string
			args   args
			want   int
			stderr []string
		}{
			name:  testGobinEmpty,
			gobin: testNoExistDir,
			args:  args{},
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get package info: can't get binary-paths installed by 'go install': open no_exist_dir: no such file or directory",
				"",
			},
		})
	}

	if err := os.Mkdir(filepath.Join("testdata", "empty_dir"), 0755); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOBIN", tt.gobin)

			orgStdout := print.Stdout
			orgStderr := print.Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			print.Stdout = pw
			print.Stderr = pw

			if got := check(defaultDependencies(), newCheckCmd(), tt.args.args); got != tt.want {
				t.Errorf("check() = %v, want %v", got, tt.want)
			}
			pw.Close()
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

	err := os.Remove(filepath.Join("testdata", "empty_dir"))
	if err != nil {
		t.Fatal(err)
	}
}

func Test_printUpdatablePkgInfo(t *testing.T) {
	type args struct {
		pkgs []goutil.Package
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "no package information",
			args: args{
				pkgs: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printUpdatablePkgInfo(tt.args.pkgs)
		})
	}
}

func Test_check_success(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	got := check(defaultDependencies(), newCheckCmd(), []string{})
	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, pr)
	if err != nil {
		t.Error(err)
	}
	pr.Close()

	// Should succeed (exit 0) or fail (exit 1) but not crash.
	// The check command runs network calls so we accept either result.
	if got != 0 && got != 1 {
		t.Errorf("check() = %v, want 0 or 1", got)
	}
}

func Test_check_ignoreGoUpdateFlag(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newCheckCmd()
	if err := cmd.Flags().Set("ignore-go-update", "true"); err != nil {
		t.Fatalf("failed to set ignore-go-update flag: %v", err)
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	got := check(defaultDependencies(), cmd, []string{})
	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, pr)
	if err != nil {
		t.Error(err)
	}
	pr.Close()

	if got != 0 && got != 1 {
		t.Errorf("check() = %v, want 0 or 1", got)
	}
}

func Test_check_jobsClamp(t *testing.T) {
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	cmd := newCheckCmd()
	if err := cmd.Flags().Set("jobs", "0"); err != nil {
		t.Fatalf("failed to set jobs flag: %v", err)
	}

	orgStdout := print.Stdout
	orgStderr := print.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	print.Stdout = pw
	print.Stderr = pw

	// Should not hang with jobs=0 (clamped to 1)
	got := check(defaultDependencies(), cmd, []string{})
	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr
	pr.Close()

	if got != 0 && got != 1 {
		t.Errorf("check() = %v, want 0 or 1", got)
	}
}

func Test_doCheck_modulePathChanged(t *testing.T) {
	const (
		oldModule = testOldModule
		newModule = testNewModule
	)

	deps := testDeps()
	deps.getLatestVer = func(_ context.Context, modulePath string) (string, error) {
		if modulePath == oldModule {
			return "", errors.New("version constraints conflict:\n" +
				"module declares its path as: " + newModule + "\n" +
				"but was required as: " + oldModule)
		}
		if modulePath == newModule {
			return testVersion123, nil
		}
		return "", errors.New("unexpected module path")
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
			Name:       testBinAir,
			ImportPath: "github.com/cosmtrek/air/cmd/air",
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
	got := doCheck(deps, pkgs, 1, 0, true, false)

	pw.Close()
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	_ = pr.Close()

	if got != 0 {
		t.Fatalf("doCheck() = %v, want 0", got)
	}
	if !strings.Contains(buf.String(), "$ gup update air ") {
		t.Fatalf("expected update hint for migrated module path, got:\n%s", buf.String())
	}
}

func Test_doCheck_customGoBuildTag_noFalsePositiveUpdate(t *testing.T) {
	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionOne, nil }

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
			Version: &goutil.Version{
				Current: testVersionOne,
			},
			GoVersion: &goutil.Version{
				Current: testGoVersionNoDwarf5,
				Latest:  testGoVersionNoDwarf5,
			},
		},
	}
	got := doCheck(deps, pkgs, 1, 0, false, false)

	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	print.Stdout = orgStdout
	print.Stderr = orgStderr

	buf := bytes.Buffer{}
	if _, err := io.Copy(&buf, pr); err != nil {
		t.Fatal(err)
	}
	_ = pr.Close()

	if got != 0 {
		t.Fatalf("doCheck() = %v, want 0", got)
	}
	if strings.Contains(buf.String(), "$ gup update tool ") {
		t.Fatalf("unexpected update hint for already-up-to-date package, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "Already up-to-date") {
		t.Fatalf("expected 'Already up-to-date' output, got:\n%s", buf.String())
	}
}

func Test_doCheck_customGoBuildTag_goVersionDiffColor(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = oldNoColor })

	deps := testDeps()
	deps.getLatestVer = func(context.Context, string) (string, error) { return testVersionOne, nil }

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
			Version: &goutil.Version{
				Current: testVersionOne,
			},
			GoVersion: &goutil.Version{
				Current: "go1.25.0-X:nodwarf5",
				Latest:  testGoVersionNoDwarf5,
			},
		},
	}

	got := doCheck(deps, pkgs, 1, 0, false, false)
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

	if got != 0 {
		t.Fatalf("doCheck() = %v, want 0", got)
	}
	if !strings.Contains(buf.String(), color.YellowString("go1.25.0-X:nodwarf5")) {
		t.Fatalf("expected current go version in yellow, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), color.GreenString(testGoVersionNoDwarf5)) {
		t.Fatalf("expected latest go version in green, got:\n%s", buf.String())
	}
}
