//nolint:paralleltest // these tests swap package-level seams (goToolchainSetting, goCommandContext)
package goutil

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// Toolchain names shared by the tests below.
const (
	toolchainAuto = "auto"
	goVer1264     = "go1.26.4"
	goVer1266     = "go1.26.6"
	goVer1268     = "go1.26.8"
	goVer1269     = "go1.26.9"
	// galBuiltGo is the Go the cmd/testdata "gal" fixture was built with. It is
	// a pre-1.21 language version, not a toolchain name.
	galBuiltGo = "go1.18"
)

// withToolchainSetting replaces the cached "go env" result for one test.
func withToolchainSetting(t *testing.T, s toolchainSetting, err error) {
	t.Helper()
	old := goToolchainSetting
	t.Cleanup(func() { goToolchainSetting = old })
	goToolchainSetting = func() (toolchainSetting, error) { return s, err }
}

func TestGoToolchainEnv(t *testing.T) {
	tests := []struct {
		name    string
		floor   string
		setting toolchainSetting
		want    string
	}{
		{"older local go is raised to the floor", goVer1266, toolchainSetting{toolchainAuto, goVer1264}, goVer1266 + autoSuffix},
		{"local+auto behaves like auto", goVer1266, toolchainSetting{"local+auto", goVer1264}, goVer1266 + autoSuffix},
		{"double digit patch compares numerically", "go1.26.10", toolchainSetting{toolchainAuto, goVer1269}, "go1.26.10+auto"},
		{"older minor is raised", "go1.27.0", toolchainSetting{toolchainAuto, goVer1269}, "go1.27.0+auto"},
		{"same version is left alone", goVer1264, toolchainSetting{toolchainAuto, goVer1264}, ""},
		{"newer local go is left alone", goVer1264, toolchainSetting{toolchainAuto, goVer1268}, ""},
		{"fixed start toolchain older than the floor", goVer1266, toolchainSetting{"go1.26.2+auto", goVer1268}, goVer1266 + autoSuffix},
		{"fixed start toolchain newer than the floor", goVer1266, toolchainSetting{"go1.26.7+auto", goVer1264}, ""},
		{"local forbids downloads", goVer1266, toolchainSetting{"local", goVer1264}, ""},
		{"path forbids downloads", goVer1266, toolchainSetting{"path", goVer1264}, ""},
		{"local+path forbids downloads", goVer1266, toolchainSetting{"local+path", goVer1264}, ""},
		{"a fixed version is the user's choice", goVer1266, toolchainSetting{"go1.26.2", "go1.26.2"}, ""},
		{"go before 1.21 has no GOTOOLCHAIN", goVer1266, toolchainSetting{"", "go1.20.14"}, ""},
		{"no floor", "", toolchainSetting{toolchainAuto, goVer1264}, ""},
		{"devel floor is not a release", "devel", toolchainSetting{toolchainAuto, goVer1264}, ""},
		{"rc floor is not a release", "go1.27rc1", toolchainSetting{toolchainAuto, goVer1264}, ""},
		{"language version floor is not a toolchain", galBuiltGo, toolchainSetting{toolchainAuto, "go1.17.13"}, ""},
		{"devel local go is left alone", goVer1266, toolchainSetting{toolchainAuto, "devel go1.27-abcdef"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withToolchainSetting(t, tt.setting, nil)
			ctx := WithMinGoToolchain(context.Background(), tt.floor)
			if got := goToolchainEnv(ctx); got != tt.want {
				t.Errorf("goToolchainEnv(floor=%q, %+v) = %q, want %q", tt.floor, tt.setting, got, tt.want)
			}
		})
	}
}

func TestGoToolchainEnv_settingUnreadable(t *testing.T) {
	withToolchainSetting(t, toolchainSetting{}, errors.New("go env failed"))
	ctx := WithMinGoToolchain(context.Background(), goVer1266)
	if got := goToolchainEnv(ctx); got != "" {
		t.Errorf("goToolchainEnv() = %q, want the user's setting kept when go env fails", got)
	}
}

func TestGoToolchainEnv_noFloorInContext(t *testing.T) {
	withToolchainSetting(t, toolchainSetting{toolchainAuto, goVer1264}, nil)
	if got := goToolchainEnv(context.Background()); got != "" {
		t.Errorf("goToolchainEnv() = %q, want empty without a floor", got)
	}
}

