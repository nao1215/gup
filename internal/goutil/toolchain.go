package goutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

// envGoToolchain is the environment variable that selects the Go toolchain.
const envGoToolchain = "GOTOOLCHAIN"

// autoSuffix is the GOTOOLCHAIN suffix that lets the go command download a
// newer toolchain when one is needed.
const autoSuffix = "+auto"

// releaseGoVersionRegex matches a Go release toolchain name such as
// "go1.26.6". Development builds, release candidates and custom suffixes are
// not releases, so they never become a toolchain floor.
var releaseGoVersionRegex = regexp.MustCompile(`^go1\.\d+\.\d+$`)

// minGoToolchainKey is the context key for the toolchain floor.
type minGoToolchainKey struct{}

// WithMinGoToolchain returns a copy of ctx asking the install functions to
// build with at least goVersion (e.g. "go1.26.6"). update and migrate pass the
// Go version the binary being replaced was built with, so reinstalling a binary
// never lowers the Go it is built with even when the local go command is older.
// A goVersion that is not a Go release is ignored.
func WithMinGoToolchain(ctx context.Context, goVersion string) context.Context {
	return context.WithValue(ctx, minGoToolchainKey{}, goVersion)
}

// MinGoToolchain returns the toolchain floor stored by WithMinGoToolchain, or
// "" when there is none.
func MinGoToolchain(ctx context.Context) string {
	v, _ := ctx.Value(minGoToolchainKey{}).(string)
	return v
}

// toolchainSetting is the go command's effective GOTOOLCHAIN and the version of
// the go command itself.
type toolchainSetting struct {
	goToolchain string
	goVersion   string
}

// goToolchainSetting reads the toolchain setting once per process. It is a
// variable so tests can replace it.
var goToolchainSetting = sync.OnceValues(readGoToolchainSetting) //nolint:gochecknoglobals

// readGoToolchainSetting runs "go env GOTOOLCHAIN GOVERSION". It runs in the
// temporary directory so a go.mod in the working directory cannot switch the
// go command to another toolchain and report that toolchain's version instead.
func readGoToolchainSetting() (toolchainSetting, error) {
	var stdout, stderr bytes.Buffer
	cmd := goCommandContext(context.Background(), "env", envGoToolchain, "GOVERSION")
	cmd.Dir = os.TempDir()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return toolchainSetting{}, fmt.Errorf("can't read GOTOOLCHAIN:\n%s", stderr.String())
	}
	goToolchain, goVersion, ok := strings.Cut(strings.TrimSpace(stdout.String()), "\n")
	if !ok || strings.Contains(goVersion, "\n") {
		return toolchainSetting{}, fmt.Errorf("unexpected 'go env' output %q", stdout.String())
	}
	return toolchainSetting{
		goToolchain: strings.TrimSpace(goToolchain),
		goVersion:   strings.TrimSpace(goVersion),
	}, nil
}

// goToolchainEnv returns the GOTOOLCHAIN value "go install" must run with so
// the binary is built with at least the floor stored in ctx, or "" to leave the
// user's setting untouched. It overrides only when the setting already allows
// the go command to download a toolchain ("auto" or "<name>+auto") and the
// toolchain it would start from is older than the floor. With "local", "path"
// or a fixed version the user has ruled out downloads, so the setting is kept.
func goToolchainEnv(ctx context.Context) string {
	floor := MinGoToolchain(ctx)
	if !IsReleaseGoVersion(floor) {
		return ""
	}
	setting, err := goToolchainSetting()
	if err != nil {
		return ""
	}
	base, ok := autoSwitchBase(setting)
	if !ok || !IsReleaseGoVersion(base) || goVersionUpToDate(base, floor) {
		return ""
	}
	return floor + autoSuffix
}

// autoSwitchBase returns the toolchain the go command starts from when the
// setting lets it download a newer one, and false when downloads are not
// allowed.
func autoSwitchBase(s toolchainSetting) (string, bool) {
	switch {
	case s.goToolchain == "auto" || s.goToolchain == "local"+autoSuffix:
		return s.goVersion, true
	case strings.HasSuffix(s.goToolchain, autoSuffix):
		return strings.TrimSuffix(s.goToolchain, autoSuffix), true
	default:
		return "", false
	}
}

// IsReleaseGoVersion reports whether v names a Go release toolchain such as
// "go1.26.6".
func IsReleaseGoVersion(v string) bool {
	return releaseGoVersionRegex.MatchString(v)
}

// IsGoDowngrade reports whether a binary rebuilt with after uses an older Go
// release than before. It is false when either side is not a Go release,
// because such versions cannot be ordered reliably.
func IsGoDowngrade(before, after string) bool {
	if !IsReleaseGoVersion(before) || !IsReleaseGoVersion(after) {
		return false
	}
	return !goVersionUpToDate(after, before)
}
