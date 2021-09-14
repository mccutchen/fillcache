# fillcache

[![Documentation](https://pkg.go.dev/badge/github.com/mccutchen/fillcache)](https://pkg.go.dev/github.com/mccutchen/fillcache)
[![Build status](https://github.com/mccutchen/fillcache/actions/workflows/test.yaml/badge.svg)](https://github.com/mccutchen/fillcache/actions/workflows/test.yaml)
[![Code coverage](https://codecov.io/gh/mccutchen/fillcache/branch/main/graph/badge.svg)](https://codecov.io/gh/mccutchen/fillcache)
[![Go report card](http://goreportcard.com/badge/github.com/mccutchen/fillcache)](https://goreportcard.com/report/github.com/mccutchen/fillcache)

An in-process cache with single-flight filling semantics.

In short: Given a function that computes the value to be cached for a key, it
will ensure that the function is called only once per key no matter how many
concurrent cache gets are issued for a key.

This might be useful if, say, you find yourself reaching for the
[`singleflight` package][singleflight] _and_ you want to cache the resulting
values in memory.


## Usage

See [example_test.go](/example_test.go) for example usage.


## Testing

```bash
make test
```


## Credits

If you like this package, all credit should go to [@jphines][jphines], who
suggested the initial design as we were working through an in-process DNS
caching mechanism.

If you don't like its design or its implementation, all blame lies with
[@mccutchen][mccutchen].


[singleflight]: https://godoc.org/golang.org/x/sync/singleflight
[jphines]: https://github.com/jphines
[mccutchen]: https://github.com/mccutchen
