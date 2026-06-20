package cmd

import (
	"context"
	"sync"
	"time"

	"github.com/nao1215/gup/internal/goutil"
)

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
