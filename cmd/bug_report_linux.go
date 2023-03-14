//go:build linux

package cmd

import (
	"os/exec"
)

func openBrowser(targetURL string) bool {
	return exec.Command("xdg-open", targetURL).Start() == nil
}
