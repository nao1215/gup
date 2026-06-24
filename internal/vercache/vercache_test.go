package vercache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/goutil"
)

const (
	testModule  = "example.com/tool"
	testVersion = "v9.9.9"
)

func TestCache_Get_cachesNonContextError(t *testing.T) {
	t.Parallel()

	networkErr := errors.New("network unavailable")
	callCount := 0
	cache := New(func(context.Context, string, goutil.UpdateChannel) (string, error) {
		callCount++
		return "", networkErr
	})

	_, err := cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest)
	if !errors.Is(err, networkErr) {
		t.Fatalf("Get() error = %v, want %v", err, networkErr)
	}

	_, err = cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest)
	if !errors.Is(err, networkErr) {
		t.Fatalf("Get() cached error = %v, want %v", err, networkErr)
	}
	if callCount != 1 {
		t.Fatalf("resolver call count = %d, want 1 for cached error", callCount)
	}
}

func TestCache_Get_doesNotCacheContextError(t *testing.T) {
	t.Parallel()

	callCount := 0
	cache := New(func(ctx context.Context, _ string, _ goutil.UpdateChannel) (string, error) {
		callCount++
		if callCount == 1 {
			return "", context.Canceled
		}
		return testVersion, nil
	})

	if _, err := cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want %v", err, context.Canceled)
	}

	// A context failure must not be cached: the next call retries and succeeds.
	got, err := cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest)
	if err != nil {
		t.Fatalf("Get() retry error = %v, want nil", err)
	}
	if got != testVersion {
		t.Fatalf("Get() retry version = %q, want %q", got, testVersion)
	}
	if callCount != 2 {
		t.Fatalf("resolver call count = %d, want 2 (no caching of context error)", callCount)
	}
}

func TestCache_Get_cachesPerChannel(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	calls := map[goutil.UpdateChannel]int{}
	cache := New(func(_ context.Context, _ string, channel goutil.UpdateChannel) (string, error) {
		mu.Lock()
		calls[channel]++
		mu.Unlock()
		return string(channel), nil
	})

	for range 2 {
		if got, _ := cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest); got != string(goutil.UpdateChannelLatest) {
			t.Fatalf("Get(latest) = %q", got)
		}
		if got, _ := cache.Get(context.Background(), testModule, goutil.UpdateChannelMain); got != string(goutil.UpdateChannelMain) {
			t.Fatalf("Get(main) = %q", got)
		}
	}

	if calls[goutil.UpdateChannelLatest] != 1 || calls[goutil.UpdateChannelMain] != 1 {
		t.Fatalf("resolver calls = %v, want one per channel", calls)
	}
}

func TestCache_Get_waiterContextCanceled(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	cache := New(func(context.Context, string, goutil.UpdateChannel) (string, error) {
		close(started)
		<-release
		return testVersion, nil
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first fetch to start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cache.Get(ctx, testModule, goutil.UpdateChannelLatest); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() waiter error = %v, want %v", err, context.Canceled)
	}

	close(release)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first fetch goroutine to finish")
	}
}

func TestCache_Get_waiterSharesFetchedResult(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	callCount := 0
	cache := New(func(context.Context, string, goutil.UpdateChannel) (string, error) {
		callCount++
		close(started)
		<-release
		return testVersion, nil
	})

	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan string, 2)
	errs := make(chan error, 2)

	go func() {
		defer wg.Done()
		v, err := cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest)
		results <- v
		errs <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first fetch to start")
	}

	go func() {
		defer wg.Done()
		v, err := cache.Get(context.Background(), testModule, goutil.UpdateChannelLatest)
		results <- v
		errs <- err
	}()

	close(release)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Get() waiter error = %v, want nil", err)
		}
	}
	count := 0
	for v := range results {
		count++
		if v != testVersion {
			t.Fatalf("Get() version = %q, want %q", v, testVersion)
		}
	}
	if count != 2 {
		t.Fatalf("result count = %d, want 2", count)
	}
	if callCount != 1 {
		t.Fatalf("resolver call count = %d, want 1", callCount)
	}
}

