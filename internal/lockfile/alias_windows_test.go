//go:build windows

// Windows is where one file answers to the most names, and both of the extra
// ones here split a sibling lock without any way to notice. These tests are the
// reason PathForFile asks the operating system which file a name means instead
// of appending ".lock" to whatever it was handed.
package lockfile

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// TestPathForFile_collapsesAnEightDotThreeAlias covers the alias NTFS keeps for
// every long name. `--file C:\Users\somebody\AppData\...\gup.json` and the same
// path spelled through GUP~1.JSO (or through a shortened directory component)
// name one file, and the sibling lock built from the second used to be
// GUP~1.JSO.lock - a different file from gup.json.lock, so two gups rewriting
// one configuration would each hold a lock the other could not see.
func TestPathForFile_collapsesAnEightDotThreeAlias(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := filepath.Join(dir, "a-deliberately-long-gup.json")
	if err := os.WriteFile(config, []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}
	short := shortPathOrSkip(t, config)

	viaLong, err := PathForFile(config)
	if err != nil {
		t.Fatalf("PathForFile(long name) = %v, want a lock path", err)
	}
	viaShort, err := PathForFile(short)
	if err != nil {
		t.Fatalf("PathForFile(8.3 name) = %v, want a lock path", err)
	}
	if viaLong != viaShort {
		t.Errorf("PathForFile() = %q for %q and %q for its 8.3 alias %q; two gups writing one file would not contend",
			viaLong, config, viaShort, short)
	}
}

// TestPathForFile_collapsesTrailingDotsAndSpaces covers the normalization Win32
// performs before the filesystem ever sees a name. `--file gup.json.` writes
// gup.json, because the trailing dot is stripped on the way in - but the lock
// built from it, gup.json..lock, ends in "lock" and has nothing to strip, so it
// is a genuinely different file from gup.json.lock.
func TestPathForFile_collapsesTrailingDotsAndSpaces(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := filepath.Join(dir, "gup.json")
	if err := os.WriteFile(config, []byte(`{"schema_version":1,"packages":[]}`), 0o600); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}

	want, err := PathForFile(config)
	if err != nil {
		t.Fatalf("PathForFile() = %v, want a lock path", err)
	}
	for _, spelling := range []string{config + ".", config + " ", config + ". ."} {
		got, err := PathForFile(spelling)
		if err != nil {
			t.Fatalf("PathForFile(%q) = %v, want a lock path", spelling, err)
		}
		if got != want {
			t.Errorf("PathForFile(%q) = %q, want %q: Win32 strips the trailing dots and spaces, so both names write one file",
				spelling, got, want)
		}
	}
}

// TestPathForFile_collapsesTrailingDotsBeforeTheFileExists is the same rule on a
// first run. There is no file to ask about yet, so the fold has to be applied to
// the name itself - otherwise two gups creating one gup.json for the first time,
// one of them having typed a trailing dot, take two locks and both create it.
func TestPathForFile_collapsesTrailingDotsBeforeTheFileExists(t *testing.T) {
	t.Parallel()

	config := filepath.Join(t.TempDir(), "gup.json")
	want, err := PathForFile(config)
	if err != nil {
		t.Fatalf("PathForFile() = %v, want a lock path", err)
	}
	got, err := PathForFile(config + ".")
	if err != nil {
		t.Fatalf("PathForFile() = %v, want a lock path", err)
	}
	if got != want {
		t.Errorf("PathForFile(%q) = %q, want %q", config+".", got, want)
	}
}

// shortPathOrSkip returns the 8.3 alias of path, skipping when the volume does
// not keep them - which is a supported configuration (fsutil 8dot3name), and one
// where the alias this test is about simply does not exist.
func shortPathOrSkip(t *testing.T, path string) string {
	t.Helper()

	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("failed to convert %q: %v", path, err)
	}
	const size = uint32(windows.MAX_PATH)
	buf := make([]uint16, size)
	n, err := windows.GetShortPathName(long, &buf[0], size)
	if err != nil || n == 0 || n >= size {
		t.Skipf("this volume does not keep 8.3 names: %v", err)
	}
	short := windows.UTF16ToString(buf[:n])
	if short == path {
		t.Skip("this volume does not keep 8.3 names, so the long name is the only name")
	}
	return short
}
