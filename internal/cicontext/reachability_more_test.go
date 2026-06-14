package cicontext

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// ── JobState.String ───────────────────────────────────────────────────────────

func TestJobStateString(t *testing.T) {
	if JobActive.String() != "active" { t.Errorf("active: got %q", JobActive.String()) }
	if JobManual.String() != "manual" { t.Errorf("manual: got %q", JobManual.String()) }
	if JobSkipped.String() != "skipped" { t.Errorf("skipped: got %q", JobSkipped.String()) }
}

// ── whenToState ───────────────────────────────────────────────────────────────

func TestWhenToState(t *testing.T) {
	cases := []struct { when string; want JobState }{
		{"never",      JobSkipped},
		{"manual",     JobManual},
		{"on_success", JobActive},
		{"always",     JobActive},
		{"",           JobActive},
	}
	for _, tc := range cases {
		got := whenToState(tc.when)
		if got != tc.want {
			t.Errorf("whenToState(%q)=%v want %v", tc.when, got, tc.want)
		}
	}
}

// ── EvalJob: only/except paths ────────────────────────────────────────────────

func TestEvalJob_OnlyExcept(t *testing.T) {
	ctx := New("main", "", "", nil)

	t.Run("only branches matches", func(t *testing.T) {
		job := model.Job{Only: []any{"branches"}}
		if EvalJob(job, ctx) != JobActive {
			t.Error("expected active on branch")
		}
	})

	t.Run("only tags excludes branch push", func(t *testing.T) {
		job := model.Job{Only: []any{"tags"}}
		if EvalJob(job, ctx) != JobSkipped {
			t.Error("expected skipped — we are on a branch, not tag")
		}
	})

	t.Run("except branches skips on branch push", func(t *testing.T) {
		job := model.Job{Script: "echo ok", Except: []any{"branches"}}
		if EvalJob(job, ctx) != JobSkipped {
			t.Error("expected skipped — branches excluded")
		}
	})

	t.Run("except non-matching source does not skip", func(t *testing.T) {
		job := model.Job{Except: []any{"schedules"}}
		if EvalJob(job, ctx) != JobActive {
			t.Error("expected active — schedule is not the source")
		}
	})

	t.Run("job when: manual with no filter", func(t *testing.T) {
		job := model.Job{When: "manual"}
		if EvalJob(job, ctx) != JobManual {
			t.Error("expected manual")
		}
	})

	t.Run("empty context returns active", func(t *testing.T) {
		emptyCtx := New("", "", "", nil)
		job := model.Job{Only: []any{"tags"}}
		if EvalJob(job, emptyCtx) != JobActive {
			t.Error("empty context should always return active")
		}
	})
}

// ── EvalJob: rules path ───────────────────────────────────────────────────────

func TestEvalJob_Rules(t *testing.T) {
	ctx := New("main", "", "", nil)

	t.Run("rule with never when excludes", func(t *testing.T) {
		job := model.Job{Rules: []model.Rule{{When: "never"}}}
		if EvalJob(job, ctx) != JobSkipped {
			t.Error("expected skipped — when:never")
		}
	})

	t.Run("no matching rule → skipped", func(t *testing.T) {
		job := model.Job{Rules: []model.Rule{
			{If: "$CI_COMMIT_BRANCH == \"develop\"", When: "on_success"},
		}}
		if EvalJob(job, ctx) != JobSkipped {
			t.Error("expected skipped — no rule matches main branch with develop condition")
		}
	})
}

// ── onlyMatches / exceptMatches ───────────────────────────────────────────────

