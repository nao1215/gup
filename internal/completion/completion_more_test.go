package completion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/fileutil"
)

// TestMain clears the XDG/ZDOTDIR variables for the whole package so the many
// tests that assert HOME-rooted completion paths are deterministic regardless of
// the ambient environment. Tests that exercise those variables set them
// explicitly via t.Setenv, which is restored after each test (#366).
func TestMain(m *testing.M) {
	_ = os.Unsetenv("XDG_DATA_HOME")
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("ZDOTDIR")
	os.Exit(m.Run())
}

func TestIsWindows(t *testing.T) {
	t.Parallel()
	if got, want := IsWindows(), runtime.GOOS == "windows"; got != want {
		t.Errorf("IsWindows() = %v, want %v", got, want)
	}
}

func TestDeployShellCompletionFileIfNeeded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := testCompletionCmd()

	if IsWindows() {
		if err := DeployShellCompletionFileIfNeeded(cmd); err != nil {
			t.Errorf("DeployShellCompletionFileIfNeeded() on Windows should be a no-op, got: %v", err)
		}
		if fileutil.IsFile(bashCompletionFilePath()) {
			t.Error("no completion files should be deployed on Windows")
		}
		return
	}

	if err := DeployShellCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("DeployShellCompletionFileIfNeeded() error = %v", err)
	}

	for _, path := range []string{
		bashCompletionFilePath(),
		fishCompletionFilePath(),
		zshCompletionFilePath(),
	} {
		if !fileutil.IsFile(path) {
			t.Errorf("completion file was not deployed: %s", path)
		}
	}

	if !fileutil.IsFile(zshrcPath()) {
		t.Error("zshrc with fpath setting was not created")
	}

	// A second run hits the "already up to date" branches and must not error
	// or duplicate the zshrc fpath block.
	if err := DeployShellCompletionFileIfNeeded(cmd); err != nil {
		t.Errorf("second DeployShellCompletionFileIfNeeded() error = %v", err)
	}
}

// TestBashCompletionFilePath_HonorsXDGDataHome verifies the #366 contract: bash
// completion installs under XDG_DATA_HOME when it is set, not under
// $HOME/.local/share.
func TestBashCompletionFilePath_HonorsXDGDataHome(t *testing.T) {
	if IsWindows() {
		t.Skip("completion install is not supported on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	cmd := testCompletionCmd()

	if err := makeBashCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("makeBashCompletionFileIfNeeded() error = %v", err)
	}

	got := bashCompletionFilePath()
	if !strings.HasPrefix(got, xdgData) {
		t.Errorf("bash completion path = %q, want it rooted at XDG_DATA_HOME %q", got, xdgData)
	}
	if strings.HasPrefix(got, filepath.Join(home, ".local")) {
		t.Errorf("bash completion path = %q, must not fall back to $HOME/.local/share when XDG_DATA_HOME is set", got)
	}
	if !fileutil.IsFile(got) {
		t.Errorf("bash completion file was not written at %q", got)
	}
}

// TestFishCompletionFilePath_HonorsXDGConfigHome verifies the #366 contract: fish
// completion installs under XDG_CONFIG_HOME when it is set, not under
// $HOME/.config.
func TestFishCompletionFilePath_HonorsXDGConfigHome(t *testing.T) {
	if IsWindows() {
		t.Skip("fish completion is not deployed on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	xdgConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	cmd := testCompletionCmd()

	if err := makeFishCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("makeFishCompletionFileIfNeeded() error = %v", err)
	}

	got := fishCompletionFilePath()
	if !strings.HasPrefix(got, xdgConfig) {
		t.Errorf("fish completion path = %q, want it rooted at XDG_CONFIG_HOME %q", got, xdgConfig)
	}
	if strings.HasPrefix(got, filepath.Join(home, ".config")) {
		t.Errorf("fish completion path = %q, must not fall back to $HOME/.config when XDG_CONFIG_HOME is set", got)
	}
	if !fileutil.IsFile(got) {
		t.Errorf("fish completion file was not written at %q", got)
	}
}

// TestZshCompletionInstall_HonorsZDOTDIR verifies the #366 contract: zsh resolves
// both the completion file and .zshrc via ZDOTDIR, the appended fpath snippet
// points at ${ZDOTDIR}, and repeated appends do not duplicate the snippet.
func TestZshCompletionInstall_HonorsZDOTDIR(t *testing.T) {
	if IsWindows() {
		t.Skip("zsh completion is not deployed on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	zdot := t.TempDir()
	t.Setenv("ZDOTDIR", zdot)
	cmd := testCompletionCmd()

	if err := makeZshCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("makeZshCompletionFileIfNeeded() error = %v", err)
	}

	comp := zshCompletionFilePath()
	if !strings.HasPrefix(comp, zdot) {
		t.Errorf("zsh completion path = %q, want it rooted at ZDOTDIR %q", comp, zdot)
	}
	if !fileutil.IsFile(comp) {
		t.Errorf("zsh completion file was not written at %q", comp)
	}

	rc := zshrcPath()
	if !strings.HasPrefix(rc, zdot) {
		t.Errorf(".zshrc path = %q, want it rooted at ZDOTDIR %q", rc, zdot)
	}
	if !fileutil.IsFile(rc) {
		t.Fatalf(".zshrc was not created at %q", rc)
	}

	data, err := os.ReadFile(filepath.Clean(rc))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "${ZDOTDIR}/.zsh/completion") {
		t.Errorf("fpath snippet should reference ${ZDOTDIR}/.zsh/completion under ZDOTDIR, got:\n%s", data)
	}

	// Repeated install must not duplicate the snippet.
	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("second appendFpathAtZshrcIfNeeded() error = %v", err)
	}
	data, err = os.ReadFile(filepath.Clean(rc))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "auto generate"); got != 1 {
		t.Errorf("fpath block appears %d times under ZDOTDIR, want 1 (no duplication)", got)
	}
}

