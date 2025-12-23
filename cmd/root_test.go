// Package cmd define subcommands provided by the gup command
//
//nolint:paralleltest
package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/google/go-cmp/cmp"
	"github.com/nao1215/gorky/file"
	"github.com/nao1215/gup/internal/cmdinfo"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

func helper_CopyFile(t *testing.T, src, dst string) {
	t.Helper()

	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close() //nolint:errcheck // ignore close error in test

	out, err := os.Create(filepath.Clean(dst))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close() //nolint:errcheck // ignore close error in test

	_, err = io.Copy(out, in)
	if err != nil {
		t.Fatal(err)
	}
}

func helper_setupFakeGoBin(t *testing.T) {
	t.Helper()

	absGobin, err := filepath.Abs(filepath.Join("testdata", "gobin_tmp"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOBIN", absGobin)

	// failsafe to ensure fake GOBIN has been set
	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(gobin, "_tmp") {
		t.Fatal("SHOULD NOT HAPPEN: GOBIN is not set to fake path")
	}
}

// Runs a gup command, and return its output split by \n
func helper_runGup(t *testing.T, args []string) ([]string, error) {
	t.Helper()

	// Redirect output
	orgStdout := print.Stdout
	orgStderr := print.Stderr
	defer func() {
		print.Stdout = orgStdout
		print.Stderr = orgStderr
	}()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close() //nolint:errcheck // ignore close error in test
	print.Stdout = pw
	print.Stderr = pw

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	// Run command
	os.Args = args
	err = Execute()
	pw.Close() //nolint:errcheck,gosec // ignore close error in test
	if err != nil {
		return nil, err
	}

	// Get output
	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, pr)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(buf.String(), "\n")

	return got, nil
}

func setupXDGBase(t *testing.T) {
	t.Helper()

	origHome := os.Getenv("HOME")
	origConfig := xdg.ConfigHome
	origData := xdg.DataHome
	origCache := xdg.CacheHome

	t.Cleanup(func() {
		_ = os.Unsetenv("HOME")
		if origHome != "" {
			_ = os.Setenv("HOME", origHome)
		}
		xdg.ConfigHome = origConfig
		xdg.DataHome = origData
		xdg.CacheHome = origCache
	})

	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("GOTELEMETRY", "off") // prevent Go toolchain telemetry files in temp home that break cleanup
	xdg.ConfigHome = filepath.Join(base, "config")
	xdg.DataHome = filepath.Join(base, "data")
	xdg.CacheHome = filepath.Join(base, "cache")

	for _, dir := range []string{xdg.ConfigHome, xdg.DataHome, xdg.CacheHome} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create XDG directory %s: %v", dir, err)
		}
	}
}

func TestExecute(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "success",
			args:    []string{""},
			wantErr: false,
		},
		{
			name:    "fail",
			args:    []string{"no-exist-subcommand", "--no-exist-option"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			err := Execute()
			gotErr := err != nil
			if tt.wantErr != gotErr {
				t.Errorf("expected error return %v, got %v: %v", tt.wantErr, gotErr, err)
			}
		})
	}
}

func TestExecute_Check(t *testing.T) {
	setupXDGBase(t)

	if runtime.GOOS == goosWindows {
		gobinDir := filepath.Join("testdata", "check_success_for_windows")
		t.Setenv("GOBIN", gobinDir)
	} else {
		gobinDir := filepath.Join("testdata", "check_success")
		t.Setenv("GOBIN", gobinDir)
	}

	got, err := helper_runGup(t, []string{"gup", "check"})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got[len(got)-2], "posixer") {
		t.Errorf("posixer package is not included in the update target: %s", got[len(got)-2])
	}
	if !strings.Contains(got[len(got)-2], "gal") {
		t.Errorf("gal package is not included in the update target: %s", got[len(got)-2])
	}
}

