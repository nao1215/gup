// Package diagnose turns the raw, often cryptic output of the Go toolchain
// (surfaced when a package fails to update) into a short, actionable next-step
// hint. gup already holds the full error context for every failed package, so
// telling the user what to try next is nearly free and saves them from
// deciphering "no matching versions for query" or "unrecognized import path"
// by hand.
//
// Hint is intentionally conservative: it only returns text for failure modes it
// can confidently recognize and returns "" otherwise, so a hint always adds
// signal and never guesses.
package diagnose

import (
	"regexp"
	"strings"

	"github.com/nao1215/gup/internal/goutil"
)

// matcher maps a recognizable failure mode to the hint to emit. A matcher fires
// when any of its needles is a substring of the lower-cased error text, or when
// its match predicate (used for non-literal cases like regexes) returns true.
// The first matching entry in matchers wins, so entries are ordered
// most-specific first.
//
// needles are kept as data (rather than hidden inside a closure) so a test can
// assert every literal needle is still present in a real Go toolchain message:
// the toolchain's English wording is an external contract that a Go release can
// change, silently breaking these matchers, and that test turns such a drift
// into a visible regression. needles must be lower-case because they are matched
// against the lower-cased error text (TestMatcherNeedlesAreLowercase guards it).
type matcher struct {
	needles []string
	match   func(lower string) bool
	hint    string
}

// matches reports whether the matcher fires for the lower-cased error text.
func (m matcher) matches(lower string) bool {
	for _, n := range m.needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	if m.match != nil {
		return m.match(lower)
	}
	return false
}

// goToolchainRegex matches the toolchain-too-old diagnostic, e.g.
// "note: module requires Go 1.23" or "requires go >= 1.23".
var goToolchainRegex = regexp.MustCompile(`requires go ?>?=? ?\d`)

// matchers is consulted in order; keep the most specific failure modes first so
// they win over the broader network/permission fallbacks.
var matchers = []matcher{ //nolint:gochecknoglobals
	{
		needles: []string{"is not installed by 'go install'"},
		hint:    "This binary has no embedded module path. Reinstall it once with `go install <importpath>@latest` so gup can manage it, then run gup again.",
	},
	{
		// "module ... found (vX), but does not contain package ..." is what the
		// go command reports when the command's import path no longer exists in
		// the module at the resolved version. The common cause is a major-version
		// bump: the tool moved to a /v2+ module path, so the old import path is
		// gone even though the v0/v1 line still resolves.
		needles: []string{"does not contain package", "no required module provides package"},
		hint:    "The module no longer provides this command at its import path. The project likely relocated the command (a separate repo/module) or bumped to a new major version (e.g. a `/v2` module path); check its current install instructions and reinstall with the new path.",
	},
	{
		// `go install` refuses modules whose go.mod carries replace directives.
		needles: []string{"replace directives", "replace directive"},
		hint:    "This module's go.mod uses `replace` directives, which `go install` cannot build. Ask the maintainer to drop them, or clone the repository and run `go install` inside it (gup will then treat the binary as built-from-source and skip it).",
	},
	{
		needles: []string{"devel-binary copied from local environment", "command-line-arguments"},
		hint:    "This binary was built from a local checkout (devel) and has no published module. Reinstall it from its upstream repository with `go install <importpath>@latest`.",
	},
	{
		match: func(lower string) bool { return goToolchainRegex.MatchString(lower) },
		hint:  "This module requires a newer Go toolchain. Upgrade Go, or set `GOTOOLCHAIN=auto` so the go command fetches the required version automatically.",
	},
	{
		needles: []string{"build constraints exclude all go files"},
		hint:    "The package has no Go files buildable for your platform (see `go env GOOS GOARCH`); this version may not support your OS/architecture.",
	},
	{
		needles: []string{"no matching versions for query"},
		hint:    "No published version matches the selected channel. Try another channel (e.g. `--main` or `--master`), or confirm the repository has tagged releases.",
	},
	{
		needles: []string{"unknown revision", "invalid version", "unknown branch"},
		hint:    "The requested branch, tag, or version does not exist. Verify the channel (main vs master) or the version selector for this package.",
	},
	{
		// Repository/auth failures are matched before the generic "permission
		// denied" case below so an SSH auth error ("permission denied
		// (publickey)") is diagnosed as an access problem, not local write
		// permission.
		needles: []string{"unrecognized import path", "repository not found", "404 not found", "410 gone",
			"terminal prompts disabled", "permission denied (publickey)", "could not read from remote repository"},
		hint: "The module path could not be resolved. The repository may be private, renamed, or deleted; check the import path and your access/credentials (e.g. GOPRIVATE, SSH/token auth).",
	},
	{
		// Generic local install-permission error. Exclude the Git/SSH auth case,
		// which is handled by the repository matcher above.
		match: func(lower string) bool {
			return (strings.Contains(lower, "permission denied") || strings.Contains(lower, "operation not permitted")) &&
				!strings.Contains(lower, "permission denied (publickey)")
		},
		hint: "Permission denied while installing. Check write access to your install directory (`go env GOBIN` / `go env GOPATH`) and the module cache.",
	},
	{
		needles: []string{"dial tcp", "i/o timeout", "connection refused", "tls handshake", "proxyconnect", "no such host", "network is unreachable", "could not connect"},
		hint:    "Network error reaching the module proxy or repository. Check your connection and the GOPROXY setting (`go env GOPROXY`).",
	},
}

// Hint returns a short, actionable next-step suggestion for a failed package
// update, or "" when the cause is not confidently recognized.
//
// Two cases are deliberately skipped: a nil error, and timeout/cancellation
// errors whose message already names the manual command and the --timeout
// remedy (adding a hint there would only duplicate that guidance).
func Hint(err error) string {
	if err == nil {
		return ""
	}

	// Module path renames get a precise hint naming the new path, reusing the
	// same detector gup uses to auto-follow renames. This is checked first
	// because the raw error also contains "version constraints conflict" text
	// that none of the substring matchers should claim.
	if declared, _, ok := goutil.DetectModulePathMismatch(err); ok {
		return "The module appears to have moved to " + declared +
			". gup tries to follow renames automatically; if it still fails, reinstall manually with `go install " + declared + "@latest`."
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// Timeout/cancellation messages already carry their own remedy. The go
	// toolchain and context package spell it "canceled" (American).
	if strings.Contains(lower, "timed out") || strings.Contains(lower, "canceled") ||
		strings.Contains(lower, "deadline exceeded") {
		return ""
	}

	for _, m := range matchers {
		if m.matches(lower) {
			return m.hint
		}
	}
	return ""
}
