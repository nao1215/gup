//go:build linux

package cmd

import (
	"context"
	"os/exec"
)

func openBrowser(targetURL string) bool {
	return exec.CommandContext(context.Background(), "xdg-open", targetURL).Start() == nil
}
