package fetcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCachePath(t *testing.T) {
	p1 := cachePath("/tmp/cache", "key1")
	p2 := cachePath("/tmp/cache", "key2")
	if p1 == p2 {
		t.Error("different keys should produce different paths")
	}
	if filepath.Dir(p1) != "/tmp/cache" {
		t.Errorf("expected /tmp/cache dir, got %q", filepath.Dir(p1))
	}
}

func TestCacheReadMiss(t *testing.T) {
	// empty dir → miss
	data, ok := cacheRead("", "key")
	if ok || data != nil {
		t.Error("empty cacheDir should always miss")
	}

	// nonexistent entry → miss
	data, ok = cacheRead(t.TempDir(), "nonexistent-key")
	if ok || data != nil {
		t.Error("missing cache entry should miss")
	}
}

func TestCacheWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	key := "https://example.com/template.yml"
	content := []byte("stages: [build]")

	cacheWrite(dir, key, content)

	got, ok := cacheRead(dir, key)
	if !ok {
		t.Fatal("expected cache hit after write")
	}
	if string(got) != string(content) {
		t.Errorf("cached content mismatch: got %q want %q", got, content)
	}
}

func TestCacheWrite_EmptyDir(t *testing.T) {
	// Should silently do nothing when dir is empty string.
	cacheWrite("", "key", []byte("data"))
}

// TestCacheWrite_DirIsFile covers the os.MkdirAll error path (cache.go:29-31)
// when the cache dir path is occupied by a regular file.
func TestCacheWrite_DirIsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	// MkdirAll(f) fails because f is a file, not a directory.
	cacheWrite(f, "key", []byte("data"))
	// No panic, no error returned — the function silently returns.
}

func TestCacheWrite_MkdirAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	cacheWrite(dir, "k", []byte("v"))
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

func TestCacheWrite_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	cacheWrite(dir, "seckey", []byte("secret"))
	info, err := os.Stat(cachePath(dir, "seckey"))
	if err != nil {
		t.Fatalf("stat cache file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("cache file mode = %04o; want 0600", mode)
	}
}

func TestCacheWrite_DirPermissions(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "glint-cache")
	cacheWrite(dir, "k", []byte("v"))
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat cache dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("cache dir mode = %04o; want 0700", mode)
	}
}
