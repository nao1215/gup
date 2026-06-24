package parallel

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	itemFail = "fail"
	itemSlow = "slow"
)

// res is the per-item result type used by the tests. The engine is generic, so
// any type works; this mirrors how cmd carries an error and a status.
type res struct {
	name   string
	err    error
	status string
}

func okWorker(_ context.Context, name string) res { return res{name: name} }

func onCancel(name string, err error) res { return res{name: name, err: err} }

func names(n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = "item-" + strconv.Itoa(i)
	}
	return items
}

func TestRun_runsWorkerForEachItem(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c"}
	got := Run(context.Background(), items, 2, 0, okWorker, onCancel, nil)

	if len(got) != len(items) {
		t.Fatalf("len(results) = %d, want %d", len(got), len(items))
	}
	for i, r := range got {
		if r.err != nil {
			t.Errorf("unexpected error for %s: %v", r.name, r.err)
		}
		if r.name != items[i] {
			t.Errorf("results[%d].name = %q, want %q", i, r.name, items[i])
		}
	}
}

func TestRun_propagatesWorkerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("test error")
	got := Run(context.Background(), []string{itemFail}, 1, 0,
		func(_ context.Context, name string) res { return res{name: name, err: wantErr} },
		onCancel, nil)

	if len(got) != 1 || !errors.Is(got[0].err, wantErr) {
		t.Fatalf("results = %+v, want one carrying %v", got, wantErr)
	}
}

func TestRun_canceledContextUsesOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := Run(ctx, []string{"x"}, 1, 0, okWorker, onCancel, nil)
	if len(got) != 1 || got[0].err == nil {
		t.Fatalf("results = %+v, want one carrying a cancellation error", got)
	}
}

func TestRun_appliesPerItemTimeout(t *testing.T) {
	t.Parallel()

	got := Run(context.Background(), []string{itemSlow}, 1, 10*time.Millisecond,
		func(ctx context.Context, name string) res {
			<-ctx.Done()
			return res{name: name, err: ctx.Err()}
		},
		onCancel, nil)

	if len(got) != 1 || !errors.Is(got[0].err, context.DeadlineExceeded) {
		t.Fatalf("results = %+v, want deadline exceeded from per-item timeout", got)
	}
}

func TestRun_noDeadlineWhenTimeoutZero(t *testing.T) {
	t.Parallel()

	got := Run(context.Background(), []string{"a"}, 1, 0,
		func(ctx context.Context, name string) res {
			if _, ok := ctx.Deadline(); ok {
				return res{name: name, err: errors.New("unexpected deadline with timeout=0")}
			}
			return res{name: name}
		},
		onCancel, nil)

	if len(got) != 1 || got[0].err != nil {
		t.Fatalf("timeout=0 should not set a deadline: %+v", got)
	}
}

func TestRun_empty(t *testing.T) {
	t.Parallel()

	got := Run(context.Background(), nil, 4, 0, okWorker, onCancel, nil)
	if len(got) != 0 {
		t.Fatalf("len(results) = %d, want 0 for no items", len(got))
	}
}

// TestRun_resultCountInvariant asserts the most safety-critical invariant:
// exactly one result per item, in input order, across several worker counts
// (including more workers than items). A dropped or double-sent job would break
// len(results) == len(items).
func TestRun_resultCountInvariant(t *testing.T) {
	t.Parallel()

	const total = 64
	for _, workers := range []int{1, 3, 8, 64, 200} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Parallel()

			items := names(total)
			got := Run(context.Background(), items, workers, 0, okWorker, onCancel, nil)

			if len(got) != total {
				t.Fatalf("len(results) = %d, want %d", len(got), total)
			}
			for i, r := range got {
				if r.err != nil {
					t.Errorf("unexpected error for %s: %v", r.name, r.err)
				}
				if r.name != items[i] {
					t.Errorf("results[%d].name = %q, want %q (input order)", i, r.name, items[i])
				}
			}
		})
	}
}

func TestRun_onCompleteSeesEveryResultInOrderCounts(t *testing.T) {
	t.Parallel()

	const total = 6
	items := names(total)

	var mu sync.Mutex
	var dones []int
	got := Run(context.Background(), items, 1, 0, okWorker, onCancel,
		func(done, tot int, _ res) {
			if tot != total {
				t.Errorf("onComplete total = %d, want %d", tot, total)
			}
			mu.Lock()
			dones = append(dones, done)
			mu.Unlock()
		})

	if len(got) != total {
		t.Fatalf("len(results) = %d, want %d", len(got), total)
	}
	// With one worker, completion order is deterministic: 1..total.
	for i, d := range dones {
		if d != i+1 {
			t.Fatalf("onComplete done counts = %v, want 1..%d", dones, total)
		}
	}
}

// TestRun_peakConcurrency verifies the pool never exceeds `workers` concurrent
// invocations and actually reaches that peak.
func TestRun_peakConcurrency(t *testing.T) {
	t.Parallel()

	const total = 50
	const workers = 6

	var inFlight, peak int64
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	atCapacity := false

	got := Run(context.Background(), names(total), workers, 0,
		func(_ context.Context, name string) res {
			cur := atomic.AddInt64(&inFlight, 1)
			for {
				old := atomic.LoadInt64(&peak)
				if cur <= old || atomic.CompareAndSwapInt64(&peak, old, cur) {
					break
				}
			}
			mu.Lock()
			if cur >= workers {
				atCapacity = true
				cond.Broadcast()
			}
			for !atCapacity {
				cond.Wait()
			}
			mu.Unlock()
			atomic.AddInt64(&inFlight, -1)
			return res{name: name}
		},
		onCancel, nil)

	if len(got) != total {
		t.Fatalf("len(results) = %d, want %d", len(got), total)
	}
	if peak != workers {
		t.Errorf("peak concurrent workers = %d, want exactly %d", peak, workers)
	}
}

