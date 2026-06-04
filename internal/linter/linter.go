package linter

import (
	"fmt"
	"strings"

	"git.k3nny.fr/gitlab-sim/internal/model"
)

type Severity string

const (
	Error   Severity = "ERROR"
	Warning Severity = "WARNING"
)

type Finding struct {
	Severity Severity
	Job      string // empty for pipeline-level findings
	Message  string
}

func (f Finding) String() string {
	if f.Job != "" {
		return fmt.Sprintf("[%s] job %q: %s", f.Severity, f.Job, f.Message)
	}
	return fmt.Sprintf("[%s] %s", f.Severity, f.Message)
}

// Lint runs all rules against p and returns findings sorted by job name.
func Lint(p *model.Pipeline) []Finding {
	var findings []Finding
	findings = append(findings, checkStages(p)...)
	findings = append(findings, checkWorkflow(p)...)
	findings = append(findings, checkJobs(p)...)
	findings = append(findings, checkNeeds(p)...)
	findings = append(findings, checkDependencies(p)...)
	return findings
}

func checkStages(p *model.Pipeline) []Finding {
	var findings []Finding
	if len(p.Stages) == 0 {
		findings = append(findings, Finding{
			Severity: Warning,
			Message:  "no stages defined; GitLab will use default stages (build, test, deploy)",
		})
	}
	return findings
}

func checkWorkflow(p *model.Pipeline) []Finding {
	if p.Workflow == nil {
		return nil
	}
	var findings []Finding
	for i, rule := range p.Workflow.Rules {
		if rule.When != "" && !validWorkflowRuleWhen[rule.When] {
			findings = append(findings, Finding{
				Severity: Error,
				Message:  fmt.Sprintf("workflow.rules[%d].when has invalid value %q; valid: always, never", i, rule.When),
			})
		}
	}
	return findings
}

func checkJobs(p *model.Pipeline) []Finding {
	var findings []Finding

	stageSet := make(map[string]bool, len(p.Stages))
	for _, s := range p.Stages {
		stageSet[s] = true
	}

	for name, job := range p.Jobs {
		findings = append(findings, checkJob(name, job, stageSet)...)
	}
	return findings
}

func checkJob(name string, job model.Job, stageSet map[string]bool) []Finding {
	var findings []Finding

	// Hidden jobs (names starting with ".") are reusable templates; skip most checks.
	isTemplate := strings.HasPrefix(name, ".")
	isTrigger := job.Trigger != nil

	// After extends resolution, a job with no script/run is an error.
	// Exceptions: trigger jobs, pages jobs (use pages: keyword), and template jobs.
	hasScript := len(job.Script) > 0 || job.Run != nil
	if !isTemplate && !isTrigger && job.Pages == nil && !hasScript {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "missing required field 'script' (or 'run')",
		})
	}

	// stage must exist in declared stages (if stages were declared)
	if job.Stage != "" && len(stageSet) > 0 && !stageSet[job.Stage] {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  fmt.Sprintf("stage %q is not defined in 'stages'", job.Stage),
		})
	}

	// only and rules are mutually exclusive
	if job.Only != nil && len(job.Rules) > 0 {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "'only' and 'rules' cannot be used together",
		})
	}

	// except and rules are mutually exclusive
	if job.Except != nil && len(job.Rules) > 0 {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "'except' and 'rules' cannot be used together",
		})
	}

	// warn about deprecated only/except
	if job.Only != nil || job.Except != nil {
		findings = append(findings, Finding{
			Severity: Warning,
			Job:      name,
			Message:  "'only'/'except' are deprecated; prefer 'rules'",
		})
	}

	findings = append(findings, checkJobKeywords(name, job)...)

	return findings
}
