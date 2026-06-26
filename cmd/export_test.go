//nolint:paralleltest
package cmd

import (
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
)

// testImportPathPosixer is the import path of the posixer fixture binary under
// testdata/check_success, shared across the export channel-preservation tests.
const testImportPathPosixer = "github.com/nao1215/posixer"

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
			p, _ := newTestPrinter()
			got := validPkgInfo(p, tt.args.pkgs)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("value is mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Test_export_succeeds_without_go_command reproduces the bug where 'gup export'
// failed with "you didn't install golang" when 'go' was absent, even though
// export only reads local build info from $GOBIN and writes gup.json, never
// invoking the Go toolchain.
//
//nolint:paralleltest // mutates process env (PATH, GOBIN, XDG)
func Test_export_succeeds_without_go_command(t *testing.T) {
	setupXDGBase(t)
	t.Setenv("PATH", "")
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	p, buf := newTestPrinter()
	cmd := newExportCmd()
	if err := cmd.Flags().Set("output", "true"); err != nil {
		t.Fatal(err)
	}
	if got := export(p, cmd, []string{}); got != 0 {
		t.Fatalf("export() without go = %d, want 0; output:\n%s", got, buf.String())
	}
	if strings.Contains(buf.String(), "you didn't install golang") {
		t.Errorf("export must not require the go command:\n%s", buf.String())
	}
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
			if tt.name == testNoConfigDir && runtime.GOOS == goosWindows {
				// This is a Unix permission test: it relies on "/root" being
				// unwritable. On Windows that path is writable, so the export
				// (which now writes an empty config on an empty env, #350) would
				// succeed. Permission tests are skipped on Windows in this repo.
				t.Skip("config-dir permission test is not portable on Windows")
			}

			t.Setenv("GOBIN", tt.gobin)

			if tt.name == testNoConfigDir {
				oldHome := xdg.ConfigHome
				xdg.ConfigHome = filepath.Join("/", "root")
				defer func() {
					xdg.ConfigHome = oldHome
				}()
			}

			p, buf := newTestPrinter()

			if got := export(p, newExportCmd(), tt.args); got != tt.want {
				t.Errorf("export() = %v, want %v", got, tt.want)
			}
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

// Test_applySavedChannels_prefersImportPath verifies that the saved channel is
// matched by import_path first (#341), so a binary keeps its channel even when
// its name differs from the saved entry.
// Test_export_preservesChannelsFromCanonicalConfig verifies the #341 contract:
// --file changes only the export destination, while saved channels are always
// resolved from the canonical user-level config. The destination here is a fresh
// file that has no channel data, so a wrong implementation would reset the
// channel to "latest".
func Test_export_preservesChannelsFromCanonicalConfig(t *testing.T) {
	if err := goutil.CanUseGoCmd(); err != nil {
		t.Skip("go command is not available")
	}

	origConfigHome := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = origConfigHome })
	xdg.ConfigHome = t.TempDir()

	// Canonical user-level config records posixer as tracked on @main.
	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	canonical := `{"schema_version":1,"packages":[{"name":"posixer","import_path":"` + testImportPathPosixer + `","version":"v0.1.0","channel":"main"}]}` + "\n"
	if err := os.WriteFile(config.FilePath(), []byte(canonical), 0o600); err != nil {
		t.Fatalf("failed to seed canonical config: %v", err)
	}

	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	dest := filepath.Join(t.TempDir(), "exported.json")
	cmd := newExportCmd()
	if err := cmd.Flags().Set("file", dest); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}

	p, _ := newTestPrinter()
	if got := export(p, cmd, []string{}); got != 0 {
		t.Fatalf("export() = %d, want 0", got)
	}

	exported, err := config.ReadConfFile(dest)
	if err != nil {
		t.Fatalf("failed to read exported config: %v", err)
	}
	var found bool
	for _, p := range exported {
		if p.ImportPath == testImportPathPosixer {
			found = true
			if p.UpdateChannel != goutil.UpdateChannelMain {
				t.Fatalf("posixer channel = %q, want %q (preserved from canonical config)", p.UpdateChannel, goutil.UpdateChannelMain)
			}
		}
	}
	if !found {
		t.Fatalf("posixer not found in exported config: %+v", exported)
	}
}

