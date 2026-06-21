package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/goutil"
	"github.com/nao1215/gup/internal/print"
)

// Package names reused across the parallel tests, extracted to constants to
// satisfy goconst (repeated string literals).
const (
	pkgFail = "fail"
	pkgSlow = "slow"
	pkgOK1  = "ok-1"
	pkgOK2  = "ok-2"
	pkgOK3  = "ok-3"
)

func TestForEachPackage(t *testing.T) {
	t.Parallel()

	t.Run("runs worker for each package", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		}

		ch := forEachPackage(context.Background(), pkgs, 2, 0, func(_ context.Context, p goutil.Package) updateResult {
			return updateResult{pkg: p}
		})

		got := map[string]bool{}
		for i := 0; i < len(pkgs); i++ {
			r := <-ch
			if r.err != nil {
				t.Errorf("unexpected error for %s: %v", r.pkg.Name, r.err)
			}
			got[r.pkg.Name] = true
		}

		for _, p := range pkgs {
			if !got[p.Name] {
				t.Errorf("missing result for package %s", p.Name)
			}
		}
	})

	t.Run("propagates worker errors", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: pkgFail}}
		wantErr := fmt.Errorf("test error")

		ch := forEachPackage(context.Background(), pkgs, 1, 0, func(_ context.Context, p goutil.Package) updateResult {
			return updateResult{pkg: p, err: wantErr}
		})

		r := <-ch
		if r.err == nil {
			t.Fatal("expected error, got nil")
		}
		if r.err.Error() != wantErr.Error() {
			t.Errorf("got error %q, want %q", r.err, wantErr)
		}
	})

	t.Run("returns error when context is canceled", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: "x"}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		ch := forEachPackage(ctx, pkgs, 1, 0, func(_ context.Context, p goutil.Package) updateResult {
			return updateResult{pkg: p}
		})

		r := <-ch
		if r.err == nil {
			t.Fatal("expected error from canceled context, got nil")
		}
	})

	t.Run("applies a per-package timeout", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: pkgSlow}}
		ch := forEachPackage(context.Background(), pkgs, 1, 10*time.Millisecond,
			func(ctx context.Context, p goutil.Package) updateResult {
				<-ctx.Done()
				return updateResult{pkg: p, err: ctx.Err()}
			})

		r := <-ch
		if !errors.Is(r.err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded from per-package timeout, got: %v", r.err)
		}
	})

	t.Run("no deadline when timeout is zero", func(t *testing.T) {
		t.Parallel()

		pkgs := []goutil.Package{{Name: "a"}}
		ch := forEachPackage(context.Background(), pkgs, 1, 0,
			func(ctx context.Context, p goutil.Package) updateResult {
				if _, ok := ctx.Deadline(); ok {
					return updateResult{pkg: p, err: fmt.Errorf("unexpected deadline with timeout=0")}
				}
				return updateResult{pkg: p}
			})

		r := <-ch
		if r.err != nil {
			t.Fatalf("timeout=0 should not set a deadline: %v", r.err)
		}
	})
}

func TestCountFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		total int
		want  string
	}{
		{1, "[%1d/%1d]"},
		{9, "[%1d/%1d]"},
		{10, "[%2d/%2d]"},
		{100, "[%3d/%3d]"},
	}
	for _, c := range cases {
		if got := countFormat(c.total); got != c.want {
			t.Errorf("countFormat(%d) = %q, want %q", c.total, got, c.want)
		}
	}
}

func TestCollectResults_AllSuccess(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	ch := make(chan updateResult, len(pkgs))
	for _, p := range pkgs {
		ch <- updateResult{pkg: p}
	}
	close(ch)

	var prefixes []string
	code, _ := collectResults(ch, len(pkgs), func(prefix string, _ updateResult) {
		prefixes = append(prefixes, prefix)
	})

	if code != 0 {
		t.Errorf("collectResults() = %d, want 0", code)
	}
	if got := strings.Join(prefixes, ","); got != "[1/3],[2/3],[3/3]" {
		t.Errorf("prefixes = %q, want [1/3],[2/3],[3/3]", got)
	}
}

