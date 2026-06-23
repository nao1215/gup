package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

// countFormat returns the "[%Nd/%Nd]" progress format string for total
// packages, where N is the digit width of total (e.g. "[%2d/%2d]" for 10).
func countFormat(total int) string {
	digit := strconv.Itoa(len(strconv.Itoa(total)))
	return "[%" + digit + "d/%" + digit + "d]"
}

// collectResults consumes one result per package from ch in completion order
// and returns 1 if any package failed (otherwise 0) together with every result
// in completion order. Failed results print the shared "[i/n] <error>" line to
// STDERR; every non-error result is passed to onResult with the formatted
// "[i/n]" prefix so callers can render command-specific success/skip output.
// onResult may be nil (used by --json callers that render from the returned
// results instead of streaming human-readable lines).
func collectResults(ch <-chan updateResult, total int, onResult func(prefix string, v updateResult)) (int, []updateResult) {
	countFmt := countFormat(total)
	result := 0
	count := 0
	results := make([]updateResult, 0, total)
	for v := range ch {
		results = append(results, v)
		prefix := fmt.Sprintf(countFmt, count+1, total)
		if v.err != nil {
			result = 1
			print.Err(fmt.Errorf("%s %s", prefix, v.err.Error()))
		} else if onResult != nil {
			onResult(prefix, v)
		}
		count++
	}
	return result, results
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
// and a per-package timeout, then reports results via collectResults. It is the
// shared operation engine used by update, check, import, and migrate. It returns
// the exit code (1 if any package failed) and the per-package results in the
// original input order.
//
// Human-readable progress is still streamed in completion order inside
// collectResults; only the returned slice is reordered, so machine-readable
// (--json) output is deterministic across runs regardless of worker scheduling
// (#365).
func executePackages(pkgs []goutil.Package, cpus int, timeout time.Duration,
	worker func(context.Context, goutil.Package) updateResult,
	onResult func(prefix string, v updateResult)) (int, []updateResult) {
	ctx, cancel, signals := newSignalCancelContext()
	defer stopSignalCancelContext(cancel, signals)

	ch := forEachPackage(ctx, pkgs, cpus, timeout, worker)
	code, results := collectResults(ch, len(pkgs), onResult)
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].inputIndex < results[j].inputIndex
	})
	return code, results
}

// forEachPackage runs fn for each package with a fixed-size worker pool.
// It returns a channel that receives exactly len(pkgs) results.
// When timeout > 0, each package is processed under its own deadline derived
// from ctx, so a stuck go subprocess fails instead of hanging indefinitely.
// Signal-based cancellation of ctx still aborts all in-flight work.
func forEachPackage(ctx context.Context, pkgs []goutil.Package, cpus int, timeout time.Duration, fn func(context.Context, goutil.Package) updateResult) <-chan updateResult {
	ch := make(chan updateResult, len(pkgs))

	if len(pkgs) == 0 {
		close(ch)
		return ch
	}
	if cpus < 1 {
		cpus = 1
	}
	if cpus > len(pkgs) {
		cpus = len(pkgs)
	}

	// Each job carries the package's position in the input slice so the result
	// can be reordered back to input order for deterministic --json output,
	// independent of which worker finishes first (#365).
	type indexedPackage struct {
		index int
		pkg   goutil.Package
	}
	jobs := make(chan indexedPackage)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()

		for job := range jobs {
			select {
			case <-ctx.Done():
				ch <- updateResult{pkg: job.pkg, err: ctx.Err(), inputIndex: job.index}
			default:
				res := runWithTimeout(ctx, timeout, job.pkg, fn)
				res.inputIndex = job.index
				ch <- res
			}
		}
	}

	wg.Add(cpus)
	for range cpus {
		go worker()
	}

	go func() {
		defer close(jobs)

		for i, p := range pkgs {
			select {
			case <-ctx.Done():
				ch <- updateResult{pkg: p, err: ctx.Err(), inputIndex: i}
			case jobs <- indexedPackage{index: i, pkg: p}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}

// runWithTimeout invokes fn for p. When timeout > 0, fn runs under a per-package
// deadline derived from ctx; otherwise it runs under ctx unchanged.
func runWithTimeout(ctx context.Context, timeout time.Duration, p goutil.Package, fn func(context.Context, goutil.Package) updateResult) updateResult {
	if timeout <= 0 {
		return fn(ctx, p)
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(opCtx, p)
}
