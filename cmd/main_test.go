package cmd

import (
	"os"
	"testing"
)

// TestMain clears XDG_DATA_HOME, XDG_CONFIG_HOME, and ZDOTDIR for the whole
// package so completion-install tests that assert HOME-rooted paths are
// deterministic regardless of the ambient environment (CI runners may set these,
// which 'gup completion --install' now honors). Tests that exercise those
// variables set them explicitly via t.Setenv, which is restored after each test
// (#366).
func TestMain(m *testing.M) {
	if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("ZDOTDIR"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
