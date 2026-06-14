package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// ── checkWhen ─────────────────────────────────────────────────────────────────

func TestCheckWhen(t *testing.T) {
	cases := []struct {
		name    string
		job     model.Job
		wantN   int
		wantRule string
	}{
		{"valid when", model.Job{When: "on_success"}, 0, ""},
		{"invalid when", model.Job{When: "bad_value"}, 1, RuleInvalidWhen},
		{"delayed requires start_in", model.Job{When: "delayed"}, 1, RuleDelayedNoStartIn},
		{"delayed with start_in", model.Job{When: "delayed", StartIn: "30 minutes"}, 0, ""},
		{"start_in without delayed", model.Job{When: "on_success", StartIn: "5m"}, 1, RuleStartInNoDelayed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkWhen("j", tc.job)
			if len(got) != tc.wantN {
				t.Errorf("got %d findings want %d: %v", len(got), tc.wantN, got)
			}
			if tc.wantRule != "" && len(got) > 0 && got[0].Rule != tc.wantRule {
				t.Errorf("rule: got %q want %q", got[0].Rule, tc.wantRule)
			}
		})
	}
}

// ── checkParallel ─────────────────────────────────────────────────────────────

func TestCheckParallel(t *testing.T) {
	cases := []struct {
		name  string
		job   model.Job
		wantN int
	}{
		{"nil parallel", model.Job{}, 0},
		{"valid int", model.Job{Parallel: 4}, 0},
		{"int too low", model.Job{Parallel: 1}, 1},
		{"int too high", model.Job{Parallel: 201}, 1},
		{"map with matrix", model.Job{Parallel: map[string]any{"matrix": []any{}}}, 0},
		{"map without matrix", model.Job{Parallel: map[string]any{"other": true}}, 1},
		{"invalid type", model.Job{Parallel: "string"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkParallel("j", tc.job)
			if len(got) != tc.wantN {
				t.Errorf("got %d findings want %d", len(got), tc.wantN)
			}
		})
	}
}

// ── checkRetry ────────────────────────────────────────────────────────────────

func TestCheckRetry(t *testing.T) {
	cases := []struct {
		name  string
		job   model.Job
		wantN int
	}{
		{"nil retry", model.Job{}, 0},
		{"valid int 0", model.Job{Retry: 0}, 0},
		{"valid int 2", model.Job{Retry: 2}, 0},
		{"invalid int -1", model.Job{Retry: -1}, 1},
		{"invalid int 3", model.Job{Retry: 3}, 1},
		{"map with valid max", model.Job{Retry: map[string]any{"max": 1}}, 0},
		{"map with invalid max", model.Job{Retry: map[string]any{"max": 5}}, 1},
		{"map with valid when string", model.Job{Retry: map[string]any{"when": "always"}}, 0},
		{"map with invalid when string", model.Job{Retry: map[string]any{"when": "bad_reason"}}, 1},
		{"map with when slice", model.Job{Retry: map[string]any{"when": []any{"always", "script_failure"}}}, 0},
		{"map with when slice invalid", model.Job{Retry: map[string]any{"when": []any{"bad_reason"}}}, 1},
		{"invalid type", model.Job{Retry: "string"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkRetry("j", tc.job)
			if len(got) != tc.wantN {
				t.Errorf("got %d findings want %d: %v", len(got), tc.wantN, got)
			}
		})
	}
}

// ── checkAllowFailure ─────────────────────────────────────────────────────────

func TestCheckAllowFailure(t *testing.T) {
	cases := []struct {
		name  string
		job   model.Job
		wantN int
	}{
		{"nil", model.Job{}, 0},
		{"bool true", model.Job{Allow: true}, 0},
		{"bool false", model.Job{Allow: false}, 0},
		{"map with exit_codes", model.Job{Allow: map[string]any{"exit_codes": []any{1, 2}}}, 0},
		{"map without exit_codes", model.Job{Allow: map[string]any{"other": true}}, 1},
		{"invalid type", model.Job{Allow: "yes"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkAllowFailure("j", tc.job)
			if len(got) != tc.wantN {
				t.Errorf("got %d findings want %d", len(got), tc.wantN)
			}
		})
	}
}

