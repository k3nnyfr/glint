package linter

import (
	"fmt"
	"regexp"
	"strings"

	"git.k3nny.fr/gitlab-sim/internal/model"
)

var validJobWhen = map[string]bool{
	"on_success": true,
	"on_failure": true,
	"always":     true,
	"manual":     true,
	"delayed":    true,
	"never":      true,
}

var validRuleWhen = map[string]bool{
	"on_success": true,
	"on_failure": true,
	"always":     true,
	"manual":     true,
	"delayed":    true,
	"never":      true,
}

var validWorkflowRuleWhen = map[string]bool{
	"always": true,
	"never":  true,
}

var validArtifactsWhen = map[string]bool{
	"on_success": true,
	"on_failure": true,
	"always":     true,
}

var validCacheWhen = map[string]bool{
	"on_success": true,
	"on_failure": true,
	"always":     true,
}

var validCachePolicy = map[string]bool{
	"pull":      true,
	"push":      true,
	"pull-push": true,
}

var validRetryWhen = map[string]bool{
	"always":                              true,
	"unknown_failure":                     true,
	"script_failure":                      true,
	"api_failure":                         true,
	"stuck_or_timeout_failure":            true,
	"runner_system_failure":               true,
	"runner_unsupported":                  true,
	"stale_schedule":                      true,
	"job_execution_timeout":               true,
	"archived_failure":                    true,
	"unmet_prerequisites":                 true,
	"scheduler_failure":                   true,
	"data_integrity_failure":              true,
	"forward_deployment_failure":          true,
	"insufficient_bridge_permissions":     true,
	"downstream_bridge_project_not_found": true,
	"invalid_bridge_trigger":              true,
	"upstream_bridge_project_not_found":   true,
	"protected_environment_failure":       true,
	"pipeline_loop_detected":              true,
	"excluded_by_rules":                   true,
}

var validEnvironmentAction = map[string]bool{
	"start":   true,
	"stop":    true,
	"prepare": true,
	"verify":  true,
	"access":  true,
}

func checkJobKeywords(name string, job model.Job) []Finding {
	var findings []Finding
	findings = append(findings, checkWhen(name, job)...)
	findings = append(findings, checkParallel(name, job)...)
	findings = append(findings, checkRetry(name, job)...)
	findings = append(findings, checkAllowFailure(name, job)...)
	findings = append(findings, checkInterruptible(name, job)...)
	findings = append(findings, checkTrigger(name, job)...)
	findings = append(findings, checkCoverage(name, job)...)
	findings = append(findings, checkRelease(name, job)...)
	findings = append(findings, checkEnvironment(name, job)...)
	findings = append(findings, checkArtifacts(name, job)...)
	findings = append(findings, checkCache(name, job)...)
	findings = append(findings, checkRules(name, job)...)
	findings = append(findings, checkImage(name, job)...)
	findings = append(findings, checkInherit(name, job)...)
	return findings
}

func checkWhen(name string, job model.Job) []Finding {
	var findings []Finding
	if job.When != "" && !validJobWhen[job.When] {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  fmt.Sprintf("'when' has invalid value %q; valid: on_success, on_failure, always, manual, delayed, never", job.When),
		})
	}
	if job.When == "delayed" && job.StartIn == "" {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "'when: delayed' requires 'start_in' (e.g. 'start_in: 30 minutes')",
		})
	}
	if job.When != "delayed" && job.StartIn != "" {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "'start_in' is only valid when 'when: delayed'",
		})
	}
	return findings
}

func checkParallel(name string, job model.Job) []Finding {
	if job.Parallel == nil {
		return nil
	}
	switch v := job.Parallel.(type) {
	case int:
		if v < 2 || v > 200 {
			return []Finding{{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("'parallel' must be between 2 and 200, got %d", v),
			}}
		}
	case map[string]any:
		if _, ok := v["matrix"]; !ok {
			return []Finding{{
				Severity: Error,
				Job:      name,
				Message:  "'parallel' map form must have a 'matrix' key",
			}}
		}
	default:
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'parallel' must be an integer (2–200) or a map with 'matrix'",
		}}
	}
	return nil
}