// Test_export_rejectsDirectoryDestination verifies the #367 contract through the
// export command entry point: `export --file <dir>` fails, the directory and its
// contents survive, and no temp/backup artifacts are left next to it.
func Test_export_rejectsDirectoryDestination(t *testing.T) {
	if err := goutil.CanUseGoCmd(); err != nil {
		t.Skip("go command is not available")
	}

	origConfigHome := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = origConfigHome })
	xdg.ConfigHome = t.TempDir()
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	parent := t.TempDir()
	dest := filepath.Join(parent, "gup.json")
	if err := os.Mkdir(dest, 0o750); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dest, "keep.txt")
	if err := os.WriteFile(child, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newExportCmd()
	if err := cmd.Flags().Set("file", dest); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}
	p, _ := newTestPrinter()
	if got := export(p, cmd, []string{}); got != 1 {
		t.Fatalf("export() = %d, want 1 when --file points to a directory", got)
	}

	if info, statErr := os.Stat(dest); statErr != nil || !info.IsDir() {
		t.Fatalf("destination directory should survive, stat err = %v", statErr)
	}
	if data, readErr := os.ReadFile(filepath.Clean(child)); readErr != nil || string(data) != "precious" {
		t.Fatalf("directory contents must be unchanged, read err = %v, data = %q", readErr, data)
	}
	assertNoTempFiles(t, parent, filepath.Base(dest))
}

// Test_export_failsFastOnMalformedChannelSource verifies the #369 contract: a
// malformed channel-source config makes export fail fast instead of silently
// exporting every package as @latest.
func Test_export_failsFastOnMalformedChannelSource(t *testing.T) {
	if err := goutil.CanUseGoCmd(); err != nil {
		t.Skip("go command is not available")
	}

	origConfigHome := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = origConfigHome })
	xdg.ConfigHome = t.TempDir()
	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(config.FilePath(), []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("failed to seed malformed config: %v", err)
	}
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	dest := filepath.Join(t.TempDir(), "exported.json")
	cmd := newExportCmd()
	if err := cmd.Flags().Set("file", dest); err != nil {
		t.Fatalf("failed to set --file: %v", err)
	}
	p, _ := newTestPrinter()
	if got := export(p, cmd, []string{}); got != 1 {
		t.Fatalf("export() = %d, want 1 on malformed channel source", got)
	}
	if fileutil.IsFile(dest) {
		t.Fatal("export must not write a destination file when the channel source is malformed")
	}
}

// Test_export_failsFastOnUnsupportedSchema verifies #369 for a config whose
// schema_version is not supported: export fails fast rather than dropping
// channels.
func Test_export_failsFastOnUnsupportedSchema(t *testing.T) {
	if err := goutil.CanUseGoCmd(); err != nil {
		t.Skip("go command is not available")
	}

	origConfigHome := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = origConfigHome })
	xdg.ConfigHome = t.TempDir()
	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	unsupported := `{"schema_version":999,"packages":[]}` + "\n"
	if err := os.WriteFile(config.FilePath(), []byte(unsupported), 0o600); err != nil {
		t.Fatalf("failed to seed unsupported-schema config: %v", err)
	}
	t.Setenv("GOBIN", filepath.Join("testdata", "check_success"))

	p, _ := newTestPrinter()
	if got := export(p, newExportCmd(), []string{}); got != 1 {
		t.Fatalf("export() = %d, want 1 on unsupported schema_version", got)
	}
}

// Test_export_emptyEnv_writesEmptyConfig verifies the #350 contract: exporting
// an empty environment succeeds (exit 0) and writes an empty gup.json instead of
// failing.
func Test_export_emptyEnv_writesEmptyConfig(t *testing.T) {
	if err := goutil.CanUseGoCmd(); err != nil {
		t.Skip("go command is not available")
	}

	origConfigHome := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = origConfigHome })
	xdg.ConfigHome = t.TempDir()

	t.Setenv("GOBIN", t.TempDir()) // existing but empty directory

	p, _ := newTestPrinter()
	if got := export(p, newExportCmd(), []string{}); got != 0 {
		t.Fatalf("export() on empty env = %d, want 0", got)
	}

	pkgs, err := config.ReadConfFile(config.FilePath())
	if err != nil {
		t.Fatalf("exported config should be readable: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("exported config should have no packages, got %d", len(pkgs))
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
