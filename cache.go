package main

import (
	"os"
	"path/filepath"
	"time"
)

const cacheMaxAge = 7 * 24 * time.Hour

// cacheDir returns the gritt cache directory, creating it if needed.
func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gritt")
	return dir, os.MkdirAll(dir, 0755)
}

// cachePath returns the full path for a cache file.
func cachePath(name string) string {
	dir, err := cacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, name)
}

// isCacheStale returns true if the file is missing or older than cacheMaxAge.
func isCacheStale(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > cacheMaxAge
}
