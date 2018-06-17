package fillcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type result struct {
	val int
	err error
}

type resultMap map[string]result

type testFiller struct {
	results    resultMap
	delay      time.Duration
	callCounts map[string]int
	mu         sync.Mutex
}

func newTestFiller(results resultMap, delay time.Duration) *testFiller {
	return &testFiller{
		results:    results,
		delay:      delay,
		callCounts: make(map[string]int),
	}
}

func newSimpleFiller(key string, val int, delay time.Duration) *testFiller {
	results := map[string]result{
		key: {
			val: val,
			err: nil,
		},
	}
	return newTestFiller(results, delay)
}

func (t *testFiller) callCount(key string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.callCounts[key]
}

func (t *testFiller) fillFunc(ctx context.Context, key string) (interface{}, error) {
	t.mu.Lock()
	t.callCounts[key]++
	t.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(t.delay):
		result, found := t.results[key]
		if !found {
			return result, fmt.Errorf("unexpected key %q", key)
		}
		return result.val, result.err
	}
}

func TestGetFillsCache(t *testing.T) {
	f := newSimpleFiller("foo", 1, 10*time.Millisecond)
	c := New(f.fillFunc)
	ctx := context.Background()

	// first get should compute the expected result
	result, err := c.Get(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	val := result.(int)
	if val != 1 {
		t.Errorf("expected val = %d, got %d", 1, val)
	}
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}

	// second get of the same key should return the same value, and should NOT
	// recompute the result
	result2, err := c.Get(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	val2 := result2.(int)
	if val2 != val {
		t.Errorf("expected val2 = %d, got %d", val, val2)
	}
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}
}

func TestParallelGetFillsCacheOnce(t *testing.T) {
	f := newSimpleFiller("foo", 1, 200*time.Millisecond)
	c := New(f.fillFunc)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := c.Get(context.Background(), "foo")
			if err != nil {
				t.Errorf("goroutine %d got unexpected error: %s", i, err)
			}
			val := result.(int)
			if val != 1 {
				t.Errorf("goroutine %d expected val = %d, got %d", i, 1, val)
			}
		}(i)
	}

	wg.Wait()
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}
}

func TestGetRespectsContexts(t *testing.T) {
	f := newSimpleFiller("foo", 1, 50*time.Millisecond)
	c := New(f.fillFunc)

	baseCtx := context.Background()

	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Millisecond)
	_, err := c.Get(ctx, "foo")
	cancel()
	if err == nil {
		t.Errorf("expected context cancelation")
	}
	if len(c.cache) > 0 {
		t.Errorf("expected empty cache after timeout, got %#v", c.cache)
	}

	// now give it time to fill the cache
	result, err := c.Get(baseCtx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	val := result.(int)

	// once cache is filled, the short timeout should succeed instantly
	ctx, cancel = context.WithTimeout(baseCtx, 5*time.Millisecond)
	result2, err := c.Get(ctx, "foo")
	cancel()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	val2 := result2.(int)

	if val2 != val {
		t.Errorf("expected val2 = %d, got %d", val, val2)
	}
}

func TestGetDoesNotCacheOnError(t *testing.T) {
	results := map[string]result{
		"foo": {err: errors.New("error")},
	}
	f := newTestFiller(results, 10*time.Millisecond)
	c := New(f.fillFunc)

	_, err := c.Get(context.Background(), "foo")
	if err == nil || err.Error() != "error" {
		t.Errorf("expected error, got: %s", err)
	}
	if len(c.cache) != 0 {
		t.Errorf("expected empty cache after error, got %#v", c.cache)
	}
}

func TestGetPropagatesErrorsToAllCallers(t *testing.T) {
	results := map[string]result{
		"foo": {err: errors.New("error")},
	}
	f := newTestFiller(results, 100*time.Millisecond)
	c := New(f.fillFunc)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Get(context.Background(), "foo")
			if err == nil || err.Error() != "error" {
				t.Errorf("goroutine %d expected error, got: %s", i, err)
			}
		}(i)
	}

	wg.Wait()
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}
}

