# fillcache

[![GoDoc](https://godoc.org/github.com/mccutchen/fillcache?status.svg)](https://godoc.org/github.com/mccutchen/fillcache)
[![Build Status](https://travis-ci.org/mccutchen/fillcache.svg?branch=master)](http://travis-ci.org/mccutchen/fillcache)
[![Coverage](http://gocover.io/_badge/github.com/mccutchen/fillcache?0)](http://gocover.io/github.com/mccutchen/fillcache)

Package `fillcache` is like a dumber, in-process version of [groupcache][].

Give it a function that knows how to fill the cache for a given key, and it
will ensure that the fill function is only called once per key no matter how
many concurrent cache gets are called for that key.

[groupcache]: https://github.com/golang/groupcache
