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

func TestCacheWrite_MkdirAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	cacheWrite(dir, "k", []byte("v"))
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}
