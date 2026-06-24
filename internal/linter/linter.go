package linter

import (
	"cmp"
	"fmt"
	"slices"
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
	Rule     string // stable rule ID, e.g. "GL003" — see rules.go
	Job      string // empty for pipeline-level findings
	File     string // source file where the finding originates
	Line     int    // line number in File (0 = unknown)
	Message  string
}

func (f Finding) String() string {
	var loc string
	if f.File != "" {
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d: ", f.File, f.Line)
		} else {
			loc = fmt.Sprintf("%s: ", f.File)
		}
	}

	rule := ""
	if f.Rule != "" {
		rule = f.Rule + " "
	}

	sev := "[" + strings.ToLower(string(f.Severity)) + "]"

	msg := f.Message
	if f.Job != "" {
		msg = fmt.Sprintf("job %q: %s", f.Job, f.Message)
	}

	return fmt.Sprintf("%s%s%s %s", loc, rule, sev, msg)
}

// Lint runs all rules against p and returns findings sorted by (File, Line, Rule).
// Findings with no File (pipeline-level) sort before file-scoped ones.
// skipped is an optional set of job names to exclude from cross-job checks
// (needs:, dependencies:); pass nil to check all jobs.
func Lint(p *model.Pipeline, skipped map[string]bool) []Finding {
	var findings []Finding
	findings = append(findings, checkStages(p)...)
	findings = append(findings, checkDuplicateStages(p)...)
	findings = append(findings, checkDefault(p)...)
	findings = append(findings, checkWorkflow(p)...)
	findings = append(findings, checkJobs(p)...)
	findings = append(findings, checkNeeds(p, skipped)...)
	findings = append(findings, checkRulesNeeds(p, skipped)...)
	findings = append(findings, checkDependencies(p, skipped)...)
	findings = append(findings, checkVariableRefs(p)...)
	findings = append(findings, checkRulesIfReachability(p)...)
	findings = append(findings, checkInheritCompleteness(p)...)
	slices.SortStableFunc(findings, func(a, b Finding) int {
		if c := cmp.Compare(a.File, b.File); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Line, b.Line); c != 0 {
			return c
		}
		return cmp.Compare(a.Rule, b.Rule)
	})
	return findings
}

func checkStages(p *model.Pipeline) []Finding {
	var findings []Finding
	if len(p.Stages) == 0 {
		findings = append(findings, Finding{
			Severity: Warning,
			Rule:     RuleNoStages,
			File:     p.SourceFile,
			Message:  "no stages defined; GitLab will use default stages (build, test, deploy)",
		})
	}
	return findings
}

// GL040: warn when a stage name appears more than once in stages:.
func checkDuplicateStages(p *model.Pipeline) []Finding {
	seen := make(map[string]bool, len(p.Stages))
	var findings []Finding
	for _, s := range p.Stages {
		if seen[s] {
			findings = append(findings, Finding{
				Severity: Warning,
				Rule:     RuleDuplicateStage,
				File:     p.SourceFile,
				Message:  fmt.Sprintf("stage %q appears more than once in 'stages'; GitLab silently merges duplicate stage entries", s),
			})
		}
		seen[s] = true
	}
	return findings
}

// checkDefault validates the pipeline-level default: block.
func checkDefault(p *model.Pipeline) []Finding {
	if p.Default == nil {
		return nil
	}
	return checkDefaultTimeout(p.Default.Timeout, p.SourceFile)
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
				Rule:     RuleWorkflowWhen,
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
			Rule:     RuleMissingScript,
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
			Rule:     RuleUnknownStage,
			Job:      name,
			Message:  fmt.Sprintf("stage %q is not defined in 'stages'", job.Stage),
		})
	}

	// only and rules are mutually exclusive
	if job.Only != nil && len(job.Rules) > 0 {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleOnlyRulesConflict,
			Job:      name,
			Message:  "'only' and 'rules' cannot be used together",
		})
	}

	// except and rules are mutually exclusive
	if job.Except != nil && len(job.Rules) > 0 {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleExceptRulesConflict,
			Job:      name,
			Message:  "'except' and 'rules' cannot be used together",
		})
	}

	// warn about deprecated only/except
	if job.Only != nil || job.Except != nil {
		findings = append(findings, Finding{
			Severity: Warning,
			Rule:     RuleDeprecatedOnly,
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
