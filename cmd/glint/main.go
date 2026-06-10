package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/graph"
	"git.k3nny.fr/glint/internal/linter"
	"git.k3nny.fr/glint/internal/model"
	"git.k3nny.fr/glint/internal/resolver"
)

// multiFlag allows a flag to be specified multiple times.
type multiFlag []string

func (f *multiFlag) String() string { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func main() {
	var (
		token     = flag.String("token", "", "GitLab personal access token (overrides GITLAB_TOKEN)")
		gitlabURL = flag.String("gitlab-url", "", "GitLab instance URL (overrides CI_SERVER_URL / GITLAB_URL)")
		graphMode = flag.String("graph", "", "graph mode: includes | pipeline | all")
		graphOut  = flag.String("graph-out", "glint-out", "output directory for pipeline graph files")
		branch    = flag.String("branch", "", "simulate a branch push (sets CI_COMMIT_BRANCH, …)")
		tag       = flag.String("tag", "", "simulate a tag push (sets CI_COMMIT_TAG, …)")
		source    = flag.String("source", "", "set CI_PIPELINE_SOURCE")
		vars      multiFlag
	)
	flag.Var(&vars, "var", "set a CI variable as KEY=VALUE; repeatable")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: glint [options] <pipeline.yml>\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := flag.Arg(0)

	cfg := fetcher.AutoConfig().WithOverrides(*gitlabURL, *token)

	p, err := model.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	warnings := resolver.ResolveIncludes(p, cfg)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "[WARNING] include %s\n", w)
	}

	if err := resolver.Resolve(p); err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving extends: %v\n", err)
		os.Exit(2)
	}

	if *graphMode != "" {
		runGraph(p, path, *graphMode, *graphOut)
		return
	}

	ctx := cicontext.New(*branch, *tag, *source, vars)
	if !ctx.IsEmpty() {
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

	jobCount := len(p.Jobs)
	stageCount := len(p.Stages)

	if len(findings) == 0 {
		fmt.Printf("OK: %s — no issues found (%d job(s), %d stage(s))\n", path, jobCount, stageCount)
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

func runGraph(p *model.Pipeline, path, mode, outDir string) {
	switch mode {
	case "includes":
		fmt.Print(graph.Includes(path, p.Include))
	case "pipeline":
		outPath, err := graph.RenderPipeline(p, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
			os.Exit(2)
		}
		fmt.Println(outPath)
	case "all":
		fmt.Print(graph.Includes(path, p.Include))
		outPath, err := graph.RenderPipeline(p, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: rendering pipeline graph: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, outPath)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown --graph value %q; use: includes, pipeline, all\n", mode)
		os.Exit(2)
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