// ── checkInterruptible ────────────────────────────────────────────────────────

func TestCheckInterruptible(t *testing.T) {
	if len(checkInterruptible("j", model.Job{})) != 0 { t.Error("nil: expected clean") }
	if len(checkInterruptible("j", model.Job{Interruptible: true})) != 0 { t.Error("bool true: expected clean") }
	if len(checkInterruptible("j", model.Job{Interruptible: false})) != 0 { t.Error("bool false: expected clean") }
	if len(checkInterruptible("j", model.Job{Interruptible: "yes"})) != 1 { t.Error("string: expected error") }
}

// ── checkTrigger ──────────────────────────────────────────────────────────────

func TestCheckTrigger(t *testing.T) {
	cases := []struct {
		name  string
		job   model.Job
		wantN int
	}{
		{"nil trigger", model.Job{}, 0},
		{"string trigger", model.Job{Trigger: "other/project"}, 0},
		{"trigger with script", model.Job{Trigger: "x", Script: []any{"echo"}}, 1},
		{"map trigger with project", model.Job{Trigger: map[string]any{"project": "g/p"}}, 0},
		{"map trigger with include", model.Job{Trigger: map[string]any{"include": "ci.yml"}}, 0},
		{"map trigger missing both", model.Job{Trigger: map[string]any{"strategy": "depend"}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkTrigger("j", tc.job)
			if len(got) != tc.wantN {
				t.Errorf("got %d findings want %d: %v", len(got), tc.wantN, got)
			}
		})
	}
}

// ── checkCoverage ─────────────────────────────────────────────────────────────

func TestCheckCoverage(t *testing.T) {
	if len(checkCoverage("j", model.Job{})) != 0 { t.Error("empty: clean") }
	if len(checkCoverage("j", model.Job{Coverage: `/\d+%/`})) != 0 { t.Error("valid: clean") }
	if len(checkCoverage("j", model.Job{Coverage: "bad"})) != 1 { t.Error("missing slashes: error") }
}

// ── checkRelease ──────────────────────────────────────────────────────────────

