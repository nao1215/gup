package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func getFlagBool(cmd *cobra.Command, name string) (bool, error) {
	v, err := cmd.Flags().GetBool(name)
	if err != nil {
		return false, fmt.Errorf("can not parse command line argument (--%s): %w", name, err)
	}
	return v, nil
}

func getFlagInt(cmd *cobra.Command, name string) (int, error) {
	v, err := cmd.Flags().GetInt(name)
	if err != nil {
		return 0, fmt.Errorf("can not parse command line argument (--%s): %w", name, err)
	}
	return v, nil
}

func getFlagString(cmd *cobra.Command, name string) (string, error) {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		return "", fmt.Errorf("can not parse command line argument (--%s): %w", name, err)
	}
	return v, nil
}

func getFlagStringSlice(cmd *cobra.Command, name string) ([]string, error) {
	v, err := cmd.Flags().GetStringSlice(name)
	if err != nil {
		return nil, fmt.Errorf("can not parse command line argument (--%s): %w", name, err)
	}
	return v, nil
}

// timeoutFlagName is the name of the shared --timeout flag.
const timeoutFlagName = "timeout"

// addTimeoutFlag registers the shared --timeout flag used by commands that run
// go subprocesses (update, check, import, migrate).
func addTimeoutFlag(cmd *cobra.Command) {
	cmd.Flags().Duration(timeoutFlagName, defaultGoOpTimeout,
		"per-package timeout for go operations (e.g. 90s, 5m); default 0 means no timeout, so a slow go install is never killed")
}

// getTimeoutFlag reads the shared --timeout flag.
func getTimeoutFlag(cmd *cobra.Command) (time.Duration, error) {
	v, err := cmd.Flags().GetDuration(timeoutFlagName)
	if err != nil {
		return 0, fmt.Errorf("can not parse command line argument (--%s): %w", timeoutFlagName, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("can not parse command line argument (--%s): must be >= 0 (use 0 to disable the timeout)", timeoutFlagName)
	}
	return v, nil
}

// noColorFlagName is the name of the persistent --no-color flag.
const noColorFlagName = "no-color"

// addNoColorFlag registers the persistent --no-color flag on the root command
// so every subcommand can disable colorized output.
func addNoColorFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(noColorFlagName, false,
		"disable colorized output (also honored via the NO_COLOR environment variable)")
}

// applyColorPreference disables colorized output when the user requests it via
// the --no-color flag or a non-empty NO_COLOR environment variable (following
// https://no-color.org/). It never re-enables color, so the automatic non-TTY
// detection that fatih/color performs at startup is preserved.
func applyColorPreference(noColor bool) {
	if noColor || os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}
