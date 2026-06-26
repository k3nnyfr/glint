package linter

import (
	"fmt"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

// predefinedVarPrefixes lists GitLab-maintained variable namespaces that are
// always available without an explicit declaration in variables: blocks.
var predefinedVarPrefixes = []string{
	"CI_",      // most predefined CI variables (CI_COMMIT_BRANCH, CI_JOB_ID, …)
	"GITLAB_",  // user/project metadata (GITLAB_USER_ID, GITLAB_FEATURES, …)
	"FF_",      // GitLab feature flags
	"RUNNER_",  // runner-level variables
	"TRIGGER_", // trigger token variables passed from upstream pipelines
	"CHAT_",    // ChatOps variables
}

func isPredefinedVar(name string) bool {
	for _, prefix := range predefinedVarPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// extractIfVars returns every variable name referenced in a rules:if: expression.
// String literals are skipped so that dollar signs inside quoted values are
// not mistaken for variable references.
func extractIfVars(expr string) []string {
	var names []string
	i := 0
	for i < len(expr) {
		switch expr[i] {
		case '"', '\'':
			quote := expr[i]
			i++
			for i < len(expr) && expr[i] != quote {
				if expr[i] == '\\' {
					i++ // skip escaped character
				}
				i++
			}
			if i < len(expr) {
				i++ // consume closing quote
			}
		case '$':
			i++ // consume '$'
			if i < len(expr) && expr[i] == '{' {
				i++ // consume '{'
				start := i
				for i < len(expr) && isVarNameByte(expr[i]) {
					i++
				}
				if i < len(expr) && expr[i] == '}' && i > start {
					names = append(names, expr[start:i])
					i++ // consume '}'
				}
			} else {
				start := i
				for i < len(expr) && isVarNameByte(expr[i]) {
					i++
				}
				if i > start {
					names = append(names, expr[start:i])
				}
			}
		default:
			i++
		}
	}
	return names
}

func isVarNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// checkVariableRefs warns when a rules:if: expression references a variable that
// is not declared in pipeline variables:, the job's own variables:, or any
// workflow:rules:variables: block. Predefined GitLab CI variables (CI_*, GITLAB_*,
// …) are always exempt. Variables set in GitLab CI/CD project settings are
// invisible to glint, so the finding is a WARNING rather than an error.
func checkVariableRefs(p *model.Pipeline) []Finding {
	pipelineVars := make(map[string]bool, len(p.Variables))
	for k := range p.Variables {
		pipelineVars[k] = true
	}

	// Union of all variables any workflow rule might inject into the context.
	workflowRuleVars := make(map[string]bool)
	if p.Workflow != nil {
		for _, rule := range p.Workflow.Rules {
			for k := range rule.Variables {
				workflowRuleVars[k] = true
			}
		}
	}

	var findings []Finding

	// Check workflow rules:if: expressions.
	if p.Workflow != nil {
		seen := make(map[string]bool)
		for i, rule := range p.Workflow.Rules {
			if rule.If == "" {
				continue
			}
			for _, varName := range extractIfVars(rule.If) {
				if isPredefinedVar(varName) || pipelineVars[varName] || seen[varName] {
					continue
				}
				seen[varName] = true
				findings = append(findings, Finding{
					Severity: Warning,
					Rule:     RuleUndeclaredVariable,
					File:     p.SourceFile,
					Message:  fmt.Sprintf("workflow.rules[%d].if: $%s is not declared in pipeline variables:", i, varName),
				})
			}
		}
	}

	// Check each job's rules:if: expressions.
	for name, job := range p.Jobs {
		jobVars := make(map[string]bool, len(job.Variables))
		for k := range job.Variables {
			jobVars[k] = true
		}

		// Deduplicate per (job, varName): report each undeclared variable once per job.
		seen := make(map[string]bool)
		for i, rule := range job.Rules {
			if rule.If == "" {
				continue
			}
			for _, varName := range extractIfVars(rule.If) {
				if isPredefinedVar(varName) || pipelineVars[varName] || jobVars[varName] || workflowRuleVars[varName] || seen[varName] {
					continue
				}
				seen[varName] = true
				findings = append(findings, Finding{
					Severity: Warning,
					Rule:     RuleUndeclaredVariable,
					Job:      name,
					File:     job.File,
					Line:     job.Line,
					Column:   job.Column,
					Message:  fmt.Sprintf("rules[%d].if: $%s is not declared in pipeline or job variables:", i, varName),
				})
			}
		}
	}

	return findings
}
