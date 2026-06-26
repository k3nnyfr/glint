package linter

import (
	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/model"
)

// checkRulesIfReachability (GL042): warn when every rule in a job's rules: block
// can be statically proven to never activate given the values of variables
// declared in the pipeline YAML. Only fires when ALL variables referenced by the
// rules:if: expressions are declared in the pipeline or job variables: blocks —
// predefined CI_* / GITLAB_* variables or variables not declared in YAML are
// treated as unknown (could have any runtime value) so the check is skipped
// conservatively to avoid false positives.
func checkRulesIfReachability(p *model.Pipeline) []Finding {
	if len(p.Jobs) == 0 {
		return nil
	}

	// Collect scalar string values from pipeline-level variables.
	pipelineVars := make(map[string]string, len(p.Variables))
	for k, v := range p.Variables {
		if s, ok := cicontext.ScalarString(v); ok {
			pipelineVars[k] = s
		}
	}

	var findings []Finding
	for name, job := range p.Jobs {
		if len(job.Rules) == 0 {
			continue
		}

		// Merge pipeline vars with job-level variable overrides.
		jobVars := make(map[string]string, len(pipelineVars)+len(job.Variables))
		for k, v := range pipelineVars {
			jobVars[k] = v
		}
		for k, v := range job.Variables {
			if s, ok := cicontext.ScalarString(v); ok {
				jobVars[k] = s
			}
		}

		if f := evalRulesReachability(name, job, jobVars); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// evalRulesReachability returns a GL042 finding when the job's rules: block
// can never activate given the provided variable map, or nil if the job
// might activate (or the check cannot be applied conservatively).
func evalRulesReachability(name string, job model.Job, jobVars map[string]string) *Finding {
	lookup := func(k string) string { return jobVars[k] }

	for _, rule := range job.Rules {
		// Explicit when: never — this rule is provably dead; continue.
		if rule.When == "never" {
			continue
		}

		// No if: condition → this rule always matches → job CAN activate.
		if rule.If == "" {
			return nil
		}

		// Check whether all variables referenced in the if: expression are
		// declared in the pipeline YAML. If any are not declared (predefined
		// CI vars, project settings, etc.), we cannot evaluate the expression
		// and must conservatively assume the job might activate.
		refs := extractIfVars(rule.If)
		allDeclared := true
		for _, ref := range refs {
			if _, ok := jobVars[ref]; !ok {
				// Variable not in YAML (predefined or unknown) → cannot evaluate.
				allDeclared = false
				break
			}
		}
		if !allDeclared {
			return nil // conservative: job might activate
		}

		// All referenced variables are declared; evaluate the expression.
		// Use strict mode (unparse → false) to avoid matching unparseable expressions.
		if cicontext.EvalIfStrict(rule.If, lookup) {
			// This rule's condition evaluates to true → job CAN activate.
			return nil
		}
		// Expression evaluated to false → this rule cannot activate; continue.
	}

	// No rule could activate the job.
	return &Finding{
		Severity: Warning,
		Rule:     RuleStaticDeadRules,
		Job:      name,
		File:     job.File,
		Line:     job.Line,
		Column:   job.Column,
		Message:  "rules: block can never activate: all if: conditions evaluate to false given the declared pipeline variables",
	}
}
