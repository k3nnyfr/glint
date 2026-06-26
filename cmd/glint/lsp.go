package main

import (
	"flag"
	"fmt"
	"os"

	"git.k3nny.fr/glint/internal/config"
	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/lsp"
)

func cmdLSP(args []string) {
	fs := flag.NewFlagSet("glint lsp", flag.ExitOnError)
	token := fs.String("token", "", "GitLab personal access token (overrides GITLAB_TOKEN)")
	gitlabURL := fs.String("gitlab-url", "", "GitLab instance URL (overrides CI_SERVER_URL / GITLAB_URL)")
	cacheDir := fs.String("cache-dir", "", "directory for caching fetched remote includes")
	offline := fs.Bool("offline", false, "skip all network calls; serve only from --cache-dir")
	proxy := fs.String("proxy", "", "HTTP proxy URL for remote includes and GitLab API calls (e.g. http://proxy:8080); overrides system proxy env vars")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, `Start a Language Server Protocol server for .gitlab-ci.yml files.

Reads JSON-RPC 2.0 messages from stdin and writes responses to stdout using
the standard Content-Length framing. Connect with any LSP client (VS Code,
Neovim, Emacs, etc.).

Usage: glint lsp [OPTIONS]

Options:
      --token <TOKEN>
          GitLab personal access token used for resolving project: and
          component: includes. Defaults to GITLAB_TOKEN env var.

      --gitlab-url <URL>
          GitLab instance URL for resolving remote includes.
          [env: CI_SERVER_URL | GITLAB_URL] [default: https://gitlab.com]

      --cache-dir <DIR>
          Cache directory for fetched remote includes. Defaults to
          ~/.cache/glint so subsequent opens are served from cache.

      --offline
          Do not make any network calls; resolve only local includes.
          Implies --cache-dir default (~/.cache/glint) when not set.

      --proxy <URL>
          HTTP proxy URL for remote includes and GitLab API calls
          (e.g. http://proxy:8080). Overrides system proxy env vars
          (HTTP_PROXY / HTTPS_PROXY). Also configurable via proxy: in
          .glint.yml.

  -h, --help
          Print help

Examples:
  glint lsp
  glint lsp --token glpat-xxxx --cache-dir ~/.cache/glint
  glint lsp --offline
  glint lsp --proxy http://proxy.example.com:8080
`)
	}
	_ = fs.Parse(args)

	// Load project config from the working directory (the project root from
	// which the LSP server is launched). CLI flags take priority.
	wd, _ := os.Getwd()
	glintCfg, cfgErr := config.Load(wd)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "glint lsp: [warning] %s: %v\n", config.Filename, cfgErr)
	}

	fetcherToken := *token
	if fetcherToken == "" {
		fetcherToken = glintCfg.Token
	}
	fetcherURL := *gitlabURL
	if fetcherURL == "" {
		fetcherURL = glintCfg.URL
	}
	resolvedCacheDir := *cacheDir
	if resolvedCacheDir == "" {
		resolvedCacheDir = glintCfg.CacheDir
	}
	if resolvedCacheDir == "" {
		resolvedCacheDir = defaultCacheDir()
	}
	resolvedProxy := *proxy
	if resolvedProxy == "" {
		resolvedProxy = glintCfg.Proxy
	}

	cfg := fetcher.AutoConfig().WithOverrides(fetcherURL, fetcherToken, resolvedCacheDir, *offline).WithProxy(resolvedProxy)

	srv := lsp.New(os.Stdin, os.Stdout, cfg, version)
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "glint lsp: %v\n", err)
		exit(2)
	}
}
