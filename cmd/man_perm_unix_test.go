//go:build !windows

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// Test_copyOneManpage_respectsUmask verifies that a newly generated man page
// gets the same permissions os.Create produced (0666 & ^umask), rather than a
// hardcoded 0644. Under a restrictive umask (e.g. 077) the file must NOT be
// world-readable, so a fixed 0644 would be an unintended permission widening.
// The cached processUmask is injected so the assertion is deterministic and does
// not mutate the real process umask.
//
//nolint:paralleltest // mutates the package-level processUmask
func Test_copyOneManpage_respectsUmask(t *testing.T) {
	tests := []struct {
		name  string
		umask int
		want  os.FileMode
	}{
		{name: "umask 022 -> 0644", umask: 0o022, want: 0o644},
		{name: "umask 077 -> 0600", umask: 0o077, want: 0o600},
		{name: "umask 002 -> 0664", umask: 0o002, want: 0o664},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := processUmask
			processUmask = tt.umask
			defer func() { processUmask = old }()

			src := filepath.Join(t.TempDir(), "gup.1")
			if err := os.WriteFile(src, []byte("manpage source"), 0o600); err != nil {
				t.Fatal(err)
			}
			dst := t.TempDir()

			if err := copyOneManpage(discardPrinter(), src, dst); err != nil {
				t.Fatalf("copyOneManpage() error = %v", err)
			}

			info, err := os.Stat(filepath.Join(dst, "gup.1.gz"))
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != tt.want {
				t.Fatalf("man page mode = %#o, want %#o (umask %#o)", got, tt.want, tt.umask)
			}
		})
	}
}

// Test_copyOneManpage_preservesExistingMode verifies that replacing an existing
// man page keeps that file's own permissions instead of resetting them to the
// new-file default, regardless of the umask.
//
//nolint:paralleltest // mutates the package-level processUmask
func Test_copyOneManpage_preservesExistingMode(t *testing.T) {
	// A permissive umask must not influence the result: the existing file's mode
	// wins when the destination already exists.
	old := processUmask
	processUmask = 0o000
	defer func() { processUmask = old }()

	src := filepath.Join(t.TempDir(), "gup.1")
	if err := os.WriteFile(src, []byte("manpage source"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	existing := filepath.Join(dst, "gup.1.gz")
	if err := os.WriteFile(existing, []byte("OLD"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Group-readable but not world-readable: a non-default mode that the real
	// ambient umask cannot influence, so the assertion stays deterministic.
	if err := os.Chmod(existing, 0o640); err != nil { //nolint:gosec // intentional non-default mode to assert preservation
		t.Fatal(err)
	}

	if err := copyOneManpage(discardPrinter(), src, dst); err != nil {
		t.Fatalf("copyOneManpage() error = %v", err)
	}

	info, err := os.Stat(existing)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("replaced man page mode = %#o, want 0640 (preserved)", got)
	}
}
