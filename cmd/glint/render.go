package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/config"
	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/model"
	"git.k3nny.fr/glint/internal/resolver"
	"gopkg.in/yaml.v3"
)

func cmdRender(args []string) {
	fs := flag.NewFlagSet("glint render", flag.ExitOnError)
	output := fs.String("output", "", "output file path (default: rendered.gitlab-ci.yml; use - for stdout)")
	token := fs.String("token", "", "GitLab personal access token (overrides GITLAB_TOKEN)")
	gitlabURL := fs.String("gitlab-url", "", "GitLab instance URL (overrides CI_SERVER_URL / GITLAB_URL)")
	cacheDir := fs.String("cache-dir", "", "directory to cache fetched remote includes")
	offline := fs.Bool("offline", false, "skip all network calls; serve only from --cache-dir")
	proxy := fs.String("proxy", "", "HTTP proxy URL for remote includes (e.g. http://proxy:8080)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, `Resolve all includes and extends into a single merged YAML file.

Performs the same include resolution and extends merging that GitLab CI
does server-side, then writes the fully flattened pipeline to a single file.
Useful for inspecting the resolved pipeline or running further local tooling.

The output file strips 'include:' (consumed by resolution) and 'extends:'
(applied to each job) keys. All other fields are preserved verbatim.
Template jobs (names starting with '.') are retained.

Usage: glint render [OPTIONS] <PIPELINE>

Arguments:
  <PIPELINE>  Path to the .gitlab-ci.yml file to resolve

Options:
      --output <FILE>
          Write the rendered pipeline to FILE.
          Use '-' to write to stdout.
          [default: rendered.gitlab-ci.yml]

      --token <TOKEN>
          GitLab personal access token for fetching project: and component:
          includes.
          [env: GITLAB_TOKEN | CI_JOB_TOKEN | GITLAB_PRIVATE_TOKEN]

      --gitlab-url <URL>
          GitLab instance URL.
          [env: CI_SERVER_URL | GITLAB_URL] [default: https://gitlab.com]

      --cache-dir <DIR>
          Cache directory for fetched remote includes.

      --offline
          Do not make any network calls; use cache only.

      --proxy <URL>
          HTTP proxy URL for remote includes and GitLab API calls.

  -h, --help
          Print help

Examples:
  glint render .gitlab-ci.yml
  glint render --output merged.yml .gitlab-ci.yml
  glint render --output - .gitlab-ci.yml | yq .
  glint render --offline --cache-dir ~/.cache/glint .gitlab-ci.yml
`)
	}
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fs.Usage()
		exit(2)
		return
	}
	path := fs.Arg(0)
	rootDir := filepath.Dir(filepath.Clean(path))

	glintCfg, cfgErr := config.Load(rootDir)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "%s: [warning] %s: %v\n", path, config.Filename, cfgErr)
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
	if *offline && resolvedCacheDir == "" {
		resolvedCacheDir = defaultCacheDir()
	}
	resolvedProxy := *proxy
	if resolvedProxy == "" {
		resolvedProxy = glintCfg.Proxy
	}

	cfg := fetcher.AutoConfig().WithOverrides(fetcherURL, fetcherToken, resolvedCacheDir, *offline).WithProxy(resolvedProxy)

	p, err := model.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		exit(2)
		return
	}

	warnings, _ := resolver.ResolveIncludes(p, cfg, rootDir)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "%s: [warning] include %s\n", path, w)
	}

	extWarnings, err := resolver.Resolve(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving extends: %v\n", err)
		exit(2)
		return
	}
	for _, w := range extWarnings {
		fmt.Fprintf(os.Stderr, "%s: [warning] job %q extends unknown job %q; extends chain skipped\n", path, w.Job, w.Base)
	}

	doc, err := buildRenderDoc(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: building output: %v\n", err)
		exit(2)
		return
	}

	outPath := *output
	if outPath == "" {
		outPath = "rendered.gitlab-ci.yml"
	}

	var w interface{ Write([]byte) (int, error) }
	if outPath == "-" {
		w = os.Stdout
	} else {
		f, err := os.Create(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: creating output file: %v\n", err)
			exit(2)
			return
		}
		defer f.Close()
		w = f
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing output: %v\n", err)
		exit(2)
		return
	}
	_ = enc.Close()

	if outPath != "-" {
		jobCount := 0
		for name := range p.Jobs {
			if !strings.HasPrefix(name, ".") {
				jobCount++
			}
		}
		fmt.Fprintf(os.Stderr, "rendered: %s (%d job(s), %d stage(s))\n", outPath, jobCount, len(p.Stages))
	}
}

// buildRenderDoc constructs an ordered yaml.Node document from the resolved
// pipeline. Pipeline-level keys (stages, variables, default, workflow) come
// first, followed by template jobs (.name) then regular jobs, both sorted
// alphabetically. 'include:' and 'extends:' are omitted — they have been
// consumed by the resolution passes.
func buildRenderDoc(p *model.Pipeline) (*yaml.Node, error) {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	addField := func(key string, val any) error {
		n, err := anyToNode(val)
		if err != nil {
			return fmt.Errorf("encoding %q: %w", key, err)
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			n,
		)
		return nil
	}

	if len(p.Stages) > 0 {
		if err := addField("stages", p.Stages); err != nil {
			return nil, err
		}
	}
	if len(p.Variables) > 0 {
		if err := addField("variables", p.Variables); err != nil {
			return nil, err
		}
	}
	if p.Default != nil {
		if err := addField("default", p.Default); err != nil {
			return nil, err
		}
	}
	if p.Workflow != nil {
		if err := addField("workflow", p.Workflow); err != nil {
			return nil, err
		}
	}

	// Collect and sort job names: template jobs first, then regular jobs.
	var templates, regular []string
	for name := range p.RawJobs {
		if strings.HasPrefix(name, ".") {
			templates = append(templates, name)
		} else {
			regular = append(regular, name)
		}
	}
	sort.Strings(templates)
	sort.Strings(regular)

	for _, name := range append(templates, regular...) {
		raw := p.RawJobs[name]
		// Copy to avoid mutating the shared map; strip resolution-consumed keys.
		cleaned := make(map[string]any, len(raw))
		for k, v := range raw {
			if k == "extends" {
				continue
			}
			cleaned[k] = v
		}
		if err := addField(name, cleaned); err != nil {
			return nil, err
		}
	}

	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return doc, nil
}

// anyToNode converts an arbitrary Go value to a *yaml.Node by round-tripping
// through yaml.Marshal / yaml.Unmarshal, which preserves all value types.
func anyToNode(v any) (*yaml.Node, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}
