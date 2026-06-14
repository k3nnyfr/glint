package linter

import (
	"fmt"
	"regexp"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

// dnsLabelPattern matches a valid hostname label: starts and ends with
// alphanumeric, middle may contain hyphens and dots; max 63 chars per label.
var dnsLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]{0,61}[a-zA-Z0-9])?$`)

// timeoutPattern matches a GitLab CI duration string: one or more pairs of
// "<number> <unit>" where unit is a recognised time word or abbreviation.
var timeoutPattern = regexp.MustCompile(
	`(?i)^\s*(\d+\s*(weeks?|w|days?|d|hours?|h|minutes?|mins?|m|seconds?|secs?|s)\s*)+$`)

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
	findings = append(findings, checkDeadRules(name, job)...)
	findings = append(findings, checkImage(name, job)...)
	findings = append(findings, checkInherit(name, job)...)
	findings = append(findings, checkServices(name, job)...)
	findings = append(findings, checkRulesGlobs(name, job)...)
	findings = append(findings, checkTimeout(name, job)...)
	findings = append(findings, checkIDTokens(name, job)...)
	findings = append(findings, checkSecrets(name, job)...)
	findings = append(findings, checkPagesKeyword(name, job)...)
	findings = append(findings, checkCacheKeyFiles(name, job)...)
	return findings
}

func checkWhen(name string, job model.Job) []Finding {
	var findings []Finding
	if job.When != "" && !validJobWhen[job.When] {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleInvalidWhen,
			Job:      name,
			Message:  fmt.Sprintf("'when' has invalid value %q; valid: on_success, on_failure, always, manual, delayed, never", job.When),
		})
	}
	if job.When == "delayed" && job.StartIn == "" {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleDelayedNoStartIn,
			Job:      name,
			Message:  "'when: delayed' requires 'start_in' (e.g. 'start_in: 30 minutes')",
		})
	}
	if job.When != "delayed" && job.StartIn != "" {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleStartInNoDelayed,
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
				Rule:     RuleInvalidParallel,
				Job:      name,
				Message:  fmt.Sprintf("'parallel' must be between 2 and 200, got %d", v),
			}}
		}
	case map[string]any:
		if _, ok := v["matrix"]; !ok {
			return []Finding{{
				Severity: Error,
				Rule:     RuleInvalidParallel,
				Job:      name,
				Message:  "'parallel' map form must have a 'matrix' key",
			}}
		}
	default:
		return []Finding{{
			Severity: Error,
			Rule:     RuleInvalidParallel,
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
				Rule:     RuleInvalidRetry,
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
					Rule:     RuleInvalidRetry,
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
			Rule:     RuleInvalidRetry,
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
				Rule:     RuleInvalidRetryWhen,
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
				Rule:     RuleInvalidAllowFailure,
				Job:      name,
				Message:  "'allow_failure' map form must contain 'exit_codes'",
			}}
		}
	default:
		_ = v
		return []Finding{{
			Severity: Error,
			Rule:     RuleInvalidAllowFailure,
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
			Rule:     RuleInvalidInterruptible,
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
	if scriptNonEmpty(job.Script) {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleTriggerWithScript,
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
				Rule:     RuleInvalidTrigger,
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
			Rule:     RuleInvalidCoverage,
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
			Rule:     RuleInvalidRelease,
			Job:      name,
			Message:  "'release' must be a map",
		}}
	}
	tagName, exists := m["tag_name"]
	if !exists || tagName == "" || tagName == nil {
		return []Finding{{
			Severity: Error,
			Rule:     RuleInvalidRelease,
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
	envName := m["name"]
	_, hasURL := m["url"]
	if (envName == nil || envName == "") && hasURL {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleInvalidEnvironment,
			Job:      name,
			Message:  "'environment.url' requires 'environment.name' to be set",
		})
	}
	if action, ok := m["action"].(string); ok && !validEnvironmentAction[action] {
		findings = append(findings, Finding{
			Severity: Error,
			Rule:     RuleInvalidEnvironment,
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
			Rule:     RuleInvalidArtifacts,
			Job:      name,
			Message:  fmt.Sprintf("'artifacts.when' has invalid value %q; valid: on_success, on_failure, always", w),
		})
	}
	if _, hasExposeAs := m["expose_as"]; hasExposeAs {
		paths := m["paths"]
		if paths == nil {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidArtifacts,
				Job:      name,
				Message:  "'artifacts.expose_as' requires 'artifacts.paths'",
			})
		}
	}
	// Pages job should publish to public/. Skip when pages: keyword is set;
	// GL039 (checkPagesKeyword) handles that case with a more precise check.
	if name == "pages" && job.Pages == nil {
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
					Rule:     RulePagesPublic,
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
				Rule:     RuleInvalidCache,
				Job:      name,
				Message:  fmt.Sprintf("'cache.when' has invalid value %q; valid: on_success, on_failure, always", w),
			})
		}
		if p, ok := m["policy"].(string); ok && !validCachePolicy[p] {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidCache,
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
				Rule:     RuleInvalidRulesWhen,
				Job:      name,
				Message:  fmt.Sprintf("rules[%d].when has invalid value %q; valid: on_success, on_failure, always, manual, delayed, never", i, rule.When),
			})
		}
	}
	return findings
}

// checkDeadRules reports when every rule in a job's rules: block has an
// explicit when: never, making the job permanently unreachable. This is a
// provably-correct static claim: no matter which if: condition matches, the
// outcome is always "never"; and if no rule matches, the implicit fallback is
// also skip. No if: evaluation is required.
func checkDeadRules(name string, job model.Job) []Finding {
	if len(job.Rules) == 0 {
		return nil
	}
	for _, r := range job.Rules {
		if r.When != "never" {
			return nil
		}
	}
	return []Finding{{
		Severity: Warning,
		Rule:     RuleDeadRules,
		Job:      name,
		Message:  "rules: block can never activate; every rule has 'when: never' — job is permanently excluded from the pipeline",
	}}
}

func checkImage(name string, job model.Job) []Finding {
	if job.Image == nil {
		return nil
	}
	m, ok := job.Image.(map[string]any)
	if !ok {
		return nil // String form is valid.
	}
	imgName := m["name"]
	if imgName == nil || imgName == "" {
		return []Finding{{
			Severity: Error,
			Rule:     RuleInvalidImage,
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
				Rule:     RuleInvalidInherit,
				Job:      name,
				Message:  fmt.Sprintf("'inherit.%s' must be a boolean or a list of names", key),
			})
		}
	}
	return findings
}

// GL034: services: map form requires 'name'; 'alias' must be a valid DNS label.
func checkServices(name string, job model.Job) []Finding {
	if len(job.Services) == 0 {
		return nil
	}
	var findings []Finding
	for i, svc := range job.Services {
		m, ok := svc.(map[string]any)
		if !ok {
			continue // string form is valid
		}
		n := m["name"]
		if n == nil || n == "" {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidService,
				Job:      name,
				Message:  fmt.Sprintf("services[%d]: map form requires a 'name' key", i),
			})
		}
		if alias, ok := m["alias"].(string); ok && alias != "" {
			if !dnsLabelPattern.MatchString(alias) {
				findings = append(findings, Finding{
					Severity: Error,
					Rule:     RuleInvalidService,
					Job:      name,
					Message:  fmt.Sprintf("services[%d]: alias %q is not a valid DNS label (only letters, digits, hyphens, and dots; must start and end with alphanumeric)", i, alias),
				})
			}
		}
	}
	return findings
}

// GL035: rules:changes and rules:exists paths must not be absolute.
func checkRulesGlobs(name string, job model.Job) []Finding {
	var findings []Finding
	for i, rule := range job.Rules {
		findings = append(findings, checkAbsolutePaths(name, fmt.Sprintf("rules[%d].changes", i), rule.Changes)...)
		findings = append(findings, checkAbsolutePaths(name, fmt.Sprintf("rules[%d].exists", i), rule.Exists)...)
	}
	return findings
}

func checkAbsolutePaths(name, field string, val any) []Finding {
	var paths []string
	switch v := val.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				paths = append(paths, s)
			}
		}
	case map[string]any:
		if ps, ok := v["paths"].([]any); ok {
			for _, item := range ps {
				if s, ok := item.(string); ok {
					paths = append(paths, s)
				}
			}
		}
	}
	var findings []Finding
	for _, p := range paths {
		if strings.HasPrefix(p, "/") {
			findings = append(findings, Finding{
				Severity: Warning,
				Rule:     RuleAbsoluteGlobPath,
				Job:      name,
				Message:  fmt.Sprintf("%s: path %q is absolute; GitLab CI paths are relative to the repository root — absolute paths will never match", field, p),
			})
		}
	}
	return findings
}

// GL036: timeout: must be a valid GitLab CI duration string.
func checkTimeout(name string, job model.Job) []Finding {
	if job.Timeout == "" {
		return nil
	}
	if !timeoutPattern.MatchString(job.Timeout) {
		return []Finding{{
			Severity: Error,
			Rule:     RuleInvalidTimeout,
			Job:      name,
			Message:  fmt.Sprintf("'timeout' has invalid value %q; expected a duration like '1h 30m', '90 minutes', '2 hours'", job.Timeout),
		}}
	}
	return nil
}

// CheckDefaultTimeout validates the pipeline-level default: timeout: field.
// Exported so it can be called from linter.go.
func checkDefaultTimeout(timeout, file string) []Finding {
	if timeout == "" {
		return nil
	}
	if !timeoutPattern.MatchString(timeout) {
		return []Finding{{
			Severity: Error,
			Rule:     RuleInvalidTimeout,
			File:     file,
			Message:  fmt.Sprintf("'default.timeout' has invalid value %q; expected a duration like '1h 30m', '90 minutes', '2 hours'", timeout),
		}}
	}
	return nil
}

// GL037: id_tokens: each entry must have an 'aud' key.
func checkIDTokens(name string, job model.Job) []Finding {
	if job.IDTokens == nil {
		return nil
	}
	m, ok := job.IDTokens.(map[string]any)
	if !ok {
		return nil
	}
	var findings []Finding
	for tokenName, tokenVal := range m {
		tokenMap, ok := tokenVal.(map[string]any)
		if !ok {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidIDToken,
				Job:      name,
				Message:  fmt.Sprintf("id_tokens.%s: must be a map with an 'aud' key", tokenName),
			})
			continue
		}
		if _, hasAud := tokenMap["aud"]; !hasAud {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidIDToken,
				Job:      name,
				Message:  fmt.Sprintf("id_tokens.%s: missing required 'aud' key", tokenName),
			})
		}
	}
	return findings
}

// GL038: secrets: each entry must have a provider key (vault, gcp_secret_manager, azure_key_vault).
func checkSecrets(name string, job model.Job) []Finding {
	if job.Secrets == nil {
		return nil
	}
	m, ok := job.Secrets.(map[string]any)
	if !ok {
		return nil
	}
	validProviders := []string{"vault", "gcp_secret_manager", "azure_key_vault"}
	var findings []Finding
	for secretName, secretVal := range m {
		secretMap, ok := secretVal.(map[string]any)
		if !ok {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidSecret,
				Job:      name,
				Message:  fmt.Sprintf("secrets.%s: must be a map with a provider key (vault, gcp_secret_manager, or azure_key_vault)", secretName),
			})
			continue
		}
		hasProvider := false
		for _, provider := range validProviders {
			if _, ok := secretMap[provider]; ok {
				hasProvider = true
				break
			}
		}
		if !hasProvider {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidSecret,
				Job:      name,
				Message:  fmt.Sprintf("secrets.%s: missing provider key; must include one of: vault, gcp_secret_manager, azure_key_vault", secretName),
			})
		}
	}
	return findings
}

// GL039: a job with the pages: keyword must have the publish directory in artifacts.paths.
func checkPagesKeyword(name string, job model.Job) []Finding {
	if job.Pages == nil {
		return nil
	}
	publishDir := "public"
	if m, ok := job.Pages.(map[string]any); ok {
		if p, ok := m["publish"].(string); ok && p != "" {
			publishDir = p
		}
	}
	if job.Artifacts == nil {
		return []Finding{{
			Severity: Warning,
			Rule:     RulePagesPublish,
			Job:      name,
			Message:  fmt.Sprintf("job uses 'pages:' keyword with publish dir %q but has no 'artifacts:' block — GitLab Pages will not deploy", publishDir),
		}}
	}
	m, ok := job.Artifacts.(map[string]any)
	if !ok {
		return nil
	}
	paths, ok := m["paths"].([]any)
	if !ok || len(paths) == 0 {
		return []Finding{{
			Severity: Warning,
			Rule:     RulePagesPublish,
			Job:      name,
			Message:  fmt.Sprintf("job uses 'pages:' keyword with publish dir %q but 'artifacts.paths' is missing — GitLab Pages will not deploy", publishDir),
		}}
	}
	for _, p := range paths {
		if s, ok := p.(string); ok && (s == publishDir || strings.HasPrefix(s, publishDir+"/")) {
			return nil
		}
	}
	return []Finding{{
		Severity: Warning,
		Rule:     RulePagesPublish,
		Job:      name,
		Message:  fmt.Sprintf("'pages.publish' is %q but 'artifacts.paths' does not include it — GitLab Pages will not deploy", publishDir),
	}}
}

// GL041: cache.key.files must be a list of exact file paths, not glob patterns.
func checkCacheKeyFiles(name string, job model.Job) []Finding {
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
		keyVal, ok := m["key"]
		if !ok {
			continue
		}
		keyMap, ok := keyVal.(map[string]any)
		if !ok {
			continue
		}
		filesVal, ok := keyMap["files"]
		if !ok {
			continue
		}
		filesList, ok := filesVal.([]any)
		if !ok {
			findings = append(findings, Finding{
				Severity: Error,
				Rule:     RuleInvalidCacheKeyFiles,
				Job:      name,
				Message:  "'cache.key.files' must be a list of file paths",
			})
			continue
		}
		for _, item := range filesList {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.ContainsAny(s, "*?[") {
				findings = append(findings, Finding{
					Severity: Warning,
					Rule:     RuleInvalidCacheKeyFiles,
					Job:      name,
					Message:  fmt.Sprintf("cache.key.files: %q looks like a glob pattern; 'key.files' must be exact file paths, not globs", s),
				})
			}
		}
	}
	return findings
}
