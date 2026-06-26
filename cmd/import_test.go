//nolint:paralleltest
package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/spf13/cobra"
)

// validImportConf is a minimal gup.json with a single package entry.
const validImportConf = `{
  "schema_version": 1,
  "packages": [
    {
      "name": "posixer",
      "import_path": "github.com/nao1215/posixer",
      "version": "v0.1.0",
      "channel": "latest"
    }
  ]
}`

// chdirToTemp switches the working directory to a fresh temp dir and restores
// it on cleanup, so that config.LocalFilePath() ("./gup.json") is isolated.
// t.Chdir restores the directory before t.TempDir's RemoveAll runs, which
// avoids a Windows cleanup failure where the current directory cannot be
// removed while still in use.
func chdirToTemp(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func Test_runImport_flagErrors(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		want int
	}{
		{
			name: "missing dry-run flag",
			cmd:  &cobra.Command{},
			want: 1,
		},
		{
			name: "missing notify flag",
			cmd: func() *cobra.Command {
				c := &cobra.Command{}
				c.Flags().Bool("dry-run", false, "")
				c.Flags().String("file", "gup.json", "")
				return c
			}(),
			want: 1,
		},
		{
			name: "missing jobs flag",
			cmd: func() *cobra.Command {
				c := &cobra.Command{}
				c.Flags().Bool("dry-run", false, "")
				c.Flags().String("file", "gup.json", "")
				c.Flags().Bool("notify", false, "")
				return c
			}(),
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newTestPrinter()

			got := runImport(p, tt.cmd, nil)

			if got != tt.want {
				t.Errorf("runImport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runImport_notUseGoCmd(t *testing.T) {
	t.Setenv("PATH", "")

	cmd := newImportCmd()

	p, buf := newTestPrinter()

	got := runImport(p, cmd, nil)

	if got != 1 {
		t.Errorf("runImport() = %v, want 1", got)
	}

	if !strings.Contains(buf.String(), "you didn't install golang") {
		t.Errorf("expected go command error, got: %s", buf.String())
	}
}

func Test_runImport_fileNotFound(t *testing.T) {
	cmd := newImportCmd()
	if err := cmd.Flags().Set("file", "/no/such/file.json"); err != nil {
		t.Fatal(err)
	}

	p, buf := newTestPrinter()

	got := runImport(p, cmd, nil)

	if got != 1 {
		t.Errorf("runImport() = %v, want 1", got)
	}

	if !strings.Contains(buf.String(), "is not found") {
		t.Errorf("expected 'is not found' error, got: %s", buf.String())
	}
}

func Test_runImport_emptyConf(t *testing.T) {
	// Create a temporary conf file with no packages
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(confPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newImportCmd()
	if err := cmd.Flags().Set("file", confPath); err != nil {
		t.Fatal(err)
	}

	p, buf := newTestPrinter()

	got := runImport(p, cmd, nil)

	if got != 1 {
		t.Errorf("runImport() = %v, want 1", got)
	}

	if !strings.Contains(buf.String(), "unable to import package") {
		t.Errorf("expected 'unable to import package' error, got: %s", buf.String())
	}
}

func Test_runImport_jobsClamp(t *testing.T) {
	// Create a conf file that will be found but has no packages
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(confPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newImportCmd()
	if err := cmd.Flags().Set("file", confPath); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("jobs", "0"); err != nil {
		t.Fatal(err)
	}

	p, _ := newTestPrinter()

	// Should not panic with jobs=0 (clamped to 1)
	got := runImport(p, cmd, nil)

	// Expect exit code 1 because the conf file has no packages
	if got != 1 {
		t.Errorf("runImport() = %v, want 1", got)
	}
}

func Test_installFromConfig_UseVersion(t *testing.T) {
	originalInstaller := installByVersionCtx
	t.Cleanup(func() {
		installByVersionCtx = originalInstaller
	})

	var gotImportPath string
	var gotVersion string
	installByVersionCtx = func(_ context.Context, importPath, version string) error {
		gotImportPath = importPath
		gotVersion = version
		return nil
	}

	pkgs := []goutil.Package{
		{
			Name:       testCmdGup,
			ImportPath: "github.com/nao1215/gup",
			Version:    &goutil.Version{Current: testVersionOne},
		},
	}

	p, _ := newTestPrinter()
	if got := installFromConfig(p, pkgs, false, false, 1, 0); got != 0 {
		t.Fatalf("installFromConfig() = %d, want 0", got)
	}

	if gotImportPath != "github.com/nao1215/gup" {
		t.Fatalf("install import path = %s, want %s", gotImportPath, "github.com/nao1215/gup")
	}
	if gotVersion != testVersionOne {
		t.Fatalf("install version = %s, want %s", gotVersion, testVersionOne)
	}
}

func Test_versionFromConfig_NormalizeDevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pkg  goutil.Package
		want string
	}{
		{
			name: "devel with parentheses",
			pkg: goutil.Package{
				Version: &goutil.Version{Current: "(devel)"},
			},
			want: latestKeyword,
		},
		{
			name: "devel without parentheses",
			pkg: goutil.Package{
				Version: &goutil.Version{Current: "devel"},
			},
			want: latestKeyword,
		},
		{
			name: "regular version",
			pkg: goutil.Package{
				Version: &goutil.Version{Current: testVersion123},
			},
			want: testVersion123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := versionFromConfig(tt.pkg)
			if err != nil {
				t.Fatalf("versionFromConfig() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("versionFromConfig() = %s, want %s", got, tt.want)
			}
		})
	}
}

func Test_versionFromConfig_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pkg  goutil.Package
	}{
		{
			name: "nil version",
			pkg:  goutil.Package{},
		},
		{
			name: "empty version string",
			pkg: goutil.Package{
				Version: &goutil.Version{Current: ""},
			},
		},
		{
			name: "whitespace only",
			pkg: goutil.Package{
				Version: &goutil.Version{Current: "   "},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := versionFromConfig(tt.pkg)
			if err == nil {
				t.Fatal("versionFromConfig() expected error, got nil")
			}
		})
	}
}

func Test_installFromConfig_installError(t *testing.T) {
	originalInstaller := installByVersionCtx
	t.Cleanup(func() {
		installByVersionCtx = originalInstaller
	})

	installByVersionCtx = func(context.Context, string, string) error {
		return errors.New("install failed")
	}

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: "github.com/example/tool",
			Version:    &goutil.Version{Current: testVersionOne},
		},
	}

	p, _ := newTestPrinter()
	if got := installFromConfig(p, pkgs, false, false, 1, 0); got != 1 {
		t.Fatalf("installFromConfig() = %d, want 1", got)
	}
}

