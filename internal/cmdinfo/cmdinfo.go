package cmdinfo

import (
	"fmt"
)

const (
	name    = "gup"
	version = "0.7.1"
)

// Version return gup command version.
func Version() string {
	return fmt.Sprintf("%s version %s (under Apache License version 2.0)",
		Name(), version)
}

// Name return command name.
func Name() string {
	return name
}
