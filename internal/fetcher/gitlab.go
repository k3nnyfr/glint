// Package fetcher retrieves remote GitLab CI YAML files via the GitLab REST API.
package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// httpClient is the shared HTTP client for all fetcher requests.
// Timeout guards against hung remote servers. Transport is intentionally nil
// so http.DefaultTransport is used dynamically — allowing tests to swap it.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// maxResponseBytes is the per-response size cap. Responses larger than this
// are rejected to prevent memory exhaustion from pathological servers.
const maxResponseBytes int64 = 10 << 20 // 10 MiB

// TokenSource describes where a token was found, which determines the correct
// authentication header to use with the GitLab API.
type TokenSource int

const (
	TokenNone     TokenSource = iota
	TokenPrivate              // GITLAB_TOKEN or GITLAB_PRIVATE_TOKEN → PRIVATE-TOKEN header
	TokenJobToken             // CI_JOB_TOKEN → JOB-TOKEN header
)

// GitLabConfig holds everything needed to reach a GitLab instance.
type GitLabConfig struct {
	BaseURL  string
	Token    string
	Source   TokenSource
	CacheDir string // local cache directory; empty = caching disabled
	Offline  bool   // when true, return an error instead of making network calls
	// ProxyURL overrides the HTTP proxy for all requests made with this config.
	// Empty string means use system proxy settings (HTTP_PROXY / HTTPS_PROXY /
	// NO_PROXY env vars honoured automatically by http.DefaultTransport).
	ProxyURL string
}

// AutoConfig builds a GitLabConfig from environment variables.
//
// Base URL resolution order: CI_SERVER_URL → GITLAB_URL → https://gitlab.com
// Token resolution order:    GITLAB_TOKEN → CI_JOB_TOKEN → GITLAB_PRIVATE_TOKEN
func AutoConfig() GitLabConfig {
	baseURL := firstNonEmpty(
		os.Getenv("CI_SERVER_URL"),
		os.Getenv("GITLAB_URL"),
		"https://gitlab.com",
	)

	cfg := GitLabConfig{BaseURL: strings.TrimRight(baseURL, "/")}

	if t := os.Getenv("GITLAB_TOKEN"); t != "" {
		cfg.Token = t
		cfg.Source = TokenPrivate
	} else if t := os.Getenv("CI_JOB_TOKEN"); t != "" {
		cfg.Token = t
		cfg.Source = TokenJobToken
	} else if t := os.Getenv("GITLAB_PRIVATE_TOKEN"); t != "" {
		cfg.Token = t
		cfg.Source = TokenPrivate
	}

	return cfg
}

// WithProxy returns a copy of cfg with ProxyURL set.
func (cfg GitLabConfig) WithProxy(proxyURL string) GitLabConfig {
	cfg.ProxyURL = proxyURL
	return cfg
}

// client returns an HTTP client for this config.
// When ProxyURL is set it takes precedence over system proxy env vars;
// otherwise http.DefaultTransport's built-in ProxyFromEnvironment is used.
func (cfg GitLabConfig) client() *http.Client {
	if cfg.ProxyURL == "" {
		return httpClient
	}
	proxyURL, err := url.Parse(cfg.ProxyURL)
	if err != nil {
		return httpClient
	}
	// Clone the default transport so all TLS/dial settings are preserved.
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return httpClient
	}
	transport := t.Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	return &http.Client{Timeout: httpClient.Timeout, Transport: transport}
}

// WithOverrides returns a copy of cfg with the provided overrides applied.
// Non-empty strings overwrite the corresponding field; booleans are always
// applied (so offline: false explicitly clears offline mode).
// cacheDir: empty string disables caching.
func (cfg GitLabConfig) WithOverrides(baseURL, token, cacheDir string, offline bool) GitLabConfig {
	if baseURL != "" {
		cfg.BaseURL = strings.TrimRight(baseURL, "/")
	}
	if token != "" {
		cfg.Token = token
		cfg.Source = TokenPrivate
	}
	cfg.CacheDir = cacheDir
	cfg.Offline = offline
	return cfg
}

// ForHost returns a copy of cfg with BaseURL set to https://<host>.
// Used when a CI/CD component reference specifies a different GitLab host.
func (cfg GitLabConfig) ForHost(host string) GitLabConfig {
	cfg.BaseURL = "https://" + host
	return cfg
}

// HasToken reports whether the config carries a usable token.
func (cfg GitLabConfig) HasToken() bool { return cfg.Token != "" }

// FetchFile downloads a single file from a GitLab project repository.
// If no token is configured the request is sent unauthenticated (works for
// public projects and GitLab CI/CD catalog components).
//
//   - project  — namespace/project path, e.g. "my-group/my-project"
//   - filePath — repository file path, e.g. "/templates/ci.yml"
//   - ref      — branch, tag, or commit SHA; empty string defaults to HEAD
func (cfg GitLabConfig) FetchFile(project, filePath, ref string) ([]byte, error) {
	if ref == "" {
		ref = "HEAD"
	}

	cKey := cfg.BaseURL + "|" + project + "|" + filePath + "|" + ref
	if data, ok := cacheRead(cfg.CacheDir, cKey); ok {
		return data, nil
	}
	if cfg.Offline {
		return nil, fmt.Errorf("offline mode: %s:%s@%s is not in the local cache (run without --offline first to populate the cache)", project, filePath, ref)
	}

	encodedProject := url.PathEscape(project)
	encodedFile := url.PathEscape(strings.TrimPrefix(filePath, "/"))

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw",
		cfg.BaseURL, encodedProject, encodedFile)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	q := req.URL.Query()
	q.Set("ref", ref)
	req.URL.RawQuery = q.Encode()

	// Add auth header only when a token is available.
	if cfg.Token != "" {
		switch cfg.Source {
		case TokenJobToken:
			req.Header.Set("JOB-TOKEN", cfg.Token)
		default:
			req.Header.Set("PRIVATE-TOKEN", cfg.Token)
		}
	}

	resp, err := cfg.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("response body exceeds maximum size (%d MiB)", maxResponseBytes>>20)
	}

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			if cfg.Token == "" {
				return nil, fmt.Errorf("authentication required (401) — set GITLAB_TOKEN or CI_JOB_TOKEN")
			}
			return nil, fmt.Errorf("authentication failed (401) — check your token")
		case http.StatusNotFound:
			return nil, fmt.Errorf("not found (404): %s@%s in project %s — check the project path, file path, and token permissions", filePath, ref, project)
		default:
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, msg)
		}
	}

	cacheWrite(cfg.CacheDir, cKey, body)
	return body, nil
}

// FetchURL downloads the content at a plain HTTPS URL without authentication.
// Used for include: remote: entries which are public by definition.
// Responses are read from and written to the local cache when CacheDir is set.
func (cfg GitLabConfig) FetchURL(rawURL string) ([]byte, error) {
	if data, ok := cacheRead(cfg.CacheDir, rawURL); ok {
		return data, nil
	}
	if cfg.Offline {
		return nil, fmt.Errorf("offline mode: %s is not in the local cache (run without --offline first to populate the cache)", rawURL)
	}

	resp, err := cfg.client().Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("response body exceeds maximum size (%d MiB)", maxResponseBytes>>20)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	cacheWrite(cfg.CacheDir, rawURL, body)
	return body, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
