package completion

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/gup/internal/fileutil"
	"github.com/nao1215/gup/internal/print"
)

// capturePrintStderr redirects print.Stderr into a buffer for the test.
func capturePrintStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	org := print.Stderr
	buf := &bytes.Buffer{}
	print.Stderr = buf
	t.Cleanup(func() { print.Stderr = org })
	return buf
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
		DeployShellCompletionFileIfNeeded(cmd)
		if fileutil.IsFile(bashCompletionFilePath()) {
			t.Error("no completion files should be deployed on Windows")
		}
		return
	}

	DeployShellCompletionFileIfNeeded(cmd)

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
	DeployShellCompletionFileIfNeeded(cmd)
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

	makeFishCompletionFileIfNeeded(cmd)

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

	makeZshCompletionFileIfNeeded(cmd)

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

	appendFpathAtZshrcIfNeeded()

	data, err := os.ReadFile(zshrcPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fpath=(~/.zsh/completion") {
		t.Fatalf("fpath block was not appended to .zshrc, got:\n%s", data)
	}

	// Calling again must not duplicate the block (contains-check branch).
	appendFpathAtZshrcIfNeeded()
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

	buf := capturePrintStderr(t)
	makeBashCompletionFileIfNeeded(cmd)

	if !strings.Contains(buf.String(), "can not create bash-completion file") {
		t.Errorf("expected MkdirAll error, got: %s", buf.String())
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

	buf := capturePrintStderr(t)
	makeBashCompletionFileIfNeeded(cmd)

	if !strings.Contains(buf.String(), "can not open .bash_completion") {
		t.Errorf("expected OpenFile error, got: %s", buf.String())
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

	buf := capturePrintStderr(t)
	makeFishCompletionFileIfNeeded(cmd)

	if !strings.Contains(buf.String(), "can not create fish-completion file") {
		t.Errorf("expected fish generation error, got: %s", buf.String())
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

	buf := capturePrintStderr(t)
	makeZshCompletionFileIfNeeded(cmd)

	if !strings.Contains(buf.String(), "can not create zsh-completion file") {
		t.Errorf("expected zsh generation error, got: %s", buf.String())
	}
}

func TestAppendFpathAtZshrcIfNeeded_OpenError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Make .zshrc a directory so the create-branch OpenFile fails.
	if err := os.MkdirAll(zshrcPath(), 0o750); err != nil {
		t.Fatal(err)
	}

	buf := capturePrintStderr(t)
	appendFpathAtZshrcIfNeeded()

	if !strings.Contains(buf.String(), "can not open .zshrc") {
		t.Errorf("expected .zshrc open error, got: %s", buf.String())
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
