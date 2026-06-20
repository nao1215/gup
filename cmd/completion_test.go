package cmd

import "testing"

func TestCompletion_NoArgsRequiresExplicitMode(t *testing.T) {
	t.Parallel()

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("completion without args should require --install")
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
	t.Setenv("HOME", t.TempDir())

	cmd := newCompletionCmd()
	cmd.SetArgs([]string{testFlagInstall})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("completion --install should succeed: %v", err)
	}
}