func checkRetry(name string, job model.Job) []Finding {
	if job.Retry == nil {
		return nil
	}
	switch v := job.Retry.(type) {
	case int:
		if v < 0 || v > 2 {
			return []Finding{{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("'retry' must be 0, 1, or 2; got %d", v),
			}}
		}
	case map[string]any:
		var findings []Finding
		if maxVal, ok := v["max"]; ok {
			if n, ok := maxVal.(int); ok && (n < 0 || n > 2) {
				findings = append(findings, Finding{
					Severity: Error,
					Job:      name,
					Message:  fmt.Sprintf("'retry.max' must be 0, 1, or 2; got %d", n),
				})
			}
		}
		if whenVal, ok := v["when"]; ok {
			findings = append(findings, validateRetryWhen(name, whenVal)...)
		}
		return findings
	default:
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'retry' must be an integer (0–2) or a map with 'max'/'when'",
		}}
	}
	return nil
}

func validateRetryWhen(name string, val any) []Finding {
	var findings []Finding
	check := func(s string) {
		if !validRetryWhen[s] {
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("'retry.when' has invalid value %q", s),
			})
		}
	}
	switch v := val.(type) {
	case string:
		check(v)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				check(s)
			}
		}
	}
	return findings
}

func checkAllowFailure(name string, job model.Job) []Finding {
	if job.Allow == nil {
		return nil
	}
	switch v := job.Allow.(type) {
	case bool:
		// valid
	case map[string]any:
		if _, ok := v["exit_codes"]; !ok {
			return []Finding{{
				Severity: Error,
				Job:      name,
				Message:  "'allow_failure' map form must contain 'exit_codes'",
			}}
		}
	default:
		_ = v
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'allow_failure' must be a boolean or a map with 'exit_codes'",
		}}
	}
	return nil
}

func checkInterruptible(name string, job model.Job) []Finding {
	if job.Interruptible == nil {
		return nil
	}
	if _, ok := job.Interruptible.(bool); !ok {
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'interruptible' must be a boolean",
		}}
	}
	return nil
}

func checkTrigger(name string, job model.Job) []Finding {
	if job.Trigger == nil {
		return nil
	}
	var findings []Finding
	if len(job.Script) > 0 {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "jobs with 'trigger' cannot use 'script'",
		})
	}
	if m, ok := job.Trigger.(map[string]any); ok {
		_, hasProject := m["project"]
		_, hasInclude := m["include"]
		if !hasProject && !hasInclude {
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  "'trigger' map must specify 'project' or 'include'",
			})
		}
	}
	return findings
}

var coveragePattern = regexp.MustCompile(`^/.*/$`)

func checkCoverage(name string, job model.Job) []Finding {
	if job.Coverage == "" {
		return nil
	}
	if !coveragePattern.MatchString(job.Coverage) {
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  fmt.Sprintf("'coverage' must be a regex pattern wrapped in '/' (e.g. '/\\d+\\.?\\d*%%/'), got %q", job.Coverage),
		}}
	}
	return nil
}

func checkRelease(name string, job model.Job) []Finding {
	if job.Release == nil {
		return nil
	}
	m, ok := job.Release.(map[string]any)
	if !ok {
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'release' must be a map",
		}}
	}
	tagName, exists := m["tag_name"]
	if !exists || tagName == "" || tagName == nil {
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'release' requires 'tag_name'",
		}}
	}
	return nil
}

