package cmd

import (
	"os"
	"testing"

	internalcmd "github.com/nao1215/gup/internal/cmd"
)

// TestExecute_runsTheInternalCommandWithTheSeams checks that the deprecated v1
// Execute still runs gup and hands GOOS and OsExit to the implementation, as
// it did before the implementation moved.
func TestExecute_runsTheInternalCommandWithTheSeams(t *testing.T) { //nolint:paralleltest // replaces os.Args and package-level seams
	exited := -1
	OsExit = func(code int) { exited = code }
	GOOS = "plan9"
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"gup", "version"}

	if err := Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if internalcmd.GOOS != "plan9" {
		t.Errorf("internal GOOS = %q, want the value set on cmd.GOOS", internalcmd.GOOS)
	}
	internalcmd.OsExit(7)
	if exited != 7 {
		t.Errorf("internal OsExit is not the function set on cmd.OsExit")
	}
}
