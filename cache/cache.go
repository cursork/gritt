// Package cache provides shared cache directory management for gritt tools.
// All tools share the same cache at ~/.cache/gritt/ (or platform equivalent).
package cache

import (
	"os"
	"path/filepath"
	"time"
)

const MaxAge = 7 * 24 * time.Hour

// Dir returns the gritt cache directory, creating it if needed.
func Dir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gritt")
	return dir, os.MkdirAll(dir, 0755)
}

// Path returns the full path for a cache file.
// Returns "" if the cache directory is unavailable.
func Path(name string) string {
	dir, err := Dir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, name)
}

// IsStale returns true if the file is missing or older than MaxAge.
func IsStale(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > MaxAge
}
