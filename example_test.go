package fillcache_test

import (
	"context"
	"fmt"
	"sync"
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

	// Create our cache with a simple cache filling function that just waits a
	// constant amount of time before returning a value
	cache := fillcache.New(func(ctx context.Context, key string) (interface{}, error) {
		<-time.After(fillDelay)
		fmt.Printf("computed value for key %q: %d\n", key, val)
		return val, nil
	})

	// Launch a thundering herd of 10 concurrent cache gets
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, _ := cache.Get(context.Background(), key)
			fmt.Printf("got value for key %q: %d\n", key, result.(int))
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

// ExampleTTL shows how to set a TTL for cache entries
func ExampleTTL() {
	var (
		key       = "foo"
		val       = 42
		fillDelay = 250 * time.Millisecond
	)

	// A simple cache filling function that just waits a constant amount of
	// time before returning a value
	filler := func(ctx context.Context, key string) (interface{}, error) {
		<-time.After(fillDelay)
		fmt.Printf("computed value for key %q: %d\n", key, val)
		return val, nil
	}

	// Create our cache with a 50ms TTL
	cache := fillcache.New(filler, fillcache.TTL(50*time.Millisecond))

	// Fetch the key every 20ms for 100ms to ensure that the value is recomputed
	// after the 50ms TTL
	for i := 0; i < 5; i++ {
		result, _ := cache.Get(context.Background(), key)
		fmt.Printf("got value for key %q: %d\n", key, result.(int))
		<-time.After(20 * time.Millisecond)
	}

	// Output:
	// computed value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
	// computed value for key "foo": 42
	// got value for key "foo": 42
	// got value for key "foo": 42
}