func TestExecute_Version(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name   string
		args   []string
		stdout []string
	}{
		{
			name:   "success",
			args:   []string{"gup", "version"},
			stdout: []string{"gup version (devel) (under Apache License version 2.0)", ""},
		},
	}
	for _, tt := range tests {
		got, err := helper_runGup(t, tt.args)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(tt.stdout, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestExecute_List(t *testing.T) {
	setupXDGBase(t)

	type test struct {
		name   string
		gobin  string
		args   []string
		stdout []string
		want   int
	}
	tests := []test{}

	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name:  "check success in windows",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{"gup", "list"},
			stdout: []string{
				"github.com/nao1215/gal/cmd/gal",
				"github.com/nao1215/posixer",
			},
			want: 2,
		})
	} else {
		tests = append(tests, test{
			name:  "check success in nix family",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{"gup", "list"},
			stdout: []string{
				"    gal: github.com/nao1215/gal/cmd/gal@v1.1.1",
				"posixer: github.com/nao1215/posixer@v0.1.0",
				" subaru: github.com/nao1215/subaru@v1.0.0",
			},
			want: 3,
		})
	}
	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		got, err := helper_runGup(t, tt.args)
		if err != nil {
			t.Fatal(err)
		}

		count := 0
		for _, g := range got {
			for _, w := range tt.stdout {
				if strings.Contains(g, w) {
					count++
				}
			}
		}
		if count != tt.want {
			t.Errorf("value is mismatch. want=%d got=%d", tt.want, count)
		}
	}
}

func TestExecute_Remove_Force(t *testing.T) {
	setupXDGBase(t)

	type test struct {
		name   string
		gobin  string
		args   []string
		stdout []string
	}
	tests := []test{}

	src := ""
	dest := ""
	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name:   "success",
			gobin:  filepath.Join("testdata", "delete_force"),
			args:   []string{"gup", "remove", "-f", "posixer.exe"},
			stdout: []string{},
		})
		if err := os.MkdirAll(filepath.Join("testdata", "delete_force"), 0750); err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(filepath.Join("testdata", "delete_force"))
			if err != nil {
				t.Fatal(err)
			}
		}()
		src = filepath.Join("testdata", "check_success_for_windows", "posixer.exe")
		dest = filepath.Join("testdata", "delete_force", "posixer.exe")
	} else {
		tests = append(tests, test{
			name:   "success",
			gobin:  filepath.Join("testdata", "delete"),
			args:   []string{"gup", "remove", "-f", "posixer"},
			stdout: []string{},
		})
		if err := os.MkdirAll(filepath.Join("testdata", "delete"), 0750); err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(filepath.Join("testdata", "delete"))
			if err != nil {
				t.Fatal(err)
			}
		}()
		src = filepath.Join("testdata", "check_success", "posixer")
		dest = filepath.Join("testdata", "delete", "posixer")
	}

	helper_CopyFile(t, src, dest)

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			if err := Execute(); err != nil {
				t.Error(err)
			}
		})

		if file.IsFile(dest) {
			t.Errorf("failed to remove posixer command")
		}
	}
}

func TestExecute_Export(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name  string
		gobin string
		args  []string
		want  string
	}{}

	if runtime.GOOS == goosWindows {
		tests = append(tests, struct {
			name  string
			gobin string
			args  []string
			want  string
		}{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{"gup", "export"},
			want: `gal.exe = github.com/nao1215/gal/cmd/gal
posixer.exe = github.com/nao1215/posixer
`,
		})
	} else {
		tests = append(tests, struct {
			name  string
			gobin string
			args  []string
			want  string
		}{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{"gup", "export"},
			want: `gal = github.com/nao1215/gal/cmd/gal
posixer = github.com/nao1215/posixer
subaru = github.com/nao1215/subaru
`,
		})
	}

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		OsExit = func(code int) {}
		defer func() {
			OsExit = os.Exit
		}()

		doBackup := false
		if file.IsFile(config.FilePath()) {
			if err := os.Rename(config.FilePath(), config.FilePath()+".backup"); err != nil {
				t.Fatal(err)
			}
			doBackup = true
		}
		defer func() {
			if doBackup {
				if err := os.Rename(config.FilePath()+".backup", config.FilePath()); err != nil {
					t.Fatal(err)
				}
			}
		}()

		os.Args = tt.args
		t.Run(tt.name, func(t *testing.T) {
			if err := Execute(); err != nil {
				t.Error(err)
			}
		})

		if !file.IsFile(config.FilePath()) {
			t.Error(config.FilePath() + " does not exist. failed to generate")
			continue
		}

		got, err := os.ReadFile(config.FilePath())
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != tt.want {
			t.Errorf("mismatch: want=%s, got=%s", tt.want, string(got))
		}
	}
}

