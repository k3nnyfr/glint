package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestCheckRulesIfReachability(t *testing.T) {
	makeJob := func(vars map[string]any, rules []model.Rule) model.Job {
		return model.Job{Variables: vars, Rules: rules}
	}

	cases := []struct {
		name        string
		pipelineVars map[string]any
		jobs        map[string]model.Job
		wantHit     []string // job names that should produce GL042
		wantMiss    []string // job names that should NOT produce GL042
	}{
		{
			name:         "declared var always false — GL042 fires",
			pipelineVars: map[string]any{"DEPLOY": "false"},
			jobs: map[string]model.Job{
				"deploy": makeJob(nil, []model.Rule{
					{If: `$DEPLOY == "true"`, When: "on_success"},
				}),
			},
			wantHit:  []string{"deploy"},
			wantMiss: nil,
		},
		{
			name:         "declared var matches — no finding",
			pipelineVars: map[string]any{"DEPLOY": "true"},
			jobs: map[string]model.Job{
				"deploy": makeJob(nil, []model.Rule{
					{If: `$DEPLOY == "true"`, When: "on_success"},
				}),
			},
			wantHit:  nil,
			wantMiss: []string{"deploy"},
		},
		{
			name:         "predefined CI_ var — skip check conservatively",
			pipelineVars: nil,
			jobs: map[string]model.Job{
				"branch-job": makeJob(nil, []model.Rule{
					{If: `$CI_COMMIT_BRANCH == "never-exists"`, When: "on_success"},
				}),
			},
			wantHit:  nil,
			wantMiss: []string{"branch-job"},
		},
		{
			name:         "undeclared var — skip check conservatively",
			pipelineVars: nil,
			jobs: map[string]model.Job{
				"my-job": makeJob(nil, []model.Rule{
					{If: `$UNKNOWN_VAR == "x"`, When: "on_success"},
				}),
			},
			wantHit:  nil,
			wantMiss: []string{"my-job"},
		},
		{
			name:         "unconditional rule — job can always activate",
			pipelineVars: map[string]any{"DEPLOY": "false"},
			jobs: map[string]model.Job{
				"my-job": makeJob(nil, []model.Rule{
					{If: `$DEPLOY == "true"`, When: "never"},
					{When: "on_success"},
				}),
			},
			wantHit:  nil,
			wantMiss: []string{"my-job"},
		},
		{
			name:         "all rules when:never — GL042 fires (not GL033 territory overlap)",
			pipelineVars: map[string]any{"DEPLOY": "false"},
			jobs: map[string]model.Job{
				"my-job": makeJob(nil, []model.Rule{
					{If: `$DEPLOY == "true"`, When: "on_success"},
					{When: "never"},
				}),
			},
			// Rule 1 evaluates to false; Rule 2 is explicit never → all dead.
			wantHit:  []string{"my-job"},
			wantMiss: nil,
		},
		{
			name:         "job-level var overrides pipeline var",
			pipelineVars: map[string]any{"FLAG": "false"},
			jobs: map[string]model.Job{
				"my-job": makeJob(map[string]any{"FLAG": "true"}, []model.Rule{
					{If: `$FLAG == "true"`, When: "on_success"},
				}),
			},
			// Job-level FLAG="true" makes the if: evaluate to true → no finding.
			wantHit:  nil,
			wantMiss: []string{"my-job"},
		},
		{
			name:         "no rules — nothing to check",
			pipelineVars: map[string]any{"DEPLOY": "false"},
			jobs: map[string]model.Job{
				"my-job": makeJob(nil, nil),
			},
			wantHit:  nil,
			wantMiss: []string{"my-job"},
		},
		{
			name:         "boolean variable evaluates correctly",
			pipelineVars: map[string]any{"FLAG": false}, // bool false
			jobs: map[string]model.Job{
				"my-job": makeJob(nil, []model.Rule{
					{If: `$FLAG == "true"`, When: "on_success"},
				}),
			},
			// ScalarString(false) → "false"; "false" == "true" → false → GL042 fires.
			wantHit:  []string{"my-job"},
			wantMiss: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &model.Pipeline{
				Variables: tc.pipelineVars,
				Jobs:      tc.jobs,
			}
			findings := checkRulesIfReachability(p)

			hitSet := make(map[string]bool)
			for _, f := range findings {
				if f.Rule == RuleStaticDeadRules {
					hitSet[f.Job] = true
				}
			}

			for _, want := range tc.wantHit {
				if !hitSet[want] {
					t.Errorf("expected GL042 for job %q but it was not reported; findings: %v", want, findings)
				}
			}
			for _, notWant := range tc.wantMiss {
				if hitSet[notWant] {
					t.Errorf("unexpected GL042 for job %q", notWant)
				}
			}
		})
	}
}
