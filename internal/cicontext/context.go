package cicontext

import "strings"

// Context holds the simulated CI execution environment used for context-aware
// pipeline evaluation (rules:if:, only:, except:, workflow:rules:).
// Variables are keyed by their name without the leading $.
type Context struct {
	Vars map[string]string
}

// New builds a Context from high-level shortcut values and optional KEY=VALUE
// overrides. Predefined CI variables are derived from the shortcuts so callers
// do not need to know their exact names.
//
// Returns an empty Context (IsEmpty() == true) when all inputs are zero values,
// preserving the existing linting behaviour when no context flags are given.
//
// Override priority (highest wins): extraVars > branch/tag/source shortcuts.
func New(branch, tag, source string, extraVars []string) *Context {
	if branch == "" && tag == "" && source == "" && len(extraVars) == 0 {
		return &Context{}
	}

	vars := make(map[string]string)

	if branch != "" {
		vars["CI_COMMIT_BRANCH"] = branch
		vars["CI_COMMIT_REF_NAME"] = branch
		vars["CI_COMMIT_REF_SLUG"] = slugify(branch)
		if source == "" {
			source = "push"
		}
	}
	if tag != "" {
		vars["CI_COMMIT_TAG"] = tag
		vars["CI_COMMIT_REF_NAME"] = tag
		vars["CI_COMMIT_REF_SLUG"] = slugify(tag)
		delete(vars, "CI_COMMIT_BRANCH") // tag pushes have no branch variable
		if source == "" {
			source = "push"
		}
	}
	if source != "" {
		vars["CI_PIPELINE_SOURCE"] = source
	}
	if _, ok := vars["CI_DEFAULT_BRANCH"]; !ok {
		vars["CI_DEFAULT_BRANCH"] = "main"
	}

	// KEY=VALUE overrides win over shortcuts.
	for _, kv := range extraVars {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			vars[k] = v
		}
	}

	return &Context{Vars: vars}
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
