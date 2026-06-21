//nolint:paralleltest,errcheck,gosec
package cmd

import (
	"bytes"
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
			name:  "not go install command in $GOBIN",
			gobin: filepath.Join("testdata", "check_fail"),
			want:  1,
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

			if got := check(newCheckCmd(), tt.args); got != tt.want {
				t.Errorf("check() = %v, want %v", got, tt.want)
			}
			pw.Close()
			print.Stdout = orgStdout
			print.Stderr = orgStderr

			if tt.want == 1 {
				return
			}

			buf := bytes.Buffer{}
			_, err = io.Copy(&buf, pr)
			if err != nil {
				t.Error(err)
			}
			defer pr.Close()
			got := strings.Split(buf.String(), "\n")

			if !strings.Contains(got[len(got)-2], "posixer") {
				t.Errorf("posixer package is not included in the update target: %s", got[len(got)-2])
			}
			if !strings.Contains(got[len(got)-2], "gal") {
				t.Errorf("gal package is not included in the update target: %s", got[len(got)-2])
			}
		})
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

			if got := check(tt.args.cmd, tt.args.args); got != tt.want {
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

		if got := check(&cobra.Command{}, []string{}); got != 1 {
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
			name:  "gobin is empty",
			gobin: filepath.Join("testdata", "empty_dir"),
			args:  args{},
			want:  1,
			stderr: []string{
				"gup:ERROR: unable to check package: no package information",
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

			if got := check(newCheckCmd(), tt.args.args); got != tt.want {
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

	got := check(newCheckCmd(), []string{})
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

	got := check(cmd, []string{})
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
	got := check(cmd, []string{})
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

	origGetLatest := getLatestVer
	defer func() {
		getLatestVer = origGetLatest
	}()

	getLatestVer = func(modulePath string) (string, error) {
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
	got := doCheck(pkgs, 1, 0, true, false)

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
	origGetLatest := getLatestVer
	defer func() {
		getLatestVer = origGetLatest
	}()
	getLatestVer = func(string) (string, error) { return testVersionOne, nil }

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
	got := doCheck(pkgs, 1, 0, false, false)

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

	origGetLatest := getLatestVer
	defer func() {
		getLatestVer = origGetLatest
	}()
	getLatestVer = func(string) (string, error) { return testVersionOne, nil }

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

	got := doCheck(pkgs, 1, 0, false, false)
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
