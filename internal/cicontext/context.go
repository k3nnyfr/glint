package cicontext

import (
	"fmt"
	"strings"
)

// Context holds the simulated CI execution environment used for context-aware
// pipeline evaluation (rules:if:, only:, except:, workflow:rules:).
// Variables are keyed by their name without the leading $.
type Context struct {
	Vars         map[string]string
	pinned       map[string]bool // vars set via --var or shortcuts; never overwritten by Inject
	changedFiles []string        // nil = not provided (permissive); non-nil = known set of changed files
}

// gitlabAlwaysVars are GitLab predefined variables that are constant across
// every pipeline, instance, and project. Injected at the lowest priority so
// that pipeline variables: blocks and --var overrides can still win.
var gitlabAlwaysVars = map[string]string{
	"CI":        "true",
	"GITLAB_CI": "true",
}

// gitlabMRVars are predefined variables GitLab injects only for
// CI_PIPELINE_SOURCE=merge_request_event pipelines. Placeholder values are
// used so that rules:if: expressions that gate on MR context evaluate
// correctly. All can be overridden with --var.
//
// CI_MERGE_REQUEST_SOURCE_BRANCH_NAME is left empty here and filled from
// CI_COMMIT_BRANCH (the --branch flag) when available.
var gitlabMRVars = map[string]string{
	"CI_MERGE_REQUEST_ID":                  "1",
	"CI_MERGE_REQUEST_IID":                 "1",
	"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME":  "",
	"CI_MERGE_REQUEST_TARGET_BRANCH_NAME":  "main",
	"CI_MERGE_REQUEST_PROJECT_PATH":        "namespace/project",
	"CI_MERGE_REQUEST_TITLE":               "Draft: placeholder",
	"CI_MERGE_REQUEST_LABELS":              "",
	"CI_OPEN_MERGE_REQUESTS":               "namespace/project!1",
}

// New builds a Context from high-level shortcut values and optional KEY=VALUE
// overrides. Predefined CI variables are derived from the shortcuts so callers
// do not need to know their exact names.
//
// Returns an empty Context (IsEmpty() == true) when all inputs are zero values,
// preserving the existing linting behaviour when no context flags are given.
//
// Override priority (highest wins): extraVars > branch/tag/source shortcuts >
// pipeline variables (via Inject) > GitLab predefined defaults.
func New(branch, tag, source string, extraVars []string) *Context {
	if branch == "" && tag == "" && source == "" && len(extraVars) == 0 {
		return &Context{}
	}

	// Seed with always-available GitLab predefined variables at lowest priority.
	vars := make(map[string]string, len(gitlabAlwaysVars)+8)
	for k, v := range gitlabAlwaysVars {
		vars[k] = v
	}
	pinned := make(map[string]bool)

	pin := func(k, v string) {
		vars[k] = v
		pinned[k] = true
	}

	if branch != "" {
		pin("CI_COMMIT_BRANCH", branch)
		pin("CI_COMMIT_REF_NAME", branch)
		pin("CI_COMMIT_REF_SLUG", slugify(branch))
		if source == "" {
			source = "push"
		}
	}
	if tag != "" {
		pin("CI_COMMIT_TAG", tag)
		pin("CI_COMMIT_REF_NAME", tag)
		pin("CI_COMMIT_REF_SLUG", slugify(tag))
		delete(vars, "CI_COMMIT_BRANCH")
		delete(pinned, "CI_COMMIT_BRANCH")
		if source == "" {
			source = "push"
		}
	}
	if source != "" {
		pin("CI_PIPELINE_SOURCE", source)
	}
	if _, ok := vars["CI_DEFAULT_BRANCH"]; !ok {
		vars["CI_DEFAULT_BRANCH"] = "main"
	}

	// For MR pipelines, inject placeholder values for the predefined MR
	// variables so that rules:if: expressions like '$CI_MERGE_REQUEST_IID'
	// evaluate as non-empty (truthy). These are non-pinned so --var overrides
	// them and pipeline variables: can also override via Inject.
	if source == "merge_request_event" {
		for k, v := range gitlabMRVars {
			if !pinned[k] {
				vars[k] = v
			}
		}
		// Derive source branch from --branch when available.
		if branch != "" && vars["CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"] == "" {
			vars["CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"] = branch
		}
	}

	// KEY=VALUE overrides win over shortcuts and everything else.
	for _, kv := range extraVars {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			pin(k, v)
		}
	}

	return &Context{Vars: vars, pinned: pinned}
}

// IsEmpty reports whether no variables have been set (no context flags given).
func (c *Context) IsEmpty() bool {
	return c == nil || len(c.Vars) == 0
}

// Get returns the value of a CI variable (key without the leading $).
// Returns an empty string when the variable is not defined.
func (c *Context) Get(key string) string {
	if c == nil {
		return ""
	}
	return c.Vars[key]
}

