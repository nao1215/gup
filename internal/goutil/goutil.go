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

// Internal variables to mock/monkey-patch behaviors in tests.
var (
	// goExe is the executable name for the go command.
	goExe = "go" //nolint:gochecknoglobals
	// keyGoBin is the key name of the env variable for "GOBIN".
	keyGoBin = "GOBIN" //nolint:gochecknoglobals
	// keyGoPath is the key name of the env variable for "GOPATH".
	keyGoPath = "GOPATH" //nolint:gochecknoglobals
	// osMkdirTemp is a copy of os.MkdirTemp to ease testing.
	osMkdirTemp = os.MkdirTemp //nolint:gochecknoglobals
	// goCommandContext builds the *exec.Cmd used to run the go toolchain. It is a
	// variable so tests can swap in the standard "helper process" pattern
	// (re-executing the test binary) and exercise the subprocess-driven helpers
	// deterministically without network access. Production code always uses the
	// default, which simply runs goExe with the given arguments.
	goCommandContext = func(ctx context.Context, args ...string) *exec.Cmd { //nolint:gochecknoglobals
		return exec.CommandContext(ctx, goExe, args...) //#nosec G204 -- args are built internally, not from untrusted input
	}
)