//nolint:paralleltest // captures the global print.Stderr
func TestCollectResults_WithError(t *testing.T) {
	orgStderr := print.Stderr
	var buf bytes.Buffer
	print.Stderr = &buf
	t.Cleanup(func() { print.Stderr = orgStderr })

	ch := make(chan updateResult, 2)
	ch <- updateResult{pkg: goutil.Package{Name: "ok"}}
	ch <- updateResult{pkg: goutil.Package{Name: "bad"}, err: errors.New("boom")}
	close(ch)

	onResultCalls := 0
	code, _ := collectResults(ch, 2, func(_ string, _ updateResult) {
		onResultCalls++
	})

	if code != 1 {
		t.Errorf("collectResults() = %d, want 1 when a result has an error", code)
	}
	if onResultCalls != 1 {
		t.Errorf("onResult called %d times, want 1 (only the successful result)", onResultCalls)
	}
	if !strings.Contains(buf.String(), "[2/2] boom") {
		t.Errorf("error output = %q, want it to contain %q", buf.String(), "[2/2] boom")
	}
}

// makePkgs builds n packages with deterministic, unique names.
func makePkgs(n int) []goutil.Package {
	pkgs := make([]goutil.Package, n)
	for i := range pkgs {
		pkgs[i] = goutil.Package{Name: "pkg-" + strconv.Itoa(i)}
	}
	return pkgs
}

// TestForEachPackage_ResultCountInvariant asserts the most safety-critical
// invariant: forEachPackage emits exactly one result per package, no more and
// no fewer, for a large package set across several cpu settings. A dropped or
// double-sent job would make collectResults read the wrong number of values
// (and deadlock in production), so this verifies len(results) == len(pkgs) and
// that every package is represented exactly once.
func TestForEachPackage_ResultCountInvariant(t *testing.T) {
	t.Parallel()

	const total = 64

	for _, cpus := range []int{1, 3, 8, 64, 200} {
		cpus := cpus
		t.Run(fmt.Sprintf("cpus=%d", cpus), func(t *testing.T) {
			t.Parallel()

			pkgs := makePkgs(total)
			ch := forEachPackage(context.Background(), pkgs, cpus, 0,
				func(_ context.Context, p goutil.Package) updateResult {
					return updateResult{pkg: p}
				})

			seen := make(map[string]int, total)
			count := 0
			for r := range ch {
				if r.err != nil {
					t.Errorf("unexpected error for %s: %v", r.pkg.Name, r.err)
				}
				seen[r.pkg.Name]++
				count++
			}

			if count != total {
				t.Fatalf("received %d results, want exactly %d (one per package)", count, total)
			}
			if len(seen) != total {
				t.Fatalf("saw %d distinct packages, want %d", len(seen), total)
			}
			for name, n := range seen {
				if n != 1 {
					t.Errorf("package %s emitted %d results, want exactly 1", name, n)
				}
			}
		})
	}
}

// TestForEachPackage_PeakConcurrency observes the achieved parallelism by
// tracking, via atomic counters, the peak number of workers simultaneously
// inside the worker function. With many packages and cpus workers the peak must
// reach cpus, and must never exceed it. A barrier holds the first cpus jobs in
// the worker until all of them have arrived so the peak is observed reliably
// without real sleeps.
func TestForEachPackage_PeakConcurrency(t *testing.T) {
	t.Parallel()

	const total = 50
	const cpus = 6

	var inFlight int64
	var peak int64

	// release blocks the first cpus workers until exactly cpus of them are
	// concurrently in-flight, guaranteeing the peak is reachable.
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	atCapacity := false

	pkgs := makePkgs(total)
	ch := forEachPackage(context.Background(), pkgs, cpus, 0,
		func(_ context.Context, p goutil.Package) updateResult {
			cur := atomic.AddInt64(&inFlight, 1)
			for {
				old := atomic.LoadInt64(&peak)
				if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
					break
				}
			}

			mu.Lock()
			if cur >= cpus {
				atCapacity = true
				cond.Broadcast()
			}
			for !atCapacity {
				cond.Wait()
			}
			mu.Unlock()

			atomic.AddInt64(&inFlight, -1)
			return updateResult{pkg: p}
		})

	count := 0
	for range ch {
		count++
	}

	if count != total {
		t.Fatalf("received %d results, want %d", count, total)
	}
	if got := atomic.LoadInt64(&peak); got != cpus {
		t.Errorf("peak concurrent workers = %d, want exactly %d", got, cpus)
	}
}

