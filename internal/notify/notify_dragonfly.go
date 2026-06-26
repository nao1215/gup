//go:build dragonfly

package notify

import "github.com/nao1215/gup/internal/print"

// Info notify information message at desktop
func Info(_ *print.Printer, _, _ string) {
	return
}

// Warn notify warning message at desktop
func Warn(_ *print.Printer, _, _ string) {
	return
}
