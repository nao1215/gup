//go:build darwin

package cmd

func openBrowser(targetURL string) bool {
	return runBrowserCommand("open", targetURL) == nil
}
