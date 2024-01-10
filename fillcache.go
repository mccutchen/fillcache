// Package fillcache is an in-process cache with single-flight filling
// semantics.
//
// In short: Given a function that computes the value a cache key, it will
// ensure that the function is called only once per key no matter how many
// concurrent cache gets are issued for a key.
package fillcache

import (
	"context"
	"sync"
	"time"
)

// Config configures a Cache.
type Config struct {
	TTL        time.Duration
	ServeStale bool
}

// Cache is a cache whose entries are calculated and filled on-demand.
type Cache[T any] struct {
	// a function that knows how to compute the value for a cache key
	filler Filler[T]

	// an optional TTL for cache entries; if unset, cache entries never
	// expire
	ttl time.Duration

	// if a cache value has expired, should the stale value be used if an error
	// is encountered during update?
	serveStale bool

	cache    map[string]*cacheEntry[T]
	inflight map[string]*fillRequest[T]
	mu       sync.RWMutex
}

// New creates a Cache whose entries will be computed by the given Filler.
func New[T any](filler Filler[T], cfg *Config) *Cache[T] {
	c := &Cache[T]{
		filler:   filler,
		cache:    make(map[string]*cacheEntry[T]),
		inflight: make(map[string]*fillRequest[T]),
	}
	if cfg != nil {
		c.ttl = cfg.TTL
		c.serveStale = cfg.ServeStale
	}
	return c
}

// Filler is a function that computes the value to cache for a given key.
type Filler[T any] func(ctx context.Context, key string) (val T, err error)

// Get returns the cache value for the given key, computing it as necessary.
func (c *Cache[T]) Get(ctx context.Context, key string) (T, error) {
	c.mu.Lock()
	entry, found := c.cache[key]
	c.mu.Unlock()
	if found && !entry.expired() {
		return entry.val, nil
	}
	val, err := c.Update(ctx, key)
	if err != nil {
		if c.serveStale && found {
			// TODO: should this return something like ErrStaleResults so that
			// consumers know an error occurred?
			return entry.val, nil
		}
		var zero T
		return zero, err
	}
	return val, err
}

// Update recomputes, stores, and returns the value for the given key. If an
// error occurs, the cache is not updated.
//
// Update can be used to proactively update cache entries without waiting for a
// Get.
func (c *Cache[T]) Update(ctx context.Context, key string) (T, error) {
	c.mu.Lock()

	// Another goroutine is updating this entry, just wait for it to finish
	if w, waiting := c.inflight[key]; waiting {
		c.mu.Unlock()
		return w.wait(ctx)
	}

	// Otherwise, we'll update this entry ourselves
	r := &fillRequest[T]{}
	r.wg.Add(1)
	c.inflight[key] = r
	c.mu.Unlock()

	val, err := c.filler(ctx, key)

	c.mu.Lock()
	defer c.mu.Unlock()

	r.finish(val, err)
	delete(c.inflight, key)

	if err == nil {
		c.cache[key] = newCacheEntry(val, c.ttl)
	}
	return val, err
}

// Size returns the number of entries in the cache
func (c *Cache[T]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// cacheEntry captures a cached value and its optional expiration time
type cacheEntry[T any] struct {
	val       T
	expiresAt time.Time
}

func newCacheEntry[T any](val T, ttl time.Duration) *cacheEntry[T] {
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	return &cacheEntry[T]{
		val:       val,
		expiresAt: expiresAt,
	}
}

func (e *cacheEntry[T]) expired() bool {
	return !e.expiresAt.IsZero() && e.expiresAt.Before(time.Now())
}

// fillRequest represents an outstanding computation of the value for a cache
// key
type fillRequest[T any] struct {
	val T
	err error

	wg sync.WaitGroup
}

func (r *fillRequest[T]) wait(ctx context.Context) (T, error) {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case <-done:
		return r.val, r.err
	}
}

func (r *fillRequest[T]) finish(val T, err error) {
	r.val = val
	r.err = err
	r.wg.Done()
}