func TestOnlyMatches(t *testing.T) {
	branchCtx := New("main", "", "", nil)
	tagCtx    := New("", "v1.0", "", nil)
	mrCtx     := New("", "", "merge_request_event", nil)

	cases := []struct {
		name string
		only any
		ctx  *Context
		want bool
	}{
		{"nil only always matches", nil, branchCtx, true},
		{"unrecognised form permissive", 42, branchCtx, true},
		{"string form — branches keyword", "branches", branchCtx, true},
		{"string form — tags keyword miss", "tags", branchCtx, false},
		{"slice — merge_requests keyword", []any{"merge_requests"}, mrCtx, true},
		{"slice — schedules keyword", []any{"schedules"}, branchCtx, false},
		{"slice — pipelines keyword", []any{"pipelines"}, New("","","pipeline",nil), true},
		{"slice — pushes keyword", []any{"pushes"}, New("","","push",nil), true},
		{"slice — web keyword", []any{"web"}, New("","","web",nil), true},
		{"slice — api keyword", []any{"api"}, New("","","api",nil), true},
		{"slice — tag keyword match", []any{"tags"}, tagCtx, true},
		{"glob pattern matches", []any{"feat/*"}, New("feat/abc","","",nil), true},
		{"glob no match", []any{"feat/*"}, branchCtx, false},
		{"regex pattern matches branch", []any{"/^main.*/"}, branchCtx, true},
		{"regex pattern no match", []any{"/^develop/"}, branchCtx, false},
		{"invalid regex treated as glob", []any{"/[invalid/"}, branchCtx, false},
		{"map form with refs key", map[string]any{"refs": []any{"branches"}}, branchCtx, true},
		{"map form no refs — permissive", map[string]any{"variables": []any{}}, branchCtx, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := onlyMatches(tc.only, tc.ctx)
			if got != tc.want {
				t.Errorf("onlyMatches: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestExceptMatches(t *testing.T) {
	branchCtx := New("main", "", "", nil)

	cases := []struct {
		name   string
		except any
		want   bool
	}{
		{"nil except never excludes", nil, false},
		{"unrecognised form not excluded", 42, false},
		{"branches excludes on branch push", []any{"branches"}, true},
		{"tags does not exclude branch push", []any{"tags"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exceptMatches(tc.except, branchCtx)
			if got != tc.want {
				t.Errorf("exceptMatches: got %v want %v", got, tc.want)
			}
		})
	}
}

// ── extractRefList ────────────────────────────────────────────────────────────

func TestExtractRefList(t *testing.T) {
	cases := []struct {
		name  string
		input any
		wantN int // -1 means nil expected
	}{
		{"string", "branches", 1},
		{"slice of strings", []any{"a", "b", "c"}, 3},
		{"slice with non-string items skipped", []any{"a", 42, "b"}, 2},
		{"map with refs key", map[string]any{"refs": []any{"branches"}}, 1},
		{"map without refs key returns nil", map[string]any{"variables": nil}, -1},
		{"unknown type returns nil", 123, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRefList(tc.input)
			if tc.wantN == -1 {
				if got != nil { t.Errorf("expected nil, got %v", got) }
			} else {
				if len(got) != tc.wantN { t.Errorf("len: got %d want %d (%v)", len(got), tc.wantN, got) }
			}
		})
	}
}

// ── globMatches / matchGlob ───────────────────────────────────────────────────

func TestGlobMatches(t *testing.T) {
	cases := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"main", "main", true},
		{"main", "develop", false},
		{"feat/*", "feat/abc", true},
		{"feat/*", "feat/", true},
		{"feat/*", "main", false},
		{"*", "anything", true},
		{"*", "", true},
		{"prefix*suffix", "prefixMIDDLEsuffix", true},
		{"prefix*suffix", "prefixsuffix", true},
		{"prefix*suffix", "prefixNO", false},
		{"a*b*c", "aXbYc", true},
		{"a*b*c", "abc", true},
	}
	for _, tc := range cases {
		t.Run(tc.pattern+"/"+tc.s, func(t *testing.T) {
			got := globMatches(tc.pattern, tc.s)
			if got != tc.want {
				t.Errorf("globMatches(%q, %q)=%v want %v", tc.pattern, tc.s, got, tc.want)
			}
		})
	}
}

// ── refPatternMatches ─────────────────────────────────────────────────────────

func TestRefPatternMatches(t *testing.T) {
	cases := []struct {
		pattern string
		ref     string
		want    bool
	}{
		{"/^main$/", "main", true},
		{"/^main$/", "develop", false},
		{"/feature/", "feature/abc", true},
		{"feat/*", "feat/x", true},
		{"main", "main", true},
		{"main", "develop", false},
		{"", "main", false},    // empty ref for pattern with no wildcard
	}
	for _, tc := range cases {
		t.Run(tc.pattern+"@"+tc.ref, func(t *testing.T) {
			got := refPatternMatches(tc.pattern, tc.ref)
			if got != tc.want {
				t.Errorf("refPatternMatches(%q, %q)=%v want %v", tc.pattern, tc.ref, got, tc.want)
			}
		})
	}
}

// TestRefListMatches_Schedule verifies the "schedules" keyword matches when
// CI_PIPELINE_SOURCE is "schedule".
func TestRefListMatches_Schedule(t *testing.T) {
	ctx := New("", "", "schedule", nil)
	if !refListMatches([]string{"schedules"}, ctx) {
		t.Error("schedules keyword should match when source is 'schedule'")
	}
}
