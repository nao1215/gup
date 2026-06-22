//nolint:paralleltest,errcheck,gosec // these tests swap package-level seams (goCommandContext) and re-exec the test binary
package goutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// Environment variables that select TestHelperProcess behavior. The harness
// re-executes the test binary with -test.run=TestHelperProcess (the Go standard
// "helper process" pattern). TestHelperProcess reads these to decide what to
// print to stdout/stderr and which exit code to use, so the subprocess-driven
// helpers in goutil.go can be exercised deterministically without any network
// access or a real go toolchain.
const (
	envHelperProcess = "GO_WANT_HELPER_PROCESS"
	envHelperStdout  = "GO_HELPER_STDOUT"
	envHelperStderr  = "GO_HELPER_STDERR"
	envHelperExit    = "GO_HELPER_EXIT"
	// envHelperFailMain makes the helper fail only when the requested ref is
	// "@main" and succeed otherwise. It is used to exercise the @main -> @master
	// fallback in InstallMainOrMaster without depending on a real network.
	envHelperFailMain = "GO_HELPER_FAIL_MAIN"
	// envHelperFailMainStderr customizes the stderr printed for the failing
	// "@main" attempt (e.g. to inject "unknown revision main").
	envHelperFailMainStderr = "GO_HELPER_FAIL_MAIN_STDERR"
)

// Shared version literals used across the helper-process tests. They are
// constants to satisfy goconst and keep the expected outputs in one place.
const (
	testVer123    = "v1.2.3"
	testGoVer1224 = "go1.22.4"
)

// helperProcessConfig describes how the helper subprocess should behave.
type helperProcessConfig struct {
	stdout string
	stderr string
	exit   int
}

// withHelperProcess swaps goCommandContext so every go invocation re-executes
// the test binary as a helper process configured by cfg. It restores the
// previous seam on cleanup. The returned function is unused but kept symmetric
// with other seam helpers in this package.
func withHelperProcess(t *testing.T, cfg helperProcessConfig) {
	t.Helper()
	old := goCommandContext
	t.Cleanup(func() { goCommandContext = old })

	goCommandContext = func(ctx context.Context, args ...string) *exec.Cmd {
		cs := append([]string{"-test.run=TestHelperProcess", "--"}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...) //#nosec G204 -- os.Args[0] is the test binary
		cmd.Env = append(os.Environ(),
			envHelperProcess+"=1",
			envHelperStdout+"="+cfg.stdout,
			envHelperStderr+"="+cfg.stderr,
			envHelperExit+"="+strconv.Itoa(cfg.exit),
		)
		return cmd
	}
}

// withHelperProcessMainMasterFallback swaps goCommandContext so that an "@main"
// install/list attempt fails (with mainStderr on stderr, exit 1) while any other
// ref succeeds. This drives the @main -> @master fallback path deterministically.
func withHelperProcessMainMasterFallback(t *testing.T, mainStderr string) {
	t.Helper()
	old := goCommandContext
	t.Cleanup(func() { goCommandContext = old })

	goCommandContext = func(ctx context.Context, args ...string) *exec.Cmd {
		cs := append([]string{"-test.run=TestHelperProcess", "--"}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...) //#nosec G204 -- os.Args[0] is the test binary
		env := append(os.Environ(),
			envHelperProcess+"=1",
			envHelperFailMain+"=1",
			envHelperFailMainStderr+"="+mainStderr,
		)
		cmd.Env = env
		return cmd
	}
}

// TestHelperProcess is not a real test. It is re-executed as a subprocess by the
// helpers above; when GO_WANT_HELPER_PROCESS is unset it returns immediately so
// it is a no-op during a normal "go test" run.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(envHelperProcess) != "1" {
		return
	}

	// Recover the original go arguments after the "--" separator.
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}

	// Fallback mode: fail only on the "@main" ref, succeed otherwise.
	if os.Getenv(envHelperFailMain) == "1" {
		if argsTargetRef(args, "main") {
			if s := os.Getenv(envHelperFailMainStderr); s != "" {
				fmt.Fprint(os.Stderr, s)
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	if s := os.Getenv(envHelperStdout); s != "" {
		fmt.Fprint(os.Stdout, s)
	}
	if s := os.Getenv(envHelperStderr); s != "" {
		fmt.Fprint(os.Stderr, s)
	}
	exit := 0
	if v := os.Getenv(envHelperExit); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			exit = n
		}
	}
	os.Exit(exit)
}

