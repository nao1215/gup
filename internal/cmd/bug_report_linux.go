//go:build linux

package cmd

func openBrowser(targetURL string) bool {
	return runBrowserCommand("xdg-open", targetURL) == nil
}
