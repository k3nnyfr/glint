package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NotFound(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Ignore) != 0 || cfg.Token != "" || cfg.URL != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_Found(t *testing.T) {
	tmp := t.TempDir()
	content := `
ignore:
  - GL007
  - GL032
severity:
  GL004: warning
stages:
  - quality
token: glpat-test
url: https://gitlab.example.com
cache_dir: /tmp/glint-cache
`
	if err := os.WriteFile(filepath.Join(tmp, Filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Ignore) != 2 || cfg.Ignore[0] != "GL007" {
		t.Errorf("ignore = %v", cfg.Ignore)
	}
	if cfg.Severity["GL004"] != "warning" {
		t.Errorf("severity = %v", cfg.Severity)
	}
	if len(cfg.Stages) != 1 || cfg.Stages[0] != "quality" {
		t.Errorf("stages = %v", cfg.Stages)
	}
	if cfg.Token != "glpat-test" {
		t.Errorf("token = %q", cfg.Token)
	}
	if cfg.URL != "https://gitlab.example.com" {
		t.Errorf("url = %q", cfg.URL)
	}
	if cfg.CacheDir != "/tmp/glint-cache" {
		t.Errorf("cache_dir = %q", cfg.CacheDir)
	}
}

func TestLoad_WalksUp(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "subdir", "pipeline")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Config at the top-level (no .git, so walk continues to tmp).
	if err := os.WriteFile(filepath.Join(tmp, Filename), []byte("token: walked-up\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(sub)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "walked-up" {
		t.Errorf("token = %q, want walked-up", cfg.Token)
	}
}

func TestLoad_StopsAtGitRoot(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "repo", "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// .git marks the repo root — walk must stop here.
	if err := os.Mkdir(filepath.Join(tmp, "repo", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Config placed ABOVE the .git root should not be found.
	if err := os.WriteFile(filepath.Join(tmp, Filename), []byte("token: should-not-load\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(sub)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("token = %q, should not have crossed .git boundary", cfg.Token)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, Filename), []byte("ignore: [unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(tmp)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
