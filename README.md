# fillcache

[![GoDoc](https://godoc.org/github.com/mccutchen/fillcache?status.svg)](https://godoc.org/github.com/mccutchen/fillcache)
[![Build Status](https://travis-ci.org/mccutchen/fillcache.svg?branch=master)](http://travis-ci.org/mccutchen/fillcache)
[![Coverage](http://gocover.io/_badge/github.com/mccutchen/fillcache?0)](http://gocover.io/github.com/mccutchen/fillcache)

Package `fillcache` is an in-process cache with single-flight filling
semantics.

In short: Given a function that computes the value to be cached for a key, it
will ensure that the function is called only once per key no matter how many
concurrent cache gets are issued for a key.
