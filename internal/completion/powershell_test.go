//nolint:paralleltest // these tests set process-wide environment variables and the goos seam
package completion

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// testAppName is the root command's name in these tests; goconst asks for it
// once it appears in more than a couple of places.
const testAppName = "gup"

// windowsInstall points the package at its Windows behavior and gives the
// install a home directory of its own, so the PowerShell path is exercised on
// whatever host runs the tests. Windows CI runs these same tests natively, where
// the seam is a no-op.
func windowsInstall(t *testing.T) string {
	t.Helper()

	original := goos
	goos = goosWindows
	t.Cleanup(func() { goos = original })

	home := t.TempDir()
	t.Setenv(envUserProfile, home)
	t.Setenv("PROFILE", "")
	t.Setenv(envHome, home)
	return home
}

// psProfile returns the profile path the install picks for home, and the
// completion script written beside it.
func psProfile(home string) (profile, completion string) {
	profile = filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	return profile, filepath.Join(filepath.Dir(profile), psCompletionFileName)
}

// readFile returns path's contents, failing the test if it cannot be read.
func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(raw)
}

// TestDeployShellCompletion_installsPowerShellOnWindows is the point of the
// change: `gup completion --install` on Windows used to refuse and hand the user
// three manual steps. It now writes the completer and wires the profile to it.
func TestDeployShellCompletion_installsPowerShellOnWindows(t *testing.T) {
	home := windowsInstall(t)
	profile, completionScript := psProfile(home)

	if err := DeployShellCompletionFileIfNeeded(&cobra.Command{Use: testAppName}); err != nil {
		t.Fatalf("DeployShellCompletionFileIfNeeded() error: %v", err)
	}

	script := readFile(t, completionScript)
	if !strings.Contains(script, "Register-ArgumentCompleter") {
		t.Errorf("%s does not look like a PowerShell completer:\n%s", completionScript, script)
	}

	got := readFile(t, profile)
	if !strings.Contains(got, psProfileMarker) || !strings.Contains(got, psProfileEndMarker) {
		t.Errorf("the profile is missing gup's managed block:\n%s", got)
	}
	// The profile must source the file gup actually wrote, guarded so a deleted
	// completer does not break every prompt.
	if !strings.Contains(got, completionScript) {
		t.Errorf("the profile does not source %s:\n%s", completionScript, got)
	}
	if !strings.Contains(got, "Test-Path") {
		t.Errorf("the profile sources the completer without a Test-Path guard:\n%s", got)
	}
}

// TestDeployShellCompletion_keepsExistingPowerShellProfileContent is the promise
// that makes this safe to run: a profile is a file users keep their own
// configuration in, and an install that rewrote it would be a data-loss bug.
func TestDeployShellCompletion_keepsExistingPowerShellProfileContent(t *testing.T) {
	home := windowsInstall(t)
	profile, _ := psProfile(home)

	existing := "Set-Alias ll Get-ChildItem\nfunction prompt { \"PS> \" }\n"
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		t.Fatalf("failed to create the profile directory: %v", err)
	}
	if err := os.WriteFile(profile, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write the existing profile: %v", err)
	}

	if err := DeployShellCompletionFileIfNeeded(&cobra.Command{Use: testAppName}); err != nil {
		t.Fatalf("DeployShellCompletionFileIfNeeded() error: %v", err)
	}

	got := readFile(t, profile)
	if !strings.HasPrefix(got, existing) {
		t.Errorf("the install disturbed the user's own profile content:\n%s", got)
	}
	if !strings.Contains(got, psProfileMarker) {
		t.Errorf("the install did not append gup's block:\n%s", got)
	}
}

// TestDeployShellCompletion_powerShellInstallIsIdempotent covers a re-run, which
// happens every time a user re-runs the documented install command or a package
// upgrade does. Appending a second block would leave the profile dot-sourcing
// the completer twice and growing on every run.
func TestDeployShellCompletion_powerShellInstallIsIdempotent(t *testing.T) {
	home := windowsInstall(t)
	profile, _ := psProfile(home)
	cmd := &cobra.Command{Use: testAppName}

	for range 3 {
		if err := DeployShellCompletionFileIfNeeded(cmd); err != nil {
			t.Fatalf("DeployShellCompletionFileIfNeeded() error: %v", err)
		}
	}

	got := readFile(t, profile)
	if n := strings.Count(got, psProfileMarker); n != 1 {
		t.Errorf("gup's block appears %d times in the profile after three installs, want 1:\n%s", n, got)
	}
	if n := strings.Count(got, "Test-Path"); n != 1 {
		t.Errorf("the dot-source line appears %d times after three installs, want 1:\n%s", n, got)
	}
}

