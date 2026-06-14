package cicontext

import (
	"testing"
)

// ── New / IsEmpty / Get / Inject ──────────────────────────────────────────────

func TestNew_Empty(t *testing.T) {
	ctx := New("", "", "", nil)
	if !ctx.IsEmpty() {
		t.Error("expected empty context")
	}
	if ctx.Get("CI_COMMIT_BRANCH") != "" {
		t.Error("expected empty Get on empty context")
	}
}

func TestNew_Branch(t *testing.T) {
	ctx := New("feature/abc", "", "", nil)
	if ctx.Get("CI_COMMIT_BRANCH") != "feature/abc" {
		t.Errorf("unexpected branch: %q", ctx.Get("CI_COMMIT_BRANCH"))
	}
	if ctx.Get("CI_COMMIT_REF_SLUG") != "feature-abc" {
		t.Errorf("unexpected slug: %q", ctx.Get("CI_COMMIT_REF_SLUG"))
	}
	if ctx.Get("CI_PIPELINE_SOURCE") != "push" {
		t.Error("expected default source=push for branch")
	}
}

func TestNew_Tag(t *testing.T) {
	ctx := New("", "v1.0.0", "", nil)
	if ctx.Get("CI_COMMIT_TAG") != "v1.0.0" {
		t.Errorf("unexpected tag: %q", ctx.Get("CI_COMMIT_TAG"))
	}
	// tag should clear CI_COMMIT_BRANCH
	if ctx.Get("CI_COMMIT_BRANCH") != "" {
		t.Error("CI_COMMIT_BRANCH should be cleared when tag is set")
	}
	if ctx.Get("CI_PIPELINE_SOURCE") != "push" {
		t.Error("expected default source=push for tag")
	}
}

func TestNew_BranchAndTagTogether(t *testing.T) {
	// branch set first, then tag overwrites REF_NAME and clears BRANCH
	ctx := New("main", "v2.0", "", nil)
	if ctx.Get("CI_COMMIT_BRANCH") != "" {
		t.Error("BRANCH should be cleared by tag")
	}
	if ctx.Get("CI_COMMIT_TAG") != "v2.0" {
		t.Errorf("unexpected tag %q", ctx.Get("CI_COMMIT_TAG"))
	}
}

func TestNew_ExtraVars(t *testing.T) {
	ctx := New("main", "", "", []string{"DEPLOY_ENV=prod", "BAD_NO_EQUALS"})
	if ctx.Get("DEPLOY_ENV") != "prod" {
		t.Errorf("extra var not set: %q", ctx.Get("DEPLOY_ENV"))
	}
	if ctx.Get("BAD_NO_EQUALS") != "" {
		t.Error("malformed KEY=VALUE should not be set")
	}
}

func TestNew_SourceOverride(t *testing.T) {
	ctx := New("", "", "schedule", nil)
	if ctx.Get("CI_PIPELINE_SOURCE") != "schedule" {
		t.Errorf("unexpected source: %q", ctx.Get("CI_PIPELINE_SOURCE"))
	}
}

func TestNew_DefaultBranch(t *testing.T) {
	ctx := New("feature", "", "", nil)
	if ctx.Get("CI_DEFAULT_BRANCH") != "main" {
		t.Errorf("unexpected default branch: %q", ctx.Get("CI_DEFAULT_BRANCH"))
	}
}

func TestInject(t *testing.T) {
	ctx := New("main", "", "", nil)
	// pinned var should not be overwritten
	ctx.Inject("CI_COMMIT_BRANCH", "other")
	if ctx.Get("CI_COMMIT_BRANCH") != "main" {
		t.Error("pinned var was overwritten by Inject")
	}
	// non-pinned var should be injected
	ctx.Inject("MY_VAR", "hello")
	if ctx.Get("MY_VAR") != "hello" {
		t.Errorf("Inject failed: %q", ctx.Get("MY_VAR"))
	}
}

func TestInject_NilVarsMap(t *testing.T) {
	ctx := &Context{}
	ctx.Inject("K", "v") // should not panic
	if ctx.Get("K") != "v" {
		t.Error("Inject on nil Vars should initialise the map")
	}
}

