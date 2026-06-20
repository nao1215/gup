//go:build !dragonfly

//nolint:paralleltest // tests mutate the global xdg.ConfigHome and print.Stderr
package notify

import (
	"bytes"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/print"
)

// useTempConfigHome points the asset directory at a fresh temp directory and
// silences print output (beeep may warn when no desktop session is available).
func useTempConfigHome(t *testing.T) {
	t.Helper()
	orgConfig := xdg.ConfigHome
	orgStderr := print.Stderr
	orgStdout := print.Stdout
	t.Cleanup(func() {
		xdg.ConfigHome = orgConfig
		print.Stderr = orgStderr
		print.Stdout = orgStdout
	})
	xdg.ConfigHome = t.TempDir()
	buf := &bytes.Buffer{}
	print.Stderr = buf
	print.Stdout = buf
}

func TestInfo(t *testing.T) {
	useTempConfigHome(t)

	// Should not panic even when no desktop notification backend is available.
	Info("gup", "info message")

	if !fileutil.IsFile(assets.InfoIconPath()) {
		t.Errorf("Info should deploy the info icon at %s", assets.InfoIconPath())
	}
}

func TestWarn(t *testing.T) {
	useTempConfigHome(t)

	Warn("gup", "warning message")

	if !fileutil.IsFile(assets.WarningIconPath()) {
		t.Errorf("Warn should deploy the warning icon at %s", assets.WarningIconPath())
	}
}