// TestForEachPackage_CPUClamping covers the boundary clamping of the cpus
// argument: values < 1 clamp up to 1, and values larger than len(pkgs) clamp
// down to len(pkgs). Clamping is asserted indirectly through the achieved peak
// concurrency (it can never exceed the effective worker count) while still
// requiring the result-count invariant to hold.
func TestForEachPackage_CPUClamping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cpus     int
		pkgCount int
		wantPeak int64 // effective worker count after clamping
	}{
		{name: "zero clamps to one", cpus: 0, pkgCount: 10, wantPeak: 1},
		{name: "negative clamps to one", cpus: -5, pkgCount: 10, wantPeak: 1},
		{name: "large clamps to len(pkgs)", cpus: 100, pkgCount: 4, wantPeak: 4},
		{name: "equal stays", cpus: 4, pkgCount: 4, wantPeak: 4},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var inFlight int64
			var peak int64

			var mu sync.Mutex
			cond := sync.NewCond(&mu)
			atCapacity := false

			pkgs := makePkgs(c.pkgCount)
			ch := forEachPackage(context.Background(), pkgs, c.cpus, 0,
				func(_ context.Context, p goutil.Package) updateResult {
					cur := atomic.AddInt64(&inFlight, 1)
					for {
						old := atomic.LoadInt64(&peak)
						if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
							break
						}
					}

					mu.Lock()
					if cur >= c.wantPeak {
						atCapacity = true
						cond.Broadcast()
					}
					for !atCapacity {
						cond.Wait()
					}
					mu.Unlock()

					atomic.AddInt64(&inFlight, -1)
					return updateResult{pkg: p}
				})

			count := 0
			for range ch {
				count++
			}

			if count != c.pkgCount {
				t.Fatalf("received %d results, want %d", count, c.pkgCount)
			}
			if got := atomic.LoadInt64(&peak); got != c.wantPeak {
				t.Errorf("peak concurrent workers = %d, want %d (effective worker count after clamping)", got, c.wantPeak)
			}
		})
	}
}

// TestForEachPackage_TimeoutIsolation injects a non-zero per-package timeout
// with a mix of fast-success, immediate-failure, and slow (timing-out)
// packages, then asserts isolation: the timed-out package fails with
// DeadlineExceeded, the failing package keeps its own error, and every
// successful package still succeeds. One package's timeout must never take down
// the others.
func TestForEachPackage_TimeoutIsolation(t *testing.T) {
	t.Parallel()

	const timeout = 30 * time.Millisecond
	failErr := errors.New("boom")

	pkgs := []goutil.Package{
		{Name: pkgOK1},
		{Name: pkgOK2},
		{Name: pkgOK3},
		{Name: pkgFail},
		{Name: pkgSlow},
	}

	ch := forEachPackage(context.Background(), pkgs, 4, timeout,
		func(ctx context.Context, p goutil.Package) updateResult {
			switch p.Name {
			case pkgFail:
				return updateResult{pkg: p, err: failErr}
			case pkgSlow:
				// Block until the per-package deadline fires.
				<-ctx.Done()
				return updateResult{pkg: p, err: ctx.Err()}
			default:
				return updateResult{pkg: p, status: statusUpdated}
			}
		})

	results := make(map[string]updateResult, len(pkgs))
	for r := range ch {
		results[r.pkg.Name] = r
	}

	if len(results) != len(pkgs) {
		t.Fatalf("received %d results, want %d", len(results), len(pkgs))
	}

	for _, name := range []string{pkgOK1, pkgOK2, pkgOK3} {
		if r := results[name]; r.err != nil {
			t.Errorf("package %s failed but should have succeeded: %v", name, r.err)
		} else if r.status != statusUpdated {
			t.Errorf("package %s status = %q, want %q", name, r.status, statusUpdated)
		}
	}

	if r := results[pkgFail]; !errors.Is(r.err, failErr) {
		t.Errorf("fail package err = %v, want %v", r.err, failErr)
	}

	if r := results[pkgSlow]; !errors.Is(r.err, context.DeadlineExceeded) {
		t.Errorf("slow package err = %v, want context.DeadlineExceeded", r.err)
	}
}

