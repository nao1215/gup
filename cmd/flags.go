package cmd

import (
	"fmt"
	"time"

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
		"per-package timeout for go operations (e.g. 90s, 5m); 0 disables the timeout")
}

// getTimeoutFlag reads the shared --timeout flag.
func getTimeoutFlag(cmd *cobra.Command) (time.Duration, error) {
	v, err := cmd.Flags().GetDuration(timeoutFlagName)
	if err != nil {
		return 0, fmt.Errorf("can not parse command line argument (--%s): %w", timeoutFlagName, err)
	}
	return v, nil
}
