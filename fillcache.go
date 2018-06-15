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
)

// FillCache is a cache whose entries are calculated and filled on-demand
type FillCache struct {
	fillFunc FillFunc

	cache    map[string]interface{}
	inflight map[string]*waiter

	mu sync.Mutex
}

// New creates a FillCache whose entries will be computed by the given FillFunc
func New(fillFunc FillFunc) *FillCache {
	return &FillCache{
		fillFunc: fillFunc,
		cache:    make(map[string]interface{}),
		inflight: make(map[string]*waiter),
	}
}

// FillFunc is a function that computes the value of a cache entry given a
// string key
type FillFunc func(context.Context, string) (interface{}, error)

// Get returns the cache value for the given key, computing it as necessary
func (c *FillCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	val, found := c.cache[key]
	c.mu.Unlock()
	if found {
		return val, nil
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
		c.cache[key] = val
	}
	return val, err
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
