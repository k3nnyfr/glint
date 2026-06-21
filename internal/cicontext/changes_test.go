package cicontext

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// ── doublestarMatch ───────────────────────────────────────────────────────────

func TestDoublestarMatch(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		// exact match
		{"Dockerfile", "Dockerfile", true},
		{"Dockerfile", "dockerfile", false},
		// single-segment wildcard
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},
		// * does not cross /
		{"*.go", "src/main.go", false},
		// ** matches multiple segments
		{"**/*.go", "src/main.go", true},
		{"**/*.go", "src/pkg/util.go", true},
		{"**/*.go", "src/main.py", false},
		// ** at end matches everything
		{"src/**", "src/main.go", true},
		{"src/**", "src/a/b/c.go", true},
		{"src/**", "other/main.go", false},
		// ** in the middle
		{"src/**/*.go", "src/pkg/main.go", true},
		{"src/**/*.go", "src/a/b/main.go", true},
		{"src/**/*.go", "test/pkg/main.go", false},
		// bare ** matches everything
		{"**", "anything/and/everything.txt", true},
		{"**", "file.go", true},
		// no wildcard, multi-segment
		{"src/main.go", "src/main.go", true},
		{"src/main.go", "src/other.go", false},
		// ** matches zero segments too
		{"src/**/main.go", "src/main.go", true},
		{"src/**/main.go", "src/pkg/main.go", true},
		// malformed pattern (gracefully handled)
		{"[invalid", "file.go", false},
	}
	for _, tc := range cases {
		t.Run(tc.pattern+"~"+tc.path, func(t *testing.T) {
			got := doublestarMatch(tc.pattern, tc.path)
			if got != tc.want {
				t.Errorf("doublestarMatch(%q, %q) = %v; want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}

// ── extractChangesPaths ───────────────────────────────────────────────────────

func TestExtractChangesPaths(t *testing.T) {
	t.Run("nil → nil", func(t *testing.T) {
		if extractChangesPaths(nil) != nil {
			t.Error("expected nil")
		}
	})
	t.Run("string", func(t *testing.T) {
		got := extractChangesPaths("Dockerfile")
		if len(got) != 1 || got[0] != "Dockerfile" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("[]string", func(t *testing.T) {
		got := extractChangesPaths([]string{"a.go", "b.go"})
		if len(got) != 2 || got[0] != "a.go" || got[1] != "b.go" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("[]any strings", func(t *testing.T) {
		got := extractChangesPaths([]any{"src/**", "*.yml"})
		if len(got) != 2 || got[0] != "src/**" || got[1] != "*.yml" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("[]any ignores non-strings", func(t *testing.T) {
		got := extractChangesPaths([]any{"ok", 42})
		if len(got) != 1 || got[0] != "ok" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("map with paths key", func(t *testing.T) {
		got := extractChangesPaths(map[string]any{
			"paths":      []any{"src/**"},
			"compare_to": "origin/main",
		})
		if len(got) != 1 || got[0] != "src/**" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("map without paths key → nil", func(t *testing.T) {
		got := extractChangesPaths(map[string]any{"compare_to": "origin/main"})
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("unknown type → nil", func(t *testing.T) {
		got := extractChangesPaths(12345)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

// ── changesMatch ──────────────────────────────────────────────────────────────

func TestChangesMatch(t *testing.T) {
	t.Run("nil changes → always true", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"anything.go"})
		if !changesMatch(nil, ctx) {
			t.Error("nil changes should always match")
		}
	})
	t.Run("nil changedFiles → permissive true", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		// changedFiles is nil by default → permissive
		if !changesMatch([]any{"src/**"}, ctx) {
			t.Error("nil changedFiles should be permissive (true)")
		}
	})
	t.Run("no match → false", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"test/main_test.go"})
		if changesMatch([]any{"src/**/*.go"}, ctx) {
			t.Error("should not match: changed file not under src/")
		}
	})
	t.Run("match → true", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"src/pkg/main.go"})
		if !changesMatch([]any{"src/**/*.go"}, ctx) {
			t.Error("should match: src/pkg/main.go satisfies src/**/*.go")
		}
	})
	t.Run("empty changedFiles + non-nil changes → false", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{}) // explicit empty list
		if changesMatch([]any{"Dockerfile"}, ctx) {
			t.Error("empty file list with active filter should be false")
		}
	})
	t.Run("one of many changed files matches", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"README.md", "src/main.go", "go.sum"})
		if !changesMatch([]any{"src/**"}, ctx) {
			t.Error("src/main.go should satisfy src/**")
		}
	})
}

// ── EvalJob with rules:changes: ───────────────────────────────────────────────

func TestEvalJob_RulesChanges(t *testing.T) {
	makeJob := func(rules []model.Rule) model.Job {
		return model.Job{Name: "job", Script: []any{"echo"}, Rules: rules}
	}

	t.Run("no changed files (permissive) — rule fires", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		// changedFiles nil → permissive
		job := makeJob([]model.Rule{{Changes: []any{"src/**/*.go"}, When: "on_success"}})
		if EvalJob(job, ctx) != JobActive {
			t.Error("expected active: permissive when no file list")
		}
	})

	t.Run("matching changed file — rule fires", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"src/main.go"})
		job := makeJob([]model.Rule{{Changes: []any{"src/**"}, When: "on_success"}})
		if EvalJob(job, ctx) != JobActive {
			t.Error("expected active: src/main.go matches src/**")
		}
	})

	t.Run("no matching file — rule skipped, no other rule → skipped", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"docs/README.md"})
		job := makeJob([]model.Rule{{Changes: []any{"src/**"}, When: "on_success"}})
		if EvalJob(job, ctx) != JobSkipped {
			t.Error("expected skipped: docs/README.md does not match src/**")
		}
	})

	t.Run("changes filter with if: both must pass", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"src/main.go"})
		job := makeJob([]model.Rule{
			{If: `$CI_COMMIT_BRANCH == "develop"`, Changes: []any{"src/**"}, When: "on_success"},
		})
		// if: fails → rule skipped even though changes match
		if EvalJob(job, ctx) != JobSkipped {
			t.Error("expected skipped: if: condition fails")
		}
	})

	t.Run("map form changes: {paths: [...]}", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"Dockerfile"})
		job := makeJob([]model.Rule{{
			Changes: map[string]any{"paths": []any{"Dockerfile"}, "compare_to": "origin/main"},
			When:    "on_success",
		}})
		if EvalJob(job, ctx) != JobActive {
			t.Error("expected active: map form changes should work")
		}
	})
}

