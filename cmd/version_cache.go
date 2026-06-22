package cmd

import (
	"context"
	"errors"
	"sync"

	"github.com/nao1215/gup/internal/goutil"
)

// latestVerCache deduplicates concurrent getLatestVer calls for the same module path.
// When multiple goroutines request the latest version of the same module,
// only one network call is made; others wait and share the result.
type latestVerCache struct {
	mu      sync.Mutex
	entries map[string]*latestVerEntry
}

type latestVerEntry struct {
	mu       sync.Mutex
	fetching bool
	waitCh   chan struct{}
	done     bool
	version  string
	err      error
}

func newLatestVerCache() *latestVerCache {
	return &latestVerCache{entries: make(map[string]*latestVerEntry)}
}

// getByChannel returns the resolved version for the given module path on the
// requested update channel. Results are cached per (module path, channel) pair
// so that, for example, a package tracked on @main is not confused with the
// same module queried on @latest.
func (c *latestVerCache) getByChannel(ctx context.Context, modulePath string, channel goutil.UpdateChannel) (string, error) {
	channel = goutil.NormalizeUpdateChannel(string(channel))
	key := modulePath + "@" + string(channel)

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		entry = &latestVerEntry{}
		c.entries[key] = entry
	}
	c.mu.Unlock()

	for {
		entry.mu.Lock()
		if entry.done {
			version, err := entry.version, entry.err
			entry.mu.Unlock()
			return version, err
		}

		if entry.fetching {
			waitCh := entry.waitCh
			entry.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-waitCh:
				continue
			}
		}

		entry.fetching = true
		entry.waitCh = make(chan struct{})
		entry.mu.Unlock()

		version, err := fetchVerForChannel(ctx, modulePath, channel)

		entry.mu.Lock()
		entry.fetching = false
		waitCh := entry.waitCh
		entry.waitCh = nil

		// Do not cache context-related failures; allow a fresh retry.
		switch {
		case err == nil:
			entry.version = version
			entry.err = nil
			entry.done = true
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			entry.version = ""
			entry.err = nil
			entry.done = false
		default:
			entry.version = ""
			entry.err = err
			entry.done = true
		}
		entry.mu.Unlock()
		close(waitCh)

		return version, err
	}
}

// fetchVerForChannel resolves the version that the given update channel would
// install, mirroring the install-time policy used by 'gup update':
//   - latest: "$ go list -m ... <module>@latest"
//   - main:   "$ go list -m ... <module>@main", falling back to @master
//     (the same @main-with-@master-fallback policy update applies on install)
//   - master: "$ go list -m ... <module>@master"
func fetchVerForChannel(ctx context.Context, modulePath string, channel goutil.UpdateChannel) (string, error) {
	switch goutil.NormalizeUpdateChannel(string(channel)) {
	case goutil.UpdateChannelMain:
		ver, err := getVerByRefCtx(ctx, modulePath, string(goutil.UpdateChannelMain))
		if err == nil {
			return ver, nil
		}
		// Do not fall back when the failure is a canceled/expired context;
		// the same cancellation would just hit @master too.
		if ctx != nil && ctx.Err() != nil {
			return "", err
		}
		// Fall back to @master only when @main fails because the main branch is
		// absent. Build/network/auth/other failures surface as-is so a
		// wrong-branch version is never silently resolved (#340).
		if !goutil.IsBranchNotFound(err, string(goutil.UpdateChannelMain)) {
			return "", err
		}
		return getVerByRefCtx(ctx, modulePath, string(goutil.UpdateChannelMaster))
	case goutil.UpdateChannelMaster:
		return getVerByRefCtx(ctx, modulePath, string(goutil.UpdateChannelMaster))
	case goutil.UpdateChannelLatest:
		return getLatestVerCtx(ctx, modulePath)
	default:
		return getLatestVerCtx(ctx, modulePath)
	}
}
