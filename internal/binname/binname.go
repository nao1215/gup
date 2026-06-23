// Package binname is the single source of truth for normalizing installed-binary
// names so they can be compared. Both the cmd subcommands (exclude/target
// matching) and internal/configstate (update-channel matching) route through it,
// so the matching rules can never diverge between the two.
package binname

import (
	"runtime"
	"strings"
)

const goosWindows = "windows"

// NormalizeForMatch normalizes a binary name for matching on the current host.
// It trims surrounding whitespace on every OS; on Windows it additionally
// lower-cases the name and strips a trailing ".exe" (case-insensitively), so a
// user-typed "Foo.EXE" matches the installed "foo". It is intentionally
// host-OS-aware: the names it compares (flag arguments, $GOBIN entries) always
// belong to the OS gup is currently running on.
func NormalizeForMatch(name string) string {
	return normalizeForMatchWith(name, runtime.GOOS)
}

// normalizeForMatchWith is the OS-parameterized core of NormalizeForMatch,
// exposed to tests so the Windows and non-Windows rules can be asserted
// deterministically regardless of the host running the tests.
func normalizeForMatchWith(name, goos string) string {
	name = strings.TrimSpace(name)
	if goos != goosWindows {
		return name
	}
	return strings.TrimSuffix(strings.ToLower(name), ".exe")
}
