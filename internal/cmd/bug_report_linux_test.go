//go:build linux

package cmd

import (
	"errors"
	"testing"
)

//nolint:paralleltest // These tests replace the global runBrowserCommand function.
func Test_openBrowser(t *testing.T) {
	origRunBrowserCommand := runBrowserCommand
	t.Cleanup(func() { runBrowserCommand = origRunBrowserCommand })

	const wantURL = "https://example.com"

	var gotCommand string
	var gotArgs []string
	runBrowserCommand = func(command string, args ...string) error {
		gotCommand = command
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if got := openBrowser(wantURL); !got {
		t.Fatal("openBrowser() = false, want true")
	}
	if gotCommand != "xdg-open" {
		t.Fatalf("command = %q, want %q", gotCommand, "xdg-open")
	}
	if len(gotArgs) != 1 || gotArgs[0] != wantURL {
		t.Fatalf("args = %v, want [%q]", gotArgs, wantURL)
	}
}

//nolint:paralleltest // These tests replace the global runBrowserCommand function.
func Test_openBrowser_error(t *testing.T) {
	origRunBrowserCommand := runBrowserCommand
	t.Cleanup(func() { runBrowserCommand = origRunBrowserCommand })

	runBrowserCommand = func(string, ...string) error {
		return errors.New("open browser failed")
	}

	if got := openBrowser("https://example.com"); got {
		t.Fatal("openBrowser() = true, want false")
	}
}
