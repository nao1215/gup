package cmd

import (
	"errors"
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

// Test_copyOneManpage_failedWriteDoesNotCorruptExisting verifies that when the
// final rename fails, an existing destination man page is left untouched (not
// truncated or partially overwritten) and no temp artifact is left behind.
//
//nolint:paralleltest // mutates the package-level renameFunc
func Test_copyOneManpage_failedWriteDoesNotCorruptExisting(t *testing.T) {
	origRename := renameFunc
	t.Cleanup(func() { renameFunc = origRename })

	src := filepath.Join(t.TempDir(), "gup.1")
	if err := os.WriteFile(src, []byte("manpage source"), 0o600); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	existing := filepath.Join(dst, "gup.1.gz")
	if err := os.WriteFile(existing, []byte("PREVIOUS"), 0o600); err != nil {
		t.Fatal(err)
	}

	renameFunc = func(_, _ string) error {
		return errors.New("forced rename failure")
	}

	if err := copyOneManpage(discardPrinter(), src, dst); err == nil {
		t.Fatal("copyOneManpage() should return an error when the rename fails")
	}

	got, err := os.ReadFile(filepath.Clean(existing))
	if err != nil {
		t.Fatalf("existing man page should survive a failed write: %v", err)
	}
	if string(got) != "PREVIOUS" {
		t.Fatalf("existing man page was corrupted: got %q, want %q", string(got), "PREVIOUS")
	}

	assertNoTempFiles(t, dst, "gup.1.gz")
}

// Test_copyOneManpage_preservesSymlink verifies that when the destination man
// page is a symlink, the atomic write rewrites the link target and keeps the
// symlink intact rather than replacing it with a regular file.
func Test_copyOneManpage_preservesSymlink(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("symlink behavior is POSIX-specific")
	}
	t.Parallel()

	src := filepath.Join(t.TempDir(), "gup.1")
	if err := os.WriteFile(src, []byte("manpage source"), 0o600); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	realTarget := filepath.Join(t.TempDir(), "real-gup.1.gz")
	if err := os.WriteFile(realTarget, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dst, "gup.1.gz")
	if err := os.Symlink(realTarget, link); err != nil {
		t.Fatal(err)
	}

	if err := copyOneManpage(discardPrinter(), src, dst); err != nil {
		t.Fatalf("copyOneManpage() error = %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("man page symlink was replaced by a regular file")
	}
	// The link target must now hold the freshly generated gzip content, not "OLD".
	got, err := os.ReadFile(filepath.Clean(realTarget))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "OLD" || len(got) == 0 {
		t.Fatalf("man page was not written through the symlink to its target: %q", string(got))
	}
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
