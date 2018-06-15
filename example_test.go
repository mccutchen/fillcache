package fillcache_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mccutchen/fillcache"
)

// ExampleUsage demonstrates basic usage of fillcache by simulating a
// "thundering herd" of 10 concurrent cache gets that result in only a single
// call to the cache filling function.
func ExampleUsage() {
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
