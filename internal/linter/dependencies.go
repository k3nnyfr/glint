package linter

import (
	"fmt"

	"git.k3nny.fr/glint/internal/model"
)

func checkDependencies(p *model.Pipeline) []Finding {
	stageIndex := make(map[string]int, len(p.Stages))
	for i, s := range p.Stages {
		stageIndex[s] = i
	}

	var findings []Finding
	for name, job := range p.Jobs {
		if len(job.Dependencies) == 0 {
			continue
		}
		jobStageIdx, jobHasStage := stageIndex[job.Stage]
		for _, dep := range job.Dependencies {
			depJob, exists := p.Jobs[dep]
			if !exists {
				findings = append(findings, Finding{
					Severity: Error,
					Job:      name,
					File:     job.File,
					Line:     job.Line,
					Message:  fmt.Sprintf("'dependencies' references unknown job %q", dep),
				})
				continue
			}
			if len(p.Stages) > 0 && jobHasStage && depJob.Stage != "" {
				depIdx, depHasStage := stageIndex[depJob.Stage]
				if depHasStage && depIdx >= jobStageIdx {
					findings = append(findings, Finding{
						Severity: Error,
						Job:      name,
						File:     job.File,
						Line:     job.Line,
						Message:  fmt.Sprintf("'dependencies' job %q must be in an earlier stage (in %q, current job is in %q)", dep, depJob.Stage, job.Stage),
					})
				}
			}
		}
	}
	return findings
}