// TestDeployShellCompletion_repairsAHandBrokenPowerShellBlock covers a profile a
// user edited: the block's body deleted, or its terminator removed. A re-run
// must repair the block in place rather than leave a broken one and append
// another beside it.
func TestDeployShellCompletion_repairsAHandBrokenPowerShellBlock(t *testing.T) {
	tests := map[string]string{
		"body deleted": "# mine\n" + psProfileMarker + "\n" + psProfileEndMarker + "\n",
		"terminator deleted": "# mine\n" + psProfileMarker +
			"\nif (Test-Path 'C:\\gone\\gup.completion.ps1') { . 'C:\\gone\\gup.completion.ps1' }\n",
	}
	for name, broken := range tests {
		t.Run(name, func(t *testing.T) {
			home := windowsInstall(t)
			profile, completionScript := psProfile(home)
			if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
				t.Fatalf("failed to create the profile directory: %v", err)
			}
			if err := os.WriteFile(profile, []byte(broken), 0o600); err != nil {
				t.Fatalf("failed to write the broken profile: %v", err)
			}

			if err := DeployShellCompletionFileIfNeeded(&cobra.Command{Use: testAppName}); err != nil {
				t.Fatalf("DeployShellCompletionFileIfNeeded() error: %v", err)
			}

			got := readFile(t, profile)
			if n := strings.Count(got, psProfileMarker); n != 1 {
				t.Errorf("gup's block appears %d times after repair, want 1:\n%s", n, got)
			}
			if !strings.Contains(got, completionScript) {
				t.Errorf("the repaired block does not source %s:\n%s", completionScript, got)
			}
			if !strings.HasPrefix(got, "# mine\n") {
				t.Errorf("repairing the block moved the user's own line:\n%s", got)
			}
		})
	}
}

// writeProfile creates a profile file with some content of the user's own, so a
// test can tell an install that appended from one that rewrote.
func writeProfile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("failed to create the profile directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("# mine\n"), 0o600); err != nil {
		t.Fatalf("failed to write the profile: %v", err)
	}
}

// windowsPowerShellProfile returns the Windows PowerShell 5.1 profile path under
// home, the sibling of the PowerShell 7 one psProfile returns.
func windowsPowerShellProfile(home string) string {
	return filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
}

// TestPowerShellProfilePaths_wiresUpEveryExistingProfile covers the version
// split, which is the case an install can silently get wrong. Windows PowerShell
// 5.1 reads Documents\WindowsPowerShell and PowerShell 7 reads
// Documents\PowerShell, and both are commonly installed side by side; wiring up
// only one leaves the other shell with no completion after a command that
// reported success.
func TestPowerShellProfilePaths_wiresUpEveryExistingProfile(t *testing.T) {
	home := windowsInstall(t)
	pwsh7, _ := psProfile(home)
	winPS := windowsPowerShellProfile(home)
	writeProfile(t, pwsh7)
	writeProfile(t, winPS)

	got, err := powerShellProfilePaths()
	if err != nil {
		t.Fatalf("powerShellProfilePaths() error: %v", err)
	}
	for _, want := range []string{pwsh7, winPS} {
		if !slices.Contains(got, want) {
			t.Errorf("powerShellProfilePaths() = %v, missing the existing profile %q", got, want)
		}
	}
}

// TestPowerShellProfilePaths_prefersAnExistingProfile covers the 5.1-only user:
// with no PowerShell 7 profile, the install must not create one and stop there.
func TestPowerShellProfilePaths_prefersAnExistingProfile(t *testing.T) {
	home := windowsInstall(t)
	winPS := windowsPowerShellProfile(home)
	writeProfile(t, winPS)

	got, err := powerShellProfilePaths()
	if err != nil {
		t.Fatalf("powerShellProfilePaths() error: %v", err)
	}
	if len(got) != 1 || got[0] != winPS {
		t.Errorf("powerShellProfilePaths() = %v, want only the existing Windows PowerShell profile %q", got, winPS)
	}
}

// TestPowerShellProfilePaths_defaultsToPowerShell7 covers a machine with no
// profile yet: the file has to be created somewhere, and PowerShell 7's path is
// the one a current install reads.
func TestPowerShellProfilePaths_defaultsToPowerShell7(t *testing.T) {
	home := windowsInstall(t)

	got, err := powerShellProfilePaths()
	if err != nil {
		t.Fatalf("powerShellProfilePaths() error: %v", err)
	}
	want := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if len(got) != 1 || got[0] != want {
		t.Errorf("powerShellProfilePaths() = %v, want only %q", got, want)
	}
}

// TestPowerShellProfilePaths_honorsPROFILE covers a user who relocated their
// profile and exported $PROFILE: they have already answered the question, so gup
// installs there and nowhere else, even when the standard locations exist.
func TestPowerShellProfilePaths_honorsPROFILE(t *testing.T) {
	home := windowsInstall(t)
	writeProfile(t, windowsPowerShellProfile(home))
	custom := filepath.Join(home, "elsewhere", "profile.ps1")
	t.Setenv("PROFILE", custom)

	got, err := powerShellProfilePaths()
	if err != nil {
		t.Fatalf("powerShellProfilePaths() error: %v", err)
	}
	if len(got) != 1 || got[0] != custom {
		t.Errorf("powerShellProfilePaths() = %v, want only the exported PROFILE %q", got, custom)
	}
}