// Inject sets key=value only if key is not already pinned (i.e. not set via
// --branch / --tag / --source / --var). Used to inject pipeline-level variable
// defaults and workflow-rule variables without overriding explicit user input.
// Calling Inject in order from lowest-priority to highest-priority source
// ensures later calls win over earlier ones.
func (c *Context) Inject(key, value string) {
	if c.pinned[key] {
		return
	}
	if c.Vars == nil {
		c.Vars = make(map[string]string)
	}
	c.Vars[key] = value
}

// SetChangedFiles records the list of files that changed for rules:changes:
// evaluation. A non-nil slice (even empty) enables strict matching: only jobs
// whose rules:changes: patterns match at least one file in the list will have
// that rule fire. A nil slice (the default) means "not provided" — rules:changes:
// conditions are treated as always satisfied (permissive), preserving the
// behaviour when no changed-file data is available.
func (c *Context) SetChangedFiles(files []string) {
	if c != nil {
		c.changedFiles = files
	}
}

// Summary returns a short human-readable description of the context for CLI output.
func (c *Context) Summary() string {
	if c.IsEmpty() {
		return ""
	}
	var parts []string
	if v := c.Get("CI_COMMIT_TAG"); v != "" {
		parts = append(parts, "tag="+v)
	} else if v := c.Get("CI_COMMIT_BRANCH"); v != "" {
		parts = append(parts, "branch="+v)
	}
	if v := c.Get("CI_PIPELINE_SOURCE"); v != "" {
		parts = append(parts, "source="+v)
	}
	if c.changedFiles != nil {
		parts = append(parts, fmt.Sprintf("%d changed file(s)", len(c.changedFiles)))
	}
	return strings.Join(parts, ", ")
}

// ExtractStringVars converts a map[string]any variable block (as used by
// Pipeline.Variables and Rule.Variables) to a flat map[string]string.
// Plain string values are used directly. Extended {value: ...} map form uses
// the "value" key. Scalar non-string values (bool, int, float64) are
// converted to their string representation, matching GitLab's own behaviour
// where all CI variable values are strings.
func ExtractStringVars(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := ScalarString(v); ok {
			out[k] = s
		}
	}
	return out
}

// ScalarString converts a YAML-decoded CI variable value to its string
// representation. Handles plain scalars and the extended {value: ...} map
// form. Returns (s, true) on success, ("", false) for unrecognised forms.
func ScalarString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case bool:
		return fmt.Sprintf("%t", val), true
	case int:
		return fmt.Sprintf("%d", val), true
	case float64:
		return fmt.Sprintf("%g", val), true
	case map[string]any:
		inner, ok := val["value"]
		if !ok {
			return "", false
		}
		return ScalarString(inner)
	}
	return "", false
}

// ExpandVars expands $VAR and ${VAR} references within every value in
// ctx.Vars, using the same map as the expansion source. Iteration repeats
// (up to 10 passes) so transitive chains like A=$B, B=$C resolve fully.
// Variables that form circular references are left as-is after the limit.
func (c *Context) ExpandVars() {
	if c == nil || len(c.Vars) == 0 {
		return
	}
	for range 10 {
		changed := false
		for k, v := range c.Vars {
			expanded := expandVarRefs(v, c.Vars)
			if expanded != v {
				c.Vars[k] = expanded
				changed = true
			}
		}
		if !changed {
			return
		}
	}
}

// expandVarRefs replaces $VAR and ${VAR} occurrences in s with their values
// from vars. Unknown variables are left unchanged.
func expandVarRefs(s string, vars map[string]string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '$' {
			sb.WriteByte(s[i])
			i++
			continue
		}
		i++ // consume '$'
		if i >= len(s) {
			sb.WriteByte('$')
			break
		}
		if s[i] == '{' {
			i++ // consume '{'
			j := i
			for j < len(s) && isIdentByte(s[j]) {
				j++
			}
			if j < len(s) && s[j] == '}' {
				name := s[i:j]
				if val, ok := vars[name]; ok {
					sb.WriteString(val)
				} else {
					sb.WriteString("${")
					sb.WriteString(name)
					sb.WriteByte('}')
				}
				i = j + 1
			} else {
				// Malformed ${…} — emit literally
				sb.WriteString("${")
				i = j
			}
		} else {
			j := i
			for j < len(s) && isIdentByte(s[j]) {
				j++
			}
			if j > i {
				name := s[i:j]
				if val, ok := vars[name]; ok {
					sb.WriteString(val)
				} else {
					sb.WriteByte('$')
					sb.WriteString(name)
				}
				i = j
			} else {
				sb.WriteByte('$')
			}
		}
	}
	return sb.String()
}

// slugify converts a ref name to its GitLab slug form:
// lowercased, non-alphanumeric characters replaced with '-', leading/trailing '-' removed.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
