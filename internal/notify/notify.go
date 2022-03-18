package notify

import (
	"github.com/gen2brain/beeep"
	"github.com/nao1215/gup/internal/assets"
	"github.com/nao1215/gup/internal/print"
)

// Info notify information message at desktop
func Info(title, message string) {
	err := beeep.Notify(title, message, assets.InfoIconPath())
	if err != nil {
		print.Warn(err)
	}
}

// Warn notify warning message at desktop
func Warn(title, message string) {
	err := beeep.Notify(title, message, assets.WarningIconPath())
	if err != nil {
		print.Warn(err)
	}
}