func TestCheckRelease(t *testing.T) {
	if len(checkRelease("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkRelease("j", model.Job{Release: "string"})) != 1 { t.Error("string: error") }
	if len(checkRelease("j", model.Job{Release: map[string]any{"tag_name": "v1"}})) != 0 { t.Error("valid map: clean") }
	if len(checkRelease("j", model.Job{Release: map[string]any{"description": "x"}})) != 1 { t.Error("no tag_name: error") }
	if len(checkRelease("j", model.Job{Release: map[string]any{"tag_name": ""}})) != 1 { t.Error("empty tag_name: error") }
	if len(checkRelease("j", model.Job{Release: map[string]any{"tag_name": nil}})) != 1 { t.Error("nil tag_name: error") }
}

// ── checkEnvironment ──────────────────────────────────────────────────────────

func TestCheckEnvironment(t *testing.T) {
	if len(checkEnvironment("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkEnvironment("j", model.Job{Environment: "production"})) != 0 { t.Error("string: clean") }
	if len(checkEnvironment("j", model.Job{Environment: map[string]any{"name": "prod"}})) != 0 { t.Error("map with name: clean") }
	// url without name
	if len(checkEnvironment("j", model.Job{Environment: map[string]any{"url": "https://x.com"}})) != 1 { t.Error("url no name: error") }
	// invalid action
	if len(checkEnvironment("j", model.Job{Environment: map[string]any{"name": "prod", "action": "badaction"}})) != 1 { t.Error("bad action: error") }
	// valid action
	if len(checkEnvironment("j", model.Job{Environment: map[string]any{"name": "prod", "action": "stop"}})) != 0 { t.Error("stop action: clean") }
}

// ── checkArtifacts ────────────────────────────────────────────────────────────

func TestCheckArtifacts(t *testing.T) {
	if len(checkArtifacts("j", model.Job{})) != 0 { t.Error("nil: clean") }
	// non-map type: no-op
	if len(checkArtifacts("j", model.Job{Artifacts: "string"})) != 0 { t.Error("string: clean") }
	// valid when
	if len(checkArtifacts("j", model.Job{Artifacts: map[string]any{"when": "on_failure"}})) != 0 { t.Error("valid when: clean") }
	// invalid when
	if len(checkArtifacts("j", model.Job{Artifacts: map[string]any{"when": "bad_when"}})) != 1 { t.Error("bad when: error") }
	// expose_as without paths
	if len(checkArtifacts("j", model.Job{Artifacts: map[string]any{"expose_as": "Coverage"}})) != 1 { t.Error("expose_as no paths: error") }
	// expose_as with paths
	if len(checkArtifacts("j", model.Job{Artifacts: map[string]any{"expose_as": "X", "paths": []any{"out/"}}})) != 0 { t.Error("expose_as with paths: clean") }
	// pages job without public in paths
	if len(checkArtifacts("pages", model.Job{Name: "pages", Artifacts: map[string]any{"paths": []any{"dist/"}}})) != 1 { t.Error("pages without public: warning") }
	// pages job with public in paths
	if len(checkArtifacts("pages", model.Job{Name: "pages", Artifacts: map[string]any{"paths": []any{"public"}}})) != 0 { t.Error("pages with public: clean") }
	// pages job with pages: keyword set — GL033 check skipped
	if len(checkArtifacts("pages", model.Job{Pages: map[string]any{}, Artifacts: map[string]any{"paths": []any{"dist/"}}})) != 0 { t.Error("pages with pages keyword: clean") }
}

// ── checkCache ────────────────────────────────────────────────────────────────

func TestCheckCache(t *testing.T) {
	if len(checkCache("j", model.Job{})) != 0 { t.Error("nil: clean") }
	// map form valid
	if len(checkCache("j", model.Job{Cache: map[string]any{"key": "abc"}})) != 0 { t.Error("map valid: clean") }
	// map form invalid when
	if len(checkCache("j", model.Job{Cache: map[string]any{"when": "bad"}})) != 1 { t.Error("invalid when: error") }
	// map form invalid policy
	if len(checkCache("j", model.Job{Cache: map[string]any{"policy": "bad_policy"}})) != 1 { t.Error("invalid policy: error") }
	// slice form
	if len(checkCache("j", model.Job{Cache: []any{
		map[string]any{"when": "bad"},
		map[string]any{"policy": "pull"},
	}})) != 1 { t.Error("slice with one bad: error") }
}

// ── checkRules ────────────────────────────────────────────────────────────────

func TestCheckRules(t *testing.T) {
	if len(checkRules("j", model.Job{})) != 0 { t.Error("no rules: clean") }
	if len(checkRules("j", model.Job{Rules: []model.Rule{{When: "on_success"}}})) != 0 { t.Error("valid when: clean") }
	if len(checkRules("j", model.Job{Rules: []model.Rule{{When: "bad_when"}}})) != 1 { t.Error("bad when: error") }
}

// ── checkImage ────────────────────────────────────────────────────────────────

func TestCheckImage(t *testing.T) {
	if len(checkImage("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkImage("j", model.Job{Image: "alpine"})) != 0 { t.Error("string: clean") }
	if len(checkImage("j", model.Job{Image: map[string]any{"name": "alpine"}})) != 0 { t.Error("map with name: clean") }
	if len(checkImage("j", model.Job{Image: map[string]any{"pull_policy": "always"}})) != 1 { t.Error("map without name: error") }
}

// ── checkInherit ──────────────────────────────────────────────────────────────

func TestCheckInherit(t *testing.T) {
	if len(checkInherit("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkInherit("j", model.Job{Inherit: "string"})) != 0 { t.Error("non-map: clean") }
	if len(checkInherit("j", model.Job{Inherit: map[string]any{"default": true}})) != 0 { t.Error("bool: clean") }
	if len(checkInherit("j", model.Job{Inherit: map[string]any{"default": []any{"image"}}})) != 0 { t.Error("list: clean") }
	if len(checkInherit("j", model.Job{Inherit: map[string]any{"variables": "string"}})) != 1 { t.Error("invalid type: error") }
}

// ── checkServices ─────────────────────────────────────────────────────────────

func TestCheckServices(t *testing.T) {
	if len(checkServices("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkServices("j", model.Job{Services: []any{"postgres"}})) != 0 { t.Error("string: clean") }
	if len(checkServices("j", model.Job{Services: []any{map[string]any{"name": "pg"}}})) != 0 { t.Error("valid map: clean") }
	if len(checkServices("j", model.Job{Services: []any{map[string]any{"port": 5432}}})) != 1 { t.Error("no name: error") }
	if len(checkServices("j", model.Job{Services: []any{map[string]any{"name": "pg", "alias": "invalid alias!"}}})) != 1 { t.Error("bad alias: error") }
	if len(checkServices("j", model.Job{Services: []any{map[string]any{"name": "pg", "alias": "valid-alias"}}})) != 0 { t.Error("good alias: clean") }
}

// ── checkRulesGlobs ───────────────────────────────────────────────────────────

func TestCheckRulesGlobs(t *testing.T) {
	if len(checkRulesGlobs("j", model.Job{})) != 0 { t.Error("no rules: clean") }
	// Relative path: clean
	relRule := model.Rule{Changes: []any{"src/**/*.go"}}
	if len(checkRulesGlobs("j", model.Job{Rules: []model.Rule{relRule}})) != 0 { t.Error("relative: clean") }
	// Absolute path: warning
	absRule := model.Rule{Changes: []any{"/absolute/path"}}
	if len(checkRulesGlobs("j", model.Job{Rules: []model.Rule{absRule}})) != 1 { t.Error("absolute: warning") }
	// Map form with paths
	mapRule := model.Rule{Changes: map[string]any{"paths": []any{"/abs"}}}
	if len(checkRulesGlobs("j", model.Job{Rules: []model.Rule{mapRule}})) != 1 { t.Error("map absolute: warning") }
	// exists
	existsRule := model.Rule{Exists: []any{"/bad"}}
	if len(checkRulesGlobs("j", model.Job{Rules: []model.Rule{existsRule}})) != 1 { t.Error("exists absolute: warning") }
}

// ── checkTimeout ─────────────────────────────────────────────────────────────

func TestCheckTimeout(t *testing.T) {
	if len(checkTimeout("j", model.Job{})) != 0 { t.Error("empty: clean") }
	if len(checkTimeout("j", model.Job{Timeout: "1h 30m"})) != 0 { t.Error("valid: clean") }
	if len(checkTimeout("j", model.Job{Timeout: "90 minutes"})) != 0 { t.Error("valid2: clean") }
	if len(checkTimeout("j", model.Job{Timeout: "not-a-duration"})) != 1 { t.Error("invalid: error") }
}

func TestCheckDefaultTimeout(t *testing.T) {
	if len(checkDefaultTimeout("", "f.yml")) != 0 { t.Error("empty: clean") }
	if len(checkDefaultTimeout("2 hours", "f.yml")) != 0 { t.Error("valid: clean") }
	if len(checkDefaultTimeout("bad", "f.yml")) != 1 { t.Error("invalid: error") }
}

// ── checkIDTokens ─────────────────────────────────────────────────────────────

func TestCheckIDTokens(t *testing.T) {
	if len(checkIDTokens("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkIDTokens("j", model.Job{IDTokens: "string"})) != 0 { t.Error("non-map: clean") }
	// valid token with aud
	if len(checkIDTokens("j", model.Job{IDTokens: map[string]any{
		"MY_TOKEN": map[string]any{"aud": "https://example.com"},
	}})) != 0 { t.Error("valid: clean") }
	// missing aud
	if len(checkIDTokens("j", model.Job{IDTokens: map[string]any{
		"MY_TOKEN": map[string]any{"other": "x"},
	}})) != 1 { t.Error("missing aud: error") }
	// not a map
	if len(checkIDTokens("j", model.Job{IDTokens: map[string]any{
		"MY_TOKEN": "string",
	}})) != 1 { t.Error("token not a map: error") }
}

// ── checkSecrets ─────────────────────────────────────────────────────────────

func TestCheckSecrets(t *testing.T) {
	if len(checkSecrets("j", model.Job{})) != 0 { t.Error("nil: clean") }
	if len(checkSecrets("j", model.Job{Secrets: "string"})) != 0 { t.Error("non-map: clean") }
	// valid vault
	if len(checkSecrets("j", model.Job{Secrets: map[string]any{
		"MY_SECRET": map[string]any{"vault": map[string]any{"engine": map[string]any{"name": "kv", "path": "x"}, "path": "y", "field": "z"}},
	}})) != 0 { t.Error("vault: clean") }
	// missing provider
	if len(checkSecrets("j", model.Job{Secrets: map[string]any{
		"MY_SECRET": map[string]any{"other": "x"},
	}})) != 1 { t.Error("missing provider: error") }
	// not a map
	if len(checkSecrets("j", model.Job{Secrets: map[string]any{
		"MY_SECRET": "string",
	}})) != 1 { t.Error("not a map: error") }
}

// ── checkPagesKeyword ─────────────────────────────────────────────────────────

func TestCheckPagesKeyword(t *testing.T) {
	if len(checkPagesKeyword("j", model.Job{})) != 0 { t.Error("nil pages: clean") }
	// pages keyword but no artifacts
	if len(checkPagesKeyword("j", model.Job{Pages: map[string]any{}})) != 1 { t.Error("no artifacts: warning") }
	// pages keyword with artifacts but no paths
	if len(checkPagesKeyword("j", model.Job{
		Pages:     map[string]any{},
		Artifacts: map[string]any{},
	})) != 1 { t.Error("no paths: warning") }
	// pages with public in paths
	if len(checkPagesKeyword("j", model.Job{
		Pages:     map[string]any{},
		Artifacts: map[string]any{"paths": []any{"public"}},
	})) != 0 { t.Error("public in paths: clean") }
	// pages with custom publish dir and matching path
	if len(checkPagesKeyword("j", model.Job{
		Pages:     map[string]any{"publish": "dist"},
		Artifacts: map[string]any{"paths": []any{"dist"}},
	})) != 0 { t.Error("custom publish match: clean") }
	// pages with custom publish dir missing from paths
	if len(checkPagesKeyword("j", model.Job{
		Pages:     map[string]any{"publish": "dist"},
		Artifacts: map[string]any{"paths": []any{"public"}},
	})) != 1 { t.Error("custom publish missing: warning") }
	// artifacts is a non-map (unexpected)
	if len(checkPagesKeyword("j", model.Job{
		Pages:     map[string]any{},
		Artifacts: "string",
	})) != 0 { t.Error("non-map artifacts: clean") }
}

// ── checkCacheKeyFiles ────────────────────────────────────────────────────────

func TestCheckCacheKeyFiles(t *testing.T) {
	if len(checkCacheKeyFiles("j", model.Job{})) != 0 { t.Error("nil: clean") }
	// valid key.files
	if len(checkCacheKeyFiles("j", model.Job{Cache: map[string]any{
		"key": map[string]any{"files": []any{"Gemfile.lock"}},
	}})) != 0 { t.Error("valid: clean") }
	// glob pattern in key.files
	if len(checkCacheKeyFiles("j", model.Job{Cache: map[string]any{
		"key": map[string]any{"files": []any{"*.lock"}},
	}})) != 1 { t.Error("glob: warning") }
	// files is not a list
	if len(checkCacheKeyFiles("j", model.Job{Cache: map[string]any{
		"key": map[string]any{"files": "Gemfile.lock"},
	}})) != 1 { t.Error("non-list: error") }
	// no key
	if len(checkCacheKeyFiles("j", model.Job{Cache: map[string]any{}})) != 0 { t.Error("no key: clean") }
	// key is not a map
	if len(checkCacheKeyFiles("j", model.Job{Cache: map[string]any{"key": "string"}})) != 0 { t.Error("key string: clean") }
	// slice cache form
	if len(checkCacheKeyFiles("j", model.Job{Cache: []any{
		map[string]any{"key": map[string]any{"files": []any{"*.lock"}}},
	}})) != 1 { t.Error("slice cache with glob: warning") }
}

// ── linter.go level checks ────────────────────────────────────────────────────

func TestCheckStages(t *testing.T) {
	// no stages → warning
	p := &model.Pipeline{}
	if len(checkStages(p)) != 1 { t.Error("no stages: warning") }
	// with stages → clean
	p2 := &model.Pipeline{Stages: []string{"build"}}
	if len(checkStages(p2)) != 0 { t.Error("with stages: clean") }
}

func TestCheckDuplicateStages(t *testing.T) {
	p := &model.Pipeline{Stages: []string{"build", "test", "build"}}
	if len(checkDuplicateStages(p)) != 1 { t.Error("duplicate: warning") }
	p2 := &model.Pipeline{Stages: []string{"build", "test"}}
	if len(checkDuplicateStages(p2)) != 0 { t.Error("no duplicate: clean") }
}

func TestCheckWorkflow(t *testing.T) {
	p := &model.Pipeline{Workflow: &model.Workflow{Rules: []model.Rule{{When: "always"}}}}
	if len(checkWorkflow(p)) != 0 { t.Error("valid when: clean") }
	p2 := &model.Pipeline{Workflow: &model.Workflow{Rules: []model.Rule{{When: "bad"}}}}
	if len(checkWorkflow(p2)) != 1 { t.Error("invalid when: error") }
	p3 := &model.Pipeline{}
	if len(checkWorkflow(p3)) != 0 { t.Error("nil workflow: clean") }
}

func TestFindingString(t *testing.T) {
	f := Finding{
		Severity: Error,
		Rule:     "GL001",
		Job:      "build",
		File:     "ci.yml",
		Line:     10,
		Message:  "missing script",
	}
	s := f.String()
	if s == "" { t.Error("expected non-empty string") }

	// Without file/line/job
	f2 := Finding{Severity: Warning, Rule: "GL002", Message: "test"}
	s2 := f2.String()
	if s2 == "" { t.Error("expected non-empty string") }

	// With file but no line
	f3 := Finding{Severity: Error, Rule: "GL003", File: "ci.yml", Message: "x"}
	if f3.String() == "" { t.Error("expected non-empty string") }
}

func TestScriptNonEmpty(t *testing.T) {
	if scriptNonEmpty(nil) { t.Error("nil") }
	if scriptNonEmpty([]any{}) { t.Error("empty slice") }
	if !scriptNonEmpty([]any{"echo"}) { t.Error("non-empty slice") }
	if !scriptNonEmpty("echo") { t.Error("string") }
	if scriptNonEmpty("") { t.Error("empty string") }
}

// ── checkNeeds ────────────────────────────────────────────────────────────────

func TestCheckNeeds(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build", Script: []any{"make"}},
			"test-job": {Name: "test-job", Stage: "test", Script: []any{"test"},
				Needs: []any{"build-job"}},
		},
	}
	if len(checkNeeds(p)) != 0 { t.Error("valid needs: clean") }

	// needs unknown job
	p2 := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {Name: "test-job", Stage: "test", Needs: []any{"nonexistent"}},
		},
	}
	if len(checkNeeds(p2)) != 1 { t.Error("unknown needs: error") }

	// optional: true for unknown job → warning not error
	p3 := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {Name: "test-job", Stage: "test",
				Needs: []any{map[string]any{"job": "ghost", "optional": true}}},
		},
	}
	findings := checkNeeds(p3)
	if len(findings) != 1 || findings[0].Severity != Warning {
		t.Error("optional unknown needs: warning")
	}

	// cross-pipeline need (ignored)
	p4 := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {Name: "test-job",
				Needs: []any{map[string]any{"pipeline": "other", "job": "j"}}},
		},
	}
	if len(checkNeeds(p4)) != 0 { t.Error("cross-pipeline: clean") }

	// needs later stage → error
	p5 := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build", Script: []any{"make"}},
			"early-job": {Name: "early-job", Stage: "build",
				Needs: []any{"build-job"}, Script: []any{"echo"}},
			"test-job": {Name: "test-job", Stage: "test", Script: []any{"test"},
				Needs: []any{"build-job"}}, // ok
		},
	}
	// Only cross-stage ordering violations should be found
	_ = checkNeeds(p5)
}

func TestCheckNeeds_Cycle(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"a": {Name: "a", Needs: []any{"b"}},
			"b": {Name: "b", Needs: []any{"a"}},
		},
	}
	findings := checkNeeds(p)
	for _, f := range findings {
		if f.Rule == RuleNeedsCycle { return }
	}
	t.Error("expected cycle detection finding")
}

