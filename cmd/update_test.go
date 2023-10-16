package cmd

import (
	"bytes"
	"io"
	"os"
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
			name: "parser --jobs argument error",
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
			if tt.name == "parser --dry-run argument error" {
				tt.args.cmd.Flags().BoolP("notify", "N", false, "enable desktop notifications")
				tt.args.cmd.Flags().BoolP("jobs", "j", false, "Specify the number of CPU cores to use")
			} else if tt.name == "parser --notify argument error" {
				tt.args.cmd.Flags().BoolP("dry-run", "n", false, "perform the trial update with no changes")
				tt.args.cmd.Flags().BoolP("jobs", "j", false, "Specify the number of CPU cores to use")
			} else if tt.name == "parser --jobs argument error" {
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
			got := extractUserSpecifyPkg(tt.args.pkgs, tt.args.targets)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_excludeUserSpecifiedPkg(t *testing.T) {
	type args struct {
		pkgs           []goutil.Package
		excludePkgList []string
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
						Name: "pkg1",
					},
					{
						Name: "pkg2",
					},
					{
						Name: "pkg3",
					},
				},
				excludePkgList: []string{"pkg1", "pkg3"},
			},
			want: []goutil.Package{
				{
					Name: "pkg2",
				},
			},
		},
		{
			name: "find user specify package (exclude all package)",
			args: args{
				pkgs: []goutil.Package{
					{
						Name: "pkg1",
					},
					{
						Name: "pkg2",
					},
					{
						Name: "pkg3",
					},
				},
				excludePkgList: []string{"pkg1", "pkg2", "pkg3"},
			},
			want: []goutil.Package{},
		},
		{
			name: "If the excluded package does not exist",
			args: args{
				pkgs: []goutil.Package{
					{
						Name: "pkg1",
					},
					{
						Name: "pkg2",
					},
					{
						Name: "pkg3",
					},
				},
				excludePkgList: []string{"pkg4"},
			},
			want: []goutil.Package{
				{
					Name: "pkg1",
				},
				{
					Name: "pkg2",
				},
				{
					Name: "pkg3",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := excludePkgs(tt.args.excludePkgList, tt.args.pkgs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
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
		cmd.Flags().IntP("jobs", "j", runtime.NumCPU(), "Specify the number of CPU cores to use")
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

func TestExtractUserSpecifyPkg(t *testing.T) {
	pkgs := []goutil.Package{
		{Name: "pkg1"},
		{Name: "pkg2.exe"},
		{Name: "pkg3"},
	}
	targets := []string{"pkg1", "pkg2.exe"}
	if runtime.GOOS == "windows" {
		targets = []string{"pkg1", "pkg2"}
	}

	expected := []goutil.Package{
		{Name: "pkg1"},
		{Name: "pkg2.exe"},
	}
	actual := extractUserSpecifyPkg(pkgs, targets)

	if diff := cmp.Diff(actual, expected); diff != "" {
		t.Errorf("value is mismatch (-actual +expected):\n%s", diff)
	}
}
