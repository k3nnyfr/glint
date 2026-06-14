package main

import (
	"flag"
	"fmt"
	"os"
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

// defaultCacheDir returns the platform-default glint cache directory:
// $XDG_CACHE_HOME/glint or ~/.cache/glint.
func defaultCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "glint")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "glint")
	}
	return ""
}

const globalUsage = `glint: Lint and visualise GitLab CI pipelines locally.

Usage: glint [OPTIONS] <COMMAND>

Commands:
  check    Lint a pipeline file — exits 0 (clean) or 1 (errors found)
  graph    Visualise the pipeline as a job tree or Mermaid graph
  explain  Show description and fix for a lint rule (e.g. glint explain GL007)

Options:
  -h, --help     Print help
  -v, --version  Print version

For help with a specific command, see: ` + "`glint <command> --help`" + `.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, globalUsage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		cmdCheck(os.Args[2:])
	case "graph":
		cmdGraph(os.Args[2:])
	case "explain":
		cmdExplain(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, globalUsage)
	case "-v", "--version", "version":
		fmt.Printf("glint %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "glint: unknown command %q\n\n%s", os.Args[1], globalUsage)
		os.Exit(2)
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
	format := fs.String("format", "text", "output format: text, json, sarif, junit, github")
	branch := fs.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
	tag := fs.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
	source := fs.String("source", "", "set CI_PIPELINE_SOURCE")
	listVars := fs.Bool("list-vars", false, "print all collected pipeline variables (from root and included files) to stderr, then continue")
	var vars multiFlag
	fs.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "glint %s\n\n", version)
		fmt.Fprint(os.Stderr, `Lint a GitLab CI pipeline file.

Resolves local includes and extends chains, then runs all lint rules.
Exits 0 when no errors are found, 1 when at least one error is reported.

Usage: glint check [OPTIONS] <PIPELINE>

Arguments:
  <PIPELINE>  Path to the .gitlab-ci.yml file to lint

Options:
      --format <FORMAT>
          Output format for findings.
          [default: text] [possible values: text, json, sarif, junit, github]

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
`)
	}
	_ = fs.Parse(args)

	// Apply implicit defaults when no context flag is given at all.
	if *branch == "" && *tag == "" && *source == "" && len(vars) == 0 {
		*branch = "main"
		*source = "push"
	}

	validFormats := map[string]bool{
		"text": true, "json": true, "sarif": true, "junit": true, "github": true,
	}
	if !validFormats[*format] {
		fmt.Fprintf(os.Stderr, "glint: unknown format %q; valid: text, json, sarif, junit, github\n", *format)
		os.Exit(2)
	}

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
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

	cfg := fetcher.AutoConfig().WithOverrides(fetcherURL, fetcherToken, resolvedCacheDir, *offline)

	p, err := model.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
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
		os.Exit(2)
	}
	for _, w := range extWarnings {
		fmt.Fprintf(os.Stderr, "%s: [warning] job %q extends unknown job %q; extends chain skipped\n", path, w.Job, w.Base)
	}

	ctx := cicontext.New(*branch, *tag, *source, vars)
	if !ctx.IsEmpty() {
		if !enrichContext(ctx, p) {
			fmt.Fprintf(os.Stderr, "%s: [warning] workflow:rules: pipeline would not start for this context\n", path)
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

	findings := linter.Lint(p)
	findings = applyConfig(findings, glintCfg, p.Suppressions)
	errCount, _ := countSeverities(findings)

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
		for _, f := range findings {
			fmt.Println(f)
		}
	}

	if len(findings) == 0 {
		fmt.Fprintf(summaryOut, "OK: %s — no issues found (%d job(s), %d stage(s))\n", path, len(p.Jobs), len(p.Stages))
	} else {
		fmt.Fprintf(summaryOut, "%d finding(s): %d error(s)\n", len(findings), errCount)
	}

	if errCount > 0 {
		os.Exit(1)
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
	out := fs.String("out", "glint-out", "output directory for Mermaid graph files (pipeline mode)")
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

      --token <TOKEN>
          GitLab personal access token. Used to fetch remote project: includes
          when building the include dependency graph.
          [env: GITLAB_TOKEN | CI_JOB_TOKEN | GITLAB_PRIVATE_TOKEN]

      --gitlab-url <URL>
          GitLab instance URL.
          [env: CI_SERVER_URL | GITLAB_URL] [default: https://gitlab.com]

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
  glint graph includes .gitlab-ci.yml > includes.mmd
  glint graph pipeline .gitlab-ci.yml
  glint graph pipeline --out /tmp/graphs .gitlab-ci.yml
  glint graph all .gitlab-ci.yml > includes.mmd
`)
	}
	branch := fs.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
	tag := fs.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
	source := fs.String("source", "", "set CI_PIPELINE_SOURCE")
	listVars := fs.Bool("list-vars", false, "print all collected pipeline variables to stderr, then continue")
	var vars multiFlag
	fs.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	_ = fs.Parse(args)

	// Apply implicit defaults when no context flag is given at all.
	if *branch == "" && *tag == "" && *source == "" && len(vars) == 0 {
		*branch = "main"
		*source = "push"
	}

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	path := fs.Arg(0)

	resolvedCacheDir := *cacheDir
	if *offline && resolvedCacheDir == "" {
		resolvedCacheDir = defaultCacheDir()
	}

	cfg := fetcher.AutoConfig().WithOverrides(*gitlabURL, *token, resolvedCacheDir, *offline)

	p, err := model.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	rootDir := filepath.Dir(filepath.Clean(path))
	resolver.ResolveIncludes(p, cfg, rootDir) //nolint:errcheck
	resolver.Resolve(p)                       //nolint:errcheck

	ctx := cicontext.New(*branch, *tag, *source, vars)
	if !ctx.IsEmpty() {
		enrichContext(ctx, p)
	}
	if *listVars {
		printVars(p, ctx)
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
		outPath, err := graph.RenderPipeline(p, *out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
			os.Exit(2)
		}
		fmt.Println(outPath)
	case "all":
		fmt.Print(graph.Includes(path, p.Include, cfg))
		outPath, err := graph.RenderPipeline(p, *out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
			os.Exit(2)
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
