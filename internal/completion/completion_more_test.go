package completion

import (
	"os"
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
