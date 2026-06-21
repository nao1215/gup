//nolint:paralleltest // tests that stub the package-level isWindows seam must not run in parallel
package cmd

import (
	"io"
	"strings"
	"testing"
)

// stubIsWindows overrides the isWindows seam for the duration of a test.
func stubIsWindows(t *testing.T, v bool) {
	t.Helper()
	org := isWindows
	isWindows = func() bool { return v }
	t.Cleanup(func() {
		isWindows = org
	})
}

func TestCompletion_NoArgsRequiresExplicitMode(t *testing.T) {
	t.Parallel()

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("completion without args should require a shell name or --install")
	}
	got := err.Error()
	for _, want := range []string{"requires a shell name", "gup completion bash", "gup completion --install"} {
		if !strings.Contains(got, want) {
			t.Errorf("error should contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Usage:") {
		t.Errorf("error should be concise, not full help, got:\n%s", got)
	}
}

func TestCompletion_InstallWithShellArg(t *testing.T) {
	t.Parallel()

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall, testShellBash})
	if err := cmd.Execute(); err == nil {
		t.Fatal("--install with shell argument should fail")
	}
}

func TestCompletion_Install(t *testing.T) {
	stubIsWindows(t, false)
	t.Setenv("HOME", t.TempDir())

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("completion --install should succeed: %v", err)
	}
}

func TestCompletion_InstallWindowsReturnsError(t *testing.T) {
	stubIsWindows(t, true)

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("completion --install should return an error on Windows")
	}
	if !strings.Contains(err.Error(), "Windows") {
		t.Errorf("error should explain Windows is unsupported, got: %v", err)
	}
	if !strings.Contains(err.Error(), "powershell") {
		t.Errorf("error should point to 'gup completion powershell', got: %v", err)
	}
}

func TestCompletion_PowerShellStdoutWorksOnWindows(t *testing.T) {
	stubIsWindows(t, true)

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testShellPowershell})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("completion powershell stdout generation should work on Windows: %v", err)
	}
}