func Test_installFromConfig_emptyImportPath(t *testing.T) {
	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: "",
			Version:    &goutil.Version{Current: testVersionOne},
		},
	}

	p, _ := newTestPrinter()
	if got := installFromConfig(p, pkgs, false, false, 1, 0); got != 1 {
		t.Fatalf("installFromConfig() = %d, want 1", got)
	}
}

func Test_installFromConfig_dryRun(t *testing.T) {
	t.Setenv("GOBIN", t.TempDir())

	originalInstaller := installByVersionCtx
	t.Cleanup(func() {
		installByVersionCtx = originalInstaller
	})

	installByVersionCtx = func(context.Context, string, string) error { return nil }

	pkgs := []goutil.Package{
		{
			Name:       testBinTool,
			ImportPath: "github.com/example/tool",
			Version:    &goutil.Version{Current: testVersionOne},
		},
	}

	p, _ := newTestPrinter()
	if got := installFromConfig(p, pkgs, true, false, 1, 0); got != 0 {
		t.Fatalf("installFromConfig() dry-run = %d, want 0", got)
	}
}

func Test_runImport_ambiguousConfig(t *testing.T) {
	setupXDGBase(t)
	chdirToTemp(t)

	// Create the user-level config.
	if err := os.MkdirAll(config.DirPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.FilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}
	// Create ./gup.json as well, making auto-detection ambiguous.
	if err := os.WriteFile(config.LocalFilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newImportCmd()

	p, buf := newTestPrinter()

	got := runImport(p, cmd, nil)

	if got != 1 {
		t.Errorf("runImport() = %v, want 1", got)
	}

	out := buf.String()
	if !strings.Contains(out, "multiple gup.json") {
		t.Errorf("expected ambiguity error, got: %s", out)
	}
	if !strings.Contains(out, "--file") {
		t.Errorf("expected error to mention --file, got: %s", out)
	}
}

func Test_runImport_autoDetectSingleCandidate(t *testing.T) {
	setupXDGBase(t)
	helper_stubImportInstaller(t)
	chdirToTemp(t)

	gobinDir := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobinDir)
	if err := os.MkdirAll(gobinDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Only ./gup.json exists: it is the sole auto-detected candidate.
	if err := os.WriteFile(config.LocalFilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newImportCmd()

	p, buf := newTestPrinter()

	got := runImport(p, cmd, nil)

	out := buf.String()
	if got != 0 {
		t.Errorf("runImport() = %v, want 0; output: %s", got, out)
	}
	if !strings.Contains(out, "start import based on") {
		t.Errorf("expected import to start from the detected file, got: %s", out)
	}
}

func Test_runImport_timeoutPropagation(t *testing.T) {
	setupXDGBase(t)
	chdirToTemp(t)

	// Capture the context deadline the install operation receives.
	org := installByVersionCtx
	var sawDeadline bool
	var remaining time.Duration
	installByVersionCtx = func(ctx context.Context, _, _ string) error {
		if dl, ok := ctx.Deadline(); ok {
			sawDeadline = true
			remaining = time.Until(dl)
		}
		return nil
	}
	t.Cleanup(func() { installByVersionCtx = org })

	gobinDir := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobinDir)
	if err := os.MkdirAll(gobinDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LocalFilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newImportCmd()
	if err := cmd.Flags().Set("timeout", "30s"); err != nil {
		t.Fatal(err)
	}
	p, _ := newTestPrinter()
	if got := runImport(p, cmd, nil); got != 0 {
		t.Fatalf("runImport() = %d, want 0", got)
	}

	if !sawDeadline {
		t.Fatal("install operation did not receive a context deadline from --timeout")
	}
	if remaining <= 0 || remaining > 30*time.Second {
		t.Errorf("unexpected remaining deadline %v, want (0, 30s]", remaining)
	}
}

func Test_runImport_timeoutZeroDisablesDeadline(t *testing.T) {
	setupXDGBase(t)
	chdirToTemp(t)

	org := installByVersionCtx
	var sawDeadline bool
	installByVersionCtx = func(ctx context.Context, _, _ string) error {
		if _, ok := ctx.Deadline(); ok {
			sawDeadline = true
		}
		return nil
	}
	t.Cleanup(func() { installByVersionCtx = org })

	gobinDir := filepath.Join(t.TempDir(), "gobin")
	t.Setenv("GOBIN", gobinDir)
	if err := os.MkdirAll(gobinDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LocalFilePath(), []byte(validImportConf), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newImportCmd()
	if err := cmd.Flags().Set("timeout", "0"); err != nil {
		t.Fatal(err)
	}
	p, _ := newTestPrinter()
	if got := runImport(p, cmd, nil); got != 0 {
		t.Fatalf("runImport() = %d, want 0", got)
	}

	if sawDeadline {
		t.Error("install operation should not receive a deadline when --timeout is 0")
	}
}
