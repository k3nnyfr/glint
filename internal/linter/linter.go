package linter

import (
	"fmt"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

type Severity string

const (
	Error   Severity = "ERROR"
	Warning Severity = "WARNING"
)

type Finding struct {
	Severity Severity
	Job      string // empty for pipeline-level findings
	File     string // source file where the finding originates
	Line     int    // line number in File (0 = unknown)
	Message  string
}

func (f Finding) String() string {
	loc := ""
	if f.File != "" {
		if f.Line > 0 {
			loc = fmt.Sprintf(" (%s:%d)", f.File, f.Line)
		} else {
			loc = fmt.Sprintf(" (%s)", f.File)
		}
	}
	if f.Job != "" {
		return fmt.Sprintf("[%s] job %q%s: %s", f.Severity, f.Job, loc, f.Message)
	}
	if loc != "" {
		return fmt.Sprintf("[%s]%s: %s", f.Severity, loc, f.Message)
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
			File:     p.SourceFile,
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
				File:     p.SourceFile,
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
	// When the job has extends:, the script may come from a base that couldn't be
	// fetched (e.g. a remote include without a token), so downgrade to warning.
	hasScript := scriptNonEmpty(job.Script) || job.Run != nil
	if !isTemplate && !isTrigger && job.Pages == nil && !hasScript {
		sev := Error
		if job.Extends != nil {
			sev = Warning
		}
		findings = append(findings, Finding{
			Severity: sev,
			Job:      name,
			Message:  "missing required field 'script' (or 'run')",
		})
	}

	// stage must exist in declared stages (if stages were declared).
	// Skip when the stage value is a component input placeholder ($[[ inputs.xxx ]]):
	// those are resolved server-side and cannot be validated locally.
	if job.Stage != "" && !strings.Contains(job.Stage, "$[[") && len(stageSet) > 0 && !stageSet[job.Stage] {
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

	// Attach source location to every job-scoped finding collected above.
	for i := range findings {
		if findings[i].Job != "" && findings[i].File == "" {
			findings[i].File = job.File
			findings[i].Line = job.Line
		}
	}
	return findings
}

// scriptNonEmpty reports whether a script/before_script/after_script field
// (which may be a []any list or a plain string) is non-empty.
func scriptNonEmpty(v any) bool {
	switch s := v.(type) {
	case []any:
		return len(s) > 0
	case string:
		return s != ""
	}
	return false
}
