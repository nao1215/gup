//go:build windows

package cmd

func openBrowser(targetURL string) bool {
	return runBrowserCommand("rundll32.exe", "url,OpenURL", targetURL) == nil
}
