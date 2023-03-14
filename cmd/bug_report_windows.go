//go:build windows

package cmd

import (
	"os/exec"
)

func openBrowser(targetURL string) bool {
	return exec.Command("cmd", "/C", "start", "msedge", targetURL).Start() == nil
}
