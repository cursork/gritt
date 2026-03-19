package main

import "github.com/cursork/gritt/cache"

func cacheDir() (string, error) { return cache.Dir() }
func cachePath(name string) string { return cache.Path(name) }
func isCacheStale(path string) bool { return cache.IsStale(path) }