// TestExecutePackages_TimeoutIsolation drives the full executePackages engine
// (the path used by update/check/import/migrate) with a non-zero timeout and a
// mix of outcomes, asserting the exit code is 1 (a package failed) while the
// successful packages are still reported. The existing tests never exercise
// executePackages with a non-zero timeout end-to-end.
func TestExecutePackages_TimeoutIsolation(t *testing.T) {
	t.Parallel()

	pkgs := []goutil.Package{
		{Name: pkgOK1},
		{Name: pkgOK2},
		{Name: pkgSlow},
	}

	var succeeded int64
	code, results := executePackages(pkgs, 2, 20*time.Millisecond,
		func(ctx context.Context, p goutil.Package) updateResult {
			if p.Name == pkgSlow {
				<-ctx.Done()
				return updateResult{pkg: p, err: ctx.Err()}
			}
			atomic.AddInt64(&succeeded, 1)
			return updateResult{pkg: p, status: statusUpdated}
		},
		func(_ string, _ updateResult) {})

	if code != 1 {
		t.Errorf("executePackages exit code = %d, want 1 (one package timed out)", code)
	}
	if len(results) != len(pkgs) {
		t.Fatalf("received %d results, want %d", len(results), len(pkgs))
	}
	if got := atomic.LoadInt64(&succeeded); got != 2 {
		t.Errorf("succeeded packages = %d, want 2 (timeout isolated from the rest)", got)
	}

	var timedOut int
	for _, r := range results {
		if errors.Is(r.err, context.DeadlineExceeded) {
			timedOut++
		}
	}
	if timedOut != 1 {
		t.Errorf("timed-out packages = %d, want exactly 1", timedOut)
	}
}

// TestForEachPackage_ProducerCancellation exercises the producer's ctx.Done
// branch while it is dispatching jobs. With a single worker and a context that
// is canceled before draining, the producer must emit ctx.Err() results for the
// undispatched packages instead of blocking forever on the unbuffered jobs
// channel. The result-count invariant must still hold (exactly one result per
// package) and at least one result must carry the cancellation error.
func TestForEachPackage_ProducerCancellation(t *testing.T) {
	t.Parallel()

	const total = 20
	pkgs := makePkgs(total)

	ctx, cancel := context.WithCancel(context.Background())

	// One worker; it processes the first job, then we cancel so the producer
	// hits its ctx.Done branch for the remaining packages.
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	ch := forEachPackage(ctx, pkgs, 1, 0,
		func(c context.Context, p goutil.Package) updateResult {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release // hold the only worker so jobs cannot drain
			if c.Err() != nil {
				return updateResult{pkg: p, err: c.Err()}
			}
			return updateResult{pkg: p}
		})

	<-started // ensure the worker is busy and the producer is mid-dispatch
	cancel()  // trigger the producer's ctx.Done branch
	close(release)

	count := 0
	canceled := 0
	for r := range ch {
		count++
		if errors.Is(r.err, context.Canceled) {
			canceled++
		}
	}

	if count != total {
		t.Fatalf("received %d results, want exactly %d even under cancellation", count, total)
	}
	if canceled == 0 {
		t.Error("expected at least one result carrying context.Canceled from the producer cancellation branch")
	}
}
