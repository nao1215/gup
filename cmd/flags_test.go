package cmd

import (
	"io"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/nao1215/gup/internal/print"
	"github.com/spf13/cobra"
)

// Test_applyColorPreference verifies that color is disabled by the --no-color
// flag or a non-empty NO_COLOR environment variable, and left untouched
// otherwise (issue #309).
func Test_applyColorPreference(t *testing.T) {
	tests := []struct {
		name       string
		noColor    bool
		noColorEnv string
		want       bool
	}{
		{name: "flag true disables color", noColor: true, noColorEnv: "", want: true},
		{name: "NO_COLOR=1 disables color", noColor: false, noColorEnv: "1", want: true},
		{name: "non-empty NO_COLOR disables color", noColor: false, noColorEnv: "0", want: true},
		{name: "flag false and empty NO_COLOR keeps color", noColor: false, noColorEnv: "", want: false},
		{name: "flag and NO_COLOR both set disable color", noColor: true, noColorEnv: "1", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldNoColor := color.NoColor
			color.NoColor = false
			t.Cleanup(func() { color.NoColor = oldNoColor })
			t.Setenv("NO_COLOR", tt.noColorEnv)

			applyColorPreference(tt.noColor)

			if color.NoColor != tt.want {
				t.Errorf("applyColorPreference(%v) with NO_COLOR=%q: color.NoColor=%v, want %v",
					tt.noColor, tt.noColorEnv, color.NoColor, tt.want)
			}
		})
	}
}

// Test_rootCmd_noColorFlag_disablesColorViaExecute verifies the --no-color
// persistent flag is inherited by subcommands and applied through the root
// PersistentPreRunE (issue #309).
func Test_rootCmd_noColorFlag_disablesColorViaExecute(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = false
	t.Setenv("NO_COLOR", "") // ensure the env var does not interfere
	t.Cleanup(func() { color.NoColor = oldNoColor })

	orgStdout := print.Stdout
	print.Stdout = io.Discard // `gup version` prints via print.Info
	t.Cleanup(func() { print.Stdout = orgStdout })

	cmd := newRootCmd()
	cmd.SetArgs([]string{"version", "--no-color"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !color.NoColor {
		t.Error("`gup version --no-color` should disable color via the root PersistentPreRunE")
	}
}

func TestGetFlagBool(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().Bool("test", true, "")
		v, err := getFlagBool(cmd, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !v {
			t.Errorf("got %v, want true", v)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		_, err := getFlagBool(cmd, "no-such-flag")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetFlagInt(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().Int("jobs", 4, "")
		v, err := getFlagInt(cmd, "jobs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 4 {
			t.Errorf("got %d, want 4", v)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		_, err := getFlagInt(cmd, "no-such-flag")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetFlagString(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().String("input", "foo.conf", "")
		v, err := getFlagString(cmd, "input")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "foo.conf" {
			t.Errorf("got %q, want %q", v, "foo.conf")
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		_, err := getFlagString(cmd, "no-such-flag")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetFlagStringSlice(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().StringSlice("exclude", []string{"a", "b"}, "")
		v, err := getFlagStringSlice(cmd, "exclude")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(v) != 2 || v[0] != "a" || v[1] != "b" {
			t.Errorf("got %v, want [a b]", v)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		_, err := getFlagStringSlice(cmd, "no-such-flag")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetTimeoutFlag(t *testing.T) {
	t.Parallel()

	t.Run("default is disabled (no timeout)", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		addTimeoutFlag(cmd)
		v, err := getTimeoutFlag(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The default must be 0 so a slow "go install" is never killed
		// (issue #318). This restores the pre-v1.3.0 behavior.
		if v != 0 {
			t.Errorf("default timeout should be 0 (disabled), got %v", v)
		}
	})

	t.Run("zero disables the timeout", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		addTimeoutFlag(cmd)
		if err := cmd.Flags().Set(timeoutFlagName, "0"); err != nil {
			t.Fatal(err)
		}
		v, err := getTimeoutFlag(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 0 {
			t.Errorf("got %v, want 0", v)
		}
	})

	t.Run("custom positive value", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		addTimeoutFlag(cmd)
		if err := cmd.Flags().Set(timeoutFlagName, "90s"); err != nil {
			t.Fatal(err)
		}
		v, err := getTimeoutFlag(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 90*time.Second {
			t.Errorf("got %v, want 90s", v)
		}
	})

	t.Run("negative value is rejected", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		addTimeoutFlag(cmd)
		if err := cmd.Flags().Set(timeoutFlagName, "-1s"); err != nil {
			t.Fatal(err)
		}
		if _, err := getTimeoutFlag(cmd); err == nil {
			t.Fatal("expected error for negative timeout, got nil")
		}
	})

	t.Run("missing flag", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		if _, err := getTimeoutFlag(cmd); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
