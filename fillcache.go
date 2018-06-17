// Package fillcache is an in-process cache with single-flight filling
// semantics.
//
// In short: Given a function that computes the value to be cached for a key,
// it will ensure that the function is called only once per key no matter how
// many concurrent cache gets are issued for a key.
package fillcache

import (
	"context"
	"sync"
	"time"
)

// FillCache is a cache whose entries are calculated and filled on-demand
type FillCache struct {
	// an optional TTL for cache entries; if unset, cache entries never
	// expire
	ttl time.Duration

	fillFunc FillFunc

	cache    map[string]*cacheEntry
	inflight map[string]*waiter
	mu       sync.Mutex
}

// New creates a FillCache whose entries will be computed by the given FillFunc
func New(fillFunc FillFunc, opts ...Option) *FillCache {
	c := &FillCache{
		fillFunc: fillFunc,
		cache:    make(map[string]*cacheEntry),
		inflight: make(map[string]*waiter),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// FillFunc is a function that computes the value of a cache entry given a
// string key
type FillFunc func(context.Context, string) (interface{}, error)

// Get returns the cache value for the given key, computing it as necessary
func (c *FillCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	entry, found := c.cache[key]
	c.mu.Unlock()
	if found && !entry.expired() {
		return entry.val, nil
	}
	return c.Update(ctx, key)
}

// Update recomputes, stores, and returns the value for the given key. If an
// error occurs, the cache is not updated.
func (c *FillCache) Update(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()

	// Another goroutine is updating this entry, just wait for it to finish
	if w, waiting := c.inflight[key]; waiting {
		c.mu.Unlock()
		return w.wait(ctx)
	}

	// Otherwise, we'll update this entry ourselves
	w := &waiter{}
	w.wg.Add(1)
	c.inflight[key] = w
	c.mu.Unlock()

	val, err := c.fillFunc(ctx, key)

	c.mu.Lock()
	defer c.mu.Unlock()

	w.finish(val, err)
	delete(c.inflight, key)

	if err == nil {
		c.cache[key] = newCacheEntry(val, c.ttl)
	}
	return val, err
}

// Option can be used to configure a FillCache instance
type Option func(c *FillCache)

// TTL sets the TTL for all cache entries
func TTL(ttl time.Duration) Option {
	return func(c *FillCache) {
		c.ttl = ttl
	}
}

func newCacheEntry(val interface{}, ttl time.Duration) *cacheEntry {
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	return &cacheEntry{
		val:       val,
		expiresAt: expiresAt,
	}
}

type cacheEntry struct {
	val       interface{}
	expiresAt time.Time
}

func (e *cacheEntry) expired() bool {
	return !e.expiresAt.IsZero() && e.expiresAt.Before(time.Now())
}

// waiter represents an outstanding computation that will fill a cache entry
type waiter struct {
	val interface{}
	err error

	wg sync.WaitGroup
}

func (w *waiter) wait(ctx context.Context) (interface{}, error) {
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return w.val, w.err
	}
}

func (w *waiter) finish(val interface{}, err error) {
	w.val = val
	w.err = err
	w.wg.Done()
}
