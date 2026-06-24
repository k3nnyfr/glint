package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// TestCheckNeeds_SkippedJob verifies that a skipped job's needs: violations are suppressed.
func TestCheckNeeds_SkippedJob(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "deploy"},
		Jobs: map[string]model.Job{
			"skipped-job": {
				Name:   "skipped-job",
				Stage:  "build",
				Script: []any{"echo"},
				Needs:  []any{"nonexistent-job"},
			},
		},
	}
	// Without skipping: should report unknown needs.
	withoutSkip := checkNeeds(p, nil)
	if len(withoutSkip) == 0 {
		t.Fatal("expected GL027 without skipped set, got none")
	}
	// With job skipped: no findings.
	withSkip := checkNeeds(p, map[string]bool{"skipped-job": true})
	if len(withSkip) != 0 {
		t.Errorf("expected no findings for skipped job, got %v", withSkip)
	}
}

// TestCheckRulesNeeds_UnknownJob verifies that an unknown job in rules:needs: produces GL044.
func TestCheckRulesNeeds_UnknownJob(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {
				Name:   "build-job",
				Stage:  "build",
				Script: []any{"make"},
			},
			"test-job": {
				Name:   "test-job",
				Stage:  "test",
				Script: []any{"make test"},
				Rules: []model.Rule{
					{
						If:    `$CI_COMMIT_BRANCH == "main"`,
						Needs: []any{"build-job", "nonexistent"},
					},
				},
			},
		},
	}
	findings := checkRulesNeeds(p, nil)
	var got bool
	for _, f := range findings {
		if f.Rule == RuleRulesNeedsUnknown && f.Severity == Error {
			got = true
		}
	}
	if !got {
		t.Errorf("expected GL044 error for unknown rules:needs: job; got %v", findings)
	}
}

// TestCheckRulesNeeds_KnownJob verifies no finding when all rules:needs: jobs exist.
func TestCheckRulesNeeds_KnownJob(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build", Script: []any{"make"}},
			"test-job": {
				Name:   "test-job",
				Stage:  "test",
				Script: []any{"make test"},
				Rules:  []model.Rule{{Needs: []any{"build-job"}}},
			},
		},
	}
	if findings := checkRulesNeeds(p, nil); len(findings) != 0 {
		t.Errorf("expected no findings for valid rules:needs:; got %v", findings)
	}
}

// TestCheckRulesNeeds_Optional verifies that optional: true downgrades to Warning.
func TestCheckRulesNeeds_Optional(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {
				Name:   "test-job",
				Script: []any{"make test"},
				Rules: []model.Rule{
					{Needs: []any{map[string]any{"job": "ghost", "optional": true}}},
				},
			},
		},
	}
	findings := checkRulesNeeds(p, nil)
	if len(findings) != 1 || findings[0].Severity != Warning {
		t.Errorf("optional unknown rules:needs: should produce Warning; got %v", findings)
	}
}

// TestCheckRulesNeeds_CrossPipeline verifies that cross-pipeline needs are ignored.
func TestCheckRulesNeeds_CrossPipeline(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {
				Name:   "test-job",
				Script: []any{"make test"},
				Rules: []model.Rule{
					{Needs: []any{map[string]any{"pipeline": "other", "job": "j"}}},
				},
			},
		},
	}
	if findings := checkRulesNeeds(p, nil); len(findings) != 0 {
		t.Errorf("cross-pipeline rules:needs: should be ignored; got %v", findings)
	}
}

// TestCheckRulesNeeds_SkippedJob verifies that a skipped job's rules:needs: violations are suppressed.
func TestCheckRulesNeeds_SkippedJob(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {
				Name:   "test-job",
				Script: []any{"make test"},
				Rules:  []model.Rule{{Needs: []any{"nonexistent"}}},
			},
		},
	}
	if findings := checkRulesNeeds(p, map[string]bool{"test-job": true}); len(findings) != 0 {
		t.Errorf("skipped job: expected no findings; got %v", findings)
	}
}

// TestCheckRulesNeeds_NoNeeds verifies no findings when rules have no needs: override.
func TestCheckRulesNeeds_NoNeeds(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {
				Name:   "test-job",
				Script: []any{"make test"},
				Rules:  []model.Rule{{When: "on_success"}},
			},
		},
	}
	if findings := checkRulesNeeds(p, nil); len(findings) != 0 {
		t.Errorf("rules without needs: should produce no findings; got %v", findings)
	}
}

// TestCheckNeeds_StageOrder verifies that a job needing a job in a later stage
// produces RuleNeedsStageOrder (line 64-76 in needs.go).
func TestCheckNeeds_StageOrder(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test", "deploy"},
		Jobs: map[string]model.Job{
			"build-job": {
				Name:   "build-job",
				Stage:  "build",
				Script: []any{"make"},
				Needs:  []any{"deploy-job"},
			},
			"deploy-job": {
				Name:   "deploy-job",
				Stage:  "deploy",
				Script: []any{"make deploy"},
			},
		},
	}
	findings := checkNeeds(p, nil)
	var gotStageOrder bool
	for _, f := range findings {
		if f.Rule == RuleNeedsStageOrder {
			gotStageOrder = true
		}
	}
	if !gotStageOrder {
		t.Errorf("build-job needing deploy-job (later stage): expected GL025 finding; got: %v", findings)
	}
}
