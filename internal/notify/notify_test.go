//go:build !dragonfly

//nolint:paralleltest // tests mutate the global xdg.ConfigHome
package notify

import (
	"bytes"
	"testing"

	"github.com/adrg/xdg"
	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/print"
)

// useTempConfigHome points the asset directory at a fresh temp directory.
func useTempConfigHome(t *testing.T) {
	t.Helper()
	orgConfig := xdg.ConfigHome
	t.Cleanup(func() {
		xdg.ConfigHome = orgConfig
	})
	xdg.ConfigHome = t.TempDir()
}

func TestInfo(t *testing.T) {
	useTempConfigHome(t)

	buf := &bytes.Buffer{}
	p := print.New(buf, buf)

	// Should not panic even when no desktop notification backend is available.
	Info(p, "gup", "info message")

	if !fileutil.IsFile(assets.InfoIconPath()) {
		t.Errorf("Info should deploy the info icon at %s", assets.InfoIconPath())
	}
}

func TestWarn(t *testing.T) {
	useTempConfigHome(t)

	buf := &bytes.Buffer{}
	p := print.New(buf, buf)

	Warn(p, "gup", "warning message")

	if !fileutil.IsFile(assets.WarningIconPath()) {
		t.Errorf("Warn should deploy the warning icon at %s", assets.WarningIconPath())
	}
}