// ── EvalWorkflow with rules:changes: ─────────────────────────────────────────

func TestEvalWorkflow_Changes(t *testing.T) {
	makePipeline := func(rules []model.Rule) *model.Pipeline {
		return &model.Pipeline{Workflow: &model.Workflow{Rules: rules}}
	}

	t.Run("no changedFiles (permissive) → pipeline runs", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		p := makePipeline([]model.Rule{{Changes: []any{"src/**"}, When: "always"}})
		runs, _ := EvalWorkflow(p, ctx)
		if !runs {
			t.Error("expected pipeline to run: permissive when no file list")
		}
	})

	t.Run("matching file → pipeline runs", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"src/app.go"})
		p := makePipeline([]model.Rule{{Changes: []any{"src/**"}, When: "always"}})
		runs, _ := EvalWorkflow(p, ctx)
		if !runs {
			t.Error("expected pipeline to run: src/app.go matches src/**")
		}
	})

	t.Run("no matching file → no rule matches → pipeline blocked", func(t *testing.T) {
		ctx := New("main", "", "", nil)
		ctx.SetChangedFiles([]string{"docs/README.md"})
		p := makePipeline([]model.Rule{{Changes: []any{"src/**"}, When: "always"}})
		runs, _ := EvalWorkflow(p, ctx)
		if runs {
			t.Error("expected pipeline blocked: docs/README.md does not match src/**")
		}
	})
}

// ── SetChangedFiles / Summary ─────────────────────────────────────────────────

func TestSetChangedFiles(t *testing.T) {
	ctx := New("main", "", "", nil)
	if ctx.changedFiles != nil {
		t.Error("changedFiles should be nil initially")
	}
	ctx.SetChangedFiles([]string{"a.go", "b.go"})
	if len(ctx.changedFiles) != 2 {
		t.Errorf("expected 2 changed files, got %d", len(ctx.changedFiles))
	}
	// nil receiver should not panic
	var nilCtx *Context
	nilCtx.SetChangedFiles([]string{"x"})
}

func TestSummary_WithChangedFiles(t *testing.T) {
	ctx := New("main", "", "", nil)
	// Without changed files
	if s := ctx.Summary(); s != "branch=main, source=push" {
		t.Errorf("unexpected summary without changes: %q", s)
	}
	// With changed files
	ctx.SetChangedFiles([]string{"a.go", "b.go"})
	s := ctx.Summary()
	if s != "branch=main, source=push, 2 changed file(s)" {
		t.Errorf("unexpected summary with changes: %q", s)
	}
	// With empty changed files (strict, 0 files)
	ctx.SetChangedFiles([]string{})
	s = ctx.Summary()
	if s != "branch=main, source=push, 0 changed file(s)" {
		t.Errorf("unexpected summary with 0 changes: %q", s)
	}
}
