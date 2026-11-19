package cmd

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func Test_browserCommandError(t *testing.T) {
	t.Parallel()

	otherErr := errors.New("some error")

	t.Run("nil error with live context succeeds", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if got := browserCommandError(ctx, nil); got != nil {
			t.Fatalf("browserCommandError(live, nil) = %v, want nil", got)
		}
	})

	t.Run("non-timeout error passes through", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if got := browserCommandError(ctx, otherErr); !errors.Is(got, otherErr) {
			t.Fatalf("browserCommandError(live, otherErr) = %v, want it to wrap %v", got, otherErr)
		}
	})

	// A launch timeout must be reported as a failure, not silently treated as
	// success, even when the raw command error is nil: the launcher commands
	// return promptly once the URL is handed to the browser, so an expired context
	// means the launch did not complete reliably and bug-report should fall back to
	// printing the issue template.
	t.Run("expired context is a failure even when err is nil", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		<-ctx.Done()
		got := browserCommandError(ctx, nil)
		if got == nil {
			t.Fatal("browserCommandError(expired, nil) = nil, want an error")
		}
		if !errors.Is(got, context.DeadlineExceeded) {
			t.Fatalf("browserCommandError(expired, nil) = %v, want it to wrap context.DeadlineExceeded", got)
		}
	})
}

// Test_runBrowserCommand_timeoutIsFailure exercises the real exec path: a
// launcher that runs longer than openBrowserTimeout must surface a (non-nil)
// error so openBrowser reports failure and bug-report shows the fallback guide,
// rather than the timeout being swallowed as a false success.
//
//nolint:paralleltest // mutates the package-level openBrowserTimeout
func Test_runBrowserCommand_timeoutIsFailure(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("relies on the POSIX 'sleep' command")
	}

	orig := openBrowserTimeout
	t.Cleanup(func() { openBrowserTimeout = orig })
	openBrowserTimeout = 50 * time.Millisecond

	err := runBrowserCommand("sleep", "5")
	if err == nil {
		t.Fatal("runBrowserCommand() should return an error when the launcher times out")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runBrowserCommand() error = %v, want it to wrap context.DeadlineExceeded", err)
	}
}
