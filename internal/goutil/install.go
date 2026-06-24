package goutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CanUseGoCmd check whether go command install in the system.
func CanUseGoCmd() error {
	_, err := exec.LookPath(goExe)
	return err
}

// InstallLatest execute "$ go install <importPath>@latest".
func InstallLatest(importPath string) error {
	return InstallLatestWithContext(context.Background(), importPath)
}

// InstallLatestWithContext executes "$ go install <importPath>@latest".
func InstallLatestWithContext(ctx context.Context, importPath string) error {
	return InstallWithContext(ctx, importPath, "latest")
}

// InstallMainOrMaster execute "$ go install <importPath>@main" or "$ go install <importPath>@master".
func InstallMainOrMaster(importPath string) error {
	return InstallMainOrMasterWithContext(context.Background(), importPath)
}

// InstallMainOrMasterWithContext executes "$ go install <importPath>@main"
// or "$ go install <importPath>@master" with context cancellation support.
//
// The @master fallback is taken only when @main fails because the main branch
// does not exist. Build failures, network/proxy/auth errors, timeouts, and
// cancellations on @main are returned as-is and never trigger a @master install
// (#340).
func InstallMainOrMasterWithContext(ctx context.Context, importPath string) error {
	mainErr := InstallWithContext(ctx, importPath, "main")
	if mainErr == nil {
		return nil
	}
	// A canceled/expired context would just hit @master too; surface the @main
	// error instead of retrying.
	if ctx != nil && ctx.Err() != nil {
		return mainErr
	}
	// Only a missing main branch justifies trying @master.
	if !IsBranchNotFound(mainErr, "main") {
		return mainErr
	}

	masterErr := InstallWithContext(ctx, importPath, "master")
	if masterErr == nil {
		return nil
	}
	const errMsg = "cannot update with @master or @main using the 'gup'. please update manually."
	return fmt.Errorf("%s\n%w", errMsg, masterErr)
}

// Install executes "$ go install <importPath>@<version>".
func Install(importPath, version string) error {
	return InstallWithContext(context.Background(), importPath, version)
}

// InstallWithContext executes "$ go install <importPath>@<version>".
func InstallWithContext(ctx context.Context, importPath, version string) error {
	if importPath == "command-line-arguments" {
		return errors.New("is devel-binary copied from local environment")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var stderr bytes.Buffer
	cmd := goCommandContext(ctx, "install", fmt.Sprintf("%s@%s", importPath, version))
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return fmt.Errorf("install of %s timed out; run `go install %s@%s` manually or raise --timeout (0 disables it): %w", importPath, importPath, version, ctxErr)
			}
			return fmt.Errorf("install of %s canceled: %w", importPath, ctxErr)
		}
		// A killed subprocess (e.g. SIGKILL) often writes nothing to stderr, so
		// fall back to err (e.g. "signal: killed") to always name a cause.
		detail := stderr.String()
		if strings.TrimSpace(detail) == "" {
			detail = err.Error()
		}
		return fmt.Errorf("can't install %s:\n%s", importPath, detail)
	}
	return nil
}