// TestDeployShellCompletionFileIfNeeded_unsetHOME verifies the #343 contract:
// with HOME unset, deployment fails fast and writes nothing.
func TestDeployShellCompletionFileIfNeeded_unsetHOME(t *testing.T) {
	if IsWindows() {
		t.Skip("completion install is not supported on Windows")
	}
	t.Setenv("HOME", "")
	t.Chdir(t.TempDir())

	cmd := testCompletionCmd()
	err := DeployShellCompletionFileIfNeeded(cmd)
	if err == nil || !strings.Contains(err.Error(), "HOME") {
		t.Fatalf("expected an error mentioning HOME, got: %v", err)
	}
	for _, stray := range []string{".local", ".config", ".zsh", ".zshrc"} {
		if _, statErr := os.Stat(stray); statErr == nil {
			t.Errorf("no %q should be created under the working directory when HOME is unset", stray)
		}
	}
}

// TestDeployShellCompletionFileIfNeeded_unsetHOMEWithXDG locks in the #366 + #343
// decision: even when XDG_DATA_HOME/XDG_CONFIG_HOME/ZDOTDIR are set, an empty
// HOME still fails fast and writes nothing into relative paths under the working
// directory.
func TestDeployShellCompletionFileIfNeeded_unsetHOMEWithXDG(t *testing.T) {
	if IsWindows() {
		t.Skip("completion install is not supported on Windows")
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ZDOTDIR", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := testCompletionCmd()
	err := DeployShellCompletionFileIfNeeded(cmd)
	if err == nil || !strings.Contains(err.Error(), "HOME") {
		t.Fatalf("expected an error mentioning HOME even with XDG/ZDOTDIR set, got: %v", err)
	}
	for _, stray := range []string{".local", ".config", ".zsh", ".zshrc"} {
		if _, statErr := os.Stat(stray); statErr == nil {
			t.Errorf("no %q should be created under the working directory when HOME is unset", stray)
		}
	}
}

// TestDeployShellCompletionFileIfNeeded_relativePathEnv verifies that a relative
// XDG_DATA_HOME, XDG_CONFIG_HOME or ZDOTDIR is rejected before any file is
// written, even when HOME is a valid absolute path. Without this guard the
// relative value would be used verbatim to build install paths, silently writing
// completion files (and the .zshrc fpath block) under the current working
// directory instead of the user's home.
func TestDeployShellCompletionFileIfNeeded_relativePathEnv(t *testing.T) {
	if IsWindows() {
		t.Skip("completion install is not supported on Windows")
	}

	cases := []struct {
		name   string
		envVar string
	}{
		{name: "relative XDG_DATA_HOME", envVar: "XDG_DATA_HOME"},
		{name: "relative XDG_CONFIG_HOME", envVar: "XDG_CONFIG_HOME"},
		{name: "relative ZDOTDIR", envVar: "ZDOTDIR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// HOME is a valid absolute path: the only problem is the relative env var.
			t.Setenv("HOME", t.TempDir())
			t.Setenv(tc.envVar, filepath.Join("relative", "completion", "dir"))
			// Run from a fresh empty directory so any stray write is detectable.
			workDir := t.TempDir()
			t.Chdir(workDir)

			cmd := testCompletionCmd()
			err := DeployShellCompletionFileIfNeeded(cmd)
			if err == nil {
				t.Fatalf("expected an error for relative %s, got nil", tc.envVar)
			}
			if !strings.Contains(err.Error(), tc.envVar) {
				t.Fatalf("error should name the offending variable %s, got: %v", tc.envVar, err)
			}

			// Nothing must have been written under the working directory.
			entries, readErr := os.ReadDir(workDir)
			if readErr != nil {
				t.Fatalf("failed to read working directory: %v", readErr)
			}
			if len(entries) != 0 {
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Errorf("no files should be written on failure, but found: %v", names)
			}
		})
	}
}

func TestExistSameBashCompletionFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := testCompletionCmd()

	if isSameBashCompletionFile(cmd) {
		t.Fatal("isSameBashCompletionFile() = true, want false when no file exists")
	}

	writeBashCompletionFile(t, generateBashCompletion(t, cmd))

	if !isSameBashCompletionFile(cmd) {
		t.Fatal("isSameBashCompletionFile() = false, want true after writing matching content")
	}
}

func TestIsSameFishCompletionFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if IsWindows() {
		t.Skip("fish completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	if isSameFishCompletionFile(cmd) {
		t.Fatal("isSameFishCompletionFile() = true, want false when no file exists")
	}

	if err := makeFishCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("makeFishCompletionFileIfNeeded() error = %v", err)
	}

	if !isSameFishCompletionFile(cmd) {
		t.Fatal("isSameFishCompletionFile() = false, want true after generation")
	}
}

func TestIsSameZshCompletionFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if IsWindows() {
		t.Skip("zsh completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	if isSameZshCompletionFile(cmd) {
		t.Fatal("isSameZshCompletionFile() = true, want false when no file exists")
	}

	if err := makeZshCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("makeZshCompletionFileIfNeeded() error = %v", err)
	}

	if !isSameZshCompletionFile(cmd) {
		t.Fatal("isSameZshCompletionFile() = false, want true after generation")
	}
}

