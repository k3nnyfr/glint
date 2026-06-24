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
