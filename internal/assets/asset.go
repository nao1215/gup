package assets

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/gup/internal/config"
	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/print"
)

//go:embed information.png
var inforIcon []byte

//go:embed warning.png
var warningIcon []byte

// DeployIconIfNeeded make icon file for notification. Diagnostics are written
// through the provided Printer.
func DeployIconIfNeeded(p *print.Printer) {
	if !fileutil.IsDir(assetsDirPath()) {
		if err := os.MkdirAll(assetsDirPath(), fileutil.FileModeCreatingDir); err != nil {
			p.Err(fmt.Errorf("%s: %w", "can not make assets directory", err))
			return
		}
	}

	if !fileutil.IsFile(InfoIconPath()) {
		err := os.WriteFile(InfoIconPath(), inforIcon, fileutil.FileModeCreatingFile)
		if err != nil {
			p.Warn(err)
		}
	}
	if !fileutil.IsFile(WarningIconPath()) {
		err := os.WriteFile(WarningIconPath(), warningIcon, fileutil.FileModeCreatingFile)
		if err != nil {
			p.Warn(err)
		}
	}
}

// InfoIconPath return absolute path of information.png.
func InfoIconPath() string {
	return filepath.Join(assetsDirPath(), "information.png")
}

// WarningIconPath return absolute path of information.png.
func WarningIconPath() string {
	return filepath.Join(assetsDirPath(), "warning.png")
}

// assetsDirPath return absolute path of assets directory.
func assetsDirPath() string {
	return filepath.Join(config.DirPath(), "assets")
}