func TestInstallWithContext_passesToolchainFloor(t *testing.T) {
	withToolchainSetting(t, toolchainSetting{toolchainAuto, goVer1264}, nil)
	withEchoToolchainHelper(t)

	ctx := WithMinGoToolchain(context.Background(), goVer1266)
	err := InstallWithContext(ctx, "github.com/example/tool", "latest")
	if err == nil {
		t.Fatal("the helper always fails; want an error carrying its output")
	}
	if !strings.Contains(err.Error(), "GOTOOLCHAIN=["+goVer1266+autoSuffix+"]") {
		t.Errorf("go install must run with GOTOOLCHAIN=go1.26.6+auto. got: %v", err)
	}
	if !strings.Contains(err.Error(), "with GOTOOLCHAIN="+goVer1266+autoSuffix) {
		t.Errorf("the error must name the toolchain gup asked for. got: %v", err)
	}
}

func TestInstallWithContext_keepsUserToolchain(t *testing.T) {
	t.Setenv(envGoToolchain, "local")
	withToolchainSetting(t, toolchainSetting{"local", goVer1264}, nil)
	withEchoToolchainHelper(t)

	ctx := WithMinGoToolchain(context.Background(), goVer1266)
	err := InstallWithContext(ctx, "github.com/example/tool", "latest")
	if err == nil {
		t.Fatal("the helper always fails; want an error carrying its output")
	}
	if !strings.Contains(err.Error(), "GOTOOLCHAIN=[local]") {
		t.Errorf("GOTOOLCHAIN=local must reach go install unchanged. got: %v", err)
	}
	if strings.Contains(err.Error(), "+auto") {
		t.Errorf("no toolchain may be forced when downloads are ruled out. got: %v", err)
	}
}

// withEchoToolchainHelper makes every go invocation fail and print the
// GOTOOLCHAIN it saw. The command's own Env is kept, so what InstallWithContext
// sets is what the helper prints.
func withEchoToolchainHelper(t *testing.T) {
	t.Helper()
	old := goCommandContext
	t.Cleanup(func() { goCommandContext = old })
	goCommandContext = func(ctx context.Context, args ...string) *exec.Cmd {
		cmd := helperCommand(ctx, args...)
		// The helper markers go into the process environment so they survive
		// InstallWithContext replacing cmd.Env with os.Environ() plus GOTOOLCHAIN.
		t.Setenv(envHelperProcess, "1")
		t.Setenv(envHelperEchoToolchain, "1")
		return cmd
	}
}

func TestReadGoToolchainSetting(t *testing.T) {
	withHelperProcess(t, helperProcessConfig{stdout: "auto\ngo1.26.4\n"})
	got, err := readGoToolchainSetting()
	if err != nil {
		t.Fatalf("readGoToolchainSetting() unexpected error: %v", err)
	}
	want := toolchainSetting{goToolchain: toolchainAuto, goVersion: goVer1264}
	if got != want {
		t.Errorf("readGoToolchainSetting() = %+v, want %+v", got, want)
	}
}

func TestReadGoToolchainSetting_errors(t *testing.T) {
	t.Run("command fails", func(t *testing.T) {
		withHelperProcess(t, helperProcessConfig{stderr: "boom", exit: 1})
		if _, err := readGoToolchainSetting(); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("want an error carrying stderr, got %v", err)
		}
	})
	t.Run("unexpected output", func(t *testing.T) {
		withHelperProcess(t, helperProcessConfig{stdout: "auto\n"})
		if _, err := readGoToolchainSetting(); err == nil {
			t.Error("want an error for a one-line output")
		}
	})
}

func TestIsGoDowngrade(t *testing.T) {
	tests := []struct {
		before, after string
		want          bool
	}{
		{goVer1266, goVer1264, true},
		{"go1.26.10", goVer1269, true},
		{"go1.27.0", goVer1269, true},
		{goVer1264, goVer1264, false},
		{goVer1264, goVer1268, false},
		{"devel", goVer1264, false},
		{goVer1266, unknown, false},
		{"", goVer1264, false},
	}
	for _, tt := range tests {
		if got := IsGoDowngrade(tt.before, tt.after); got != tt.want {
			t.Errorf("IsGoDowngrade(%q, %q) = %v, want %v", tt.before, tt.after, got, tt.want)
		}
	}
}
