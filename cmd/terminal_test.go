package cmd

import (
	"os"
	"runtime"
	"testing"
)

// TestIsTerminalRejectsCharacterDevicesThatAreNotTerminals is the regression for
// the second half of #422. The guard that decides whether `gup remove` may ask
// for confirmation read `os.Stdin.Stat()` and called anything with
// os.ModeCharDevice a terminal — but /dev/null is a character device too, so
// `gup remove x < /dev/null` was treated as interactive, reached the prompt, and
// failed on the EOF that came back immediately.
//
// The distinction cannot be made from the file mode: a TTY and /dev/null carry
// the same ModeDevice|ModeCharDevice bits. It takes an ioctl, which is what
// golang.org/x/term asks the kernel for.
func TestIsTerminalRejectsCharacterDevicesThatAreNotTerminals(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		// NUL is Windows' /dev/null, but opening it does not produce a handle the
		// console API answers for, so the case this guards does not arise there.
		t.Skip("the character-device confusion is a POSIX device-file property")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })

	info, err := devNull.Stat()
	if err != nil {
		t.Fatalf("stat %s: %v", os.DevNull, err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		t.Skipf("%s is not a character device here, so it cannot be mistaken for one", os.DevNull)
	}

	if isTerminal(devNull) {
		t.Errorf("%s is reported as a terminal; a redirect from it would reach the confirmation prompt", os.DevNull)
	}
}

// TestIsTerminalRejectsARegularFile covers the ordinary redirect, which the mode
// check did get right. It is here so the two answers are pinned together: a fix
// that makes every file a non-terminal would pass the test above on its own.
func TestIsTerminalRejectsARegularFile(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + string(os.PathSeparator) + "input"
	if err := os.WriteFile(path, []byte("y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path) //nolint:gosec // a path this test just created
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if isTerminal(f) {
		t.Error("a regular file is reported as a terminal")
	}
}
