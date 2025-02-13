package cmdinfo

import (
	"fmt"
	"runtime/debug"
)

// Version value is set by ldflags
var Version string

// Name is command name
const Name = "gup"

// GetVersion return gup command version.
// Version global variable is set by ldflags.
func GetVersion() string {
	version := "(devel)"
	if Version != "" {
		version = Version
	} else if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Main.Version != "" {
			version = buildInfo.Main.Version
		}
	}
	return fmt.Sprintf("%s version %s (under Apache License version 2.0)", Name, version)
}
