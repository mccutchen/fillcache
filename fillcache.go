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

// NewFillCache creates a FillCache whose entries will be computed by the given
// FillFunc
func NewFillCache(fillFunc FillFunc) *FillCache {
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
	if val, found := c.cache[key]; found {
		c.mu.Unlock()
		return val, nil
	}

	if w, waiting := c.inflight[key]; waiting {
		c.mu.Unlock()
		return w.wait(ctx)
	}

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

// Update recomputes the value for the given key, unless another goroutine is
// already computing the value, and returns a bool indicating whether the value
// was updated
func (c *FillCache) Update(ctx context.Context, key string) (bool, error) {
	c.mu.Lock()

	// Another goroutine is updating this entry, so we don't need to
	if _, waiting := c.inflight[key]; waiting {
		c.mu.Unlock()
		return false, nil
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

	if w.err == nil {
		c.cache[key] = w.val
		return true, nil
	}
	return false, w.err
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
