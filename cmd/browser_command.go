package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// openBrowserTimeout bounds how long we wait for the browser launcher to return.
// It is a var (not a const) so tests can shrink it to exercise the timeout path.
var openBrowserTimeout = 5 * time.Second //nolint:gochecknoglobals // overridden in tests

// runBrowserCommand executes browser launcher commands. This is swapped in tests.
var runBrowserCommand = func(command string, args ...string) error { //nolint:gochecknoglobals
	ctx, cancel := context.WithTimeout(context.Background(), openBrowserTimeout)
	defer cancel()

	// Command names are hard-coded in openBrowser and URL is internally generated.
	err := exec.CommandContext(ctx, command, args...).Run() //nolint:gosec
	return browserCommandError(ctx, err)
}

// browserCommandError maps the result of launching the browser to success or
// failure for bug-report. A launch timeout is treated as a failure, not success:
// the launcher commands (xdg-open/open/rundll32) return promptly once the URL is
// handed to the browser, so a launcher still running after openBrowserTimeout
// means the launch did not complete reliably. Reporting it as an error lets
// bug-report fall back to printing the issue template instead of silently doing
// nothing. The decision is driven by ctx.Err() rather than the raw command error
// because a killed process surfaces as an OS "signal: killed" error that does not
// wrap context.DeadlineExceeded.
func browserCommandError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("browser launch did not complete within %s: %w", openBrowserTimeout, ctx.Err())
	}
	return err
}
