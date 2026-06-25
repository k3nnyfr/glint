package main

import (
	"flag"
	"fmt"
	"os"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/lsp"
)

func cmdLSP(args []string) {
	fs := flag.NewFlagSet("glint lsp", flag.ExitOnError)
	token := fs.String("token", "", "GitLab personal access token (overrides GITLAB_TOKEN)")
	gitlabURL := fs.String("gitlab-url", "", "GitLab instance URL (overrides CI_SERVER_URL / GITLAB_URL)")
	cacheDir := fs.String("cache-dir", "", "directory for caching fetched remote includes")
	offline := fs.Bool("offline", false, "skip all network calls; serve only from --cache-dir")
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

  -h, --help
          Print help

Examples:
  glint lsp
  glint lsp --token glpat-xxxx --cache-dir ~/.cache/glint
  glint lsp --offline
`)
	}
	_ = fs.Parse(args)

	resolvedCacheDir := *cacheDir
	if resolvedCacheDir == "" {
		resolvedCacheDir = defaultCacheDir()
	}
	if *offline && resolvedCacheDir == "" {
		resolvedCacheDir = defaultCacheDir()
	}

	cfg := fetcher.AutoConfig().WithOverrides(*gitlabURL, *token, resolvedCacheDir, *offline)

	srv := lsp.New(os.Stdin, os.Stdout, cfg, version)
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "glint lsp: %v\n", err)
		exit(2)
	}
}
