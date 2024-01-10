package fillcache_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mccutchen/fillcache"
)

// Example demonstrates basic usage of fillcache by simulating a "thundering
// herd" of 10 concurrent cache gets that result in only a single call to the
// cache filling function.
func Example() {
	var (
		key       = "foo"
		val       = 42
		fillDelay = 250 * time.Millisecond
		wg        sync.WaitGroup
	)

	// A simple cache filling function that just waits a constant amount of
	// time before returning a value
	filler := func(ctx context.Context, key string) (int, error) {
		<-time.After(fillDelay)
		fmt.Printf("computed value for key %q: %d\n", key, val)
		return val, nil
	}

	// Create our cache with the simple cache filling function above.
	cache := fillcache.New(filler, nil)

	// Launch a thundering herd of 10 concurrent cache gets
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, _ := cache.Get(context.Background(), key)
			fmt.Printf("got value for key %q: %d\n", key, result)
		}(i)
	}

	wg.Wait()

	// Output:
	// computed value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
}

// ExampleNew shows how to customize a cache instance with configuration
// options.
func ExampleConfig() {
	var (
		key       = "foo"
		val       = 42
		fillDelay = 250 * time.Millisecond
		calls     = &atomic.Int64{}
	)

	// A fallible cache filling function that waits a constant amount of time
	// before returning a value or an error.
	filler := func(ctx context.Context, key string) (int, error) {
		<-time.After(fillDelay)
		if calls.Add(1)%2 == 0 {
			fmt.Printf("error filling cache for key %q, should serve stale\n", key)
			return 0, fmt.Errorf("error")
		}
		fmt.Printf("computed value for key %q: %d\n", key, val)
		return val, nil
	}

	// Create a cache configured with a 50ms TTL and configured to serve stale
	// values in the face of a fill error.
	cache := fillcache.New(filler, &fillcache.Config{
		// Cache entries will be re-filled after 50ms
		TTL: 50 * time.Millisecond,
		// If an error occurs during re-fill but we have a previously cached
		// value, it will be returned instead of the error.
		ServeStale: true,
	})

	// Fetch the key every 20ms for 100ms to ensure that the value is recomputed
	// after the 50ms TTL
	for i := 0; i < 5; i++ {
		result, _ := cache.Get(context.Background(), key)
		fmt.Printf("got value for key %q: %d\n", key, result)
		<-time.After(20 * time.Millisecond)
	}

	// Output:
	// computed value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// error filling cache for key "foo", should serve stale
	// got value for key "foo": 42
	// computed value for key "foo": 42
	// got value for key "foo": 42
}
