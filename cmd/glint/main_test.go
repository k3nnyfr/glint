package main

import (
	"os"
	"path/filepath"
	"testing"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/model"
)

// captureExit replaces the exit variable with a function that records the code,
// and restores it after the test. Call the returned cleanup func in defer.
func captureExit(t *testing.T) *int {
	t.Helper()
	orig := exit
	code := -1
	exit = func(c int) { code = c }
	t.Cleanup(func() { exit = orig })
	return &code
}

// writePipeline writes a minimal valid pipeline file to a temp dir and returns its path.
func writePipeline(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitlab-ci.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const minimalPipeline = `
stages: [build]
build-job:
  stage: build
  script: make
`

// ── main() ───────────────────────────────────────────────────────────────────

func TestMain_NoArgs(t *testing.T) {
	code := captureExit(t)
	os.Args = []string{"glint"}
	main()
	if *code != 2 { t.Errorf("no args: want exit(2), got %d", *code) }
}

func TestMain_HelpFlag(t *testing.T) {
	captureExit(t) // should not exit
	os.Args = []string{"glint", "--help"}
	main()
}

func TestMain_VersionFlag(t *testing.T) {
	captureExit(t)
	os.Args = []string{"glint", "--version"}
	main()
}

func TestMain_HelpAlias(t *testing.T) {
	captureExit(t)
	os.Args = []string{"glint", "help"}
	main()
}

func TestMain_VersionAlias(t *testing.T) {
	captureExit(t)
	os.Args = []string{"glint", "version"}
	main()
}

func TestMain_UnknownCommand(t *testing.T) {
	code := captureExit(t)
	os.Args = []string{"glint", "badcmd"}
	main()
	if *code != 2 { t.Errorf("unknown cmd: want exit(2), got %d", *code) }
}

// ── cmdCheck ─────────────────────────────────────────────────────────────────

func TestCmdCheck_ValidPipeline(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	os.Args = []string{"glint", "check", path}
	cmdCheck([]string{path})
	if *code != -1 { t.Errorf("valid pipeline: want no exit, got %d", *code) }
}

func TestCmdCheck_NoArgs(t *testing.T) {
	code := captureExit(t)
	cmdCheck([]string{})
	if *code != 2 { t.Errorf("no args: want exit(2), got %d", *code) }
}

func TestCmdCheck_InvalidFormat(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--format", "badformat", path})
	if *code != 2 { t.Errorf("bad format: want exit(2), got %d", *code) }
}

func TestCmdCheck_MissingFile(t *testing.T) {
	code := captureExit(t)
	cmdCheck([]string{"/nonexistent/pipeline.yml"})
	if *code != 2 { t.Errorf("missing file: want exit(2), got %d", *code) }
}

func TestCmdCheck_WithErrors_ExitsOne(t *testing.T) {
	code := captureExit(t)
	// Pipeline with an error finding (invalid stage reference)
	content := `
stages: [build]
test-job:
  stage: nonexistent
  script: echo
`
	path := writePipeline(t, content)
	cmdCheck([]string{path})
	if *code != 1 { t.Errorf("pipeline with errors: want exit(1), got %d", *code) }
}

func TestCmdCheck_FormatJSON(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--format", "json", path})
}

func TestCmdCheck_FormatSARIF(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--format", "sarif", path})
}

func TestCmdCheck_FormatJUnit(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--format", "junit", path})
}

func TestCmdCheck_FormatGitHub(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--format", "github", path})
}

func TestCmdCheck_WithBranch(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--branch", "develop", path})
}

func TestCmdCheck_WithTag(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--tag", "v1.0.0", path})
}

func TestCmdCheck_WithSource(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--source", "schedule", path})
}

func TestCmdCheck_WithVar(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--var", "MY_VAR=hello", path})
}

func TestCmdCheck_ListVars(t *testing.T) {
	captureExit(t)
	content := `
stages: [build]
variables:
  MY_VAR: hello
build-job:
  stage: build
  script: echo
`
	path := writePipeline(t, content)
	cmdCheck([]string{"--list-vars", "--branch", "main", path})
}

func TestCmdCheck_WithOffline(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--offline", path})
}

func TestCmdCheck_WorkflowRulesExclude(t *testing.T) {
	captureExit(t)
	content := `
stages: [build]
workflow:
  rules:
    - if: '$CI_COMMIT_TAG'
      when: always
    - when: never
build-job:
  stage: build
  script: echo
`
	path := writePipeline(t, content)
	// branch push — workflow rule won't match a tag, so pipeline won't start
	cmdCheck([]string{"--branch", "main", path})
}

// ── cmdGraph ─────────────────────────────────────────────────────────────────

func TestCmdGraph_NoArgs(t *testing.T) {
	code := captureExit(t)
	cmdGraph([]string{})
	if *code != 2 { t.Errorf("no args: want exit(2), got %d", *code) }
}

func TestCmdGraph_Tree(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", path})
}

func TestCmdGraph_Includes(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"includes", path})
}

func TestCmdGraph_Default(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{path})
}

func TestCmdGraph_Pipeline(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	outDir := t.TempDir()
	cmdGraph([]string{"pipeline", "--out", outDir, path})
}

func TestCmdGraph_All(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	outDir := t.TempDir()
	cmdGraph([]string{"all", "--out", outDir, path})
}

func TestCmdGraph_MissingFile(t *testing.T) {
	code := captureExit(t)
	cmdGraph([]string{"/nonexistent.yml"})
	if *code != 2 { t.Errorf("missing file: want exit(2), got %d", *code) }
}