// TestMakeZshCompletionFileIfNeeded_RepairsMissingZshrcBlock verifies the
// self-healing contract: when _gup is already up to date but the .zshrc fpath
// block has been removed, re-running the install restores the block instead of
// early-returning and leaving completion broken.
func TestMakeZshCompletionFileIfNeeded_RepairsMissingZshrcBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Pin .zshrc/fpath resolution to the temp HOME even if ZDOTDIR is exported,
	// so the test never touches a real $ZDOTDIR/.zshrc.
	t.Setenv("ZDOTDIR", "")
	if IsWindows() {
		t.Skip("zsh completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	// First install creates _gup and the .zshrc block.
	if err := makeZshCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("first makeZshCompletionFileIfNeeded() error = %v", err)
	}
	if !isSameZshCompletionFile(cmd) {
		t.Fatal("precondition: _gup should be up to date after the first install")
	}

	// Simulate the user deleting gup's block from .zshrc (keeping other config).
	if err := os.WriteFile(zshrcPath(), []byte("# unrelated config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Re-running must restore the block even though _gup itself is unchanged.
	if err := makeZshCompletionFileIfNeeded(cmd); err != nil {
		t.Fatalf("second makeZshCompletionFileIfNeeded() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(zshrcPath()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fpath=(~/.zsh/completion") {
		t.Errorf("deleted .zshrc block was not repaired, got:\n%s", data)
	}
	if !strings.Contains(string(data), "# unrelated config") {
		t.Errorf("unrelated .zshrc content was dropped, got:\n%s", data)
	}
	if got := strings.Count(string(data), "auto generate"); got != 1 {
		t.Errorf("fpath block appears %d times, want exactly 1", got)
	}
}

// TestAppendFpathAtZshrcIfNeeded_RepairsStaleBlock verifies that a marker that is
// present but points at an out-of-date completion directory is replaced in place
// (updated, not duplicated), so the marker-based reconcile actually self-heals an
// old block.
func TestAppendFpathAtZshrcIfNeeded_RepairsStaleBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ZDOTDIR", "") // pin .zshrc resolution to the temp HOME

	stale := "# existing config\n\n" + zshFpathMarker + "\n" +
		"fpath=(/old/wrong/path $fpath)\n" +
		"autoload -Uz compinit && compinit -i\n"
	if err := os.WriteFile(zshrcPath(), []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("appendFpathAtZshrcIfNeeded() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(zshrcPath()))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/old/wrong/path") {
		t.Errorf("stale fpath was not replaced, got:\n%s", data)
	}
	if !strings.Contains(string(data), "fpath=(~/.zsh/completion") {
		t.Errorf("fresh fpath line is missing, got:\n%s", data)
	}
	if !strings.Contains(string(data), "# existing config") {
		t.Errorf("unrelated content was dropped, got:\n%s", data)
	}
	if got := strings.Count(string(data), "auto generate"); got != 1 {
		t.Errorf("fpath block appears %d times, want exactly 1 (replaced, not duplicated)", got)
	}

	// Repairing again must be a no-op: the now-correct block is left untouched.
	before := string(data)
	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("second appendFpathAtZshrcIfNeeded() error = %v", err)
	}
	after, err := os.ReadFile(filepath.Clean(zshrcPath()))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Errorf("re-running on a correct .zshrc rewrote it:\nbefore:\n%q\nafter:\n%q", before, string(after))
	}
}

// TestAppendFpathAtZshrcIfNeeded_PreservesSurroundingLines guards against the
// reconcile merging adjacent user lines: a stale block wedged between user lines
// with single-newline spacing (no blank separators) must be repaired in place
// while every surrounding line keeps its own line.
func TestAppendFpathAtZshrcIfNeeded_PreservesSurroundingLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ZDOTDIR", "") // pin .zshrc resolution to the temp HOME

	before := "export PATH=$PATH\n"
	after := "export EDITOR=vim\n"
	staleBlock := zshFpathMarker + "\n" +
		"fpath=(/old/wrong/path $fpath)\n" +
		"autoload -Uz compinit && compinit -i\n"
	if err := os.WriteFile(zshrcPath(), []byte(before+staleBlock+after), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("appendFpathAtZshrcIfNeeded() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(zshrcPath()))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, line := range []string{"export PATH=$PATH", "export EDITOR=vim"} {
		if !strings.Contains(got, line+"\n") {
			t.Errorf("surrounding line %q lost its own line (possible merge), got:\n%q", line, got)
		}
	}
	if strings.Contains(got, "$PATHexport") || strings.Contains(got, "compinit && compinit -iexport") {
		t.Errorf("adjacent user lines were merged into the block, got:\n%q", got)
	}
	if strings.Contains(got, "/old/wrong/path") {
		t.Errorf("stale fpath was not replaced, got:\n%q", got)
	}
	if got2 := strings.Count(got, "auto generate"); got2 != 1 {
		t.Errorf("fpath block appears %d times, want exactly 1", got2)
	}
}

// TestAppendFpathAtZshrcIfNeeded_RepairsBrokenBlock verifies that a marker left
// behind without its fpath/autoload lines (a hand-broken block) is reconciled
// into a complete, single block.
func TestAppendFpathAtZshrcIfNeeded_RepairsBrokenBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ZDOTDIR", "") // pin .zshrc resolution to the temp HOME

	broken := "# existing config\n\n" + zshFpathMarker + "\n"
	if err := os.WriteFile(zshrcPath(), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("appendFpathAtZshrcIfNeeded() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(zshrcPath()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fpath=(~/.zsh/completion") {
		t.Errorf("broken block was not repaired with the fpath line, got:\n%s", data)
	}
	if !strings.Contains(string(data), "autoload -Uz compinit") {
		t.Errorf("broken block was not repaired with the autoload line, got:\n%s", data)
	}
	if got := strings.Count(string(data), "auto generate"); got != 1 {
		t.Errorf("fpath block appears %d times, want exactly 1", got)
	}
}

func TestAppendFpathAtZshrcIfNeeded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Pre-create .zshrc with unrelated content so the fpath block is appended.
	if err := os.WriteFile(zshrcPath(), []byte("# existing config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("appendFpathAtZshrcIfNeeded() error = %v", err)
	}

	data, err := os.ReadFile(zshrcPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fpath=(~/.zsh/completion") {
		t.Fatalf("fpath block was not appended to .zshrc, got:\n%s", data)
	}

	// Calling again must not duplicate the block (contains-check branch).
	if err := appendFpathAtZshrcIfNeeded(); err != nil {
		t.Fatalf("second appendFpathAtZshrcIfNeeded() error = %v", err)
	}
	data, err = os.ReadFile(zshrcPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "auto generate"); got != 1 {
		t.Errorf("fpath block appears %d times, want 1 (no duplication)", got)
	}
}

func TestMakeBashCompletionFileIfNeeded_MkdirError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if IsWindows() {
		t.Skip("bash completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	// Put a regular file where a parent directory must be created, so MkdirAll fails.
	if err := os.WriteFile(filepath.Join(home, ".local"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := makeBashCompletionFileIfNeeded(cmd)
	if err == nil || !strings.Contains(err.Error(), "can not create bash-completion file") {
		t.Errorf("expected MkdirAll error, got: %v", err)
	}
}

func TestMakeBashCompletionFileIfNeeded_OpenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if IsWindows() {
		t.Skip("bash completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	// Create the completion path itself as a directory so OpenFile fails.
	if err := os.MkdirAll(bashCompletionFilePath(), 0o750); err != nil {
		t.Fatal(err)
	}

	err := makeBashCompletionFileIfNeeded(cmd)
	if err == nil || !strings.Contains(err.Error(), "can not open bash-completion file") {
		t.Errorf("expected OpenFile error, got: %v", err)
	}
}

func TestMakeFishCompletionFileIfNeeded_GenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if IsWindows() {
		t.Skip("fish completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	// Create the completion path as a directory so the file cannot be written.
	if err := os.MkdirAll(fishCompletionFilePath(), 0o750); err != nil {
		t.Fatal(err)
	}

	err := makeFishCompletionFileIfNeeded(cmd)
	if err == nil || !strings.Contains(err.Error(), "can not open fish-completion file") {
		t.Errorf("expected fish generation error, got: %v", err)
	}
}

func TestMakeZshCompletionFileIfNeeded_GenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if IsWindows() {
		t.Skip("zsh completion is not deployed on Windows")
	}
	cmd := testCompletionCmd()

	// Create the completion path as a directory so the file cannot be written.
	if err := os.MkdirAll(zshCompletionFilePath(), 0o750); err != nil {
		t.Fatal(err)
	}

	err := makeZshCompletionFileIfNeeded(cmd)
	if err == nil || !strings.Contains(err.Error(), "can not open zsh-completion file") {
		t.Errorf("expected zsh generation error, got: %v", err)
	}
}

func TestAppendFpathAtZshrcIfNeeded_OpenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Make .zshrc a directory so the create-branch OpenFile fails.
	if err := os.MkdirAll(zshrcPath(), 0o750); err != nil {
		t.Fatal(err)
	}

	err := appendFpathAtZshrcIfNeeded()
	if err == nil || !strings.Contains(err.Error(), "can not open .zshrc") {
		t.Errorf("expected .zshrc open error, got: %v", err)
	}
}

func TestCompletionFilePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, path := range []string{
		bashCompletionFilePath(),
		fishCompletionFilePath(),
		zshCompletionFilePath(),
		zshrcPath(),
	} {
		if !strings.HasPrefix(path, home) {
			t.Errorf("path %s is not rooted at HOME %s", path, home)
		}
	}
}
