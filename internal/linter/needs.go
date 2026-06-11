package linter

import (
	"fmt"

	"git.k3nny.fr/glint/internal/model"
)

func checkNeeds(p *model.Pipeline) []Finding {
	var findings []Finding

	// Build a stage-index map for ordering checks.
	stageIndex := make(map[string]int, len(p.Stages))
	for i, s := range p.Stages {
		stageIndex[s] = i
	}

	// needsGraph maps each job to the list of jobs it depends on.
	// Used for cycle detection after individual checks.
	needsGraph := make(map[string][]string)

	for name, job := range p.Jobs {
		if len(job.Needs) == 0 {
			continue
		}

		neededNames := parseNeedJobNames(job.Needs)
		needsGraph[name] = neededNames

		jobStageIdx, jobHasStage := stageIndex[job.Stage]

		for _, needed := range neededNames {
			neededJob, exists := p.Jobs[needed]
			if !exists {
				findings = append(findings, Finding{
					Severity: Error,
					Job:      name,
					File:     job.File,
					Line:     job.Line,
					Message:  fmt.Sprintf("needs unknown job %q", needed),
				})
				continue
			}

			// A job cannot need a job in a later stage.
			if len(p.Stages) > 0 && jobHasStage && neededJob.Stage != "" {
				neededStageIdx, neededHasStage := stageIndex[neededJob.Stage]
				if neededHasStage && neededStageIdx > jobStageIdx {
					findings = append(findings, Finding{
						Severity: Error,
						Job:      name,
						File:     job.File,
						Line:     job.Line,
						Message: fmt.Sprintf(
							"needs %q which is in a later stage (%q after %q)",
							needed, neededJob.Stage, job.Stage,
						),
					})
				}
			}
		}
	}

	findings = append(findings, detectNeedsCycles(needsGraph, p.Jobs)...)
	return findings
}

// parseNeedJobNames extracts job names from a needs: list.
// Each element is either a plain string or a map with a "job" key.
// Cross-pipeline needs (maps with a "pipeline" key) are skipped.
func parseNeedJobNames(needs []any) []string {
	var names []string
	for _, n := range needs {
		switch v := n.(type) {
		case string:
			names = append(names, v)
		case map[string]any:
			if _, crossPipeline := v["pipeline"]; crossPipeline {
				continue
			}
			if job, ok := v["job"].(string); ok {
				names = append(names, job)
			}
		}
	}
	return names
}

func detectNeedsCycles(graph map[string][]string, jobs map[string]model.Job) []Finding {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int, len(graph))
	var findings []Finding
	reported := make(map[string]bool)

	var visit func(name string, path []string)
	visit = func(name string, path []string) {
		switch state[name] {
		case visited:
			return
		case visiting:
			if !reported[name] {
				reported[name] = true
				j := jobs[name]
				findings = append(findings, Finding{
					Severity: Error,
					Job:      name,
					File:     j.File,
					Line:     j.Line,
					Message:  fmt.Sprintf("circular dependency in needs: %v → %s", path, name),
				})
			}
			return
		}
		state[name] = visiting
		for _, dep := range graph[name] {
			visit(dep, append(path, name))
		}
		state[name] = visited
	}

	for name := range graph {
		visit(name, nil)
	}
	return findings
}
