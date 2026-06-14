package graph

import (
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestPipeline_EmptyJobs(t *testing.T) {
	p := &model.Pipeline{}
	out := Pipeline(p)
	if !strings.Contains(out, "no jobs defined") {
		t.Errorf("expected 'no jobs defined', got:\n%s", out)
	}
}

func TestPipeline_BasicTwoStages(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
			"test-job":  {Name: "test-job", Stage: "test"},
			".tmpl":     {Name: ".tmpl", Stage: "build"}, // hidden — excluded
		},
	}
	out := Pipeline(p)
	if !strings.Contains(out, "build-job") { t.Error("expected build-job") }
	if !strings.Contains(out, "test-job") { t.Error("expected test-job") }
	if strings.Contains(out, ".tmpl") { t.Error(".tmpl should be excluded") }
	// Classic mode: stage → stage connector
	if !strings.Contains(out, "Classic") { t.Error("expected classic mode comment") }
}

func TestPipeline_DAGMode(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
			"test-job":  {Name: "test-job", Stage: "test", Needs: []any{"build-job"}},
		},
	}
	out := Pipeline(p)
	if !strings.Contains(out, "DAG") { t.Error("expected DAG mode comment") }
}

func TestPipeline_DAGMode_CrossPipelineNeed(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build",
				Needs: []any{map[string]any{"pipeline": "other", "job": "j"}}},
		},
	}
	out := Pipeline(p)
	// cross-pipeline need should not add an arrow (job not in local map)
	if !strings.Contains(out, "DAG") { t.Error("expected DAG mode comment") }
}

func TestPipeline_JobNoStage(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"myjob": {Name: "myjob"},
		},
	}
	out := Pipeline(p)
	if !strings.Contains(out, "myjob") { t.Error("expected myjob") }
}

func TestPipeline_SingleStage(t *testing.T) {
	// Single stage → no connector
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"a": {Name: "a", Stage: "build"},
		},
	}
	out := Pipeline(p)
	if strings.Contains(out, "-->") { t.Error("single stage should not have connectors") }
}

// ── stageID / sanitizeID ──────────────────────────────────────────────────────

func TestStageID(t *testing.T) {
	if stageID("build") != "stage_build" { t.Error("plain name") }
	if stageID("my-stage") != "stage_my_stage" { t.Error("hyphens replaced") }
}

func TestSanitizeID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"build", "build"},
		{"my-stage", "my_stage"},
		{"with space", "with_space"},
		{"ABC_123", "ABC_123"},
	}
	for _, tc := range cases {
		if sanitizeID(tc.in) != tc.want {
			t.Errorf("sanitizeID(%q)=%q want %q", tc.in, sanitizeID(tc.in), tc.want)
		}
	}
}

// ── jobClass ──────────────────────────────────────────────────────────────────

func TestJobClass(t *testing.T) {
	if jobClass(model.Job{When: "manual"}) != "manual" { t.Error("manual") }
	if jobClass(model.Job{When: "delayed"}) != "delayed" { t.Error("delayed") }
	if jobClass(model.Job{Trigger: "x"}) != "trigger" { t.Error("trigger") }
	if jobClass(model.Job{}) != "regular" { t.Error("regular") }
}

// ── needsJobName ──────────────────────────────────────────────────────────────

func TestNeedsJobName(t *testing.T) {
	if needsJobName("job-a") != "job-a" { t.Error("string form") }
	if needsJobName(map[string]any{"job": "job-b"}) != "job-b" { t.Error("map form") }
	if needsJobName(map[string]any{"pipeline": "other", "job": "j"}) != "" { t.Error("cross-pipeline should return empty") }
	if needsJobName(map[string]any{"no-job": true}) != "" { t.Error("map without job key") }
	if needsJobName(42) != "" { t.Error("unexpected type") }
}