func TestChannelResolver(t *testing.T) {
	t.Parallel()

	const (
		latestVer = "v1.0.0"
		mainVer   = "v1.1.0-main"
		masterVer = "v1.2.0-master"
	)

	t.Run("latest channel uses getLatest", func(t *testing.T) {
		t.Parallel()
		resolve := ChannelResolver(
			func(context.Context, string) (string, error) { return latestVer, nil },
			func(context.Context, string, string) (string, error) {
				t.Fatal("getByRef should not be called for @latest")
				return "", nil
			},
		)
		got, err := resolve(context.Background(), testModule, goutil.UpdateChannelLatest)
		if err != nil || got != latestVer {
			t.Fatalf("resolve(latest) = %q, %v; want %q, nil", got, err, latestVer)
		}
	})

	t.Run("main channel uses getByRef(main)", func(t *testing.T) {
		t.Parallel()
		resolve := ChannelResolver(
			func(context.Context, string) (string, error) { return latestVer, nil },
			func(_ context.Context, _ string, ref string) (string, error) {
				if ref != string(goutil.UpdateChannelMain) {
					t.Fatalf("getByRef ref = %q, want main", ref)
				}
				return mainVer, nil
			},
		)
		got, err := resolve(context.Background(), testModule, goutil.UpdateChannelMain)
		if err != nil || got != mainVer {
			t.Fatalf("resolve(main) = %q, %v; want %q, nil", got, err, mainVer)
		}
	})

	t.Run("main falls back to master when the main branch is absent", func(t *testing.T) {
		t.Parallel()
		var refs []string
		resolve := ChannelResolver(
			func(context.Context, string) (string, error) { return latestVer, nil },
			func(_ context.Context, _ string, ref string) (string, error) {
				refs = append(refs, ref)
				if ref == string(goutil.UpdateChannelMain) {
					return "", errors.New("go: unknown revision main")
				}
				return masterVer, nil
			},
		)
		got, err := resolve(context.Background(), testModule, goutil.UpdateChannelMain)
		if err != nil || got != masterVer {
			t.Fatalf("resolve(main->master) = %q, %v; want %q, nil", got, err, masterVer)
		}
		want := []string{string(goutil.UpdateChannelMain), string(goutil.UpdateChannelMaster)}
		if len(refs) != 2 || refs[0] != want[0] || refs[1] != want[1] {
			t.Fatalf("refs = %v, want %v", refs, want)
		}
	})

	t.Run("main surfaces a non-branch error without trying master", func(t *testing.T) {
		t.Parallel()
		buildErr := errors.New("build failed: compile error")
		calls := 0
		resolve := ChannelResolver(
			func(context.Context, string) (string, error) { return latestVer, nil },
			func(_ context.Context, _ string, ref string) (string, error) {
				calls++
				if ref == string(goutil.UpdateChannelMain) {
					return "", buildErr
				}
				t.Fatal("getByRef(master) must not be called for a non-branch error")
				return "", nil
			},
		)
		_, err := resolve(context.Background(), testModule, goutil.UpdateChannelMain)
		if !errors.Is(err, buildErr) {
			t.Fatalf("resolve(main) error = %v, want %v", err, buildErr)
		}
		if calls != 1 {
			t.Fatalf("getByRef calls = %d, want 1 (no master fallback)", calls)
		}
	})

	t.Run("canceled context on main does not fall back to master", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		resolve := ChannelResolver(
			func(context.Context, string) (string, error) { return latestVer, nil },
			func(_ context.Context, _ string, ref string) (string, error) {
				calls++
				if ref == string(goutil.UpdateChannelMaster) {
					t.Fatal("getByRef(master) must not be called when the context is canceled")
				}
				return "", errors.New("go: unknown revision main")
			},
		)
		if _, err := resolve(ctx, testModule, goutil.UpdateChannelMain); err == nil {
			t.Fatal("resolve(main) error = nil, want the main error")
		}
		if calls != 1 {
			t.Fatalf("getByRef calls = %d, want 1 (no master fallback)", calls)
		}
	})

	t.Run("master channel uses getByRef(master)", func(t *testing.T) {
		t.Parallel()
		resolve := ChannelResolver(
			func(context.Context, string) (string, error) { return latestVer, nil },
			func(_ context.Context, _ string, ref string) (string, error) {
				if ref != string(goutil.UpdateChannelMaster) {
					t.Fatalf("getByRef ref = %q, want master", ref)
				}
				return masterVer, nil
			},
		)
		got, err := resolve(context.Background(), testModule, goutil.UpdateChannelMaster)
		if err != nil || got != masterVer {
			t.Fatalf("resolve(master) = %q, %v; want %q, nil", got, err, masterVer)
		}
	})
}
