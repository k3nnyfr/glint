package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/config"
	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/graph"
	"git.k3nny.fr/glint/internal/linter"
	"git.k3nny.fr/glint/internal/model"
	"git.k3nny.fr/glint/internal/resolver"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

// exit is a variable so tests can capture exit calls without terminating.
var exit = os.Exit

// userHomeDirFn is a variable so tests can simulate UserHomeDir failure.
var userHomeDirFn = os.UserHomeDir

// defaultCacheDir returns the platform-default glint cache directory:
// $XDG_CACHE_HOME/glint or ~/.cache/glint.
func defaultCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "glint")
	}
	if home, err := userHomeDirFn(); err == nil {
		return filepath.Join(home, ".cache", "glint")
	}
	return ""
}

// execCommandOutput is a variable so tests can mock external command execution.
var execCommandOutput = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// gitDiffFiles runs "git diff --name-only <ref>" and returns the list of changed
// file paths. Returns nil + error when the command fails (e.g. not in a git repo
// or the ref doesn't exist).
func gitDiffFiles(ref string) ([]string, error) {
	out, err := execCommandOutput("git", "diff", "--name-only", ref)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s: %w", ref, err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

const globalUsage = `glint: Lint and visualise GitLab CI pipelines locally.

Usage: glint [OPTIONS] <COMMAND>

Commands:
  check    Lint a pipeline file — exits 0 (clean), 2 (errors), or 10 (warnings only)
  render   Resolve all includes and extends into a single merged YAML file
  graph    Visualise the pipeline as a job tree or Mermaid graph
  explain  Show description and fix for a lint rule (e.g. glint explain GL007)
  lsp      Start a Language Server Protocol server (stdin/stdout)

Options:
  -h, --help     Print help
  -v, --version  Print version

For help with a specific command, see: ` + "`glint <command> --help`" + `.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, globalUsage)
		exit(2)
		return
	}
	switch os.Args[1] {
	case "check":
		cmdCheck(os.Args[2:])
	case "render":
		cmdRender(os.Args[2:])
	case "graph":
		cmdGraph(os.Args[2:])
	case "explain":
		cmdExplain(os.Args[2:])
	case "lsp":
		cmdLSP(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, globalUsage)
	case "-v", "--version", "version":
		fmt.Printf("glint %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "glint: unknown command %q\n\n%s", os.Args[1], globalUsage)
		exit(2)
	}
}

// multiFlag allows a flag to be specified multiple times.
type multiFlag []string

func (f *multiFlag) String() string { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func cmdCheck(args []string) {
	fs := flag.NewFlagSet("glint check", flag.ExitOnError)
	token := fs.String("token", "", "GitLab personal access token (overrides GITLAB_TOKEN)")
	gitlabURL := fs.String("gitlab-url", "", "GitLab instance URL (overrides CI_SERVER_URL / GITLAB_URL)")
	cacheDir := fs.String("cache-dir", "", "directory to cache fetched remote includes (created if needed)")
	offline := fs.Bool("offline", false, "skip all network calls; serve only from --cache-dir")
	proxy := fs.String("proxy", "", "HTTP proxy URL for remote includes and GitLab API calls (e.g. http://proxy:8080); overrides system proxy env vars")
	format := fs.String("format", "text", "output format: text, json, sarif, junit, github")
	noWarn := fs.Bool("no-warn", false, "suppress warning findings; only errors are shown and affect the exit code")
	branch := fs.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
	tag := fs.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
	source := fs.String("source", "", "set CI_PIPELINE_SOURCE")
	listVars := fs.Bool("list-vars", false, "print all collected pipeline variables (from root and included files) to stderr, then continue")
	var vars multiFlag
	fs.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	var changesFiles multiFlag
	fs.Var(&changesFiles, "changes", "mark a file path as changed for rules:changes: evaluation; repeatable")
	changesFrom := fs.String("changes-from", "", "git ref to diff against for rules:changes: evaluation (e.g. HEAD~1, origin/main)")
	var contexts multiFlag
	fs.Var(&contexts, "context", "simulation context as KEY=VALUE[,...]; repeatable for multi-context comparison table")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, `Lint a GitLab CI pipeline file.

Resolves local includes and extends chains, then runs all lint rules.

Exit codes:
  0   no findings (clean)
  2   one or more errors
  10  one or more warnings, no errors

Usage: glint check [OPTIONS] <PIPELINE>

Arguments:
  <PIPELINE>  Path to the .gitlab-ci.yml file to lint

Options:
      --format <FORMAT>
          Output format for findings.
          [default: text] [possible values: text, json, sarif, junit, github]

      --no-warn
          Suppress warning findings. Only errors are printed and counted toward
          the exit code; warnings are ignored entirely.

      --token <TOKEN>
          GitLab personal access token. Required to fetch project: includes;
          component: includes are attempted unauthenticated.
          [env: GITLAB_TOKEN | CI_JOB_TOKEN | GITLAB_PRIVATE_TOKEN]

      --gitlab-url <URL>
          GitLab instance URL.
          [env: CI_SERVER_URL | GITLAB_URL] [default: https://gitlab.com]

      --cache-dir <DIR>
          Cache fetched remote templates (project: and component: includes) in
          DIR. The directory is created on first use. Subsequent runs read from
          cache first, avoiding repeated network calls.

      --offline
          Do not make any network calls. All remote includes must already be
          present in --cache-dir; missing entries emit a warning (same as
          having no token). Implies the default cache dir (~/.cache/glint) when
          --cache-dir is not set.

      --proxy <URL>
          HTTP proxy URL for remote includes and GitLab API calls
          (e.g. http://proxy:8080). Overrides system proxy env vars
          (HTTP_PROXY / HTTPS_PROXY). Also configurable via proxy: in
          .glint.yml.

      --branch <NAME>
          Simulate a branch push. Populates: CI_COMMIT_BRANCH,
          CI_COMMIT_REF_NAME, CI_COMMIT_REF_SLUG, CI_PIPELINE_SOURCE=push.
          [default: main]

      --tag <NAME>
          Simulate a tag push. Populates: CI_COMMIT_TAG, CI_COMMIT_REF_NAME,
          CI_COMMIT_REF_SLUG, CI_PIPELINE_SOURCE=push. Clears CI_COMMIT_BRANCH.

      --source <EVENT>
          Override CI_PIPELINE_SOURCE.
          [default: push] [possible values: push, merge_request_event, schedule, web, api]

      --var <KEY=VALUE>
          Set or override a CI variable. Takes precedence over --branch, --tag,
          and --source. Repeatable.

      --changes <PATH>
          Mark a file as changed for rules:changes: evaluation. Repeatable.
          When given, only jobs whose rules:changes: patterns match at least one
          --changes path will have that rule fire; without --changes or
          --changes-from the condition is treated as always matching (permissive).

      --changes-from <REF>
          Run "git diff --name-only <REF>" to determine changed files for
          rules:changes: evaluation. Combined with --changes if both are given.

      --context <KEY=VALUE[,...]>
          Define a simulation context. Repeatable: each --context flag adds one
          column to a comparison table showing every job's state across contexts.
          Known keys: branch, tag, source. Any other KEY=VALUE is treated as a
          CI variable override. Examples:
            --context branch=main --context branch=develop
            --context tag=v1.0.0
            --context branch=main,DEPLOY_ENV=prod

      --list-vars
          Print all pipeline-level variables collected from the root file and
          every included file (sorted KEY=VALUE) to stderr, then continue
          normally. Useful for debugging variable resolution and GL032 findings.

  -h, --help
          Print help

Note: when none of --branch, --tag, --source, or --var are given, glint
defaults to --branch main --source push so that rules:if: expressions are
always evaluated.

Examples:
  glint check .gitlab-ci.yml
  glint check --format json .gitlab-ci.yml
  glint check --format sarif .gitlab-ci.yml | upload-to-github-code-scanning
  glint check --format junit .gitlab-ci.yml > junit.xml
  glint check --format github .gitlab-ci.yml
  glint check --branch develop .gitlab-ci.yml
  glint check --tag v1.0.0 .gitlab-ci.yml
  glint check --source merge_request_event .gitlab-ci.yml
  glint check --list-vars .gitlab-ci.yml
  GITLAB_TOKEN=glpat-xxxx glint check .gitlab-ci.yml
  glint check --token glpat-xxxx --gitlab-url https://gitlab.example.com .gitlab-ci.yml
  glint check --branch main --var DEPLOY_ENV=production .gitlab-ci.yml
  glint check --cache-dir ~/.cache/glint .gitlab-ci.yml
  glint check --offline --cache-dir ~/.cache/glint .gitlab-ci.yml
  glint check --no-warn .gitlab-ci.yml
  glint check --changes src/main.go --changes Dockerfile .gitlab-ci.yml
  glint check --changes-from origin/main .gitlab-ci.yml
  glint check --context branch=main --context branch=develop .gitlab-ci.yml
  glint check --context branch=main --context tag=v1.0.0 --context source=schedule .gitlab-ci.yml
  glint check --proxy http://proxy.example.com:8080 .gitlab-ci.yml
`)
	}
	_ = fs.Parse(args)

	// Apply implicit defaults only in single-context mode when no flags are given.
	if len(contexts) == 0 && *branch == "" && *tag == "" && *source == "" && len(vars) == 0 {
		*branch = "main"
		*source = "push"
	}

	validFormats := map[string]bool{
		"text": true, "json": true, "sarif": true, "junit": true, "github": true,
	}
	if !validFormats[*format] {
		fmt.Fprintf(os.Stderr, "glint: unknown format %q; valid: text, json, sarif, junit, github\n", *format)
		exit(2)
		return
	}

	if fs.NArg() != 1 {
		fs.Usage()
		exit(2)
		return
	}
	path := fs.Arg(0)
	rootDir := filepath.Dir(filepath.Clean(path))

	// Load project config (.glint.yml), searching from the pipeline directory
	// up to the git root.
	glintCfg, cfgErr := config.Load(rootDir)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "%s: [warning] %s: %v\n", path, config.Filename, cfgErr)
	}

	// CLI flags take priority over config file values, which take priority over
	// environment variables (read by AutoConfig).
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

	// Merge config-defined stages into the pipeline before linting so that
	// GL004 does not fire for jobs in stages declared only in .glint.yml.
	stageSet := make(map[string]bool, len(p.Stages))
	for _, s := range p.Stages {
		stageSet[s] = true
	}
	for _, s := range glintCfg.Stages {
		if !stageSet[s] {
			p.Stages = append(p.Stages, s)
			stageSet[s] = true
		}
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

	// Compute changed files once; shared across single and multi-context modes.
	var allChanged []string
	var changesReliable bool
	if *changesFrom != "" || len(changesFiles) > 0 {
		changesReliable = len(changesFiles) > 0
		if *changesFrom != "" {
			files, err := gitDiffFiles(*changesFrom)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: [warning] --changes-from: %v\n", path, err)
			} else {
				allChanged = append(allChanged, files...)
				changesReliable = true
			}
		}
		allChanged = append(allChanged, changesFiles...)
		if changesReliable && allChanged == nil {
			allChanged = []string{}
		}
	}

	var skipped map[string]bool // jobs excluded from cross-job lint checks

	if len(contexts) > 0 {
		// Multi-context mode: build one context per --context flag, then print a comparison table.
		ctxList := make([]*cicontext.Context, 0, len(contexts))
		runs := make([]bool, 0, len(contexts))
		for _, spec := range contexts {
			cb, ct, cs, cv := parseContextSpec(spec)
			c := cicontext.New(cb, ct, cs, cv)
			if changesReliable {
				c.SetChangedFiles(allChanged)
			}
			ran := enrichContext(c, p)
			ctxList = append(ctxList, c)
			runs = append(runs, ran)
		}
		if *format == "text" {
			printContextTable(p, ctxList, contexts, runs)
		}
	} else {
		// Single-context mode: existing flow.
		ctx := cicontext.New(*branch, *tag, *source, vars)
		if changesReliable {
			ctx.SetChangedFiles(allChanged)
		}
		if !ctx.IsEmpty() {
			if !enrichContext(ctx, p) {
				fmt.Fprintf(os.Stderr, "%s: [warning] workflow:rules: pipeline would not start for this context\n", path)
			}
			// Build skipped set: jobs statically unreachable in this context are
			// excluded from needs:/dependencies: cross-checks to avoid false positives.
			skipped = make(map[string]bool)
			for name, job := range p.Jobs {
				if cicontext.EvalJob(job, ctx) == cicontext.JobSkipped {
					skipped[name] = true
				}
			}
			if len(skipped) == 0 {
				skipped = nil
			}
		}
		if *listVars {
			printVars(p, ctx)
		}
		// Context summary only makes sense in plain-text output; suppress it in
		// structured formats so stdout contains only the machine-readable payload.
		if !ctx.IsEmpty() && *format == "text" {
			printContext(p, ctx)
		}
	}

	findings := linter.Lint(p, skipped)
	findings = applyConfig(findings, glintCfg, p.Suppressions)

	// --no-warn: discard warnings before any output or exit-code calculation.
	if *noWarn {
		kept := findings[:0]
		for _, f := range findings {
			if f.Severity != linter.Warning {
				kept = append(kept, f)
			}
		}
		findings = kept
	}

	errCount, warnCount := countSeverities(findings)

	// In structured formats the summary line goes to stderr so stdout is clean.
	summaryOut := os.Stdout
	if *format != "text" {
		summaryOut = os.Stderr
	}

	switch *format {
	case "json":
		writeJSON(os.Stdout, findings, path)
	case "sarif":
		writeSARIF(os.Stdout, findings, path)
	case "junit":
		writeJUnit(os.Stdout, findings, path)
	case "github":
		writeGitHub(os.Stdout, findings)
	default: // "text"
		writeTextFindings(os.Stdout, findings)
	}

	if len(findings) == 0 {
		fmt.Fprintf(summaryOut, "OK: %s — no issues found (%d job(s), %d stage(s))\n", path, len(p.Jobs), len(p.Stages))
	} else {
		switch {
		case errCount > 0 && warnCount > 0:
			fmt.Fprintf(summaryOut, "%d finding(s): %d error(s), %d warning(s)\n", len(findings), errCount, warnCount)
		case errCount > 0:
			fmt.Fprintf(summaryOut, "%d finding(s): %d error(s)\n", len(findings), errCount)
		default:
			fmt.Fprintf(summaryOut, "%d finding(s): %d warning(s)\n", len(findings), warnCount)
		}
	}

	switch {
	case errCount > 0:
		exit(2)
	case warnCount > 0:
		exit(10)
	}
}

var knownGraphModes = map[string]bool{
	"tree": true, "includes": true, "pipeline": true, "all": true,
}

func cmdGraph(args []string) {
	// Optional mode word must come before any flags or the file path.
	mode := "default"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && knownGraphModes[args[0]] {
		mode = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("glint graph", flag.ExitOnError)
	token := fs.String("token", "", "GitLab personal access token (overrides GITLAB_TOKEN)")
	gitlabURL := fs.String("gitlab-url", "", "GitLab instance URL (overrides CI_SERVER_URL / GITLAB_URL)")
	cacheDir := fs.String("cache-dir", "", "directory to cache fetched remote includes (created if needed)")
	offline := fs.Bool("offline", false, "skip all network calls; serve only from --cache-dir")
	proxy := fs.String("proxy", "", "HTTP proxy URL for remote includes and GitLab API calls (e.g. http://proxy:8080)")
	out := fs.String("out", "glint-out", "output directory for rendered graph files (pipeline mode)")
	format := fs.String("format", "svg", "pipeline output format: svg, mermaid, or html")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, `Visualise the pipeline as a job tree and/or Mermaid graph.

Usage: glint graph [MODE] [OPTIONS] <PIPELINE>

Arguments:
  [MODE]      Graph mode; must appear before options [default: tree+includes]
              [possible values: tree, includes, pipeline, all]
  <PIPELINE>  Path to the .gitlab-ci.yml file

Options:
      --out <DIR>
          Output directory for rendered graph files.
          Used by the pipeline and all modes only. [default: glint-out]

      --format <FORMAT>
          Output format for pipeline mode: svg (default), mermaid, or html.
          svg:     write a GitLab CI-style SVG/PNG to --out (converted to PNG
                   when rsvg-convert, inkscape, or magick is available).
          mermaid: print a Mermaid flowchart to stdout (paste into mermaid.live).
          html:    write a self-contained HTML file with pan/zoom and a
                   job-detail sidebar to --out.
          [default: svg] [possible values: svg, mermaid, html]

      --token <TOKEN>
          GitLab personal access token. Used to fetch remote project: includes
          when building the include dependency graph.
          [env: GITLAB_TOKEN | CI_JOB_TOKEN | GITLAB_PRIVATE_TOKEN]

      --gitlab-url <URL>
          GitLab instance URL.
          [env: CI_SERVER_URL | GITLAB_URL] [default: https://gitlab.com]

      --cache-dir <DIR>
          Cache fetched remote templates (project: and component: includes) in
          DIR. The directory is created on first use. Subsequent runs read from
          cache first, avoiding repeated network calls.

      --offline
          Do not make any network calls. All remote includes must already be
          present in --cache-dir; missing entries emit a warning. Implies the
          default cache dir (~/.cache/glint) when --cache-dir is not set.

      --proxy <URL>
          HTTP proxy URL for remote includes and GitLab API calls
          (e.g. http://proxy:8080). Overrides system proxy env vars
          (HTTP_PROXY / HTTPS_PROXY). Also configurable via proxy: in
          .glint.yml.

      --branch <NAME>
          Simulate a branch push. Jobs in tree output are annotated with their
          evaluated state ([skipped] or [manual]; no tag means active).
          Populates: CI_COMMIT_BRANCH, CI_COMMIT_REF_NAME, CI_COMMIT_REF_SLUG,
          CI_PIPELINE_SOURCE=push.
          [default: main]

      --tag <NAME>
          Simulate a tag push. Populates: CI_COMMIT_TAG, CI_COMMIT_REF_NAME,
          CI_COMMIT_REF_SLUG, CI_PIPELINE_SOURCE=push. Clears CI_COMMIT_BRANCH.

      --source <EVENT>
          Override CI_PIPELINE_SOURCE.
          [default: push] [possible values: push, merge_request_event, schedule, web, api]

      --var <KEY=VALUE>
          Set or override a CI variable. Repeatable.

      --changes <PATH>
          Mark a file path as changed for rules:changes: evaluation. Repeatable.

      --changes-from <REF>
          Run "git diff --name-only <REF>" to determine changed files for
          rules:changes: evaluation.

      --no-skipped
          Omit jobs that would be skipped in the given context. Requires at
          least one context flag (--branch, --tag, --source, --var) or the
          implicit default context (branch=main). Skipped jobs are removed
          from the tree, SVG, HTML, and Mermaid output entirely.

      --list-vars
          Print all pipeline-level variables collected from the root file and
          every included file (sorted KEY=VALUE) to stderr, then continue
          normally. Useful for debugging variable resolution.

  -h, --help
          Print help

Note: when none of --branch, --tag, --source, or --var are given, glint
defaults to --branch main --source push so that rules:if: expressions are
always evaluated.

Examples:
  glint graph .gitlab-ci.yml
  glint graph tree .gitlab-ci.yml
  glint graph tree --branch develop .gitlab-ci.yml
  glint graph tree --tag v1.0.0 .gitlab-ci.yml
  glint graph tree --list-vars .gitlab-ci.yml
  glint graph tree --changes src/main.go .gitlab-ci.yml
  glint graph includes .gitlab-ci.yml > includes.mmd
  glint graph pipeline .gitlab-ci.yml
  glint graph pipeline --out /tmp/graphs .gitlab-ci.yml
  glint graph pipeline --format mermaid .gitlab-ci.yml
  glint graph pipeline --format html .gitlab-ci.yml
  glint graph tree --no-skipped --branch main .gitlab-ci.yml
  glint graph pipeline --no-skipped --branch develop .gitlab-ci.yml
  glint graph all .gitlab-ci.yml > includes.mmd
`)
	}
	branch := fs.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
	tag := fs.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
	source := fs.String("source", "", "set CI_PIPELINE_SOURCE")
	noSkipped := fs.Bool("no-skipped", false, "hide jobs that would be skipped in the given context (requires a context)")
	listVars := fs.Bool("list-vars", false, "print all collected pipeline variables to stderr, then continue")
	var vars multiFlag
	fs.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	var changesFiles multiFlag
	fs.Var(&changesFiles, "changes", "mark a file path as changed for rules:changes: evaluation; repeatable")
	changesFrom := fs.String("changes-from", "", "git ref to diff against for rules:changes: evaluation (e.g. HEAD~1, origin/main)")
	_ = fs.Parse(args)

	// Apply implicit defaults when no context flag is given at all.
	if *branch == "" && *tag == "" && *source == "" && len(vars) == 0 {
		*branch = "main"
		*source = "push"
	}

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

	resolver.ResolveIncludes(p, cfg, rootDir) //nolint:errcheck
	resolver.Resolve(p)                       //nolint:errcheck

	ctx := cicontext.New(*branch, *tag, *source, vars)
	// Wire up rules:changes: evaluation when file-change data is provided.
	if *changesFrom != "" || len(changesFiles) > 0 {
		var allChanged []string
		reliable := len(changesFiles) > 0
		if *changesFrom != "" {
			files, err := gitDiffFiles(*changesFrom)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: [warning] --changes-from: %v\n", path, err)
			} else {
				allChanged = append(allChanged, files...)
				reliable = true
			}
		}
		allChanged = append(allChanged, changesFiles...)
		if reliable {
			if allChanged == nil {
				allChanged = []string{}
			}
			ctx.SetChangedFiles(allChanged)
		}
	}
	if !ctx.IsEmpty() {
		enrichContext(ctx, p)
	}
	if *listVars {
		printVars(p, ctx)
	}

	// --no-skipped: remove jobs that evaluate to skipped in the current context
	// before handing the pipeline to any graph function.
	if *noSkipped && !ctx.IsEmpty() {
		for name, job := range p.Jobs {
			if !strings.HasPrefix(name, ".") && cicontext.EvalJob(job, ctx) == cicontext.JobSkipped {
				delete(p.Jobs, name)
			}
		}
	}

	switch mode {
	case "default":
		fmt.Print(graph.Tree(p, ctx))
		fmt.Println("---")
		fmt.Print(graph.Includes(path, p.Include, cfg))
	case "tree":
		fmt.Print(graph.Tree(p, ctx))
	case "includes":
		fmt.Print(graph.Includes(path, p.Include, cfg))
	case "pipeline":
		switch *format {
		case "mermaid":
			fmt.Print(graph.Pipeline(p))
		case "html":
			outPath, err := graph.RenderHTML(p, *out, ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
				exit(2)
				return
			}
			fmt.Println(outPath)
		default: // "svg"
			outPath, err := graph.RenderPipeline(p, *out, ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
				exit(2)
				return
			}
			fmt.Println(outPath)
		}
	case "all":
		fmt.Print(graph.Includes(path, p.Include, cfg))
		outPath, err := graph.RenderPipeline(p, *out, ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
			exit(2)
			return
		}
		fmt.Fprintln(os.Stderr, outPath)
	}
}

// printVars prints the collected variable namespaces to stderr:
//  1. Pipeline variables — declared in variables: blocks across the root file
//     and all included files (merged by ResolveIncludes).
//  2. Workflow-rule variables — union of variables: from every workflow:rules
//     entry; any one of them may be injected at runtime.
//  3. Effective context variables — only when ctx is non-empty; shows the
//     fully merged set visible to job rules:if: after enrichContext.
func printVars(p *model.Pipeline, ctx *cicontext.Context) {
	fmt.Fprintln(os.Stderr, "Pipeline variables (YAML, root + includes):")
	printVarMap(p.Variables)

	if p.Workflow != nil {
		union := map[string]any{}
		for _, rule := range p.Workflow.Rules {
			for k, v := range rule.Variables {
				union[k] = v
			}
		}
		if len(union) > 0 {
			fmt.Fprintln(os.Stderr, "Workflow-rule variables (union across all rules):")
			printVarMap(union)
		}
	}

	if !ctx.IsEmpty() {
		fmt.Fprintln(os.Stderr, "Effective context variables (after workflow + CLI flags):")
		keys := make([]string, 0, len(ctx.Vars))
		for k := range ctx.Vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "  %s=%s\n", k, ctx.Vars[k])
		}
	}
}

func printVarMap(m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		fmt.Fprintln(os.Stderr, "  (none)")
		return
	}
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %s=%s\n", k, varValueString(m[k]))
	}
}

func varValueString(v any) string {
	if s, ok := cicontext.ScalarString(v); ok {
		return s
	}
	return "(complex)"
}

// enrichContext injects pipeline-level variable defaults and then
// workflow-rule-generated variables into ctx before job evaluation.
// Injection respects pinned variables (--branch/--tag/--source/--var always win).
// Returns false when workflow:rules: would prevent the pipeline from starting.
func enrichContext(ctx *cicontext.Context, p *model.Pipeline) bool {
	// Pipeline variables: injected as defaults (lowest priority).
	for k, v := range cicontext.ExtractStringVars(p.Variables) {
		ctx.Inject(k, v)
	}
	// Workflow rules: evaluate to find which rule matches, then inject its variables.
	runs, ruleVars := cicontext.EvalWorkflow(p, ctx)
	for k, v := range ruleVars {
		ctx.Inject(k, v)
	}
	// Expand $VAR / ${VAR} references within variable values now that all
	// sources (pipeline, workflow rules, CLI) have been merged.
	ctx.ExpandVars()
	return runs
}

func printContext(p *model.Pipeline, ctx *cicontext.Context) {
	fmt.Printf("Context: %s\n\n", ctx.Summary())

	byState := map[cicontext.JobState][]string{}
	for name, job := range p.Jobs {
		if strings.HasPrefix(name, ".") {
			continue
		}
		state := cicontext.EvalJob(job, ctx)
		byState[state] = append(byState[state], name)
	}
	for _, jobs := range byState {
		sort.Strings(jobs)
	}

	printJobGroup("Active ", byState[cicontext.JobActive])
	printJobGroup("Manual ", byState[cicontext.JobManual])
	printJobGroup("Skipped", byState[cicontext.JobSkipped])
	fmt.Println()
}

func printJobGroup(label string, jobs []string) {
	if len(jobs) == 0 {
		return
	}
	fmt.Printf("%s (%d): %s\n", label, len(jobs), strings.Join(jobs, ", "))
}

// parseContextSpec parses "branch=main,DEPLOY_ENV=prod" into its parts.
// Known keys (branch, tag, source) are extracted; everything else goes into extraVars.
func parseContextSpec(spec string) (branch, tag, source string, extraVars []string) {
	for _, kv := range strings.Split(spec, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		key := parts[0]
		val := ""
		if len(parts) == 2 {
			val = parts[1]
		}
		switch strings.ToLower(key) {
		case "branch":
			branch = val
		case "tag":
			tag = val
		case "source":
			source = val
		default:
			extraVars = append(extraVars, kv)
		}
	}
	return
}

// sortedJobNames returns non-hidden job names ordered by stage position, then alphabetically.
func sortedJobNames(p *model.Pipeline) []string {
	stageIdx := make(map[string]int, len(p.Stages))
	for i, s := range p.Stages {
		stageIdx[s] = i
	}
	type entry struct {
		name  string
		stage int
	}
	entries := make([]entry, 0, len(p.Jobs))
	for name, job := range p.Jobs {
		if strings.HasPrefix(name, ".") {
			continue
		}
		idx, ok := stageIdx[job.Stage]
		if !ok {
			idx = len(p.Stages)
		}
		entries = append(entries, entry{name: name, stage: idx})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].stage != entries[j].stage {
			return entries[i].stage < entries[j].stage
		}
		return entries[i].name < entries[j].name
	})
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

// printContextTable prints a side-by-side comparison of job states across contexts.
func printContextTable(p *model.Pipeline, ctxs []*cicontext.Context, labels []string, runs []bool) {
	jobs := sortedJobNames(p)
	if len(jobs) == 0 {
		return
	}

	// Evaluate all jobs for all contexts up front so we can compute column widths.
	states := make([][]string, len(jobs))
	for r, name := range jobs {
		states[r] = make([]string, len(ctxs))
		for c, ctx := range ctxs {
			if !runs[c] {
				states[r][c] = "blocked"
			} else {
				switch cicontext.EvalJob(p.Jobs[name], ctx) {
				case cicontext.JobActive:
					states[r][c] = "active"
				case cicontext.JobManual:
					states[r][c] = "manual"
				default:
					states[r][c] = "skipped"
				}
			}
		}
	}

	// Compute column widths.
	jobCol := len("JOB")
	for _, name := range jobs {
		if len(name) > jobCol {
			jobCol = len(name)
		}
	}
	ctxCols := make([]int, len(labels))
	for i, lbl := range labels {
		ctxCols[i] = len(lbl)
	}
	for r := range jobs {
		for c, s := range states[r] {
			if len(s) > ctxCols[c] {
				ctxCols[c] = len(s)
			}
		}
	}

	// Print header.
	fmt.Println("Context comparison:")
	fmt.Println()
	fmt.Printf("%-*s", jobCol, "JOB")
	for i, lbl := range labels {
		fmt.Printf("  %-*s", ctxCols[i], lbl)
	}
	fmt.Println()
	fmt.Print(strings.Repeat("-", jobCol))
	for _, w := range ctxCols {
		fmt.Print("  " + strings.Repeat("-", w))
	}
	fmt.Println()

	// Print rows.
	for r, name := range jobs {
		fmt.Printf("%-*s", jobCol, name)
		for c, s := range states[r] {
			fmt.Printf("  %-*s", ctxCols[c], s)
		}
		fmt.Println()
	}
	fmt.Println()
}
