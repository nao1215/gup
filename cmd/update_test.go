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
		cmd.Flags().Bool("ignore-go-update", false, "Ignore updates to the Go toolchain")
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