// argsTargetRef reports whether any argument ends with "@<ref>", i.e. whether
// the go command was asked to operate on that ref (e.g. importpath@main).
func argsTargetRef(args []string, ref string) bool {
	for _, a := range args {
		if strings.HasSuffix(a, "@"+ref) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// GetVerWithContext / GetLatestVerWithContext
// ---------------------------------------------------------------------------

func TestGetVerWithContext_helperProcess_versionString(t *testing.T) {
	// The go subprocess prints the resolved version with a trailing newline,
	// which GetVerWithContext must trim.
	withHelperProcess(t, helperProcessConfig{stdout: testVer123 + "\n"})

	out, err := GetVerWithContext(context.Background(), "github.com/nao1215/gup", "latest")
	if err != nil {
		t.Fatalf("GetVerWithContext() unexpected error: %v", err)
	}
	if out != testVer123 {
		t.Errorf("GetVerWithContext() = %q, want %q (trailing newline trimmed)", out, testVer123)
	}
}

func TestGetLatestVerWithContext_helperProcess_versionString(t *testing.T) {
	withHelperProcess(t, helperProcessConfig{stdout: "v0.9.0\n\n"})

	out, err := GetLatestVerWithContext(context.Background(), "github.com/nao1215/gup")
	if err != nil {
		t.Fatalf("GetLatestVerWithContext() unexpected error: %v", err)
	}
	// Only trailing newlines are trimmed (strings.TrimRight on "\n").
	if out != "v0.9.0" {
		t.Errorf("GetLatestVerWithContext() = %q, want %q", out, "v0.9.0")
	}
}

func TestGetVerWithContext_helperProcess_stderrBranch(t *testing.T) {
	// Distinct stdout (empty) + stderr + non-zero exit: the error must surface
	// the stderr text as the cause, not the empty stdout.
	withHelperProcess(t, helperProcessConfig{
		stderr: "go: unknown revision main\n",
		exit:   1,
	})

	out, err := GetVerWithContext(context.Background(), "github.com/nao1215/gup", "main")
	if err == nil {
		t.Fatalf("GetVerWithContext() should fail when the subprocess exits non-zero. got out=%q", out)
	}
	if out != "" {
		t.Errorf("GetVerWithContext() should return empty string on error. got: %q", out)
	}
	if !strings.Contains(err.Error(), "unknown revision main") {
		t.Errorf("error should carry the stderr cause. got: %v", err)
	}
	if !strings.Contains(err.Error(), "can't check") {
		t.Errorf("error should report the version-check failure. got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// InstallWithContext
// ---------------------------------------------------------------------------

func TestInstallWithContext_helperProcess_success(t *testing.T) {
	// Exit code 0 with no output: install succeeds.
	withHelperProcess(t, helperProcessConfig{})

	if err := InstallWithContext(context.Background(), "github.com/nao1215/gup", "latest"); err != nil {
		t.Fatalf("InstallWithContext() unexpected error: %v", err)
	}
}

func TestInstallWithContext_helperProcess_stderrBranch(t *testing.T) {
	// Distinct stdout vs stderr vs exit-code: stdout is ignored for failures,
	// the stderr text is reported as the cause.
	withHelperProcess(t, helperProcessConfig{
		stdout: "this stdout must be ignored\n",
		stderr: "build failed: some compile error\n",
		exit:   2,
	})

	err := InstallWithContext(context.Background(), "github.com/nao1215/gup", "latest")
	if err == nil {
		t.Fatal("InstallWithContext() should fail when the subprocess exits non-zero")
	}
	if !strings.Contains(err.Error(), "build failed: some compile error") {
		t.Errorf("error should carry the stderr cause. got: %v", err)
	}
	if strings.Contains(err.Error(), "must be ignored") {
		t.Errorf("error should not include stdout. got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// InstallMainOrMasterWithContext: @main failure -> @master fallback
// ---------------------------------------------------------------------------

func TestInstallMainOrMasterWithContext_helperProcess_masterFallbackSucceeds(t *testing.T) {
	// @main fails with "unknown revision main"; @master then succeeds. The
	// overall call must succeed (the @main error is swallowed).
	withHelperProcessMainMasterFallback(t, "go: unknown revision main\n")

	if err := InstallMainOrMasterWithContext(context.Background(), "github.com/example/tool"); err != nil {
		t.Fatalf("InstallMainOrMasterWithContext() should succeed via @master fallback. got: %v", err)
	}
}

func TestInstallMainOrMasterWithContext_helperProcess_bothFail(t *testing.T) {
	// @main is missing (branch-not-found), so @master is tried and also fails
	// (exit 1). The combined error must mention the manual-update guidance and
	// surface the @master failure.
	withHelperProcess(t, helperProcessConfig{
		stderr: "go: unknown revision main\n",
		exit:   1,
	})

	err := InstallMainOrMasterWithContext(context.Background(), "github.com/example/tool")
	if err == nil {
		t.Fatal("InstallMainOrMasterWithContext() should fail when both @main and @master fail")
	}
	if !strings.Contains(err.Error(), "cannot update with @master or @main") {
		t.Errorf("error should include manual-update guidance. got: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown revision main") {
		t.Errorf("error should surface the underlying stderr. got: %v", err)
	}
}

// TestInstallMainOrMasterWithContext_noFallbackOnGenericMainError verifies the
// #340 contract: when @main fails for a reason other than a missing branch
// (e.g. a build failure), gup must NOT silently install from @master. Here the
// fallback helper would let @master succeed, so a successful return would prove
// a wrong-branch install slipped through.
func TestInstallMainOrMasterWithContext_noFallbackOnGenericMainError(t *testing.T) {
	withHelperProcessMainMasterFallback(t, "go: build failed: some compile error\n")

	err := InstallMainOrMasterWithContext(context.Background(), "github.com/example/tool")
	if err == nil {
		t.Fatal("InstallMainOrMasterWithContext() must not fall back to @master when @main fails for non-branch reasons")
	}
	if !strings.Contains(err.Error(), "build failed: some compile error") {
		t.Errorf("error should surface the @main failure. got: %v", err)
	}
	if strings.Contains(err.Error(), "cannot update with @master or @main") {
		t.Errorf("error must not include the @master fallback guidance for a non-branch @main failure. got: %v", err)
	}
}

func TestIsBranchNotFound(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		branch string
		want   bool
	}{
		{"nil error", nil, string(UpdateChannelMain), false},
		{"main missing", fmt.Errorf("can't install x:\ngo: unknown revision main"), string(UpdateChannelMain), true},
		{"master missing", fmt.Errorf("go: unknown revision master"), string(UpdateChannelMaster), true},
		{"build failure is not branch-not-found", fmt.Errorf("build failed: compile error"), string(UpdateChannelMain), false},
		{"network failure is not branch-not-found", fmt.Errorf("dial tcp: i/o timeout"), string(UpdateChannelMain), false},
		{"wrong branch name does not match", fmt.Errorf("go: unknown revision master"), string(UpdateChannelMain), false},
		{"longer branch name is not a partial match", fmt.Errorf("go: unknown revision mainline"), string(UpdateChannelMain), false},
		{"branch token followed by newline matches", fmt.Errorf("go: unknown revision main\n"), string(UpdateChannelMain), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBranchNotFound(tt.err, tt.branch); got != tt.want {
				t.Errorf("IsBranchNotFound(%v, %q) = %v, want %v", tt.err, tt.branch, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetInstalledGoVersion
// ---------------------------------------------------------------------------

func TestGetInstalledGoVersion_helperProcess_golden(t *testing.T) {
	// Mimic real "go version" stdout; the embedded goX.Y.Z token is extracted.
	withHelperProcess(t, helperProcessConfig{
		stdout: "go version " + testGoVer1224 + " linux/amd64\n",
	})

	got, err := GetInstalledGoVersion()
	if err != nil {
		t.Fatalf("GetInstalledGoVersion() unexpected error: %v", err)
	}
	if got != testGoVer1224 {
		t.Errorf("GetInstalledGoVersion() = %q, want %q", got, testGoVer1224)
	}
}

func TestGetInstalledGoVersion_helperProcess_commandFails(t *testing.T) {
	// Non-zero exit with stderr: the error reports the version-check failure and
	// carries the stderr text.
	withHelperProcess(t, helperProcessConfig{
		stderr: "go: cannot run version\n",
		exit:   1,
	})

	_, err := GetInstalledGoVersion()
	if err == nil {
		t.Fatal("GetInstalledGoVersion() should fail when the subprocess exits non-zero")
	}
	if !strings.Contains(err.Error(), "can't check go version") {
		t.Errorf("error should report the go-version failure. got: %v", err)
	}
	if !strings.Contains(err.Error(), "cannot run version") {
		t.Errorf("error should carry the stderr cause. got: %v", err)
	}
}

func TestGetInstalledGoVersion_helperProcess_noVersionToken(t *testing.T) {
	// Exit 0 but stdout has no goX.Y.Z token: distinct stdout/exit combination
	// that still yields an error from the regexp-miss branch.
	withHelperProcess(t, helperProcessConfig{
		stdout: "not a version line\n",
	})

	_, err := GetInstalledGoVersion()
	if err == nil {
		t.Fatal("GetInstalledGoVersion() should fail when stdout has no go version token")
	}
	if !strings.Contains(err.Error(), "can't find go version string") {
		t.Errorf("error should report the missing version token. got: %v", err)
	}
}
