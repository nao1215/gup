//go:build linux

package cmd

import (
	"runtime"
	"testing"
)

func Test_openBrowser(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("skipping test on non-linux system")
	}

	got := openBrowser("https://example.com")
	if !got {
		t.Error("openBrowser() = false; want true")
	}
}

func Test_openBrowser_invalidScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{name: "ftp scheme", url: "ftp://example.com"},
		{name: "javascript scheme", url: "javascript:alert(1)"},
		{name: "empty string", url: ""},
		{name: "no scheme", url: "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := openBrowser(tt.url); got {
				t.Errorf("openBrowser(%q) = true; want false", tt.url)
			}
		})
	}
}