func TestExecute_Export_WithOutputOption(t *testing.T) {
	setupXDGBase(t)

	type test struct {
		name  string
		gobin string
		args  []string
		want  []string
	}

	tests := []test{}
	if runtime.GOOS == goosWindows {
		tests = append(tests, test{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success_for_windows"),
			args:  []string{"gup", "export", "--output"},
			want: []string{
				"gal.exe = github.com/nao1215/gal/cmd/gal",
				"posixer.exe = github.com/nao1215/posixer",
				""},
		})
	} else {
		tests = append(tests, test{
			name:  "success",
			gobin: filepath.Join("testdata", "check_success"),
			args:  []string{"gup", "export", "--output"},
			want: []string{
				"gal = github.com/nao1215/gal/cmd/gal",
				"posixer = github.com/nao1215/posixer",
				"subaru = github.com/nao1215/subaru",
				""},
		})
	}

	for _, tt := range tests {
		t.Setenv("GOBIN", tt.gobin)

		got, err := helper_runGup(t, tt.args)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("value is mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestExecute_Import_WithInputOption(t *testing.T) {
	if os.Getenv("GUP_RUN_UPDATE_TESTS") != "1" {
		t.Skip("skip import/install test without explicit opt-in")
	}

	setupXDGBase(t)

	gobinDir := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobinDir)
	if err := os.MkdirAll(gobinDir, 0o750); err != nil {
		t.Fatal(err)
	}

	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	confFile := "testdata/gup_config/nix.conf"
	if runtime.GOOS == goosWindows {
		confFile = "testdata/gup_config/windows.conf"
	}

	got, err := helper_runGup(t, []string{"gup", "import", "-i", confFile})
	if err != nil {
		t.Fatal(err)
	}

	contain := false
	for _, v := range got {
		if strings.Contains(v, "[1/1] github.com/nao1215/gup") {
			contain = true
		}
	}

	if !contain {
		t.Error("failed import")
	}
}

func TestExecute_Import_WithBadInputFile(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		name      string
		inputFile string
		want      []string
	}{
		{
			name:      "specify not exist file",
			inputFile: "not_exist_file_path",
			want: []string{
				"gup:ERROR: not_exist_file_path is not found",
				"",
			},
		},
		{
			name:      "specify empty file",
			inputFile: "testdata/gup_config/empty.conf",
			want: []string{
				"gup:ERROR: unable to import package: no package information",
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

			got, err := helper_runGup(t, []string{"gup", "import", "-i", tt.inputFile})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExecute_Update(t *testing.T) {
	if os.Getenv("GUP_RUN_UPDATE_TESTS") != "1" {
		t.Skip("skip update test without explicit opt-in")
	}

	setupXDGBase(t)
	helper_setupFakeGoBin(t)
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(gobin); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(gobin, 0750); err != nil {
		t.Fatal(err)
	}

	targetPath := ""
	binName := ""
	if runtime.GOOS == goosWindows {
		binName = "gal.exe"
		targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
	} else {
		binName = "gal"
		targetPath = filepath.Join("testdata", "check_success", binName)
	}

	helper_CopyFile(t, targetPath, filepath.Join(gobin, binName))

	if err = os.Chmod(filepath.Join(gobin, binName), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := helper_runGup(t, []string{"gup", "update"})
	if err != nil {
		t.Fatal(err)
	}

	contain := false
	for _, v := range got {
		if strings.Contains(v, "[1/1] github.com/nao1215/gal/cmd/gal") {
			contain = true
		}
	}
	if !contain {
		t.Errorf("failed to update gal command")
	}
}

func TestExecute_Update_DryRunAndNotify(t *testing.T) {
	if os.Getenv("GUP_RUN_UPDATE_TESTS") != "1" {
		t.Skip("skip update test without explicit opt-in")
	}

	setupXDGBase(t)
	helper_setupFakeGoBin(t)
	OsExit = func(code int) {}
	defer func() {
		OsExit = os.Exit
	}()

	gobin, err := goutil.GoBin()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(gobin); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(gobin, 0750); err != nil {
		t.Fatal(err)
	}

	targetPath := ""
	binName := ""
	if runtime.GOOS == goosWindows {
		binName = "posixer.exe"
		targetPath = filepath.Join("testdata", "check_success_for_windows", binName)
	} else {
		binName = "posixer"
		targetPath = filepath.Join("testdata", "check_success", binName)
	}

	helper_CopyFile(t, targetPath, filepath.Join(gobin, binName))

	if err = os.Chmod(filepath.Join(gobin, binName), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := helper_runGup(t, []string{"gup", "update", "--dry-run", "--notify"})
	if err != nil {
		t.Fatal(err)
	}

	contain := false
	for _, v := range got {
		if strings.Contains(v, "[1/1] github.com/nao1215/posixer") {
			contain = true
		}
	}
	if !contain {
		t.Errorf("failed to update posixer command")
	}
}

func TestExecute_Completion(t *testing.T) {
	setupXDGBase(t)

	t.Run("generate completion file", func(t *testing.T) {
		os.Args = []string{"gup", "completion"}
		if err := Execute(); err != nil {
			t.Error(err)
		}

		bash := filepath.Join(os.Getenv("HOME"), ".local", "share", "bash-completion", "completions", cmdinfo.Name)
		if runtime.GOOS == goosWindows {
			if file.IsFile(bash) {
				t.Errorf("generate %s, however shell completion file is not generated on Windows", bash)
			}
		} else {
			if !file.IsFile(bash) {
				t.Errorf("failed to generate %s", bash)
			}
		}

		fish := filepath.Join(os.Getenv("HOME"), ".config", "fish", "completions", cmdinfo.Name+".fish")
		if runtime.GOOS == goosWindows {
			if file.IsFile(fish) {
				t.Errorf("generate %s, however shell completion file is not generated on Windows", fish)
			}
		} else {
			if !file.IsFile(fish) {
				t.Errorf("failed to generate %s", fish)
			}
		}

		zsh := filepath.Join(os.Getenv("HOME"), ".zsh", "completion", "_"+cmdinfo.Name)
		if runtime.GOOS == goosWindows {
			if file.IsFile(zsh) {
				t.Errorf("generate %s, however shell completion file is not generated on Windows", zsh)
			}
		} else {
			if !file.IsFile(zsh) {
				t.Errorf("failed to generate  %s", zsh)
			}
		}
	})
}

func TestExecute_CompletionForShell(t *testing.T) {
	setupXDGBase(t)

	tests := []struct {
		shell      string
		wantOutput bool
		wantErr    bool
	}{
		{
			shell:      "bash",
			wantOutput: true,
			wantErr:    false,
		},
		{
			shell:      "fish",
			wantOutput: true,
			wantErr:    false,
		},
		{
			shell:      "zsh",
			wantOutput: true,
			wantErr:    false,
		},
		{
			shell:      "unknown-shell",
			wantOutput: false,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			got, err := helper_runGup(t, []string{"gup", "completion", tt.shell})

			gotErr := err != nil
			if tt.wantErr != gotErr {
				t.Errorf("expected error return %v, got %v", tt.wantErr, gotErr)
			}

			gotOutput := len(got) != 0
			if tt.wantOutput != gotOutput {
				t.Errorf("expected output %v, got %v", tt.wantOutput, gotOutput)
			}
		})
	}
}
