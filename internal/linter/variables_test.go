package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestExtractIfVars(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want []string
	}{
		{
			name: "simple variable",
			expr: `$MY_VAR == "value"`,
			want: []string{"MY_VAR"},
		},
		{
			name: "curly brace syntax",
			expr: `${MY_VAR} != null`,
			want: []string{"MY_VAR"},
		},
		{
			name: "multiple variables",
			expr: `$BRANCH == "main" && $DEPLOY_ENV == "prod"`,
			want: []string{"BRANCH", "DEPLOY_ENV"},
		},
		{
			name: "dollar sign inside string literal is skipped",
			expr: `$REAL_VAR == "$not_a_var"`,
			want: []string{"REAL_VAR"},
		},
		{
			name: "no variables",
			expr: `"main" == "main"`,
			want: nil,
		},
		{
			name: "regex rhs variable",
			expr: `$BRANCH =~ $PATTERN`,
			want: []string{"BRANCH", "PATTERN"},
		},
		{
			name: "multiline expression",
			expr: "$BRANCH == \"main\" ||\n$BRANCH == \"develop\"",
			want: []string{"BRANCH", "BRANCH"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIfVars(tc.expr)
			if len(got) != len(tc.want) {
				t.Fatalf("extractIfVars(%q) = %v, want %v", tc.expr, got, tc.want)
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("[%d] got %q, want %q", i, v, tc.want[i])
				}
			}
		})
	}
}

func TestIsPredefinedVar(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"CI_COMMIT_BRANCH", true},
		{"CI_JOB_TOKEN", true},
		{"GITLAB_USER_ID", true},
		{"GITLAB_FEATURES", true},
		{"FF_SOME_FLAG", true},
		{"RUNNER_ID", true},
		{"TRIGGER_PAYLOAD", true},
		{"CHAT_INPUT", true},
		{"MY_CUSTOM_VAR", false},
		{"DEPLOY_ENV", false},
		{"FEATURE_ENABLED", false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := isPredefinedVar(tc.input); got != tc.want {
				t.Errorf("isPredefinedVar(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCheckVariableRefs(t *testing.T) {
	cases := []struct {
		name         string
		pipeline     *model.Pipeline
		wantWarnings int
	}{
		{
			name: "declared pipeline variable — no warning",
			pipeline: &model.Pipeline{
				Variables: map[string]any{"MY_VAR": "value"},
				Jobs: map[string]model.Job{
					"job-a": {Rules: []model.Rule{{If: `$MY_VAR == "value"`}}},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "predefined CI variable — no warning",
			pipeline: &model.Pipeline{
				Jobs: map[string]model.Job{
					"job-a": {Rules: []model.Rule{{If: `$CI_COMMIT_BRANCH == "main"`}}},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "undeclared variable — one warning",
			pipeline: &model.Pipeline{
				Jobs: map[string]model.Job{
					"job-a": {Rules: []model.Rule{{If: `$UNDEFINED_VAR == "yes"`}}},
				},
			},
			wantWarnings: 1,
		},
		{
			name: "same undeclared var in multiple rules — one warning per job",
			pipeline: &model.Pipeline{
				Jobs: map[string]model.Job{
					"job-a": {
						Rules: []model.Rule{
							{If: `$UNDEFINED_VAR == "yes"`},
							{If: `$UNDEFINED_VAR == "no"`},
						},
					},
				},
			},
			wantWarnings: 1,
		},
		{
			name: "job-level variable — no warning",
			pipeline: &model.Pipeline{
				Jobs: map[string]model.Job{
					"job-a": {
						Variables: map[string]any{"LOCAL_VAR": "value"},
						Rules:     []model.Rule{{If: `$LOCAL_VAR == "value"`}},
					},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "workflow rule variable available to job rules — no warning",
			pipeline: &model.Pipeline{
				Workflow: &model.Workflow{
					Rules: []model.Rule{
						{
							If:        `$CI_COMMIT_BRANCH == "main"`,
							Variables: map[string]any{"DEPLOY_ENV": "production"},
						},
					},
				},
				Jobs: map[string]model.Job{
					"job-a": {Rules: []model.Rule{{If: `$DEPLOY_ENV == "production"`}}},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "undeclared variable in workflow rules:if",
			pipeline: &model.Pipeline{
				Workflow: &model.Workflow{
					Rules: []model.Rule{{If: `$UNDECLARED == "main"`}},
				},
				Jobs: map[string]model.Job{},
			},
			wantWarnings: 1,
		},
		{
			name: "two jobs each with a different undeclared variable — two warnings",
			pipeline: &model.Pipeline{
				Jobs: map[string]model.Job{
					"job-a": {Rules: []model.Rule{{If: `$UNDEF_A == "x"`}}},
					"job-b": {Rules: []model.Rule{{If: `$UNDEF_B == "y"`}}},
				},
			},
			wantWarnings: 2,
		},
		{
			name: "no rules — no warnings",
			pipeline: &model.Pipeline{
				Jobs: map[string]model.Job{
					"job-a": {Script: "echo ok"},
				},
			},
			wantWarnings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := checkVariableRefs(tc.pipeline)
			count := 0
			for _, f := range findings {
				if f.Severity == Warning && f.Rule == RuleUndeclaredVariable {
					count++
				}
			}
			if count != tc.wantWarnings {
				t.Errorf("got %d GL032 warnings, want %d; findings: %v", count, tc.wantWarnings, findings)
			}
		})
	}
}
