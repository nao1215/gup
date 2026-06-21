//nolint:paralleltest // tests mutate global function variables for stubbing
package cmd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nao1215/gup/internal/goutil"
)

func Test_latestVerCache_get_nonContextErrorCached(t *testing.T) {
	origGetLatestVerCtx := getLatestVerCtx
	defer func() { getLatestVerCtx = origGetLatestVerCtx }()

	networkErr := errors.New("network unavailable")
	callCount := 0
	getLatestVerCtx = func(context.Context, string) (string, error) {
		callCount++
		return "", networkErr
	}

	cache := newLatestVerCache()
	_, err := cache.getByChannel(context.Background(), "example.com/tool", goutil.UpdateChannelLatest)
	if !errors.Is(err, networkErr) {
		t.Fatalf("latestVerCache.get() error = %v, want %v", err, networkErr)
	}

	_, err = cache.getByChannel(context.Background(), "example.com/tool", goutil.UpdateChannelLatest)
	if !errors.Is(err, networkErr) {
		t.Fatalf("latestVerCache.get() cached error = %v, want %v", err, networkErr)
	}
	if callCount != 1 {
		t.Fatalf("getLatestVerCtx call count = %d, want 1 for cached error", callCount)
	}
}

func Test_latestVerCache_get_waiterContextCanceled(t *testing.T) {
	origGetLatestVerCtx := getLatestVerCtx
	defer func() { getLatestVerCtx = origGetLatestVerCtx }()

	started := make(chan struct{})
	release := make(chan struct{})
	getLatestVerCtx = func(context.Context, string) (string, error) {
		close(started)
		<-release
		return testVersionNine, nil
	}

	cache := newLatestVerCache()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = cache.getByChannel(context.Background(), "example.com/tool", goutil.UpdateChannelLatest)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first fetch to start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.getByChannel(ctx, "example.com/tool", goutil.UpdateChannelLatest)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("latestVerCache.get() error = %v, want %v", err, context.Canceled)
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

func Test_latestVerCache_get_waiterSharesFetchedResult(t *testing.T) {
	origGetLatestVerCtx := getLatestVerCtx
	defer func() { getLatestVerCtx = origGetLatestVerCtx }()

	started := make(chan struct{})
	release := make(chan struct{})
	callCount := 0
	getLatestVerCtx = func(context.Context, string) (string, error) {
		callCount++
		close(started)
		<-release
		return testVersionNine, nil
	}

	cache := newLatestVerCache()
	var wg sync.WaitGroup
	wg.Add(2)

	results := make(chan string, 2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		v, err := cache.getByChannel(context.Background(), "example.com/tool", goutil.UpdateChannelLatest)
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
		v, err := cache.getByChannel(context.Background(), "example.com/tool", goutil.UpdateChannelLatest)
		results <- v
		errs <- err
	}()

	close(release)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("latestVerCache.get() waiter error = %v, want nil", err)
		}
	}
	gotCount := 0
	for v := range results {
		gotCount++
		if v != testVersionNine {
			t.Fatalf("latestVerCache.get() version = %q, want %q", v, testVersionNine)
		}
	}
	if gotCount != 2 {
		t.Fatalf("result count = %d, want 2", gotCount)
	}
	if callCount != 1 {
		t.Fatalf("getLatestVerCtx call count = %d, want 1", callCount)
	}
}
