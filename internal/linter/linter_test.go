package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// TestCheckDefault exercises the non-nil default block code path.
func TestCheckDefault(t *testing.T) {
	// nil default → nil return (already covered implicitly; included for clarity)
	if got := checkDefault(&model.Pipeline{}); got != nil {
		t.Errorf("nil default: want nil findings, got %v", got)
	}

	// non-nil default with empty timeout → no findings
	if got := checkDefault(&model.Pipeline{Default: &model.DefaultConfig{}}); got != nil {
		t.Errorf("empty default: want no findings, got %v", got)
	}

	// non-nil default with invalid timeout → finding produced
	findings := checkDefault(&model.Pipeline{Default: &model.DefaultConfig{Timeout: "bad-value"}})
	if len(findings) == 0 {
		t.Error("invalid timeout in default block should produce a finding")
	}
}

// TestCheckJob_MissingScript covers the error/warning paths for jobs without a
// script field.
func TestCheckJob_MissingScript(t *testing.T) {
	stageSet := map[string]bool{"build": true}

	// No script, no extends → Error.
	findings := checkJob("my-job", model.Job{Stage: "build"}, stageSet)
	var gotErr bool
	for _, f := range findings {
		if f.Rule == RuleMissingScript && f.Severity == Error {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("job with no script and no extends: expected Error finding GL003")
	}
}

// TestCheckJob_ExtendsNoScript covers the Warning variant: a job that extends
// a base template but has no script (the script may come from the base, which
// could not be fetched).
func TestCheckJob_ExtendsNoScript(t *testing.T) {
	stageSet := map[string]bool{"build": true}
	job := model.Job{Stage: "build", Extends: ".base-template"}
	findings := checkJob("my-job", job, stageSet)
	var gotWarn bool
	for _, f := range findings {
		if f.Rule == RuleMissingScript && f.Severity == Warning {
			gotWarn = true
		}
	}
	if !gotWarn {
		t.Error("job with extends but no script: expected Warning finding GL003")
	}
}

// TestCheckJob_StageInputPlaceholder verifies that a stage value containing
// "$[[" (a component input placeholder) skips the unknown-stage check.
func TestCheckJob_StageInputPlaceholder(t *testing.T) {
	stageSet := map[string]bool{"build": true}
	job := model.Job{Stage: "$[[inputs.stage]]", Script: []any{"echo"}}
	for _, f := range checkJob("my-job", job, stageSet) {
		if f.Rule == RuleUnknownStage {
			t.Error("stage with $[[ placeholder should not produce GL004")
		}
	}
}

// TestCheckJob_OnlyAndRules covers the only+rules conflict (GL005).
func TestCheckJob_OnlyAndRules(t *testing.T) {
	stageSet := map[string]bool{"build": true}
	job := model.Job{
		Stage:  "build",
		Script: []any{"echo"},
		Only:   []any{"branches"},
		Rules:  []model.Rule{{When: "on_success"}},
	}
	var gotConflict bool
	for _, f := range checkJob("my-job", job, stageSet) {
		if f.Rule == RuleOnlyRulesConflict {
			gotConflict = true
		}
	}
	if !gotConflict {
		t.Error("only+rules: expected GL005 conflict finding")
	}
}

// TestCheckJob_ExceptAndRules covers the except+rules conflict (GL006).
func TestCheckJob_ExceptAndRules(t *testing.T) {
	stageSet := map[string]bool{"build": true}
	job := model.Job{
		Stage:  "build",
		Script: []any{"echo"},
		Except: []any{"tags"},
		Rules:  []model.Rule{{When: "on_success"}},
	}
	var gotConflict bool
	for _, f := range checkJob("my-job", job, stageSet) {
		if f.Rule == RuleExceptRulesConflict {
			gotConflict = true
		}
	}
	if !gotConflict {
		t.Error("except+rules: expected GL006 conflict finding")
	}
}

// TestCheckJob_RunField verifies that a job with a 'run:' block (instead of
// 'script:') is not flagged for missing script.
func TestCheckJob_RunField(t *testing.T) {
	stageSet := map[string]bool{"build": true}
	job := model.Job{Stage: "build", Run: map[string]any{"steps": []any{"echo hi"}}}
	for _, f := range checkJob("my-job", job, stageSet) {
		if f.Rule == RuleMissingScript {
			t.Error("job with run: should not produce GL003 (missing script)")
		}
	}
}

// TestCheckJob_UnknownStage verifies that a job with a declared stage value that
// is not in the stageSet produces a GL004 finding (line 178-185).
func TestCheckJob_UnknownStage(t *testing.T) {
	stageSet := map[string]bool{"build": true, "test": true}
	job := model.Job{Stage: "unknown-stage", Script: []any{"echo"}}
	var gotGL004 bool
	for _, f := range checkJob("my-job", job, stageSet) {
		if f.Rule == RuleUnknownStage {
			gotGL004 = true
		}
	}
	if !gotGL004 {
		t.Error("job with undeclared stage: expected GL004 finding")
	}
}
