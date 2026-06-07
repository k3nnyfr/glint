// Package fetcher retrieves remote GitLab CI YAML files via the GitLab REST API.
package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

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
	BaseURL string
	Token   string
	Source  TokenSource
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

// WithOverrides returns a copy of cfg with non-empty overrides applied.
func (cfg GitLabConfig) WithOverrides(baseURL, token string) GitLabConfig {
	if baseURL != "" {
		cfg.BaseURL = strings.TrimRight(baseURL, "/")
	}
	if token != "" {
		cfg.Token = token
		cfg.Source = TokenPrivate
	}
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
	encodedProject := url.PathEscape(project)
	encodedFile := url.PathEscape(strings.TrimPrefix(filePath, "/"))

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw",
		cfg.BaseURL, encodedProject, encodedFile)

	if ref == "" {
		ref = "HEAD"
	}

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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
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
