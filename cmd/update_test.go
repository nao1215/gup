package cmd

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

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
			name: "paser --dry-run argument error",
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
			name: "paser --notify argument error",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "paser --dry-run argument error" {
				tt.args.cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
			} else if tt.name != "paser --notify argument error" {
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

func Test_extractUserSpecifyPkg(t *testing.T) {
	type args struct {
		pkgs    []goutil.Package
		targets []string
	}
	tests := []struct {
		name string
		args args
		want []goutil.Package
	}{
		{
			name: "find user specify package",
			args: args{
				pkgs: []goutil.Package{
					{
						Name: "test1",
					},
					{
						Name: "test2",
					},
					{
						Name: "test3",
					},
				},
				targets: []string{"test2"},
			},
			want: []goutil.Package{
				{
					Name: "test2",
				},
			},
		},
		{
			name: "can notfind user specify package",
			args: args{
				pkgs: []goutil.Package{
					{
						Name: "test1",
					},
					{
						Name: "test2",
					},
					{
						Name: "test3",
					},
				},
				targets: []string{"test4"},
			},
			want: []goutil.Package{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractUserSpecifyPkg(tt.args.pkgs, tt.args.targets); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractUserSpecifyPkg() = %v, want %v", got, tt.want)
			}
		})
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

		cmd := &cobra.Command{}
		cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
		cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
		if got := gup(cmd, []string{}); got != 1 {
			t.Errorf("gup() = %v, want %v", got, 1)
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
