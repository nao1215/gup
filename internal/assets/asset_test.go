//nolint:paralleltest // tests mutate the global xdg.ConfigHome and print.Stderr
package assets

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/print"
)

// useTempConfigHome points config.DirPath() at a fresh temp directory.
func useTempConfigHome(t *testing.T) {
	t.Helper()
	org := xdg.ConfigHome
	t.Cleanup(func() { xdg.ConfigHome = org })
	xdg.ConfigHome = t.TempDir()
}

func TestIconPaths(t *testing.T) {
	useTempConfigHome(t)

	assetsDir := filepath.Join(config.DirPath(), "assets")
	if got, want := InfoIconPath(), filepath.Join(assetsDir, "information.png"); got != want {
		t.Errorf("InfoIconPath() = %s, want %s", got, want)
	}
	if got, want := WarningIconPath(), filepath.Join(assetsDir, "warning.png"); got != want {
		t.Errorf("WarningIconPath() = %s, want %s", got, want)
	}
}

func TestDeployIconIfNeeded(t *testing.T) {
	useTempConfigHome(t)

	DeployIconIfNeeded()

	if !fileutil.IsFile(InfoIconPath()) {
		t.Fatalf("info icon was not deployed at %s", InfoIconPath())
	}
	if !fileutil.IsFile(WarningIconPath()) {
		t.Fatalf("warning icon was not deployed at %s", WarningIconPath())
	}

	got, err := os.ReadFile(InfoIconPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, inforIcon) {
		t.Error("deployed info icon content does not match the embedded asset")
	}

	// Calling again must be a no-op and must not error or change the files.
	DeployIconIfNeeded()
	if !fileutil.IsFile(InfoIconPath()) || !fileutil.IsFile(WarningIconPath()) {
		t.Error("icons should still exist after a second DeployIconIfNeeded call")
	}
}

func TestDeployIconIfNeeded_MkdirError(t *testing.T) {
	useTempConfigHome(t)

	// Create a regular file where the gup config directory should be, so
	// MkdirAll for the assets directory fails.
	if err := os.WriteFile(config.DirPath(), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	orig := print.Stderr
	var buf bytes.Buffer
	print.Stderr = &buf
	t.Cleanup(func() { print.Stderr = orig })

	DeployIconIfNeeded()

	if !strings.Contains(buf.String(), "can not make assets directory") {
		t.Errorf("expected assets directory error, got: %s", buf.String())
	}
	if fileutil.IsFile(InfoIconPath()) {
		t.Error("no icon should be created when the assets directory cannot be made")
	}
}
