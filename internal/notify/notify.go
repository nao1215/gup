//go:build !dragonfly

package notify

import (
	"github.com/gen2brain/beeep"
	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/print"
)

// Info notify information message at desktop. Diagnostics are written through
// the provided Printer.
func Info(p *print.Printer, title, message string) {
	assets.DeployIconIfNeeded(p)
	err := beeep.Notify(title, message, assets.InfoIconPath())
	if err != nil {
		p.Warn(err)
	}
}

// Warn notify warning message at desktop. Diagnostics are written through the
// provided Printer.
func Warn(p *print.Printer, title, message string) {
	assets.DeployIconIfNeeded(p)
	err := beeep.Notify(title, message, assets.WarningIconPath())
	if err != nil {
		p.Warn(err)
	}
}
