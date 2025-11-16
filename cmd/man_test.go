package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateManpages(t *testing.T) {
	t.Parallel()

	t.Run("Generate man pages", func(t *testing.T) {
		dst, err := os.MkdirTemp("", "test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer func() {
			if removeErr := os.RemoveAll(dst); removeErr != nil {
				t.Fatal(removeErr)
			}
		}()

		if err := generateManpages(dst); err != nil {
			t.Fatalf("generateManpages() failed: %v", err)
		}

		manFiles, err := filepath.Glob(filepath.Join(dst, "*.1.gz"))
		if err != nil {
			t.Errorf("Failed to glob man files: %v", err)
		}
		if len(manFiles) == 0 {
			t.Error("No man files found")
		}
	})
}

func TestManPaths(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == goosWindows {
		t.Skip("Skip test on Windows")
	}

	t.Run("Return default man path when MANPATH is empty", func(t *testing.T) {
		t.Parallel()

		paths := manPaths("")
		want := []string{filepath.Join("/", "usr", "share", "man", "man1")}
		if diff := cmp.Diff(want, paths); diff != "" {
			t.Fatalf("manPaths() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Return paths split by MANPATH", func(t *testing.T) {
		t.Parallel()

		paths := manPaths("/usr/local/share/man:/usr/share/man1:/opt/man")
		want := []string{
			filepath.Join("/", "usr", "local", "share", "man", "man1"),
			filepath.Join("/", "usr", "share", "man1"),
			filepath.Join("/", "opt", "man", "man1"),
		}
		if diff := cmp.Diff(want, paths); diff != "" {
			t.Fatalf("manPaths() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Return default when MANPATH only includes empty paths", func(t *testing.T) {
		t.Parallel()

		paths := manPaths("::")
		want := []string{filepath.Join("/", "usr", "share", "man", "man1")}
		if diff := cmp.Diff(want, paths); diff != "" {
			t.Fatalf("manPaths() mismatch (-want +got):\n%s", diff)
		}
	})
}
