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
	"github.com/nao1215/gup/internal/file"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

func Test_validPkgInfo(t *testing.T) {
	type args struct {
		pkgs []goutil.Package
	}
	tests := []struct {
		name string
		args args
		want []goutil.Package
	}{
		{
			name: "old go version binary",
			args: args{
				pkgs: []goutil.Package{
					{
						Name:       "test",
						ImportPath: "",
					},
				},
			},
			want: []goutil.Package{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validPkgInfo(tt.args.pkgs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validPkgInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_export_not_use_go_cmd(t *testing.T) {
	t.Run("Not found go command", func(t *testing.T) {
		oldPATH := os.Getenv("PATH")
		if err := os.Setenv("PATH", ""); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Setenv("PATH", oldPATH); err != nil {
				t.Fatal(err)
			}
		}()

		orgStdout := print.Stdout
		orgStderr := print.Stderr
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		print.Stdout = pw
		print.Stderr = pw

		if got := export(&cobra.Command{}, []string{}); got != 1 {
			t.Errorf("export() = %v, want %v", got, 1)
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

func Test_export(t *testing.T) {
	type args struct {
		cmd  *cobra.Command
		args []string
	}
	tests := []struct {
		name   string
		args   args
		gobin  string
		want   int
		stderr []string
	}{
		{
			name: "can not make .config directory",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			gobin:  "",
			want:   1,
			stderr: []string{},
		},
		{
			name: "no package information",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			gobin: "testdata/text",
			want:  1,
			stderr: []string{
				"gup:WARN : can't get 'dummy.txt'package path information. old go version binary",
				"gup:ERROR: no package information",
				"",
			},
		},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name   string
			args   args
			gobin  string
			want   int
			stderr []string
		}{

			name: "not exist gobin directory",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			gobin: "testdata/dummy",
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get binary-paths installed by 'go install': open testdata/dummy: The system cannot find the file specified.",
				"",
			},
		})
	} else {
		tests = append(tests, struct {
			name   string
			args   args
			gobin  string
			want   int
			stderr []string
		}{

			name: "not exist gobin directory",
			args: args{
				cmd:  &cobra.Command{},
				args: []string{},
			},
			gobin: "testdata/dummy",
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get binary-paths installed by 'go install': open testdata/dummy: no such file or directory",
				"",
			},
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldGoBin := os.Getenv("GOBIN")
			if err := os.Setenv("GOBIN", tt.gobin); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Setenv("GOBIN", oldGoBin); err != nil {
					t.Fatal(err)
				}
			}()

			if tt.name == "can not make .config directory" {
				oldHome := os.Getenv("HOME")
				if err := os.Setenv("HOME", "/root"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.Setenv("HOME", oldHome); err != nil {
						t.Fatal(err)
					}
				}()
			}

			orgStdout := print.Stdout
			orgStderr := print.Stderr
			pr, pw, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			print.Stdout = pw
			print.Stderr = pw

			if got := export(tt.args.cmd, tt.args.args); got != tt.want {
				t.Errorf("export() = %v, want %v", got, tt.want)
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

			if tt.name != "can not make .config directory" {
				if diff := cmp.Diff(tt.stderr, got); diff != "" {
					t.Errorf("value is mismatch (-want +got):\n%s", diff)
				}
			} else {
				if file.IsFile("/.config") {
					t.Errorf("permissions are incomplete because '/.config' was created")
				}
			}
		})
	}
}
