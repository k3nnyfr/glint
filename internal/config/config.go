package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Filename is the name of the project-level glint configuration file.
const Filename = ".glint.yml"

// Config holds project-level glint configuration loaded from .glint.yml.
type Config struct {
	// Ignore lists rule IDs to suppress entirely (e.g. ["GL007", "GL032"]).
	Ignore []string `yaml:"ignore"`

	// Severity maps rule IDs to overridden severity levels.
	// Valid values: "error", "warning", "ignore".
	// "ignore" is equivalent to listing the rule in Ignore.
	Severity map[string]string `yaml:"severity"`

	// Stages lists additional stage names that are considered valid for this
	// project, beyond what is declared in the pipeline's own stages: block.
	// Jobs in these stages are not flagged by GL004.
	Stages []string `yaml:"stages"`

	// Token is a default GitLab personal access token used when neither the
	// --token flag nor GITLAB_TOKEN (/ CI_JOB_TOKEN / GITLAB_PRIVATE_TOKEN)
	// environment variables are set.
	Token string `yaml:"token"`

	// URL is the default GitLab instance URL. Overridden by --gitlab-url and
	// the CI_SERVER_URL / GITLAB_URL environment variables.
	URL string `yaml:"url"`

	// CacheDir is the default directory for caching fetched remote includes.
	// Overridden by the --cache-dir flag.
	CacheDir string `yaml:"cache_dir"`

	// Proxy is the HTTP proxy URL for fetching remote includes and GitLab API
	// calls (e.g. "http://proxy.example.com:8080"). Overridden by the --proxy
	// flag. When empty, system proxy settings (HTTP_PROXY / HTTPS_PROXY /
	// NO_PROXY env vars) are used automatically.
	Proxy string `yaml:"proxy"`
}

// Load searches for a .glint.yml file starting from dir and walking up toward
// the filesystem root. The walk stops at the first .git directory found (the
// repository root) or at the filesystem root. Returns an empty Config (and no
// error) when no config file is found.
func Load(dir string) (Config, error) {
	dir = filepath.Clean(dir)
	for {
		candidate := filepath.Join(dir, Filename)
		data, err := os.ReadFile(candidate)
		if err == nil {
			var cfg Config
			if yerr := yaml.Unmarshal(data, &cfg); yerr != nil {
				return Config{}, fmt.Errorf("parsing %s: %w", candidate, yerr)
			}
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("reading %s: %w", candidate, err)
		}

		// Stop when we reach a git root so we don't wander into parent repos.
		if _, serr := os.Stat(filepath.Join(dir, ".git")); serr == nil {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root
		}
		dir = parent
	}
	return Config{}, nil
}
