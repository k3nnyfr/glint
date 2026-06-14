package linter

import (
	"fmt"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

// checkInheritCompleteness (GL043): warn when a job's 'inherit: default:'
// declaration is dead — either because there is no 'default:' block in the
// pipeline, or because the list form references fields that are not defined
// in the 'default:' block.
func checkInheritCompleteness(p *model.Pipeline) []Finding {
	var findings []Finding
	for name, job := range p.Jobs {
		findings = append(findings, checkJobInheritCompleteness(p, name, job)...)
	}
	return findings
}

func checkJobInheritCompleteness(p *model.Pipeline, name string, job model.Job) []Finding {
	if job.Inherit == nil {
		return nil
	}
	m, ok := job.Inherit.(map[string]any)
	if !ok {
		return nil
	}
	defaultVal, hasDefault := m["default"]
	if !hasDefault {
		return nil
	}

	// Case 1: no default: block at all — the entire declaration is a no-op.
	if p.Default == nil {
		return []Finding{{
			Severity: Warning,
			Rule:     RuleInheritNoDefault,
			Job:      name,
			File:     job.File,
			Line:     job.Line,
			Message:  "'inherit: default:' is declared but the pipeline has no 'default:' block — declaration has no effect",
		}}
	}

	// Case 2: list form — check for field names not set in default:.
	list, ok := defaultVal.([]any)
	if !ok {
		// bool form (true/false) with a non-nil default: block is always valid.
		return nil
	}

	var dead []string
	for _, item := range list {
		field, ok := item.(string)
		if !ok {
			continue
		}
		if !defaultBlockHasField(p.Default, field) {
			dead = append(dead, field)
		}
	}
	if len(dead) == 0 {
		return nil
	}
	return []Finding{{
		Severity: Warning,
		Rule:     RuleInheritNoDefault,
		Job:      name,
		File:     job.File,
		Line:     job.Line,
		Message: fmt.Sprintf(
			"'inherit: default: [%s]': %s not defined in the 'default:' block — %s",
			strings.Join(dead, ", "),
			pluralIs(len(dead)),
			"these entries have no effect",
		),
	}}
}

// defaultBlockHasField reports whether the given field name has a non-zero value
// in the default: block. Unknown field names return true (conservative).
func defaultBlockHasField(d *model.DefaultConfig, field string) bool {
	switch field {
	case "image":
		return d.Image != nil
	case "before_script":
		return d.BeforeScript != nil
	case "after_script":
		return d.AfterScript != nil
	case "cache":
		return d.Cache != nil
	case "artifacts":
		return d.Artifacts != nil
	case "retry":
		return d.Retry != nil
	case "timeout":
		return d.Timeout != ""
	case "tags":
		return len(d.Tags) > 0
	}
	// Unknown field name — conservatively assume it might be defined.
	return true
}

func pluralIs(n int) string {
	if n == 1 {
		return "this field is"
	}
	return "these fields are"
}