func TestGet_NilContext(t *testing.T) {
	var ctx *Context
	if ctx.Get("X") != "" {
		t.Error("nil context Get should return empty string")
	}
}

// ── Summary ───────────────────────────────────────────────────────────────────

func TestSummary(t *testing.T) {
	cases := []struct {
		name   string
		branch string
		tag    string
		source string
		want   string
	}{
		{"empty context", "", "", "", ""},
		{"branch only", "main", "", "", "branch=main, source=push"},
		{"tag only", "", "v1.0", "", "tag=v1.0, source=push"},
		{"source only", "", "", "schedule", "source=schedule"},
		{"branch + source", "develop", "", "merge_request_event", "branch=develop, source=merge_request_event"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := New(tc.branch, tc.tag, tc.source, nil)
			got := ctx.Summary()
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// ── ScalarString ──────────────────────────────────────────────────────────────

func TestScalarString(t *testing.T) {
	cases := []struct {
		name    string
		input   any
		wantS   string
		wantOK  bool
	}{
		{"string", "hello", "hello", true},
		{"bool true", true, "true", true},
		{"bool false", false, "false", true},
		{"int", 42, "42", true},
		{"float64", 3.14, "3.14", true},
		{"map with value key", map[string]any{"value": "v"}, "v", true},
		{"map without value key", map[string]any{"description": "x"}, "", false},
		{"nil", nil, "", false},
		{"slice", []any{1, 2}, "", false},
		{"nested map value", map[string]any{"value": 99}, "99", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := ScalarString(tc.input)
			if ok != tc.wantOK || s != tc.wantS {
				t.Errorf("got (%q, %v) want (%q, %v)", s, ok, tc.wantS, tc.wantOK)
			}
		})
	}
}

// ── ExpandVars / expandVarRefs ────────────────────────────────────────────────

func TestExpandVars(t *testing.T) {
	t.Run("simple expansion", func(t *testing.T) {
		ctx := &Context{Vars: map[string]string{"A": "hello", "B": "$A world"}}
		ctx.ExpandVars()
		if ctx.Vars["B"] != "hello world" {
			t.Errorf("got %q", ctx.Vars["B"])
		}
	})

	t.Run("transitive chain", func(t *testing.T) {
		ctx := &Context{Vars: map[string]string{"A": "x", "B": "$A", "C": "$B"}}
		ctx.ExpandVars()
		if ctx.Vars["C"] != "x" {
			t.Errorf("got %q", ctx.Vars["C"])
		}
	})

	t.Run("circular reference stops", func(t *testing.T) {
		ctx := &Context{Vars: map[string]string{"A": "$B", "B": "$A"}}
		ctx.ExpandVars() // must not infinite-loop
	})

	t.Run("nil context is noop", func(t *testing.T) {
		var ctx *Context
		ctx.ExpandVars() // must not panic
	})

	t.Run("empty vars is noop", func(t *testing.T) {
		ctx := &Context{}
		ctx.ExpandVars() // must not panic
	})
}

func TestExpandVarRefs(t *testing.T) {
	vars := map[string]string{"FOO": "bar", "X": "123"}
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"no dollar", "hello", "hello"},
		{"simple ref", "$FOO", "bar"},
		{"braced ref", "${FOO}", "bar"},
		{"unknown ref unchanged", "$UNKNOWN", "$UNKNOWN"},
		{"unknown braced unchanged", "${UNKNOWN}", "${UNKNOWN}"},
		{"trailing dollar", "abc$", "abc$"},
		{"dollar at end after ident", "$X_", "$X_"},
		// Malformed ${…} without closing brace: "${" emitted, ident chars consumed.
		{"malformed brace no close", "${FOO", "${"},
		{"mixed", "pre_${FOO}_$X", "pre_bar_123"},
		{"dollar no ident after", "$ abc", "$ abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expandVarRefs(tc.input, vars)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// ── slugify ───────────────────────────────────────────────────────────────────

func TestSlugify(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"main", "main"},
		{"feature/my-branch", "feature-my-branch"},
		{"Feature_123", "feature-123"},
		{"--leading", "leading"},
		{"trailing--", "trailing"},
		{"v1.2.3", "v1-2-3"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := slugify(tc.input)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
