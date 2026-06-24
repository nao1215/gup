// Package vercache resolves and caches the version that an update channel would
// install for a module path. It deduplicates concurrent lookups so that, when
// many packages are updated in parallel, the same module is queried only once
// per (module path, channel) pair.
//
// The version lookup itself is injected as a Resolver, keeping this package free
// of any dependency on the go-toolchain wrappers (and letting tests drive it
// without touching globals). ChannelResolver builds the Resolver that implements
// gup's install-time channel policy from the underlying version lookups.
package vercache

import (
	"context"
	"errors"
	"sync"

	"github.com/nao1215/gup/internal/goutil"
)

// errPinnedNotResolvable is returned if a pinned channel ever reaches the
// version resolver: pinned packages install an exact recorded version and are
// handled before the cache, so reaching here is a programming error, never a
// reason to fall back to @latest.
var errPinnedNotResolvable = errors.New("pinned channel has no resolvable version; install the recorded version directly")

// Resolver looks up the version that the given update channel would install for
// modulePath. It mirrors the install-time policy of 'gup update'.
type Resolver func(ctx context.Context, modulePath string, channel goutil.UpdateChannel) (string, error)

// Cache deduplicates concurrent Resolver calls per (module path, channel).
// When multiple goroutines request the same key, only one lookup runs; the
// others wait and share the result.
type Cache struct {
	resolve Resolver
	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	mu       sync.Mutex
	fetching bool
	waitCh   chan struct{}
	done     bool
	version  string
	err      error
}

// New returns a Cache backed by resolve.
func New(resolve Resolver) *Cache {
	return &Cache{resolve: resolve, entries: make(map[string]*entry)}
}

// Get returns the resolved version for modulePath on the requested update
// channel. Results are cached per (module path, channel) pair so that, for
// example, a package tracked on @main is not confused with the same module
// queried on @latest. Context failures are not cached, so a later call retries.
func (c *Cache) Get(ctx context.Context, modulePath string, channel goutil.UpdateChannel) (string, error) {
	channel = goutil.NormalizeUpdateChannel(string(channel))
	key := modulePath + "@" + string(channel)

	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		e = &entry{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	for {
		e.mu.Lock()
		if e.done {
			version, err := e.version, e.err
			e.mu.Unlock()
			return version, err
		}

		if e.fetching {
			waitCh := e.waitCh
			e.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-waitCh:
				continue
			}
		}

		e.fetching = true
		e.waitCh = make(chan struct{})
		e.mu.Unlock()

		version, err := c.resolve(ctx, modulePath, channel)

		e.mu.Lock()
		e.fetching = false
		waitCh := e.waitCh
		e.waitCh = nil

		// Do not cache context-related failures; allow a fresh retry.
		switch {
		case err == nil:
			e.version = version
			e.err = nil
			e.done = true
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			e.version = ""
			e.err = nil
			e.done = false
		default:
			e.version = ""
			e.err = err
			e.done = true
		}
		e.mu.Unlock()
		close(waitCh)

		return version, err
	}
}

// GetLatestFunc resolves a module's version on the @latest channel.
type GetLatestFunc func(ctx context.Context, modulePath string) (string, error)

// GetByRefFunc resolves a module's version at an explicit ref (e.g. "main",
// "master").
type GetByRefFunc func(ctx context.Context, modulePath, ref string) (string, error)

// ChannelResolver builds a Resolver implementing gup's install-time channel
// policy from the underlying version lookups:
//   - latest: getLatest(module)
//   - main:   getByRef(module, "main"), falling back to getByRef(module,
//     "master") only when @main fails because the main branch is absent
//     (the same @main-with-@master-fallback policy update applies on install)
//   - master: getByRef(module, "master")
//
// A build/network/auth/other @main failure surfaces as-is so a wrong-branch
// version is never silently resolved (#340), and a canceled/expired context is
// never retried on @master.
func ChannelResolver(getLatest GetLatestFunc, getByRef GetByRefFunc) Resolver {
	return func(ctx context.Context, modulePath string, channel goutil.UpdateChannel) (string, error) {
		switch goutil.NormalizeUpdateChannel(string(channel)) {
		case goutil.UpdateChannelMain:
			ver, err := getByRef(ctx, modulePath, string(goutil.UpdateChannelMain))
			if err == nil {
				return ver, nil
			}
			// Do not fall back when the failure is a canceled/expired context;
			// the same cancellation would just hit @master too.
			if ctx != nil && ctx.Err() != nil {
				return "", err
			}
			// Fall back to @master only when @main fails because the main branch
			// is absent. Other failures surface as-is (#340).
			if !goutil.IsBranchNotFound(err, string(goutil.UpdateChannelMain)) {
				return "", err
			}
			return getByRef(ctx, modulePath, string(goutil.UpdateChannelMaster))
		case goutil.UpdateChannelMaster:
			return getByRef(ctx, modulePath, string(goutil.UpdateChannelMaster))
		case goutil.UpdateChannelPinned:
			// A pinned package is installed at its exact recorded version and must
			// never be resolved against the proxy; callers handle it before reaching
			// the cache. Surface a clear error instead of silently resolving @latest.
			return "", errPinnedNotResolvable
		case goutil.UpdateChannelLatest:
			return getLatest(ctx, modulePath)
		default:
			return getLatest(ctx, modulePath)
		}
	}
}
