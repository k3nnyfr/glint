package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/graph"
	"git.k3nny.fr/glint/internal/linter"
	"git.k3nny.fr/glint/internal/model"
	"git.k3nny.fr/glint/internal/resolver"
)

const globalUsage = `glint: Lint and visualise GitLab CI pipelines locally.

Usage: glint [OPTIONS] <COMMAND>

Commands:
  check    Lint a pipeline file — exits 0 (clean) or 1 (errors found)
  graph    Visualise the pipeline as a job tree or Mermaid graph

Options:
  -h, --help  Print help

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
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, globalUsage)
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
	branch := fs.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
	tag := fs.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
	source := fs.String("source", "", "set CI_PIPELINE_SOURCE")
	var vars multiFlag
	fs.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Lint a GitLab CI pipeline file.

Resolves local includes and extends chains, then runs all lint rules.
Exits 0 when no errors are found, 1 when at least one error is reported.

Usage: glint check [OPTIONS] <PIPELINE>

Arguments:
  <PIPELINE>  Path to the .gitlab-ci.yml file to lint

Options:
      --token <TOKEN>
          GitLab personal access token. Required to fetch project: includes;
          component: includes are attempted unauthenticated.
          [env: GITLAB_TOKEN | CI_JOB_TOKEN | GITLAB_PRIVATE_TOKEN]

      --gitlab-url <URL>
          GitLab instance URL.
          [env: CI_SERVER_URL | GITLAB_URL] [default: https://gitlab.com]

      --branch <NAME>
          Simulate a branch push. Populates: CI_COMMIT_BRANCH,
          CI_COMMIT_REF_NAME, CI_COMMIT_REF_SLUG, CI_PIPELINE_SOURCE=push.

      --tag <NAME>
          Simulate a tag push. Populates: CI_COMMIT_TAG, CI_COMMIT_REF_NAME,
          CI_COMMIT_REF_SLUG, CI_PIPELINE_SOURCE=push. Clears CI_COMMIT_BRANCH.

      --source <EVENT>
          Override CI_PIPELINE_SOURCE.
          [possible values: push, merge_request_event, schedule, web, api]

      --var <KEY=VALUE>
          Set or override a CI variable. Takes precedence over --branch, --tag,
          and --source. Repeatable.

  -h, --help
          Print help

Examples:
  glint check .gitlab-ci.yml
  glint check --branch main .gitlab-ci.yml
  GITLAB_TOKEN=glpat-xxxx glint check .gitlab-ci.yml
  glint check --token glpat-xxxx --gitlab-url https://gitlab.example.com .gitlab-ci.yml
  glint check --branch main --var DEPLOY_ENV=production .gitlab-ci.yml
`)
	}
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	path := fs.Arg(0)

	cfg := fetcher.AutoConfig().WithOverrides(*gitlabURL, *token)

	p, err := model.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	rootDir := filepath.Dir(filepath.Clean(path))
	warnings, _ := resolver.ResolveIncludes(p, cfg, rootDir)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "[WARNING] include %s\n", w)
	}

	extWarnings, err := resolver.Resolve(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving extends: %v\n", err)
		os.Exit(2)
	}
	for _, w := range extWarnings {
		fmt.Fprintf(os.Stderr, "[WARNING] job %q extends unknown job %q; extends chain skipped\n", w.Job, w.Base)
	}

	ctx := cicontext.New(*branch, *tag, *source, vars)
	if !ctx.IsEmpty() {
		enrichContext(ctx, p)
		printContext(p, ctx)
	}

	findings := linter.Lint(p)
	hasErrors := false
	for _, f := range findings {
		fmt.Println(f)
		if f.Severity == linter.Error {
			hasErrors = true
		}
	}

	if len(findings) == 0 {
		fmt.Printf("OK: %s — no issues found (%d job(s), %d stage(s))\n", path, len(p.Jobs), len(p.Stages))
	} else {
		errCount := 0
		for _, f := range findings {
			if f.Severity == linter.Error {
				errCount++
			}
		}
		fmt.Printf("%d finding(s): %d error(s)\n", len(findings), errCount)
	}

	if hasErrors {
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
	out := fs.String("out", "glint-out", "output directory for Mermaid graph files (pipeline mode)")
	fs.Usage = func() {
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

      --tag <NAME>
          Simulate a tag push. Populates: CI_COMMIT_TAG, CI_COMMIT_REF_NAME,
          CI_COMMIT_REF_SLUG, CI_PIPELINE_SOURCE=push. Clears CI_COMMIT_BRANCH.

      --source <EVENT>
          Override CI_PIPELINE_SOURCE.
          [possible values: push, merge_request_event, schedule, web, api]

      --var <KEY=VALUE>
          Set or override a CI variable. Repeatable.

  -h, --help
          Print help

Examples:
  glint graph .gitlab-ci.yml
  glint graph tree .gitlab-ci.yml
  glint graph tree --branch main .gitlab-ci.yml
  glint graph includes .gitlab-ci.yml > includes.mmd
  glint graph pipeline .gitlab-ci.yml
  glint graph pipeline --out /tmp/graphs .gitlab-ci.yml
  glint graph all .gitlab-ci.yml > includes.mmd
`)
	}
	branch := fs.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
	tag := fs.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
	source := fs.String("source", "", "set CI_PIPELINE_SOURCE")
	var vars multiFlag
	fs.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	path := fs.Arg(0)

	cfg := fetcher.AutoConfig().WithOverrides(*gitlabURL, *token)

	p, err := model.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	rootDir := filepath.Dir(filepath.Clean(path))
	resolver.ResolveIncludes(p, cfg, rootDir) //nolint:errcheck
	resolver.Resolve(p)                        //nolint:errcheck

	ctx := cicontext.New(*branch, *tag, *source, vars)
	if !ctx.IsEmpty() {
		enrichContext(ctx, p)
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

// enrichContext injects pipeline-level variable defaults and then
// workflow-rule-generated variables into ctx before job evaluation.
// Injection respects pinned variables (--branch/--tag/--source/--var always win).
func enrichContext(ctx *cicontext.Context, p *model.Pipeline) {
	// Pipeline variables: injected as defaults (lowest priority).
	for k, v := range cicontext.ExtractStringVars(p.Variables) {
		ctx.Inject(k, v)
	}
	// Workflow rules: evaluate to find which rule matches, then inject its variables.
	runs, ruleVars := cicontext.EvalWorkflow(p, ctx)
	if !runs {
		fmt.Fprintln(os.Stderr, "[WARNING] workflow:rules: pipeline would not start for this context")
	}
	for k, v := range ruleVars {
		ctx.Inject(k, v)
	}
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
