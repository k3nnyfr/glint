package cicontext

import "strings"

// Context holds the simulated CI execution environment used for context-aware
// pipeline evaluation (rules:if:, only:, except:, workflow:rules:).
// Variables are keyed by their name without the leading $.
type Context struct {
	Vars   map[string]string
	pinned map[string]bool // vars set via --var or shortcuts; never overwritten by Inject
}

// New builds a Context from high-level shortcut values and optional KEY=VALUE
// overrides. Predefined CI variables are derived from the shortcuts so callers
// do not need to know their exact names.
//
// Returns an empty Context (IsEmpty() == true) when all inputs are zero values,
// preserving the existing linting behaviour when no context flags are given.
//
// Override priority (highest wins): extraVars > branch/tag/source shortcuts.
// Both shortcut-derived and extraVar variables are pinned — they will not be
// overwritten by Inject (used for pipeline-level and workflow-rule variables).
func New(branch, tag, source string, extraVars []string) *Context {
	if branch == "" && tag == "" && source == "" && len(extraVars) == 0 {
		return &Context{}
	}

	vars := make(map[string]string)
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
	return strings.Join(parts, ", ")
}

// ExtractStringVars converts a map[string]any variable block (as used by
// Pipeline.Variables and Rule.Variables) to a flat map[string]string.
// Plain string values are used directly. Extended {value: "..."} map form
// uses the "value" key. Other forms are skipped.
func ExtractStringVars(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			out[k] = val
		case map[string]any:
			if s, ok := val["value"].(string); ok {
				out[k] = s
			}
		}
	}
	return out
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
