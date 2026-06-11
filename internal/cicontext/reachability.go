package cicontext

import (
	"regexp"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

// JobState describes whether a job would be included in a pipeline run for the
// given context.
type JobState int

const (
	JobActive  JobState = iota // job will run (on_success, always, delayed, …)
	JobManual                  // job is gated behind a manual trigger
	JobSkipped                 // job is excluded from the pipeline (when:never or no rule matched)
)

func (s JobState) String() string {
	switch s {
	case JobActive:
		return "active"
	case JobManual:
		return "manual"
	default:
		return "skipped"
	}
}

// EvalWorkflow evaluates the pipeline's workflow:rules block against ctx.
// Returns (runs, ruleVars):
//   - runs=false means the pipeline would not start for this context.
//   - ruleVars holds any variables: defined on the matching rule; inject these
//     into the context so job rules can reference them.
//
// Returns (true, nil) when ctx is empty, when there is no workflow block, or
// when no rules are configured.
func EvalWorkflow(p *model.Pipeline, ctx *Context) (bool, map[string]string) {
	if ctx.IsEmpty() || p.Workflow == nil || len(p.Workflow.Rules) == 0 {
		return true, nil
	}
	vars := ctx.Get
	for _, rule := range p.Workflow.Rules {
		if !ruleIfMatches(rule.If, vars) {
			continue
		}
		when := rule.When
		if when == "" {
			when = "always"
		}
		return when != "never", ExtractStringVars(rule.Variables)
	}
	return false, nil // no rule matched → pipeline does not run
}

// EvalJob returns the effective JobState for job in the given context.
// When ctx is empty every job is treated as Active (preserves existing
// behaviour when no context flags are given).
func EvalJob(job model.Job, ctx *Context) JobState {
	if ctx.IsEmpty() {
		return JobActive
	}

	vars := ctx.Get

	// rules: takes priority over only/except.
	if len(job.Rules) > 0 {
		for _, rule := range job.Rules {
			if !ruleIfMatches(rule.If, vars) {
				continue
			}
			return whenToState(rule.When)
		}
		return JobSkipped // no rule matched → job is excluded
	}

	// Legacy only:/except: filtering.
	if job.Only != nil || job.Except != nil {
		if !onlyMatches(job.Only, ctx) || exceptMatches(job.Except, ctx) {
			return JobSkipped
		}
	}

	return whenToState(job.When)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func ruleIfMatches(ifExpr string, vars func(string) string) bool {
	if ifExpr == "" {
		return true // no if: condition → rule always matches
	}
	return EvalIf(ifExpr, vars)
}

func whenToState(when string) JobState {
	switch when {
	case "never":
		return JobSkipped
	case "manual":
		return JobManual
	default:
		// on_success, always, delayed, "" (default) → active
		return JobActive
	}
}

// onlyMatches returns true when the job's only: filter is satisfied by ctx.
func onlyMatches(only any, ctx *Context) bool {
	if only == nil {
		return true // no only → runs everywhere
	}
	refs := extractRefList(only)
	if refs == nil {
		return true // unrecognised form → permissive
	}
	return refListMatches(refs, ctx)
}

// exceptMatches returns true when the job's except: filter excludes the job.
func exceptMatches(except any, ctx *Context) bool {
	if except == nil {
		return false // no except → nothing excluded
	}
	refs := extractRefList(except)
	if refs == nil {
		return false // unrecognised form → don't exclude
	}
	return refListMatches(refs, ctx)
}

// extractRefList normalises the only:/except: value into a flat []string.
// Returns nil for forms that cannot be statically evaluated.
func extractRefList(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var refs []string
		for _, item := range x {
			if s, ok := item.(string); ok {
				refs = append(refs, s)
			}
		}
		return refs
	case map[string]any:
		// Map form may carry { refs: [...], variables: [...], kubernetes: ... }.
		// We only evaluate the refs list; skip anything more complex.
		if raw, ok := x["refs"]; ok {
			return extractRefList(raw)
		}
		return nil
	}
	return nil
}

// refListMatches returns true when at least one entry in refs matches ctx.
func refListMatches(refs []string, ctx *Context) bool {
	branch := ctx.Get("CI_COMMIT_BRANCH")
	tag := ctx.Get("CI_COMMIT_TAG")
	source := ctx.Get("CI_PIPELINE_SOURCE")

	for _, ref := range refs {
		switch ref {
		case "branches":
			if branch != "" {
				return true
			}
		case "tags":
			if tag != "" {
				return true
			}
		case "merge_requests":
			if source == "merge_request_event" {
				return true
			}
		case "schedules":
			if source == "schedule" {
				return true
			}
		case "pipelines":
			if source == "pipeline" {
				return true
			}
		case "pushes":
			if source == "push" {
				return true
			}
		case "web":
			if source == "web" {
				return true
			}
		case "api":
			if source == "api" {
				return true
			}
		default:
			// Branch name glob or /regex/ pattern.
			if refPatternMatches(ref, branch) || refPatternMatches(ref, tag) {
				return true
			}
		}
	}
	return false
}

func refPatternMatches(pattern, ref string) bool {
	if ref == "" {
		return false
	}
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		re, err := regexp.Compile(pattern[1 : len(pattern)-1])
		if err != nil {
			return false
		}
		return re.MatchString(ref)
	}
	return globMatches(pattern, ref)
}

// globMatches supports the simple '*' wildcard used in GitLab only/except refs.
func globMatches(pattern, s string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	// Recursive match: consume pattern character by character.
	return matchGlob(pattern, s)
}

func matchGlob(pattern, s string) bool {
	for len(pattern) > 0 {
		if pattern[0] == '*' {
			pattern = pattern[1:]
			if len(pattern) == 0 {
				return true // trailing '*' matches everything
			}
			// Try matching the rest of the pattern at every position in s.
			for i := range s {
				if matchGlob(pattern, s[i:]) {
					return true
				}
			}
			return false
		}
		if len(s) == 0 || pattern[0] != s[0] {
			return false
		}
		pattern = pattern[1:]
		s = s[1:]
	}
	return len(s) == 0
}
