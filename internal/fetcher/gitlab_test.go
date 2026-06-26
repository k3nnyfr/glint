package fetcher

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// roundTripFunc allows constructing a custom http.RoundTripper from a function.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// errReader is an io.Reader that always returns an error.
type errReader struct{}

func (e errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

// replaceTransport temporarily replaces http.DefaultTransport and restores it.
func replaceTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// ── firstNonEmpty ─────────────────────────────────────────────────────────────

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", "", "c") != "c" { t.Error("should return first non-empty") }
	if firstNonEmpty("a", "b") != "a" { t.Error("should return first") }
	if firstNonEmpty("", "") != "" { t.Error("all empty → empty") }
}

// ── AutoConfig ────────────────────────────────────────────────────────────────

func TestAutoConfig(t *testing.T) {
	// Clear relevant env vars
	for _, k := range []string{"CI_SERVER_URL", "GITLAB_URL", "GITLAB_TOKEN", "CI_JOB_TOKEN", "GITLAB_PRIVATE_TOKEN"} {
		os.Unsetenv(k)
	}
	cfg := AutoConfig()
	if cfg.BaseURL != "https://gitlab.com" {
		t.Errorf("default BaseURL: %q", cfg.BaseURL)
	}
	if cfg.Token != "" {
		t.Error("expected no token")
	}
}

func TestAutoConfig_EnvVars(t *testing.T) {
	os.Setenv("CI_SERVER_URL", "https://my.gitlab.example.com/")
	os.Setenv("GITLAB_TOKEN", "glpat-test")
	defer func() {
		os.Unsetenv("CI_SERVER_URL")
		os.Unsetenv("GITLAB_TOKEN")
	}()
	cfg := AutoConfig()
	if cfg.BaseURL != "https://my.gitlab.example.com" {
		t.Errorf("trailing slash not trimmed: %q", cfg.BaseURL)
	}
	if cfg.Token != "glpat-test" || cfg.Source != TokenPrivate {
		t.Errorf("token: %q source: %v", cfg.Token, cfg.Source)
	}
}

func TestAutoConfig_CIJobToken(t *testing.T) {
	os.Unsetenv("GITLAB_TOKEN")
	os.Setenv("CI_JOB_TOKEN", "job-token-123")
	defer os.Unsetenv("CI_JOB_TOKEN")
	cfg := AutoConfig()
	if cfg.Token != "job-token-123" || cfg.Source != TokenJobToken {
		t.Errorf("unexpected: %+v", cfg)
	}
}

func TestAutoConfig_PrivateToken(t *testing.T) {
	os.Unsetenv("GITLAB_TOKEN")
	os.Unsetenv("CI_JOB_TOKEN")
	os.Setenv("GITLAB_PRIVATE_TOKEN", "legacy-token")
	defer os.Unsetenv("GITLAB_PRIVATE_TOKEN")
	cfg := AutoConfig()
	if cfg.Token != "legacy-token" || cfg.Source != TokenPrivate {
		t.Errorf("unexpected: %+v", cfg)
	}
}

func TestAutoConfig_GitlabURL(t *testing.T) {
	os.Unsetenv("CI_SERVER_URL")
	os.Setenv("GITLAB_URL", "https://gl.local")
	defer os.Unsetenv("GITLAB_URL")
	cfg := AutoConfig()
	if cfg.BaseURL != "https://gl.local" {
		t.Errorf("unexpected BaseURL: %q", cfg.BaseURL)
	}
}

// ── WithOverrides ─────────────────────────────────────────────────────────────

func TestWithOverrides(t *testing.T) {
	base := GitLabConfig{BaseURL: "https://gitlab.com", Token: "old", Source: TokenPrivate}
	got := base.WithOverrides("https://other.com/", "new-token", "/cache", true)
	if got.BaseURL != "https://other.com" { t.Errorf("BaseURL: %q", got.BaseURL) }
	if got.Token != "new-token" { t.Error("Token not overridden") }
	if got.CacheDir != "/cache" { t.Error("CacheDir not set") }
	if !got.Offline { t.Error("Offline not set") }
	// empty string leaves BaseURL alone
	got2 := base.WithOverrides("", "", "", false)
	if got2.BaseURL != "https://gitlab.com" { t.Error("empty override should not change BaseURL") }
}

// ── ForHost ───────────────────────────────────────────────────────────────────

func TestForHost(t *testing.T) {
	cfg := GitLabConfig{BaseURL: "https://gitlab.com"}
	got := cfg.ForHost("my-host.example.com")
	if got.BaseURL != "https://my-host.example.com" {
		t.Errorf("unexpected BaseURL: %q", got.BaseURL)
	}
}

// ── HasToken ──────────────────────────────────────────────────────────────────

func TestHasToken(t *testing.T) {
	if (GitLabConfig{}).HasToken() { t.Error("empty config should have no token") }
	if !(GitLabConfig{Token: "x"}).HasToken() { t.Error("config with token should have token") }
}

// ── FetchFile ─────────────────────────────────────────────────────────────────

func TestFetchFile_FromCache(t *testing.T) {
	dir := t.TempDir()
	key := "https://gitlab.com|my/project|/templates/ci.yml|HEAD"
	cacheWrite(dir, key, []byte("cached: true"))
	cfg := GitLabConfig{BaseURL: "https://gitlab.com", CacheDir: dir}
	data, err := cfg.FetchFile("my/project", "/templates/ci.yml", "")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if string(data) != "cached: true" { t.Errorf("got %q", data) }
}