// ── checkDependencies ─────────────────────────────────────────────────────────

func TestCheckDependencies(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build", Script: []any{"make"}},
			"test-job": {Name: "test-job", Stage: "test", Script: []any{"test"},
				Dependencies: []string{"build-job"}},
		},
	}
	if len(checkDependencies(p)) != 0 { t.Error("valid deps: clean") }

	// unknown dep
	p2 := &model.Pipeline{
		Jobs: map[string]model.Job{
			"test-job": {Name: "test-job", Stage: "test", Dependencies: []string{"ghost"}},
		},
	}
	if len(checkDependencies(p2)) != 1 { t.Error("unknown dep: error") }

	// dep in same or later stage → error
	p3 := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"test-job1": {Name: "test-job1", Stage: "test"},
			"test-job2": {Name: "test-job2", Stage: "test", Dependencies: []string{"test-job1"}},
		},
	}
	if len(checkDependencies(p3)) != 1 { t.Error("same stage dep: error") }
}

// ── checkCacheKeyFiles ────────────────────────────────────────────────────────

func TestCheckCacheKeyFiles_NoFilesKey(t *testing.T) {
	// cache.key is a map but has no "files" key → continue (line 823-824).
	job := model.Job{
		Cache: map[string]any{
			"key": map[string]any{"prefix": "v1"},
		},
	}
	if got := checkCacheKeyFiles("j", job); len(got) != 0 {
		t.Errorf("key map without 'files': expected no findings, got %v", got)
	}
}

func TestCheckCacheKeyFiles_FilesNotList(t *testing.T) {
	// cache.key.files is a scalar (not a list) → error finding (line 827-834).
	job := model.Job{
		Cache: map[string]any{
			"key": map[string]any{"files": "single-file.lock"},
		},
	}
	got := checkCacheKeyFiles("j", job)
	if len(got) == 0 {
		t.Error("files as scalar: expected error finding")
		return
	}
	if got[0].Rule != RuleInvalidCacheKeyFiles || got[0].Severity != Error {
		t.Errorf("unexpected finding: %v", got[0])
	}
}

func TestCheckCacheKeyFiles_NonStringItem(t *testing.T) {
	// cache.key.files list has a non-string item → skip it (line 838-839).
	job := model.Job{
		Cache: map[string]any{
			"key": map[string]any{
				"files": []any{42, "go.sum"},  // 42 is non-string, go.sum is valid
			},
		},
	}
	// 42 is skipped; "go.sum" has no glob chars → no findings.
	if got := checkCacheKeyFiles("j", job); len(got) != 0 {
		t.Errorf("non-string item: expected no findings, got %v", got)
	}
}