func TestCmdGraph_WithBranch(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", "--branch", "develop", path})
}

func TestCmdGraph_WithTag(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", "--tag", "v1.0.0", path})
}

func TestCmdGraph_WithListVars(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", "--list-vars", "--branch", "main", path})
}

func TestCmdGraph_Offline(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", "--offline", path})
}

// ── cmdExplain ────────────────────────────────────────────────────────────────

func TestCmdExplain_NoArgs_ListsRules(t *testing.T) {
	captureExit(t)
	cmdExplain([]string{})
}

func TestCmdExplain_ValidRule(t *testing.T) {
	captureExit(t)
	cmdExplain([]string{"GL001"})
}

func TestCmdExplain_UnknownRule(t *testing.T) {
	code := captureExit(t)
	cmdExplain([]string{"GL999"})
	if *code != 2 { t.Errorf("unknown rule: want exit(2), got %d", *code) }
}

func TestCmdExplain_LowercaseRule(t *testing.T) {
	captureExit(t)
	// lowercase should work (converted to upper inside)
	cmdExplain([]string{"gl001"})
}

// ── defaultCacheDir ───────────────────────────────────────────────────────────

func TestDefaultCacheDir(t *testing.T) {
	// Should return a non-empty string (either XDG or ~/.cache/glint)
	got := defaultCacheDir()
	if got == "" { t.Error("expected non-empty cache dir") }
}

func TestDefaultCacheDir_XDG(t *testing.T) {
	orig := os.Getenv("XDG_CACHE_HOME")
	os.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
	defer os.Setenv("XDG_CACHE_HOME", orig)

	got := defaultCacheDir()
	if got != "/tmp/xdg-cache/glint" {
		t.Errorf("XDG_CACHE_HOME: got %q want /tmp/xdg-cache/glint", got)
	}
}

// ── multiFlag ────────────────────────────────────────────────────────────────

func TestMultiFlag(t *testing.T) {
	var f multiFlag
	if f.String() != "" { t.Error("empty: expected empty string") }
	if err := f.Set("a"); err != nil { t.Fatal(err) }
	if err := f.Set("b"); err != nil { t.Fatal(err) }
	if f.String() != "a, b" { t.Errorf("got %q", f.String()) }
}

// ── printVars ─────────────────────────────────────────────────────────────────

func TestPrintVars(t *testing.T) {
	p := &model.Pipeline{
		Variables: map[string]any{"KEY": "val", "OTHER": map[string]any{"value": "v2"}},
		Workflow: &model.Workflow{
			Rules: []model.Rule{{Variables: map[string]any{"RULE_VAR": "x"}}},
		},
	}
	ctx := cicontext.New("main", "", "", nil)
	printVars(p, ctx)       // should not panic
	printVars(p, nil)       // nil ctx: no context vars printed
}

func TestPrintVarMap(t *testing.T) {
	printVarMap(nil)                             // nil map: prints (none)
	printVarMap(map[string]any{})                // empty: prints (none)
	printVarMap(map[string]any{"A": "1", "B": map[string]any{"value": "2"}})
	printVarMap(map[string]any{"C": 42})         // non-scalar: (complex)
}

func TestVarValueString(t *testing.T) {
	if varValueString("hello") != "hello" { t.Error("string") }
	if varValueString(map[string]any{"value": "v"}) != "v" { t.Error("map with value") }
	if varValueString(map[string]any{"other": "x"}) != "(complex)" { t.Error("complex") }
}

// ── enrichContext ─────────────────────────────────────────────────────────────

func TestEnrichContext(t *testing.T) {
	p := &model.Pipeline{
		Variables: map[string]any{"ENV": "prod"},
	}
	ctx := cicontext.New("main", "", "", nil)
	runs := enrichContext(ctx, p)
	if !runs { t.Error("expected pipeline to run on main branch with no workflow rules") }
	if ctx.Get("ENV") != "prod" { t.Error("pipeline var should be injected") }
}

// ── printContext ──────────────────────────────────────────────────────────────

func TestPrintContext(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"active-job":  {Name: "active-job", Script: []any{"echo"}, Stage: "build"},
			"manual-job":  {Name: "manual-job", Script: []any{"echo"}, When: "manual"},
			"skipped-job": {Name: "skipped-job", Rules: []model.Rule{{If: `$CI_COMMIT_TAG != ""`, When: "on_success"}}},
			".hidden":     {Name: ".hidden"},
		},
	}
	ctx := cicontext.New("main", "", "", nil)
	printContext(p, ctx) // should not panic
}

// ── printJobGroup ─────────────────────────────────────────────────────────────

func TestPrintJobGroup(t *testing.T) {
	printJobGroup("Active ", []string{"a", "b"}) // should not panic
	printJobGroup("Empty ", []string{})           // empty: no output
}

// ── isSuppressed ─────────────────────────────────────────────────────────────

func TestIsSuppressed(t *testing.T) {
	suppressions := map[string][]string{
		"build-job": {"GL001", "GL002"},
		"all-job":   {"*"},
	}
	if isSuppressed("", "GL001", suppressions) { t.Error("empty job: not suppressed") }
	if isSuppressed("build-job", "GL003", suppressions) { t.Error("GL003 not in list") }
	if !isSuppressed("build-job", "GL001", suppressions) { t.Error("GL001 should be suppressed") }
	if !isSuppressed("all-job", "GL042", suppressions) { t.Error("wildcard should suppress") }
	if isSuppressed("unknown-job", "GL001", suppressions) { t.Error("unknown job: not suppressed") }
}
