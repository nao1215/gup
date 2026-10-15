package goutil

import (
	"context"
	"os"
	"os/exec"
)

const unknown = "unknown"

// develVersion and develVersionParen are the placeholder versions the go
// toolchain reports for a locally built binary; neither names a concrete,
// installable module version.
const (
	develVersion      = "devel"
	develVersionParen = "(devel)"
)

// goExe is the executable name for the go command.
const goExe = "go"

// Internal variables to mock/monkey-patch behaviors in tests.
var (
	// keyGoBin is the key name of the env variable for "GOBIN".
	keyGoBin = "GOBIN" //nolint:gochecknoglobals // tests replace environment keys and system calls
	// keyGoPath is the key name of the env variable for "GOPATH".
	keyGoPath = "GOPATH" //nolint:gochecknoglobals // tests replace environment keys and system calls
	// osMkdirTemp is a copy of os.MkdirTemp to ease testing.
	osMkdirTemp = os.MkdirTemp //nolint:gochecknoglobals // tests replace environment keys and system calls
	// goCommandContext builds the *exec.Cmd used to run the go toolchain. It is a
	// variable so tests can swap in either a stand-in executable (e.g. "echo") or
	// the standard "helper process" pattern (re-executing the test binary) and
	// exercise the subprocess-driven helpers deterministically without network
	// access. Production code always uses the default, which runs the go command
	// with the given arguments.
	goCommandContext = func(ctx context.Context, args ...string) *exec.Cmd { //nolint:gochecknoglobals // tests replace environment keys and system calls
		return exec.CommandContext(ctx, goExe, args...) //#nosec G204 -- args are built internally, not from untrusted input
	}
)
