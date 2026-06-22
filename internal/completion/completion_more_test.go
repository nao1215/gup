package completion

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/fileutil"
)

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

func TestExistSameBashCompletionFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := testCompletionCmd()

	if existSameBashCompletionFile(cmd) {
		t.Fatal("existSameBashCompletionFile() = true, want false when no file exists")
	}

	writeBashCompletionFile(t, generateBashCompletion(t, cmd))

	if !existSameBashCompletionFile(cmd) {
		t.Fatal("existSameBashCompletionFile() = false, want true after writing matching content")
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
	if err == nil || !strings.Contains(err.Error(), "can not open .bash_completion") {
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
	if err == nil || !strings.Contains(err.Error(), "can not create fish-completion file") {
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
	if err == nil || !strings.Contains(err.Error(), "can not create zsh-completion file") {
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
