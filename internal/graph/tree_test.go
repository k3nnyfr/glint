package graph

import (
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/model"
)

func makeSimplePipeline() *model.Pipeline {
	return &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
			"test-job":  {Name: "test-job", Stage: "test"},
			".template": {Name: ".template", Stage: "build"},
		},
	}
}

func TestTree_Basic(t *testing.T) {
	p := makeSimplePipeline()
	out := Tree(p, nil)
	if !strings.Contains(out, "pipeline") {
		t.Error("expected 'pipeline' root node")
	}
	if !strings.Contains(out, "build-job") {
		t.Error("expected build-job in tree")
	}
	if !strings.Contains(out, "test-job") {
		t.Error("expected test-job in tree")
	}
	// hidden template jobs should not appear
	if strings.Contains(out, ".template") {
		t.Error(".template job should be excluded from tree")
	}
}

func TestTree_WithContext_Skipped(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"run-on-tag": {Name: "run-on-tag", Stage: "build", Rules: []model.Rule{
				{If: `$CI_COMMIT_TAG != ""`, When: "on_success"},
			}},
		},
	}
	ctx := cicontext.New("main", "", "", nil)
	out := Tree(p, ctx)
	if !strings.Contains(out, "[skipped]") {
		t.Errorf("expected [skipped] annotation, got:\n%s", out)
	}
}

func TestTree_WithContext_Manual(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"deploy"},
		Jobs: map[string]model.Job{
			"deploy": {Name: "deploy", Stage: "deploy", When: "manual"},
		},
	}
	ctx := cicontext.New("main", "", "", nil)
	out := Tree(p, ctx)
	if !strings.Contains(out, "[manual]") {
		t.Errorf("expected [manual] annotation, got:\n%s", out)
	}
}

func TestTree_NoContext_Annotations(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"manual-job":  {Name: "manual-job", Stage: "build", When: "manual"},
			"delayed-job": {Name: "delayed-job", Stage: "build", When: "delayed"},
			"trigger-job": {Name: "trigger-job", Stage: "build", Trigger: "other/project"},
		},
	}
	out := Tree(p, nil)
	if !strings.Contains(out, "[manual]") { t.Error("expected [manual]") }
	if !strings.Contains(out, "[delayed]") { t.Error("expected [delayed]") }
	if !strings.Contains(out, "[trigger]") { t.Error("expected [trigger]") }
}

func TestTree_JobWithNoStage(t *testing.T) {
	// Jobs without a stage should default to "test"
	p := &model.Pipeline{
		Stages: []string{"test"},
		Jobs: map[string]model.Job{
			"nostage": {Name: "nostage"},
		},
	}
	out := Tree(p, nil)
	if !strings.Contains(out, "test") { t.Error("expected default stage 'test'") }
}

func TestTree_UndeclaredStage(t *testing.T) {
	// Job in a stage not listed in p.Stages — should still appear
	p := &model.Pipeline{
		Stages: []string{},
		Jobs: map[string]model.Job{
			"myjob": {Name: "myjob", Stage: "custom"},
		},
	}
	out := Tree(p, nil)
	if !strings.Contains(out, "myjob") { t.Error("expected myjob in output") }
}

// ── branchChars ───────────────────────────────────────────────────────────────

func TestBranchChars(t *testing.T) {
	b, c := branchChars(true)
	if b != "└── " || c != "    " {
		t.Errorf("last: branch=%q cont=%q", b, c)
	}
	b, c = branchChars(false)
	if b != "├── " || c != "│   " {
		t.Errorf("not-last: branch=%q cont=%q", b, c)
	}
}

// ── jobLabel ─────────────────────────────────────────────────────────────────

func TestJobLabel(t *testing.T) {
	job := model.Job{Name: "j"}
	if jobLabel(job, "j", nil) != "j" {
		t.Error("plain job should have plain label")
	}

	// Multiple tags
	job2 := model.Job{Name: "j2", When: "manual", Trigger: "other"}
	label := jobLabel(job2, "j2", nil)
	if !strings.Contains(label, "manual") || !strings.Contains(label, "trigger") {
		t.Errorf("expected manual+trigger in label: %q", label)
	}

	// With context — active job has no annotation
	emptyCtx := &cicontext.Context{}
	if jobLabel(model.Job{}, "j", emptyCtx) != "j" {
		t.Error("active job with context should have no annotation")
	}
}

// TestJobLabel_ActiveWithContext covers the 'return name' path when a job runs
// normally (neither skipped nor manual) with a non-empty context.
func TestJobLabel_ActiveWithContext(t *testing.T) {
	// Branch context with a job that has no rules → the job is active.
	ctx := cicontext.New("main", "", "", nil)
	activeJob := model.Job{Name: "build", Script: []any{"make"}}
	label := jobLabel(activeJob, "build", ctx)
	if label != "build" {
		t.Errorf("active job with context: expected 'build', got %q", label)
	}
}
