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
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func Test_list_not_found_go_command(t *testing.T) {
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

		if got := list(&cobra.Command{}, []string{}); got != 1 {
			t.Errorf("list() = %v, want %v", got, 1)
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
		defer func() {
			_ = pr.Close()
		}()
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

func Test_list_gobin_is_empty(t *testing.T) {
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
				"gup:ERROR: unable to list up package: no package information",
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
			name:  "$GOBIN is empty",
			gobin: "no_exist_dir",
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
			name:  "$GOBIN is empty",
			gobin: "no_exist_dir",
			args:  args{},
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get package info: can't get binary-paths installed by 'go install': open no_exist_dir: no such file or directory",
				"",
			},
		})
	}

	if err := os.Mkdir(filepath.Join("testdata", "empty_dir"), 0o755); err != nil { //nolint:gosec
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

			if got := list(tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("list() = %v, want %v", got, tt.want)
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
			defer func() {
				_ = pr.Close()
			}()
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