func TestFetchFile_Offline_Miss(t *testing.T) {
	cfg := GitLabConfig{BaseURL: "https://gitlab.com", Offline: true}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil { t.Fatal("expected error in offline mode") }
}

func TestFetchFile_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("script: echo ok"))
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL, Token: "tok", Source: TokenPrivate}
	data, err := cfg.FetchFile("ns/proj", "/ci.yml", "main")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if string(data) != "script: echo ok" { t.Errorf("got %q", data) }
}

func TestFetchFile_Unauthorized_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil { t.Fatal("expected error") }
}

func TestFetchFile_Unauthorized_WithToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL, Token: "tok", Source: TokenPrivate}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil { t.Fatal("expected error") }
}

func TestFetchFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil { t.Fatal("expected error") }
}

func TestFetchFile_OtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil { t.Fatal("expected error") }
}

func TestFetchFile_JobToken(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("JOB-TOKEN")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL, Token: "ci-job-tok", Source: TokenJobToken}
	cfg.FetchFile("p/q", "/f.yml", "main")
	if gotHeader != "ci-job-tok" {
		t.Errorf("JOB-TOKEN header not sent, got %q", gotHeader)
	}
}

func TestFetchFile_WritesToCache(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("stage: build"))
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL, CacheDir: dir}
	_, err := cfg.FetchFile("p/q", "/ci.yml", "main")
	if err != nil { t.Fatal(err) }
	// Second call should hit cache
	data, ok := cacheRead(dir, srv.URL+"|p/q|/ci.yml|main")
	if !ok { t.Fatal("cache miss after fetch") }
	if string(data) != "stage: build" { t.Errorf("unexpected cache content: %q", data) }
}

// ── FetchURL ──────────────────────────────────────────────────────────────────

func TestFetchURL_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("remote: content"))
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL}
	data, err := cfg.FetchURL(srv.URL + "/template.yml")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if string(data) != "remote: content" { t.Errorf("got %q", data) }
}

func TestFetchURL_FromCache(t *testing.T) {
	dir := t.TempDir()
	rawURL := "https://example.com/tmpl.yml"
	cacheWrite(dir, rawURL, []byte("cached"))
	cfg := GitLabConfig{CacheDir: dir}
	data, err := cfg.FetchURL(rawURL)
	if err != nil { t.Fatal(err) }
	if string(data) != "cached" { t.Errorf("got %q", data) }
}

func TestFetchURL_Offline(t *testing.T) {
	cfg := GitLabConfig{Offline: true}
	_, err := cfg.FetchURL("https://example.com/tmpl.yml")
	if err == nil { t.Fatal("expected error in offline mode") }
}

// TestFetchFile_NewRequestFails covers gitlab.go:113-115 — http.NewRequest error
// when the BaseURL contains a control character (null byte) making the URL invalid.
func TestFetchFile_NewRequestFails(t *testing.T) {
	cfg := GitLabConfig{BaseURL: "https://example.com\x00"}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil {
		t.Fatal("expected error for URL with null byte")
	}
}

// TestFetchFile_DoFails covers gitlab.go:132-134 — http.DefaultClient.Do error.
func TestFetchFile_DoFails(t *testing.T) {
	replaceTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("transport error")
	}))
	cfg := GitLabConfig{BaseURL: "https://example.com"}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil {
		t.Fatal("expected error when transport fails")
	}
}

// TestFetchFile_ReadBodyFails covers gitlab.go:138-140 — io.ReadAll error on the
// response body.
func TestFetchFile_ReadBodyFails(t *testing.T) {
	replaceTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errReader{}),
			Header:     make(http.Header),
		}, nil
	}))
	cfg := GitLabConfig{BaseURL: "https://example.com"}
	_, err := cfg.FetchFile("p/q", "/f.yml", "main")
	if err == nil {
		t.Fatal("expected error when body read fails")
	}
}

// TestFetchURL_GetFails covers gitlab.go:173-175 — http.Get error.
func TestFetchURL_GetFails(t *testing.T) {
	replaceTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("get failed")
	}))
	cfg := GitLabConfig{}
	_, err := cfg.FetchURL("https://example.com/template.yml")
	if err == nil {
		t.Fatal("expected error when http.Get fails")
	}
}

// TestFetchURL_ReadBodyFails covers gitlab.go:178-180 — io.ReadAll error on the
// FetchURL response body.
func TestFetchURL_ReadBodyFails(t *testing.T) {
	replaceTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errReader{}),
			Header:     make(http.Header),
		}, nil
	}))
	cfg := GitLabConfig{}
	_, err := cfg.FetchURL("https://example.com/template.yml")
	if err == nil {
		t.Fatal("expected error when FetchURL body read fails")
	}
}

func TestFetchURL_NotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	cfg := GitLabConfig{}
	_, err := cfg.FetchURL(srv.URL)
	if err == nil { t.Fatal("expected error for non-200 status") }
}

func TestFetchFile_ResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write maxResponseBytes+1 bytes to exceed the cap.
		chunk := make([]byte, 4096)
		written := int64(0)
		for written <= maxResponseBytes {
			n, _ := w.Write(chunk)
			written += int64(n)
		}
	}))
	defer srv.Close()
	cfg := GitLabConfig{BaseURL: srv.URL}
	_, err := cfg.FetchFile("group/project", "/ci.yml", "main")
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchURL_ResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 4096)
		written := int64(0)
		for written <= maxResponseBytes {
			n, _ := w.Write(chunk)
			written += int64(n)
		}
	}))
	defer srv.Close()
	cfg := GitLabConfig{}
	_, err := cfg.FetchURL(srv.URL)
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("unexpected error: %v", err)
	}
}
