//go:build !darwin && !linux && !windows

package cmd

func openBrowser(string) bool {
	return false
}
