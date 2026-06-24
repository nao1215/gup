// Package parallel provides a generic bounded worker pool. It runs a function
// over a slice of items using a fixed number of workers, applies an optional
// per-item timeout, supports context cancellation, and returns the results in
// the original input order.
//
// It is the shared operation engine behind the update, check, import, and
// migrate commands: each supplies its own item and result types, so the pool
// itself stays free of any command-specific concerns (printing, exit codes,
// status formatting), which live in the thin command layer.
package parallel

import (
	"context"
	"sync"
	"time"
)

// Run executes fn for each item using a pool of at most `workers` goroutines.
// Each item runs under its own deadline derived from ctx when timeout > 0,
// so a single stuck item fails instead of hanging the rest; cancellation of ctx
// aborts all in-flight and pending work.
//
// `workers` is clamped to the range [1, len(items)]. For an item that is
// skipped because ctx is already done when it would be dispatched or started,
// onCancel builds its result from the item and ctx.Err(). Each completed result
// is delivered to onComplete (when non-nil) in completion order, with its
// 1-based completion count and the total item count, so callers can stream
// progress. onComplete is invoked from Run's own goroutine, never concurrently.
//
// Run returns exactly one result per item, reordered to match the input order
// regardless of which worker finished first (#365).
func Run[T, R any](
	ctx context.Context,
	items []T,
	workers int,
	timeout time.Duration,
	fn func(context.Context, T) R,
	onCancel func(item T, err error) R,
	onComplete func(done, total int, r R),
) []R {
	total := len(items)
	if total == 0 {
		return make([]R, 0)
	}
	if workers < 1 {
		workers = 1
	}
	if workers > total {
		workers = total
	}

	type indexedResult struct {
		index int
		r     R
	}
	type job struct {
		index int
		item  T
	}

	// Buffered to total so no worker or the producer ever blocks on send; the
	// single consumer below drains it.
	resultCh := make(chan indexedResult, total)
	jobs := make(chan job)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			select {
			case <-ctx.Done():
				resultCh <- indexedResult{j.index, onCancel(j.item, ctx.Err())}
			default:
				resultCh <- indexedResult{j.index, runWithTimeout(ctx, timeout, j.item, fn)}
			}
		}
	}

	wg.Add(workers)
	for range workers {
		go worker()
	}

	go func() {
		defer close(jobs)
		for i, item := range items {
			select {
			case <-ctx.Done():
				resultCh <- indexedResult{i, onCancel(item, ctx.Err())}
			case jobs <- job{index: i, item: item}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]R, total)
	done := 0
	for ir := range resultCh {
		results[ir.index] = ir.r
		done++
		if onComplete != nil {
			onComplete(done, total, ir.r)
		}
	}
	return results
}

// runWithTimeout invokes fn for item. When timeout > 0, fn runs under a per-item
// deadline derived from ctx; otherwise it runs under ctx unchanged.
func runWithTimeout[T, R any](ctx context.Context, timeout time.Duration, item T, fn func(context.Context, T) R) R {
	if timeout <= 0 {
		return fn(ctx, item)
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(opCtx, item)
}
