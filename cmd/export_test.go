package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/config"
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
		t.Setenv("PATH", "")

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
			gobin: filepath.Join("testdata", "text"),
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
			gobin: filepath.Join("testdata", "dummy"),
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get binary-paths installed by 'go install': open " + filepath.Join("testdata", "dummy") + ": The system cannot find the file specified.",
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
			gobin: filepath.Join("testdata", "dummy"),
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get binary-paths installed by 'go install': open testdata/dummy: no such file or directory",
				"",
			},
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOBIN", tt.gobin)

			if tt.name == "can not make .config directory" {
				oldHome := xdg.ConfigHome
				xdg.ConfigHome = filepath.Join("/", "root")
				defer func() {
					xdg.ConfigHome = oldHome
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

			tt.args.cmd.Flags().BoolP("output", "o", false, "print command path information at STDOUT")
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
				if file.IsFile(filepath.Join("/", ".config")) {
					t.Errorf("permissions are incomplete because '/.config' was created")
				}
			}
		})
	}
}

func Test_export_parse_error(t *testing.T) {
	t.Run("parse argument error", func(t *testing.T) {
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

		want := []string{
			"gup:ERROR: can not parse command line argument (--output): flag accessed but not defined: output",
			"",
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	})
}

func Test_writeConfigFile(t *testing.T) {
	type args struct {
		pkgs []goutil.Package
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "failed to open config file",
			args: args{
				pkgs: []goutil.Package{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.ConfigFileName = ""
			defer func() { config.ConfigFileName = "gup.conf" }()

			if err := writeConfigFile(tt.args.pkgs); (err != nil) != tt.wantErr {
				t.Errorf("writeConfigFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
