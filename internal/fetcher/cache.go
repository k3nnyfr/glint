package fetcher

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// cacheRead returns the cached bytes for the given cache key, or (nil, false)
// on a miss (key not present, dir empty, or any read error).
func cacheRead(dir, key string) ([]byte, bool) {
	if dir == "" {
		return nil, false
	}
	data, err := os.ReadFile(cachePath(dir, key))
	if err != nil {
		return nil, false
	}
	return data, true
}

// cacheWrite stores bytes in the cache for the given key. Write errors are
// silently ignored so cache failures never block the normal fetch path.
func cacheWrite(dir, key string, data []byte) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	_ = os.WriteFile(cachePath(dir, key), data, 0o600)
}

// cachePath returns the filesystem path for a cache entry.
// The filename is the SHA-256 hex digest of the key so arbitrary keys (URLs,
// "project:file@ref" strings) map to safe, stable filenames.
func cachePath(dir, key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(dir, fmt.Sprintf("%x.yml", h))
}