func checkEnvironment(name string, job model.Job) []Finding {
	if job.Environment == nil {
		return nil
	}
	m, ok := job.Environment.(map[string]any)
	if !ok {
		// String form: just the environment name — valid.
		return nil
	}
	var findings []Finding
	envName, _ := m["name"]
	_, hasURL := m["url"]
	if (envName == nil || envName == "") && hasURL {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  "'environment.url' requires 'environment.name' to be set",
		})
	}
	if action, ok := m["action"].(string); ok && !validEnvironmentAction[action] {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  fmt.Sprintf("'environment.action' has invalid value %q; valid: start, stop, prepare, verify, access", action),
		})
	}
	return findings
}

func checkArtifacts(name string, job model.Job) []Finding {
	if job.Artifacts == nil {
		return nil
	}
	m, ok := job.Artifacts.(map[string]any)
	if !ok {
		return nil
	}
	var findings []Finding
	if w, ok := m["when"].(string); ok && !validArtifactsWhen[w] {
		findings = append(findings, Finding{
			Severity: Error,
			Job:      name,
			Message:  fmt.Sprintf("'artifacts.when' has invalid value %q; valid: on_success, on_failure, always", w),
		})
	}
	if _, hasExposeAs := m["expose_as"]; hasExposeAs {
		paths, _ := m["paths"]
		if paths == nil {
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  "'artifacts.expose_as' requires 'artifacts.paths'",
			})
		}
	}
	// Pages job should publish to public/
	if name == "pages" {
		if paths, ok := m["paths"].([]any); ok {
			found := false
			for _, p := range paths {
				if s, ok := p.(string); ok && (s == "public" || strings.HasPrefix(s, "public/")) {
					found = true
					break
				}
			}
			if !found {
				findings = append(findings, Finding{
					Severity: Warning,
					Job:      name,
					Message:  "the 'pages' job should include 'public' in 'artifacts.paths' for GitLab Pages to deploy",
				})
			}
		}
	}
	return findings
}

func checkCache(name string, job model.Job) []Finding {
	if job.Cache == nil {
		return nil
	}
	var maps []map[string]any
	switch v := job.Cache.(type) {
	case map[string]any:
		maps = []map[string]any{v}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				maps = append(maps, m)
			}
		}
	}
	var findings []Finding
	for _, m := range maps {
		if w, ok := m["when"].(string); ok && !validCacheWhen[w] {
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("'cache.when' has invalid value %q; valid: on_success, on_failure, always", w),
			})
		}
		if p, ok := m["policy"].(string); ok && !validCachePolicy[p] {
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("'cache.policy' has invalid value %q; valid: pull, push, pull-push", p),
			})
		}
	}
	return findings
}

func checkRules(name string, job model.Job) []Finding {
	var findings []Finding
	for i, rule := range job.Rules {
		if rule.When != "" && !validRuleWhen[rule.When] {
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("rules[%d].when has invalid value %q; valid: on_success, on_failure, always, manual, delayed, never", i, rule.When),
			})
		}
	}
	return findings
}

func checkImage(name string, job model.Job) []Finding {
	if job.Image == nil {
		return nil
	}
	m, ok := job.Image.(map[string]any)
	if !ok {
		return nil // String form is valid.
	}
	imgName, _ := m["name"]
	if imgName == nil || imgName == "" {
		return []Finding{{
			Severity: Error,
			Job:      name,
			Message:  "'image' map form requires a 'name' key",
		}}
	}
	return nil
}

func checkInherit(name string, job model.Job) []Finding {
	if job.Inherit == nil {
		return nil
	}
	m, ok := job.Inherit.(map[string]any)
	if !ok {
		return nil
	}
	var findings []Finding
	for _, key := range []string{"default", "variables"} {
		val, exists := m[key]
		if !exists {
			continue
		}
		switch val.(type) {
		case bool, []any:
			// valid: false/true or list of names
		default:
			findings = append(findings, Finding{
				Severity: Error,
				Job:      name,
				Message:  fmt.Sprintf("'inherit.%s' must be a boolean or a list of names", key),
			})
		}
	}
	return findings
}
