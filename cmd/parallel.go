package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/nao1215/gup/internal/diagnose"
	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/parallel"
	"github.com/nao1215/gup/internal/print"
)

// countFormat returns the "[%Nd/%Nd]" progress format string for total
// packages, where N is the digit width of total (e.g. "[%2d/%2d]" for 10).
func countFormat(total int) string {
	digit := strconv.Itoa(len(strconv.Itoa(total)))
	return "[%" + digit + "d/%" + digit + "d]"
}

// summarizeResults builds the one-line summary printed in --quiet mode from the
// per-package results. isCheck selects the check wording (update-available)
// over the update wording (updated). Failures are counted first because a
// failed result may still carry a non-error status.
//
// The status cases below cover every non-failed result that check and update
// produce: check sets statusUpToDate or statusUpdateAvailable, and update sets
// statusUpToDate or statusUpdated. statusError is reached only with v.err set
// (counted as failed above), and statusInstalled is list-only, so neither needs
// a status case here. summarizeResults is not used by other commands.
func summarizeResults(results []updateResult, isCheck bool) string {
	var updated, upToDate, available, failed int
	for _, v := range results {
		switch {
		case v.err != nil:
			failed++
		case v.status == statusUpdateAvailable:
			available++
		case v.status == statusUpdated:
			updated++
		case v.status == statusUpToDate:
			upToDate++
		}
	}
	if isCheck {
		return fmt.Sprintf("gup: %d update available, %d up-to-date, %d failed", available, upToDate, failed)
	}
	return fmt.Sprintf("gup: %d updated, %d up-to-date, %d failed", updated, upToDate, failed)
}

// executePackages runs worker over each package with signal-based cancellation
// and a per-package timeout, returning the exit code (1 if any package failed)
// and the per-package results in the original input order. It is the thin
// command-side adapter over the generic internal/parallel engine: it owns the
// signal context and the command-specific output (the shared "[i/n] <error>"
// line on STDERR for failures, and the command's own success/skip line via
// onResult for everything else).
//
// Human-readable progress is streamed in completion order, while the returned
// slice is in input order, so machine-readable (--json) output is deterministic
// across runs regardless of worker scheduling (#365). onResult may be nil
// (used by --json callers that render from the returned results instead).
func executePackages(pkgs []goutil.Package, cpus int, timeout time.Duration,
	worker func(context.Context, goutil.Package) updateResult,
	onResult func(prefix string, v updateResult)) (int, []updateResult) {
	ctx, cancel, signals := newSignalCancelContext()
	defer stopSignalCancelContext(cancel, signals)

	countFmt := countFormat(len(pkgs))
	exitCode := 0
	results := parallel.Run(ctx, pkgs, cpus, timeout, worker,
		func(p goutil.Package, err error) updateResult {
			return updateResult{pkg: p, err: err}
		},
		func(done, total int, v updateResult) {
			prefix := fmt.Sprintf(countFmt, done, total)
			if v.err != nil {
				exitCode = 1
				print.Err(fmt.Errorf("%s %s", prefix, v.err.Error()))
				if hint := diagnose.Hint(v.err); hint != "" {
					print.Hint(hint)
				}
			} else if onResult != nil {
				onResult(prefix, v)
			}
		})
	return exitCode, results
}
