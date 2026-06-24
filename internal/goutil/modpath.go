package goutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var moduleDeclaresPathRegex = regexp.MustCompile(`(?m)module declares its path as:\s*(\S+)`)
var requiredAsPathRegex = regexp.MustCompile(`(?m)but was required as:\s*(\S+)`)

// GetLatestVer execute "$ go list -m -f {{.Version}} <importPath>@latest".
func GetLatestVer(modulePath string) (string, error) {
	return GetLatestVerWithContext(context.Background(), modulePath)
}

// GetLatestVerWithContext execute "$ go list -m -f {{.Version}} <importPath>@latest"
// with context cancellation support.
func GetLatestVerWithContext(ctx context.Context, modulePath string) (string, error) {
	return GetVerWithContext(ctx, modulePath, "latest")
}

// GetVerWithContext execute "$ go list -m -f {{.Version}} <modulePath>@<ref>"
// with context cancellation support. ref is the version selector understood by
// the go toolchain, such as "latest", "main", "master" or a concrete version.
func GetVerWithContext(ctx context.Context, modulePath, ref string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var stderr bytes.Buffer
	cmd := goCommandContext(ctx, "list", "-m", "-f", "{{.Version}}", modulePath+"@"+ref)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return "", fmt.Errorf("version check of %s timed out; run `go list -m %s@%s` manually or raise --timeout (0 disables it): %w", modulePath, modulePath, ref, ctxErr)
			}
			return "", fmt.Errorf("version check of %s canceled: %w", modulePath, ctxErr)
		}
		// A killed subprocess (e.g. SIGKILL) often writes nothing to stderr, so
		// fall back to err (e.g. "signal: killed") to always name a cause.
		detail := stderr.String()
		if strings.TrimSpace(detail) == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("can't check %s:\n%s", modulePath, detail)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// IsBranchNotFound reports whether err indicates that the given branch (e.g.
// "main") does not exist in the module's repository, as opposed to a build,
// network, authentication, or other failure. The go toolchain reports a missing
// branch as "unknown revision <branch>". This is the only condition under which
// gup is allowed to fall back from @main to @master (#340); any other failure
// must surface as-is so a wrong-branch version is never silently installed.
//
// The branch is matched as a whole token, so branch "main" does not match
// "unknown revision mainline".
func IsBranchNotFound(err error, branch string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	marker := "unknown revision " + branch
	for start := 0; ; {
		i := strings.Index(msg[start:], marker)
		if i < 0 {
			return false
		}
		after := start + i + len(marker)
		// The branch token must be followed by the end of the message or a
		// non-word byte (e.g. newline), so a prefix like "main" in "mainline"
		// is not treated as a match.
		if after == len(msg) || !isBranchWordByte(msg[after]) {
			return true
		}
		start = after
	}
}

// isBranchWordByte reports whether b is part of a branch-name token (letters,
// digits, or underscore), matching the \b word-boundary semantics used to
// delimit the branch name in IsBranchNotFound.
func isBranchWordByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// DetectModulePathMismatch detects module path mismatch errors from go command output.
// It returns:
//   - declaredPath: module path declared in go.mod
//   - requiredPath: module path that was originally required
//   - ok: true when both paths are detected and they differ
func DetectModulePathMismatch(err error) (declaredPath, requiredPath string, ok bool) {
	if err == nil {
		return "", "", false
	}

	declared := moduleDeclaresPathRegex.FindStringSubmatch(err.Error())
	required := requiredAsPathRegex.FindStringSubmatch(err.Error())
	if len(declared) < 2 || len(required) < 2 {
		return "", "", false
	}

	declaredPath = strings.TrimSpace(declared[1])
	requiredPath = strings.TrimSpace(required[1])
	if declaredPath == "" || requiredPath == "" || declaredPath == requiredPath {
		return "", "", false
	}
	return declaredPath, requiredPath, true
}
