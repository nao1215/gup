//nolint:paralleltest // tests that set process-wide environment variables must not run in parallel
package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

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
	// HOME drives the POSIX install and PROFILE the Windows one, so the same test
	// exercises whichever platform it runs on without pretending to be the other.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PROFILE", filepath.Join(home, "Microsoft.PowerShell_profile.ps1"))

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("completion --install should succeed: %v", err)
	}
}

// TestCompletion_InstallUnsetHOME verifies the #343 contract: with HOME unset,
// 'completion --install' fails fast with a clear message and writes nothing into
// relative paths under the current working directory.
func TestCompletion_InstallUnsetHOME(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("completion --install is a no-op on Windows (file install is unsupported)")
	}
	t.Setenv("HOME", "")

	// Run from an isolated temp working directory so we can detect stray writes.
	t.Chdir(t.TempDir())

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("completion --install should fail when HOME is unset")
	}
	if !strings.Contains(err.Error(), "HOME") {
		t.Errorf("error should mention HOME, got: %v", err)
	}
	for _, stray := range []string{".local", ".config", ".zsh", ".zshrc"} {
		if _, statErr := os.Stat(stray); statErr == nil {
			t.Errorf("completion --install must not create %q under the working directory when HOME is unset", stray)
		}
	}
}

// TestCompletion_InstallWriteErrorFails verifies the #343 contract: when a
// completion file cannot be written, the command exits non-zero instead of
// silently succeeding.
func TestCompletion_InstallWriteErrorFails(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("completion --install is a no-op on Windows (file install is unsupported)")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Place a regular file where the bash-completion parent directory must be
	// created so MkdirAll fails.
	if err := os.WriteFile(filepath.Join(home, ".local"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	if err := cmd.Execute(); err == nil {
		t.Fatal("completion --install should fail when a completion file cannot be written")
	}
}

// TestCompletion_InstallReportsAFailure covers the command's own error handling:
// whatever the platform's install decides, a failure has to reach the user as a
// non-zero exit rather than being swallowed.
func TestCompletion_InstallReportsAFailure(t *testing.T) {
	original := deployCompletion
	deployCompletion = func(*cobra.Command) error { return errors.New("boom") }
	t.Cleanup(func() { deployCompletion = original })

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("completion --install should surface the install failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should carry the install failure, got: %v", err)
	}
}

func TestCompletion_PowerShellStdoutWorks(t *testing.T) {
	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testShellPowershell})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("completion powershell stdout generation should work: %v", err)
	}
}
