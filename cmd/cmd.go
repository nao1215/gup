// Package cmd runs the gup command line.
//
// Deprecated: gup is a command, not a library. Its implementation moved to an
// internal package, and this package remains only so that v1 importers keep
// building; it will be removed in v2. Run the gup binary instead of importing
// it.
package cmd

import (
	"os"
	"runtime"

	internalcmd "github.com/nao1215/gup/internal/cmd"
)

// GOOS is the operating system gup behaves as when it resolves binary names.
//
// Deprecated: it is a test seam of gup's own tests. Execute copies it into the
// internal implementation; it will be removed in v2.
var GOOS = runtime.GOOS //nolint:gochecknoglobals // kept for v1 compatibility

// OsExit is called with a subcommand's exit status.
//
// Deprecated: it is a test seam of gup's own tests. Execute copies it into the
// internal implementation; it will be removed in v2.
var OsExit = os.Exit //nolint:gochecknoglobals // kept for v1 compatibility

// Execute runs gup with the process arguments, as the gup binary does.
//
// Deprecated: run the gup binary instead; this function will be removed in v2.
func Execute() error {
	internalcmd.GOOS = GOOS
	internalcmd.OsExit = OsExit
	return internalcmd.Execute()
}
