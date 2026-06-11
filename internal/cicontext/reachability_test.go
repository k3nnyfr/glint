package cicontext

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestEvalWorkflow(t *testing.T) {
	makePipeline := func(rules []model.Rule) *model.Pipeline {
		return &model.Pipeline{Workflow: &model.Workflow{Rules: rules}}
	}

	tests := []struct {
		name     string
		rules    []model.Rule
		branch   string
		wantRuns bool
		wantVars map[string]string
	}{
		{
			name:     "no workflow block",
			rules:    nil,
			branch:   "main",
			wantRuns: true,
			wantVars: nil,
		},
		{
			name: "matching rule runs always",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, When: "always"},
			},
			branch:   "main",
			wantRuns: true,
		},
		{
			name: "matching rule when never",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, When: "never"},
			},
			branch:   "main",
			wantRuns: false,
		},
		{
			name: "no rule matched",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`},
			},
			branch:   "develop",
			wantRuns: false,
		},
		{
			name: "matching rule with variables",
			rules: []model.Rule{
				{
					If:   `$CI_COMMIT_BRANCH == "main"`,
					When: "always",
					Variables: map[string]any{
						"DEPLOY_TARGET": "production",
						"ENVIRONMENT":   "prod",
					},
				},
				{When: "always"},
			},
			branch:   "main",
			wantRuns: true,
			wantVars: map[string]string{
				"DEPLOY_TARGET": "production",
				"ENVIRONMENT":   "prod",
			},
		},
		{
			name: "fallback rule with different variables",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, Variables: map[string]any{"DEPLOY_TARGET": "production"}},
				{When: "always", Variables: map[string]any{"DEPLOY_TARGET": "staging"}},
			},
			branch:   "develop",
			wantRuns: true,
			wantVars: map[string]string{"DEPLOY_TARGET": "staging"},
		},
		{
			name: "empty context always runs",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, When: "never"},
			},
			branch:   "", // no context
			wantRuns: true,
			wantVars: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var p *model.Pipeline
			if tc.rules == nil {
				p = &model.Pipeline{}
			} else {
				p = makePipeline(tc.rules)
			}
			ctx := New(tc.branch, "", "", nil)
			runs, vars := EvalWorkflow(p, ctx)
			if runs != tc.wantRuns {
				t.Errorf("EvalWorkflow runs = %v, want %v", runs, tc.wantRuns)
			}
			for k, want := range tc.wantVars {
				if got := vars[k]; got != want {
					t.Errorf("EvalWorkflow vars[%q] = %q, want %q", k, got, want)
				}
			}
			if len(vars) != len(tc.wantVars) {
				t.Errorf("EvalWorkflow returned %d vars, want %d; got %v", len(vars), len(tc.wantVars), vars)
			}
		})
	}
}

func TestContextInject(t *testing.T) {
	t.Run("inject does not overwrite pinned var", func(t *testing.T) {
		ctx := New("main", "", "", []string{"DEPLOY_TARGET=override"})
		ctx.Inject("DEPLOY_TARGET", "workflow-value")
		if got := ctx.Get("DEPLOY_TARGET"); got != "override" {
			t.Errorf("Inject overwrote pinned var: got %q, want %q", got, "override")
		}
	})
	t.Run("inject does not overwrite shortcut var", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.Inject("CI_COMMIT_BRANCH", "other")
		if got := ctx.Get("CI_COMMIT_BRANCH"); got != "main" {
			t.Errorf("Inject overwrote shortcut var: got %q, want %q", got, "main")
		}
	})
	t.Run("inject sets new variable", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.Inject("DEPLOY_TARGET", "production")
		if got := ctx.Get("DEPLOY_TARGET"); got != "production" {
			t.Errorf("Inject did not set variable: got %q", got)
		}
	})
	t.Run("inject later call overrides earlier call", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.Inject("DEPLOY_TARGET", "pipeline-default")
		ctx.Inject("DEPLOY_TARGET", "workflow-override")
		if got := ctx.Get("DEPLOY_TARGET"); got != "workflow-override" {
			t.Errorf("second Inject did not win: got %q, want %q", got, "workflow-override")
		}
	})
}

func TestExtractStringVars(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want map[string]string
	}{
		{
			name: "plain strings",
			in:   map[string]any{"A": "hello", "B": "world"},
			want: map[string]string{"A": "hello", "B": "world"},
		},
		{
			name: "extended value form",
			in: map[string]any{
				"KEY": map[string]any{"value": "extended", "description": "some desc"},
			},
			want: map[string]string{"KEY": "extended"},
		},
		{
			name: "mixed forms",
			in: map[string]any{
				"PLAIN":   "str",
				"COMPLEX": map[string]any{"value": "val"},
			},
			want: map[string]string{"PLAIN": "str", "COMPLEX": "val"},
		},
		{
			name: "nil map",
			in:   nil,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractStringVars(tc.in)
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("ExtractStringVars[%q] = %q, want %q", k, got[k], want)
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("ExtractStringVars returned %d entries, want %d", len(got), len(tc.want))
			}
		})
	}
}

// TestWorkflowVarsJobEval verifies the end-to-end flow: workflow rule injects
// DEPLOY_TARGET, which is then used in a job's rules:if: expression.
func TestWorkflowVarsJobEval(t *testing.T) {
	rules := []model.Rule{
		{If: `$CI_COMMIT_BRANCH == "main"`, Variables: map[string]any{"DEPLOY_TARGET": "production"}},
		{When: "always", Variables: map[string]any{"DEPLOY_TARGET": "staging"}},
	}
	p := &model.Pipeline{
		Workflow: &model.Workflow{Rules: rules},
		Jobs: map[string]model.Job{
			"deploy-prod": {
				Rules: []model.Rule{
					{If: `$DEPLOY_TARGET == "production"`, When: "on_success"},
					{When: "never"},
				},
			},
			"deploy-staging": {
				Rules: []model.Rule{
					{If: `$DEPLOY_TARGET == "staging"`, When: "on_success"},
					{When: "never"},
				},
			},
		},
	}

	for _, tc := range []struct {
		branch      string
		wantProd    JobState
		wantStaging JobState
	}{
		{"main", JobActive, JobSkipped},
		{"develop", JobSkipped, JobActive},
	} {
		ctx := New(tc.branch, "", "", nil)
		_, ruleVars := EvalWorkflow(p, ctx)
		for k, v := range ruleVars {
			ctx.Inject(k, v)
		}
		if got := EvalJob(p.Jobs["deploy-prod"], ctx); got != tc.wantProd {
			t.Errorf("branch=%q deploy-prod = %v, want %v", tc.branch, got, tc.wantProd)
		}
		if got := EvalJob(p.Jobs["deploy-staging"], ctx); got != tc.wantStaging {
			t.Errorf("branch=%q deploy-staging = %v, want %v", tc.branch, got, tc.wantStaging)
		}
	}
}
