package linter

import (
	"fmt"

	"git.k3nny.fr/glint/internal/model"
)

// needEntry is a parsed element from a job's needs: list.
type needEntry struct {
	job      string
	optional bool // true when the needs entry carries optional: true
}

func checkNeeds(p *model.Pipeline, skipped map[string]bool) []Finding {
	var findings []Finding

	// Build a stage-index map for ordering checks.
	stageIndex := make(map[string]int, len(p.Stages))
	for i, s := range p.Stages {
		stageIndex[s] = i
	}

	// needsGraph maps each job to the jobs it depends on (existing jobs only).
	// Used for cycle detection after individual checks.
	needsGraph := make(map[string][]string)

	for name, job := range p.Jobs {
		if skipped[name] {
			continue
		}
		if len(job.Needs) == 0 {
			continue
		}

		entries := parseNeedEntries(job.Needs)
		jobStageIdx, jobHasStage := stageIndex[job.Stage]

		for _, entry := range entries {
			neededJob, exists := p.Jobs[entry.job]
			if !exists {
				// optional: true means GitLab CI will silently skip the
				// dependency when the job is absent (e.g. from a conditional
				// include). Downgrade to warning so users are informed without
				// failing the lint.
				sev := Error
				if entry.optional {
					sev = Warning
				}
				findings = append(findings, Finding{
					Severity: sev,
					Rule:     RuleNeedsUnknown,
					Job:      name,
					File:     job.File,
					Line:     job.Line,
					Column:   job.Column,
					Message:  fmt.Sprintf("needs unknown job %q", entry.job),
				})
				continue
			}

			// Add to the cycle-detection graph only when the dep exists.
			needsGraph[name] = append(needsGraph[name], entry.job)

			// A job cannot need a job in a later stage.
			if len(p.Stages) > 0 && jobHasStage && neededJob.Stage != "" {
				neededStageIdx, neededHasStage := stageIndex[neededJob.Stage]
				if neededHasStage && neededStageIdx > jobStageIdx {
					findings = append(findings, Finding{
						Severity: Error,
						Rule:     RuleNeedsStageOrder,
						Job:      name,
						File:     job.File,
						Line:     job.Line,
						Column:   job.Column,
						Message: fmt.Sprintf(
							"needs %q which is in a later stage (%q after %q)",
							entry.job, neededJob.Stage, job.Stage,
						),
					})
				}
			}
		}
	}

	findings = append(findings, detectNeedsCycles(needsGraph, p.Jobs)...)
	return findings
}

// checkRulesNeeds validates rules:needs: entries across all jobs. Each entry
// in a rule's needs: list must reference a job that exists in the pipeline.
// Cross-pipeline needs (maps with a "pipeline" key) are ignored. Skipped jobs
// are excluded from checking (same semantics as top-level needs: via GL027).
func checkRulesNeeds(p *model.Pipeline, skipped map[string]bool) []Finding {
	var findings []Finding
	for name, job := range p.Jobs {
		if skipped[name] {
			continue
		}
		for i, rule := range job.Rules {
			if len(rule.Needs) == 0 {
				continue
			}
			for _, entry := range parseNeedEntries(rule.Needs) {
				if _, exists := p.Jobs[entry.job]; !exists {
					sev := Error
					if entry.optional {
						sev = Warning
					}
					findings = append(findings, Finding{
						Severity: sev,
						Rule:     RuleRulesNeedsUnknown,
						Job:      name,
						File:     job.File,
						Line:     job.Line,
						Column:   job.Column,
						Message: fmt.Sprintf(
							"rules[%d].needs: references unknown job %q",
							i, entry.job,
						),
					})
				}
			}
		}
	}
	return findings
}

// parseNeedEntries extracts needs entries from a needs: list, preserving the
// optional flag. Each element is a plain string (job name) or a map with a
// "job" key. Cross-pipeline needs (maps with a "pipeline" key) are skipped.
func parseNeedEntries(needs []any) []needEntry {
	var entries []needEntry
	for _, n := range needs {
		switch v := n.(type) {
		case string:
			entries = append(entries, needEntry{job: v})
		case map[string]any:
			if _, crossPipeline := v["pipeline"]; crossPipeline {
				continue
			}
			if job, ok := v["job"].(string); ok {
				optional, _ := v["optional"].(bool)
				entries = append(entries, needEntry{job: job, optional: optional})
			}
		}
	}
	return entries
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
					Rule:     RuleNeedsCycle,
					Job:      name,
					File:     j.File,
					Line:     j.Line,
					Column:   j.Column,
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
