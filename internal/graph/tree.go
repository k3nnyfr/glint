package graph

import (
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/model"
)

// Tree returns a tree-command-style text representation of pipeline jobs
// grouped by stage.
//
// Hidden template jobs (names starting with ".") are excluded.
// When ctx is non-nil and non-empty, each job is annotated with its evaluated
// state ([skipped] or [manual]); active jobs carry no annotation.
// Without a context, job-type annotations ([manual], [delayed], [trigger]) are
// shown instead.
func Tree(p *model.Pipeline, ctx *cicontext.Context) string {
	var sb strings.Builder

	// Collect visible jobs.
	var visible []string
	for name := range p.Jobs {
		if !strings.HasPrefix(name, ".") {
			visible = append(visible, name)
		}
	}
	sort.Strings(visible)

	// Group by stage.
	byStage := make(map[string][]string)
	for _, name := range visible {
		stage := p.Jobs[name].Stage
		if stage == "" {
			stage = "test"
		}
		byStage[stage] = append(byStage[stage], name)
	}

	// Build ordered stage list: declared first, then extras.
	seen := make(map[string]bool)
	var stages []string
	for _, s := range p.Stages {
		if len(byStage[s]) > 0 && !seen[s] {
			stages = append(stages, s)
			seen[s] = true
		}
	}
	for _, name := range visible {
		stage := p.Jobs[name].Stage
		if stage == "" {
			stage = "test"
		}
		if !seen[stage] {
			stages = append(stages, stage)
			seen[stage] = true
		}
	}

	sb.WriteString("pipeline\n")

	for si, stage := range stages {
		lastStage := si == len(stages)-1
		stagePrefix, childPrefix := branchChars(lastStage)

		sb.WriteString(stagePrefix + stage + "\n")

		jobs := byStage[stage]
		sort.Strings(jobs)
		for ji, name := range jobs {
			lastJob := ji == len(jobs)-1
			jobBranch, _ := branchChars(lastJob)
			sb.WriteString(childPrefix + jobBranch + jobLabel(p.Jobs[name], name, ctx) + "\n")
		}
	}

	return sb.String()
}

func branchChars(last bool) (branch, continuation string) {
	if last {
		return "└── ", "    "
	}
	return "├── ", "│   "
}

func jobLabel(job model.Job, name string, ctx *cicontext.Context) string {
	if ctx != nil && !ctx.IsEmpty() {
		switch cicontext.EvalJob(job, ctx) {
		case cicontext.JobSkipped:
			return name + " [skipped]"
		case cicontext.JobManual:
			return name + " [manual]"
		}
		return name
	}
	// No context: annotate by job type.
	var tags []string
	if job.When == "manual" {
		tags = append(tags, "manual")
	}
	if job.When == "delayed" {
		tags = append(tags, "delayed")
	}
	if job.Trigger != nil {
		tags = append(tags, "trigger")
	}
	if len(tags) == 0 {
		return name
	}
	return name + " [" + strings.Join(tags, ", ") + "]"
}