func TestUpdateRespectsContexts(t *testing.T) {
	f := newSimpleFiller("foo", 1, 50*time.Millisecond)
	c := New(f.fillFunc)

	baseCtx := context.Background()

	ctx, cancel := context.WithTimeout(baseCtx, 5*time.Millisecond)
	_, err := c.Update(ctx, "foo")
	cancel()
	if err == nil {
		t.Errorf("expected context cancelation")
	}
	if len(c.cache) > 0 {
		t.Errorf("expected empty cache after timeout, got %#v", c.cache)
	}

	// now give it time to fill the cache
	_, err = c.Update(baseCtx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(c.cache) != 1 {
		t.Errorf("expected cache to be updated")
	}
}

func TestUpdateDoesNotCacheOnError(t *testing.T) {
	results := map[string]result{
		"foo": {err: errors.New("error")},
	}
	f := newTestFiller(results, 10*time.Millisecond)
	c := New(f.fillFunc)

	_, err := c.Update(context.Background(), "foo")
	if err == nil || err.Error() != "error" {
		t.Errorf("expected error, got: %s", err)
	}
	if len(c.cache) != 0 {
		t.Errorf("expected empty cache after error, got %#v", c.cache)
	}
}

func TestParallelUpdateFillsCacheOnce(t *testing.T) {
	f := newSimpleFiller("foo", 1, 200*time.Millisecond)
	c := New(f.fillFunc)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.Update(context.Background(), "foo")
			if err != nil {
				t.Errorf("goroutine %d got unexpected error: %s", i, err)
			}
		}(i)
	}

	wg.Wait()
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}
}

func TestParallelGetsAndUpdatesFillCacheOnce(t *testing.T) {
	f := newSimpleFiller("foo", 1, 200*time.Millisecond)
	c := New(f.fillFunc)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, err := c.Update(context.Background(), "foo")
				if err != nil {
					t.Errorf("goroutine %d got unexpected error: %s", i, err)
				}
			} else {
				result, err := c.Get(context.Background(), "foo")
				if err != nil {
					t.Errorf("goroutine %d got unexpected error: %s", i, err)
				}
				val := result.(int)
				if val != 1 {
					t.Errorf("goroutine %d expected val = %d, got %d", i, 1, val)
				}
			}
		}(i)
	}

	wg.Wait()
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}
}

func TestWaiterRespectsContexts(t *testing.T) {
	// A test to ensure that a waiter will respect the timeout in the context
	// it is given.
	//
	// Given a fill func that will take 200ms to return, spin up two goroutines
	// to get the same cache key concurrently. The first goroutine applies no
	// timeout, so it should succeed in filling the cache and getting a result.
	// The second applies a 5ms timeout, so it should fail while waiting for
	// the cache fill to complete.
	//
	// The cache fill should succeed, because there was no timeout applied to
	// the context that triggered the fill.
	f := newSimpleFiller("foo", 1, 200*time.Millisecond)
	c := New(f.fillFunc)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := c.Get(context.Background(), "foo")
		if err != nil {
			t.Errorf("goroutine 1 got unexpected error: %s", err)
		}
	}()

	// wait to ensure that the first goroutine starts before the second
	<-time.After(1 * time.Millisecond)

	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()
		_, err := c.Get(ctx, "foo")
		if err == nil {
			t.Errorf("goroutine 2 expected error")
		}
	}()

	wg.Wait()
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}

	// ensure that the cache has been filled correctly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	result, err := c.Get(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if val := result.(int); val != 1 {
		t.Errorf("expected result = 1, got %v", val)
	}
}

func TestFillSuccessDeterminedByFirstContext(t *testing.T) {
	// A test that captures a potentially confusing corner case: A cache fill
	// respects the timeout (or not) of the context that triggered the fill.
	//
	// This test is basically the opposite of TestWaiterRespectsContexts:
	//
	// Given a fill func that will take 200ms to return, spin up two goroutines
	// to get the same cache key concurrently. The first applies a short
	// timeout, so the cache fill will fail. The second applies no timeout, but
	// will be waiting on the same cache fill and so will also fail.
	//
	// Afterwards, the cache will still be empty.
	f := newSimpleFiller("foo", 1, 200*time.Millisecond)
	c := New(f.fillFunc)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := c.Get(ctx, "foo")
		if err == nil {
			t.Errorf("goroutine 1 expected error")
		}
	}()

	// wait to ensure that the first goroutine starts before the second
	<-time.After(1 * time.Millisecond)

	go func() {
		defer wg.Done()
		_, err := c.Get(context.Background(), "foo")
		if err == nil {
			t.Errorf("goroutine 2 expected error")
		}
	}()

	wg.Wait()
	if count := f.callCount("foo"); count != 1 {
		t.Errorf("expected %d call to fill func, got %d", 1, count)
	}

	if len(c.cache) != 0 {
		t.Errorf("expected empty cache after fill failures, got cache size %d", len(c.cache))
	}
}

func TestExpiration(t *testing.T) {
	key := "foo"
	val := 1
	ttl := 25 * time.Millisecond
	f := newSimpleFiller(key, val, time.Duration(0))
	c := New(f.fillFunc, TTL(ttl))

	for i := 0; i < 15; i++ {
		result, err := c.Get(context.Background(), key)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got := result.(int); got != val {
			t.Fatalf("expected val %d, got %d", val, got)
		}
		<-time.After(2 * time.Millisecond)
	}

	if count := f.callCount(key); count != 2 {
		t.Errorf("expected 2 calls to fill func, got %d", count)
	}
}
