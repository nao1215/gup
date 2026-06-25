package configstate

import (
	"slices"
	"strings"

	"github.com/nao1215/gup/internal/goutil"
)

// identityPathPrefix and identityNamePrefix namespace the two kinds of package
// identity keys so an import_path can never collide with a binary name that
// happens to be spelled the same.
const (
	identityPathPrefix = "path\x00"
	identityNamePrefix = "name\x00"
	// maxIdentityKeys is the most keys identityKeys can return (import_path + name).
	maxIdentityKeys = 2
)

// nameIdentityKey returns the name-based identity key for a binary name, applying
// the shared cross-OS normalization (trim + case-insensitive ".exe" via
// normalizeBinName). A bare binary name - such as a renamed-from entry that
// carries no import_path - can only be identified by this key.
func nameIdentityKey(name string) string {
	return identityNamePrefix + normalizeBinName(name)
}

// identityKeys returns the keys that identify p as a logical package, strongest
// first: its import_path when present, then its cross-OS normalized name (via
// nameIdentityKey). This is the single identity rule shared by
// ApplySavedChannels, ResolveChannels and MergePackages; looking the keys up in
// order yields "prefer import_path, fall back to name", so the same gup.json
// entry is interpreted identically on every command path (#341).
func identityKeys(p goutil.Package) []string {
	keys := make([]string, 0, maxIdentityKeys)
	if ip := strings.TrimSpace(p.ImportPath); ip != "" {
		keys = append(keys, identityPathPrefix+ip)
	}
	keys = append(keys, nameIdentityKey(p.Name))
	return keys
}

// normalizeBinName makes a binary name comparable regardless of the OS the
// config was exported from. It trims surrounding whitespace and strips a
// trailing ".exe" suffix case-insensitively, so a channel saved on Windows
// ("foo.exe", or a hand-edited "foo.EXE") still matches the same binary named
// "foo" on any OS (#341). Unlike binname.NormalizeForMatch this is intentionally
// OS-independent: the saved config may originate from a different OS than the
// one currently running gup. (The host-OS-aware matching used for
// channel/exclude flags lives in internal/binname, the single source of truth.)
func normalizeBinName(name string) string {
	name = strings.TrimSpace(name)
	const exe = ".exe"
	if len(name) >= len(exe) && strings.EqualFold(name[len(name)-len(exe):], exe) {
		return name[:len(name)-len(exe)]
	}
	return name
}

// sameIdentity reports whether a and b name the same logical package under the
// shared identity rule (import_path first, then cross-OS normalized name).
func sameIdentity(a, b goutil.Package) bool {
	keysB := identityKeys(b)
	for _, ka := range identityKeys(a) {
		if slices.Contains(keysB, ka) {
			return true
		}
	}
	return false
}