// TestRun_workerClamping covers the boundary clamping of the workers argument:
// values < 1 clamp up to 1, and values larger than len(items) clamp down to
// len(items). Clamping is asserted through the achieved peak concurrency.
func TestRun_workerClamping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		workers  int
		count    int
		wantPeak int64
	}{
		{name: "zero clamps to one", workers: 0, count: 10, wantPeak: 1},
		{name: "negative clamps to one", workers: -5, count: 10, wantPeak: 1},
		{name: "large clamps to len", workers: 100, count: 4, wantPeak: 4},
		{name: "equal stays", workers: 4, count: 4, wantPeak: 4},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var inFlight, peak int64
			var mu sync.Mutex
			cond := sync.NewCond(&mu)
			atCapacity := false

			got := Run(context.Background(), names(c.count), c.workers, 0,
				func(_ context.Context, name string) res {
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
					return res{name: name}
				},
				onCancel, nil)

			if len(got) != c.count {
				t.Fatalf("len(results) = %d, want %d", len(got), c.count)
			}
			if peak != c.wantPeak {
				t.Errorf("peak concurrent workers = %d, want %d (effective count after clamping)", peak, c.wantPeak)
			}
		})
	}
}

// TestRun_timeoutIsolation injects a non-zero per-item timeout with a mix of
// fast-success, immediate-failure, and slow (timing-out) items, then asserts one
// item's timeout never takes down the others.
func TestRun_timeoutIsolation(t *testing.T) {
	t.Parallel()

	const timeout = 30 * time.Millisecond
	failErr := errors.New("boom")

	const okStatus = "updated"
	items := []string{"ok-1", "ok-2", "ok-3", itemFail, itemSlow}
	got := Run(context.Background(), items, 4, timeout,
		func(ctx context.Context, name string) res {
			switch name {
			case itemFail:
				return res{name: name, err: failErr}
			case itemSlow:
				<-ctx.Done()
				return res{name: name, err: ctx.Err()}
			default:
				return res{name: name, status: okStatus}
			}
		},
		onCancel, nil)

	byName := make(map[string]res, len(got))
	for _, r := range got {
		byName[r.name] = r
	}
	if len(byName) != len(items) {
		t.Fatalf("got %d distinct results, want %d", len(byName), len(items))
	}
	for _, name := range []string{"ok-1", "ok-2", "ok-3"} {
		if r := byName[name]; r.err != nil || r.status != okStatus {
			t.Errorf("%s = %+v, want a clean success", name, r)
		}
	}
	if r := byName[itemFail]; !errors.Is(r.err, failErr) {
		t.Errorf("fail err = %v, want %v", r.err, failErr)
	}
	if r := byName[itemSlow]; !errors.Is(r.err, context.DeadlineExceeded) {
		t.Errorf("slow err = %v, want context.DeadlineExceeded", r.err)
	}
}

// TestRun_stableInputOrder verifies results are returned in input order even
// when workers finish in the reverse order (#365). Earlier items sleep longer so
// completion order is inverted relative to input order.
func TestRun_stableInputOrder(t *testing.T) {
	t.Parallel()

	const total = 8
	items := names(total)
	sleepByName := make(map[string]time.Duration, total)
	for i, name := range items {
		sleepByName[name] = time.Duration(total-i) * 3 * time.Millisecond
	}

	got := Run(context.Background(), items, total, 0,
		func(_ context.Context, name string) res {
			time.Sleep(sleepByName[name])
			return res{name: name}
		},
		onCancel, nil)

	for i, r := range got {
		if r.name != items[i] {
			t.Errorf("results[%d].name = %q, want %q (input order)", i, r.name, items[i])
		}
	}
}

// TestRun_producerCancellation exercises the producer's ctx.Done branch while it
// is dispatching jobs. With a single worker held busy and a context canceled
// mid-dispatch, the producer must emit onCancel results for the undispatched
// items instead of blocking on the unbuffered jobs channel. Exactly one result
// per item must still be produced.
func TestRun_producerCancellation(t *testing.T) {
	t.Parallel()

	const total = 20
	items := names(total)

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	var got []res
	done := make(chan struct{})
	go func() {
		got = Run(ctx, items, 1, 0,
			func(c context.Context, name string) res {
				select {
				case started <- struct{}{}:
				default:
				}
				<-release // hold the only worker so jobs cannot drain
				return res{name: name, err: c.Err()}
			},
			onCancel, nil)
		close(done)
	}()

	<-started // worker is busy, producer is mid-dispatch
	cancel()  // trigger the producer's ctx.Done branch
	close(release)
	<-done

	if len(got) != total {
		t.Fatalf("len(results) = %d, want exactly %d even under cancellation", len(got), total)
	}
	canceled := 0
	for _, r := range got {
		if errors.Is(r.err, context.Canceled) {
			canceled++
		}
	}
	if canceled == 0 {
		t.Error("expected at least one result carrying context.Canceled from the producer cancellation branch")
	}
}
