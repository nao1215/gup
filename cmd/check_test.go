package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

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
			name: "paser --jobs argument error",
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

func Test_check(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	tests := []struct {
		name  string
		gobin string
		args  args
		want  int
	}{
		{
			name:  "not go install command in $GOBIN",
			gobin: filepath.Join("testdata", "check_fail"),
			args:  args{},
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

			cmd := &cobra.Command{}
			cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
			if got := check(cmd, tt.args.args); got != tt.want {
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
		if runtime.GOOS == "windows" {
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
		cmd  *cobra.Command
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

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name   string
			gobin  string
			args   args
			want   int
			stderr []string
		}{
			name:  "$GOBIN is empty",
			gobin: "no_exist_dir",
			args:  args{},
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get binary-paths installed by 'go install': open no_exist_dir: The system cannot find the file specified.",
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
			name:  "$GOBIN is empty",
			gobin: "no_exist_dir",
			args:  args{},
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get binary-paths installed by 'go install': open no_exist_dir: no such file or directory",
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

			cmd := &cobra.Command{}
			cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
			if got := check(cmd, tt.args.args); got != tt.want {
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
