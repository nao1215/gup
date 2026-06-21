package cmd

import (
	"context"
	"fmt"
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

// executePackages runs worker over each package with signal-based cancellation
// and a per-package timeout, then reports results via collectResults. It is the
// shared operation engine used by update, check, import, and migrate. It returns
// the exit code (1 if any package failed) and the per-package results in
// completion order.
func executePackages(pkgs []goutil.Package, cpus int, timeout time.Duration,
	worker func(context.Context, goutil.Package) updateResult,
	onResult func(prefix string, v updateResult)) (int, []updateResult) {
	ctx, cancel, signals := newSignalCancelContext()
	defer stopSignalCancelContext(cancel, signals)

	ch := forEachPackage(ctx, pkgs, cpus, timeout, worker)
	return collectResults(ch, len(pkgs), onResult)
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

	jobs := make(chan goutil.Package)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()

		for p := range jobs {
			select {
			case <-ctx.Done():
				ch <- updateResult{pkg: p, err: ctx.Err()}
			default:
				ch <- runWithTimeout(ctx, timeout, p, fn)
			}
		}
	}

	wg.Add(cpus)
	for i := 0; i < cpus; i++ {
		go worker()
	}

	go func() {
		defer close(jobs)

		for _, p := range pkgs {
			select {
			case <-ctx.Done():
				ch <- updateResult{pkg: p, err: ctx.Err()}
			case jobs <- p:
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