// TestDeployShellCompletion_installsIntoBothPowerShellProfiles is the same rule
// end to end: with both shells' profiles present, both get a completer beside
// them and a block that sources it.
func TestDeployShellCompletion_installsIntoBothPowerShellProfiles(t *testing.T) {
	home := windowsInstall(t)
	pwsh7, pwsh7Completion := psProfile(home)
	winPS := windowsPowerShellProfile(home)
	writeProfile(t, pwsh7)
	writeProfile(t, winPS)

	if err := DeployShellCompletionFileIfNeeded(&cobra.Command{Use: testAppName}); err != nil {
		t.Fatalf("DeployShellCompletionFileIfNeeded() error: %v", err)
	}

	winPSCompletion := filepath.Join(filepath.Dir(winPS), psCompletionFileName)
	for profile, completion := range map[string]string{pwsh7: pwsh7Completion, winPS: winPSCompletion} {
		if !strings.Contains(readFile(t, completion), "Register-ArgumentCompleter") {
			t.Errorf("%s is not a PowerShell completer", completion)
		}
		got := readFile(t, profile)
		if !strings.HasPrefix(got, "# mine\n") {
			t.Errorf("the install disturbed the user's own content in %s:\n%s", profile, got)
		}
		if !strings.Contains(got, completion) {
			t.Errorf("%s does not source %s:\n%s", profile, completion, got)
		}
	}
}

// TestPowerShellProfilePaths_rejectsRelativePaths mirrors the POSIX install's
// fail-fast rule: gup never absolutizes a relative HOME-like value, because
// doing so writes configuration into whatever directory the user was standing
// in.
func TestPowerShellProfilePaths_rejectsRelativePaths(t *testing.T) {
	tests := map[string]struct {
		env   map[string]string
		named string
	}{
		"relative PROFILE":     {env: map[string]string{"PROFILE": "profile.ps1"}, named: "PROFILE"},
		"relative USERPROFILE": {env: map[string]string{envUserProfile: "home", envHome: ""}, named: envUserProfile},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			windowsInstall(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := powerShellProfilePaths()
			if err == nil {
				t.Fatal("powerShellProfilePaths() accepted a relative path")
			}
			if !strings.Contains(err.Error(), tt.named) {
				t.Errorf("powerShellProfilePaths() error %q does not name %s", err, tt.named)
			}
		})
	}
}

// TestPowerShellProfilePaths_requiresAHomeDirectory covers the environment the
// POSIX install already fails fast on, from the Windows side: with nothing
// naming a home, there is no correct place to write, so gup says so instead of
// writing into the current directory.
func TestPowerShellProfilePaths_requiresAHomeDirectory(t *testing.T) {
	windowsInstall(t)
	t.Setenv(envUserProfile, "")
	t.Setenv(envHome, "")
	t.Setenv("PROFILE", "")

	_, err := powerShellProfilePaths()
	if err == nil {
		t.Fatal("powerShellProfilePaths() succeeded with no home directory set")
	}
	for _, want := range []string{envUserProfile, envHome} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("powerShellProfilePaths() error %q does not mention %s", err, want)
		}
	}
}

// TestPowerShellProfileBlock_quotesThePath covers a home directory containing a
// single quote, which PowerShell would otherwise read as the end of the string
// literal and turn into a syntax error in the user's profile.
func TestPowerShellProfileBlock_quotesThePath(t *testing.T) {
	got := powerShellProfileBlock(`C:\Users\O'Brien\gup.completion.ps1`)
	if !strings.Contains(got, `'C:\Users\O''Brien\gup.completion.ps1'`) {
		t.Errorf("powerShellProfileBlock() did not double the embedded quote:\n%s", got)
	}
}

// TestSyncPowerShellProfile_leavesAnUpToDateProfileUntouched covers the re-run
// case at the filesystem level: an install that rewrote an unchanged profile
// would churn its modification time and, with a dotfile manager watching, look
// like a change the user made.
func TestSyncPowerShellProfile_leavesAnUpToDateProfileUntouched(t *testing.T) {
	home := windowsInstall(t)
	profile, completionScript := psProfile(home)
	if err := os.MkdirAll(filepath.Dir(profile), 0o750); err != nil {
		t.Fatalf("failed to create the profile directory: %v", err)
	}
	if err := syncPowerShellProfile(profile, completionScript); err != nil {
		t.Fatalf("syncPowerShellProfile() error: %v", err)
	}
	// Backdate the file so a rewrite is unmistakable: comparing against "now"
	// would pass on a filesystem whose timestamp resolution is coarser than the
	// test.
	backdated := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(profile, backdated, backdated); err != nil {
		t.Fatalf("failed to backdate the profile: %v", err)
	}

	if err := syncPowerShellProfile(profile, completionScript); err != nil {
		t.Fatalf("second syncPowerShellProfile() error: %v", err)
	}
	after, err := os.Stat(profile)
	if err != nil {
		t.Fatalf("failed to stat the profile: %v", err)
	}
	if !after.ModTime().Equal(backdated) {
		t.Errorf("an already-correct profile was rewritten (mtime moved from %v to %v)", backdated, after.ModTime())
	}
}
