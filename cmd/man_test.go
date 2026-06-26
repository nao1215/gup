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

		p, _ := newTestPrinter()
		if err := generateManpages(p, dst); err != nil {
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

func TestMan(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("man command generation test is not supported on Windows")
	}

	t.Run("success with writable MANPATH man1 dir", func(t *testing.T) {
		base := t.TempDir()
		dst := filepath.Join(base, "man1")
		if err := os.MkdirAll(dst, 0o750); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MANPATH", dst)

		if got := man(discardPrinter(), nil, nil); got != 0 {
			t.Fatalf("man() = %d, want 0", got)
		}

		manFiles, err := filepath.Glob(filepath.Join(dst, "*.1.gz"))
		if err != nil {
			t.Fatalf("glob failed: %v", err)
		}
		if len(manFiles) == 0 {
			t.Fatal("no generated man page files found")
		}
	})

	t.Run("success creates a missing MANPATH man1 dir", func(t *testing.T) {
		// A valid custom MANPATH whose man1 directory does not exist yet must be
		// created rather than causing a failure (#344).
		base := t.TempDir()
		manpath := filepath.Join(base, "shareman")
		t.Setenv("MANPATH", manpath)

		if got := man(discardPrinter(), nil, nil); got != 0 {
			t.Fatalf("man() = %d, want 0", got)
		}

		dst := filepath.Join(manpath, "man1")
		manFiles, err := filepath.Glob(filepath.Join(dst, "*.1.gz"))
		if err != nil {
			t.Fatalf("glob failed: %v", err)
		}
		if len(manFiles) == 0 {
			t.Fatalf("man pages were not written to the auto-created dir %s", dst)
		}
	})

	t.Run("failure when MANPATH target is not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		base := t.TempDir()
		readonly := filepath.Join(base, "readonly")
		if err := os.MkdirAll(readonly, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MANPATH", filepath.Join(readonly, "shareman"))

		if got := man(discardPrinter(), nil, nil); got != 1 {
			t.Fatalf("man() = %d, want 1", got)
		}
	})
}
