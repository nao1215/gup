//go:build linux

package cmd

import (
	"runtime"
	"testing"
)

func Test_openBrowser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping test on non-linux system")
	}

	got := openBrowser("https://example.com")
	if !got {
		t.Error("openBrowser() = false; want true")
	}
}
