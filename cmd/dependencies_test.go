package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"testing"

	"github.com/nao1215/gup/internal/print"
)

// discardPrinter returns a Printer that throws away all output, for tests that
// exercise a function's behavior or exit code but do not assert on its output.
func discardPrinter() *print.Printer {
	return print.New(io.Discard, io.Discard)
}

// skipIfDirWriteFaultUnsupported skips a test that forces an I/O failure by
// making a directory read-only (chmod 0500). That trick only works where Unix
// directory permissions gate writes: root bypasses the read-only bit, and
// Windows ignores these bits for directory writability. gup's coverage CI runs
// as the non-root Linux runner user, so these branches are still exercised
// there.
func skipIfDirWriteFaultUnsupported(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("skipping: read-only permissions are bypassed when running as root")
	}
	if runtime.GOOS == goosWindows {
		t.Skip("skipping: Windows does not enforce read-only dir permissions for writes")
	}
}

// errReader is an io.Reader that always fails, used to drive the io.Copy error
// branch in the gzip/atomic-write helpers.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("forced read failure") }

// newTestPrinter returns a Printer whose normal and error output both go to a
// single buffer, plus the buffer for assertions. It replaces the old pattern of
// pointing the package-level print.Stdout/Stderr writers at one pipe: a test
// constructs the Printer, passes it to the function under test, and reads the
// buffer. The progress callbacks that drive parallel output are invoked from a
// single consumer goroutine, so the shared buffer is never written concurrently.
func newTestPrinter() (*print.Printer, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return print.New(buf, buf), buf
}

// testDeps returns a dependencies value whose operations are harmless stubs: the
// version lookups return an empty version and the installs are no-ops. A test
// overrides only the fields it exercises and passes the value in directly, so it
// owns its dependencies instead of mutating package globals.
func testDeps() dependencies {
	return dependencies{
		getLatestVer:        func(context.Context, string) (string, error) { return "", nil },
		getVerByRef:         func(context.Context, string, string) (string, error) { return "", nil },
		installLatest:       func(context.Context, string) error { return nil },
		installMainOrMaster: func(context.Context, string) error { return nil },
		installByVersion:    func(context.Context, string, string) error { return nil },
	}
}

// stubUpdateDeps returns dependencies that make every package look outdated
// (latest == v9.9.9) with no-op installs, so an update/check run reaches the
// install path without performing real installs or network lookups. It replaces
// the old helper_stubUpdateOps global-swap helper.
func stubUpdateDeps() dependencies {
	d := testDeps()
	d.getLatestVer = func(context.Context, string) (string, error) { return testVersionNine, nil }
	// The channel-aware skip/update decision resolves @main/@master versions
	// through this ref lookup, so stub it alongside the @latest lookup.
	d.getVerByRef = func(context.Context, string, string) (string, error) { return testVersionNine, nil }
	return d
}
