package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

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
	findings := checkNeeds(p)
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
