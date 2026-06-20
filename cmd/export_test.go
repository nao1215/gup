//nolint:paralleltest
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

	"github.com/adrg/xdg"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
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
			got := validPkgInfo(tt.args.pkgs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
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

func Test_export(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		gobin  string
		want   int
		stderr []string
	}{
		{
			name:   testNoConfigDir,
			gobin:  "",
			want:   1,
			stderr: []string{},
		},
		{
			name:  "no package information",
			gobin: filepath.Join("testdata", "text"),
			want:  1,
			stderr: []string{
				"gup:WARN : could not read Go build info from " + filepath.Join("testdata", "text", "dummy.txt") + ": unrecognized file format",
				"gup:ERROR: no package information",
				"",
			},
		},
	}

	if runtime.GOOS == goosWindows {
		tests = append(tests, struct {
			name   string
			args   []string
			gobin  string
			want   int
			stderr []string
		}{

			name:  "not exist gobin directory",
			gobin: filepath.Join("testdata", "dummy"),
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get package info: can't get binary-paths installed by 'go install': open " + filepath.Join("testdata", "dummy") + ": The system cannot find the file specified.",
				"",
			},
		})
	} else {
		tests = append(tests, struct {
			name   string
			args   []string
			gobin  string
			want   int
			stderr []string
		}{

			name:  "not exist gobin directory",
			gobin: filepath.Join("testdata", "dummy"),
			want:  1,
			stderr: []string{
				"gup:ERROR: can't get package info: can't get binary-paths installed by 'go install': open testdata/dummy: no such file or directory",
				"",
			},
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOBIN", tt.gobin)

			if tt.name == testNoConfigDir {
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

			if got := export(newExportCmd(), tt.args); got != tt.want {
				t.Errorf("export() = %v, want %v", got, tt.want)
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

			if tt.name != testNoConfigDir {
				if diff := cmp.Diff(tt.stderr, got); diff != "" {
					t.Errorf("value is mismatch (-want +got):\n%s", diff)
				}
			} else {
				if fileutil.IsFile(filepath.Join("/", ".config")) {
					t.Errorf("permissions are incomplete because '/.config' was created")
				}
			}
		})
	}
}

func Test_writeConfigFile(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("writeConfigFile permission test is not portable on Windows")
	}
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
			origConfig := xdg.ConfigHome
			t.Cleanup(func() { xdg.ConfigHome = origConfig })

			noWrite := filepath.Join(t.TempDir(), "deny")
			if err := os.MkdirAll(noWrite, 0o500); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}
			xdg.ConfigHome = noWrite

			if err := writeConfigFile(config.FilePath(), tt.args.pkgs); (err != nil) != tt.wantErr {
				t.Errorf("writeConfigFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeConfigFile_atomicOnWriteError(t *testing.T) {
	origWriteConfFile := writeConfFile
	t.Cleanup(func() { writeConfFile = origWriteConfFile })

	path := filepath.Join(t.TempDir(), "gup.json")
	original := `{"schema_version":1,"packages":[]}` + "\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("failed to seed original config: %v", err)
	}

	writeConfFile = func(w io.Writer, _ []goutil.Package) error {
		if _, err := w.Write([]byte(`{"schema_version":1,`)); err != nil {
			return err
		}
		return errors.New("forced write failure")
	}

	err := writeConfigFile(path, []goutil.Package{{Name: "dummy"}})
	if err == nil {
		t.Fatal("writeConfigFile() should return error")
	}

	got, readErr := os.ReadFile(filepath.Clean(path))
	if readErr != nil {
		t.Fatalf("failed to read config after failed write: %v", readErr)
	}
	if string(got) != original {
		t.Fatalf("config should remain unchanged on failure: got=%q want=%q", string(got), original)
	}

	tmpFiles, globErr := filepath.Glob(filepath.Join(filepath.Dir(path), "gup.json.tmp-*"))
	if globErr != nil {
		t.Fatalf("failed to list temp files: %v", globErr)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("temporary files should be cleaned up, found: %v", tmpFiles)
	}
}
