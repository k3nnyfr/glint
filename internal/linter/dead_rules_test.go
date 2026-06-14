package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestCheckDeadRules(t *testing.T) {
	cases := []struct {
		name     string
		rules    []model.Rule
		wantHit  bool // whether GL033 should fire
	}{
		{
			name:    "no rules — not dead",
			rules:   nil,
			wantHit: false,
		},
		{
			name:    "single bare when:never — dead",
			rules:   []model.Rule{{When: "never"}},
			wantHit: true,
		},
		{
			name: "all rules when:never with if — dead",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, When: "never"},
				{If: `$CI_COMMIT_BRANCH == "develop"`, When: "never"},
				{When: "never"},
			},
			wantHit: true,
		},
		{
			name: "first rule on_success — not dead",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, When: "on_success"},
				{When: "never"},
			},
			wantHit: false,
		},
		{
			name: "rule with empty when (defaults to on_success) — not dead",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`},
				{When: "never"},
			},
			wantHit: false,
		},
		{
			name:    "when:manual — not dead",
			rules:   []model.Rule{{When: "manual"}},
			wantHit: false,
		},
		{
			name:    "when:always — not dead",
			rules:   []model.Rule{{When: "always"}},
			wantHit: false,
		},
		{
			name:    "when:on_failure — not dead",
			rules:   []model.Rule{{When: "on_failure"}},
			wantHit: false,
		},
		{
			name: "mixed never and manual — not dead",
			rules: []model.Rule{
				{If: `$CI_COMMIT_BRANCH == "main"`, When: "never"},
				{When: "manual"},
			},
			wantHit: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := model.Job{Rules: tc.rules}
			findings := checkDeadRules("test-job", job)
			hit := false
			for _, f := range findings {
				if f.Rule == RuleDeadRules {
					hit = true
				}
			}
			if hit != tc.wantHit {
				t.Errorf("checkDeadRules: got hit=%v, want hit=%v; findings=%v", hit, tc.wantHit, findings)
			}
		})
	}
}
