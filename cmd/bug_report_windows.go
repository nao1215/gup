//go:build windows

package cmd

import (
	"os/exec"
)

func openBrowser(targetURL string) bool {
	return exec.Command("rundll32.exe", "url,OpenURL", targetURL).Start() == nil
}
