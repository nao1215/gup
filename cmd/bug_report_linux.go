//go:build linux

package cmd

import (
	"context"
	"net/url"
	"os/exec"
)

func openBrowser(targetURL string) bool {
	u, err := url.Parse(targetURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return exec.CommandContext(context.Background(), "xdg-open", u.String()).Start() == nil // #nosec G204 -- targetURL is a validated http(s) URL
}
