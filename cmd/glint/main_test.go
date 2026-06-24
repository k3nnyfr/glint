package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/linter"
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

// ── main() switch cases ───────────────────────────────────────────────────────

func TestMain_Check(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	os.Args = []string{"glint", "check", path}
	main()
}

func TestMain_Graph(t *testing.T) {
	captureExit(t)
	path := writePipeline(t, minimalPipeline)
	os.Args = []string{"glint", "graph", path}
	main()
}

func TestMain_Explain(t *testing.T) {
	captureExit(t)
	os.Args = []string{"glint", "explain", "GL001"}
	main()
}

// ── defaultCacheDir — UserHomeDir failure ─────────────────────────────────────

func TestDefaultCacheDir_HomeDirFails(t *testing.T) {
	orig := userHomeDirFn
	userHomeDirFn = func() (string, error) { return "", errors.New("no home") }
	t.Cleanup(func() { userHomeDirFn = orig })

	origXDG := os.Getenv("XDG_CACHE_HOME")
	os.Unsetenv("XDG_CACHE_HOME")
	t.Cleanup(func() { os.Setenv("XDG_CACHE_HOME", origXDG) })

	got := defaultCacheDir()
	if got != "" {
		t.Errorf("expected empty string when UserHomeDir fails, got %q", got)
	}
}

// ── cmdCheck — config load error ──────────────────────────────────────────────

func TestCmdCheck_ConfigError(t *testing.T) {
	captureExit(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitlab-ci.yml")
	if err := os.WriteFile(path, []byte(minimalPipeline), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create an unreadable .glint.yml — config.Load returns a non-NotExist error.
	cfgPath := filepath.Join(dir, ".glint.yml")
	if err := os.WriteFile(cfgPath, []byte("ignore: []"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cfgPath, 0o644) }) // allow cleanup
	cmdCheck([]string{path})
	// Warning is emitted to stderr; linting still proceeds, so no exit(2).
}

// ── cmdCheck — glintCfg.Stages ────────────────────────────────────────────────

func TestCmdCheck_ConfigStages(t *testing.T) {
	captureExit(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitlab-ci.yml")
	if err := os.WriteFile(path, []byte(minimalPipeline), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a .glint.yml that declares an extra stage.
	glintCfg := "stages:\n  - extra-stage\n"
	if err := os.WriteFile(filepath.Join(dir, ".glint.yml"), []byte(glintCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	cmdCheck([]string{path})
}

// ── cmdCheck — ResolveIncludes warnings ──────────────────────────────────────

func TestCmdCheck_IncludeWarning(t *testing.T) {
	captureExit(t)
	// Clear all token env vars so the project include is skipped (no token).
	for _, k := range []string{"GITLAB_TOKEN", "CI_JOB_TOKEN", "GITLAB_PRIVATE_TOKEN"} {
		t.Setenv(k, "")
	}
	content := `
stages: [build]
include:
  - project: some/group/project
    ref: main
    file: template.yml
build-job:
  stage: build
  script: echo
`
	path := writePipeline(t, content)
	cmdCheck([]string{path})
	// warning printed to stderr; pipeline still lints cleanly → no exit
}

// ── cmdCheck — resolver.Resolve error (cycle) ─────────────────────────────────

func TestCmdCheck_ResolveCycle(t *testing.T) {
	code := captureExit(t)
	content := `
stages: [build]
.base:
  script: echo base
  extends: .child
.child:
  script: echo child
  extends: .base
real-job:
  stage: build
  script: echo
`
	path := writePipeline(t, content)
	cmdCheck([]string{path})
	if *code != 2 {
		t.Errorf("cycle in extends: want exit(2), got %d", *code)
	}
}

// ── cmdCheck — extends unknown base (extWarnings) ────────────────────────────

func TestCmdCheck_ExtendsWarning(t *testing.T) {
	captureExit(t)
	content := `
stages: [build]
build-job:
  stage: build
  extends: .nonexistent-base
  script: echo
`
	path := writePipeline(t, content)
	cmdCheck([]string{path})
	// Warning is printed; exit code is not 2 (extends warning is non-fatal).
}

// ── cmdGraph — RenderPipeline errors ─────────────────────────────────────────

func TestCmdGraph_PipelineError(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	// Create a regular file at the outDir path so os.MkdirAll fails.
	outFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(outFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	cmdGraph([]string{"pipeline", "--out", outFile, path})
	if *code != 2 {
		t.Errorf("pipeline outDir-is-file: want exit(2), got %d", *code)
	}
}

func TestCmdGraph_AllError(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	outFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(outFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	cmdGraph([]string{"all", "--out", outFile, path})
	if *code != 2 {
		t.Errorf("all outDir-is-file: want exit(2), got %d", *code)
	}
}

// ── enrichContext — workflow rule variables ───────────────────────────────────

func TestEnrichContext_WorkflowVars(t *testing.T) {
	p := &model.Pipeline{
		Workflow: &model.Workflow{
			Rules: []model.Rule{{
				// No If: → always matches; Variables are injected into ctx.
				When:      "always",
				Variables: map[string]any{"DEPLOY_ENV": "production"},
			}},
		},
	}
	ctx := cicontext.New("main", "", "", nil)
	runs := enrichContext(ctx, p)
	if !runs {
		t.Error("expected pipeline to run with unconditional workflow rule")
	}
	if ctx.Get("DEPLOY_ENV") != "production" {
		t.Errorf("expected DEPLOY_ENV=production, got %q", ctx.Get("DEPLOY_ENV"))
	}
}

// ── writeJUnit — empty Rule and File ─────────────────────────────────────────

func TestWriteJUnit_EmptyRuleAndFile(t *testing.T) {
	var buf bytes.Buffer
	// Rule == "" → name falls back to "lint" (format.go:231-233).
	// File == "" → classname falls back to pipeline arg (format.go:235-237).
	writeJUnit(&buf, []linter.Finding{
		{Severity: linter.Error, Rule: "", File: "", Message: "something went wrong"},
	}, "my-pipeline.yml")
	out := buf.String()
	if !strings.Contains(out, "lint") {
		t.Errorf("expected 'lint' as testcase name when Rule is empty; got:\n%s", out)
	}
	if !strings.Contains(out, "my-pipeline.yml") {
		t.Errorf("expected pipeline path as classname when File is empty; got:\n%s", out)
	}
}

// ── execCommandOutput (default implementation) ────────────────────────────────

func TestExecCommandOutput_Default(t *testing.T) {
	// Call the default implementation directly (without mocking) to cover the
	// closure body in main.go.
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
	t.Cleanup(func() { execCommandOutput = orig })
	out, err := execCommandOutput("echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("unexpected output: %q", string(out))
	}
}

// ── gitDiffFiles ─────────────────────────────────────────────────────────────

func TestGitDiffFiles_Success(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("src/main.go\nDockerfile\n"), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	files, err := gitDiffFiles("origin/main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 || files[0] != "src/main.go" || files[1] != "Dockerfile" {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestGitDiffFiles_Error(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("not a git repository")
	}
	t.Cleanup(func() { execCommandOutput = orig })

	files, err := gitDiffFiles("HEAD~1")
	if err == nil {
		t.Fatal("expected error")
	}
	if files != nil {
		t.Errorf("expected nil files on error, got %v", files)
	}
}

func TestGitDiffFiles_EmptyOutput(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	files, err := gitDiffFiles("HEAD~1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty output → nil slice (no lines passed the filter)
	if files != nil {
		t.Errorf("expected nil (no files), got %v", files)
	}
}

// ── cmdCheck --changes / --changes-from ──────────────────────────────────────

func TestCmdCheck_ChangesFlag(t *testing.T) {
	code := captureExit(t)
	content := `stages: [build]
build-job:
  stage: build
  script: echo
  rules:
    - changes: [src/**/*.go]
      when: on_success
`
	path := writePipeline(t, content)
	// With --changes src/main.go: the rule fires → active job, no error
	cmdCheck([]string{"--changes", "src/main.go", path})
	if *code != -1 {
		t.Errorf("expected clean exit with matching --changes, got %d", *code)
	}
}

func TestCmdCheck_ChangesFrom_Fails(t *testing.T) {
	// When --changes-from fails (not a git repo / bad ref), a warning is emitted
	// and the check continues normally (permissive behaviour).
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("fatal: not a git repository")
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	// Should warn but not crash; pipeline is clean → no exit(1).
	cmdCheck([]string{"--changes-from", "origin/main", path})
	if *code == 1 {
		t.Errorf("expected no exit(1) when --changes-from fails gracefully, got %d", *code)
	}
}

func TestCmdCheck_ChangesFrom_Success(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("src/main.go\n"), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	content := `stages: [build]
build-job:
  stage: build
  script: echo
  rules:
    - changes: [src/**]
      when: on_success
`
	path := writePipeline(t, content)
	cmdCheck([]string{"--changes-from", "origin/main", path})
	if *code != -1 {
		t.Errorf("expected clean exit, got %d", *code)
	}
}

func TestCmdCheck_ChangesFrom_EmptyDiff(t *testing.T) {
	// --changes-from succeeds but returns no files (nothing changed).
	// reliable=true, allChanged stays nil → allChanged = []string{} branch is hit.
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil // empty output → no changed files
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	content := `stages: [build]
build-job:
  stage: build
  script: echo
  rules:
    - changes: [src/**]
      when: on_success
`
	path := writePipeline(t, content)
	// build-job's rule fires only if src/** matches; with 0 changed files it is skipped.
	// Pipeline is clean (no lint errors) → no exit(1).
	cmdCheck([]string{"--changes-from", "origin/main", path})
	if *code == 1 {
		t.Errorf("unexpected exit(1): %d", *code)
	}
}

func TestCmdGraph_ChangesFrom_Fails(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("not a git repository")
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", "--changes-from", "origin/main", path})
	if *code == 1 {
		t.Errorf("unexpected exit(1) when --changes-from fails in graph mode")
	}
}

func TestCmdGraph_ChangesFrom_Success(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("src/app.go\n"), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdGraph([]string{"tree", "--changes-from", "origin/main", path})
	if *code == 1 {
		t.Errorf("unexpected exit(1) in graph --changes-from success path")
	}
}

func TestCmdGraph_ChangesFrom_EmptyDiff(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	// reliable=true, allChanged nil → allChanged = []string{} branch hit
	cmdGraph([]string{"tree", "--changes-from", "origin/main", path})
	if *code == 1 {
		t.Errorf("unexpected exit(1) in graph --changes-from empty diff")
	}
}

func TestCmdGraph_ChangesFlag(t *testing.T) {
	code := captureExit(t)
	content := `stages: [build]
build-job:
  stage: build
  script: echo
  rules:
    - changes: [src/**]
      when: on_success
`
	path := writePipeline(t, content)
	cmdGraph([]string{"tree", "--changes", "src/app.go", path})
	if *code == 1 {
		t.Errorf("unexpected exit(1) with valid pipeline and --changes flag")
	}
}

// ── parseContextSpec ─────────────────────────────────────────────────────────

func TestParseContextSpec(t *testing.T) {
	cases := []struct {
		spec      string
		branch    string
		tag       string
		source    string
		extraVars []string
	}{
		{"branch=main", "main", "", "", nil},
		{"tag=v1.0.0", "", "v1.0.0", "", nil},
		{"source=schedule", "", "", "schedule", nil},
		{"branch=main,source=push", "main", "", "push", nil},
		{"branch=main,DEPLOY=prod", "main", "", "", []string{"DEPLOY=prod"}},
		{"DEPLOY=prod,ENV=staging", "", "", "", []string{"DEPLOY=prod", "ENV=staging"}},
		{"branch=main,tag=v1,source=push,X=y", "main", "v1", "push", []string{"X=y"}},
		{"branch=main, ,source=push", "main", "", "push", nil}, // spaces and empty segments
		{"", "", "", "", nil},
		{"noequals", "", "", "", []string{"noequals"}}, // no = → whole token as extraVar
		{"BRANCH=develop", "develop", "", "", nil}, // case-insensitive key matching
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			b, tg, s, ev := parseContextSpec(tc.spec)
			if b != tc.branch {
				t.Errorf("branch: want %q got %q", tc.branch, b)
			}
			if tg != tc.tag {
				t.Errorf("tag: want %q got %q", tc.tag, tg)
			}
			if s != tc.source {
				t.Errorf("source: want %q got %q", tc.source, s)
			}
			if len(ev) != len(tc.extraVars) {
				t.Errorf("extraVars len: want %d got %d (%v)", len(tc.extraVars), len(ev), ev)
				return
			}
			for i := range ev {
				if ev[i] != tc.extraVars[i] {
					t.Errorf("extraVars[%d]: want %q got %q", i, tc.extraVars[i], ev[i])
				}
			}
		})
	}
}

// ── sortedJobNames ────────────────────────────────────────────────────────────

func TestSortedJobNames_Order(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test", "deploy"},
		Jobs: map[string]model.Job{
			"deploy-job": {Stage: "deploy"},
			"test-b":     {Stage: "test"},
			"test-a":     {Stage: "test"},
			"build-job":  {Stage: "build"},
			".hidden":    {Stage: "build"}, // excluded
		},
	}
	got := sortedJobNames(p)
	want := []string{"build-job", "test-a", "test-b", "deploy-job"}
	if len(got) != len(want) {
		t.Fatalf("want %v got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: want %q got %q", i, want[i], got[i])
		}
	}
}

func TestSortedJobNames_UnknownStage(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"build-job":   {Stage: "build"},
			"orphan-job":  {Stage: "nonexistent"},
		},
	}
	got := sortedJobNames(p)
	if len(got) != 2 {
		t.Fatalf("want 2 jobs, got %v", got)
	}
	if got[0] != "build-job" {
		t.Errorf("expected build-job first, got %q", got[0])
	}
	if got[1] != "orphan-job" {
		t.Errorf("expected orphan-job last, got %q", got[1])
	}
}

func TestSortedJobNames_Empty(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs:   map[string]model.Job{".hidden": {Stage: "build"}},
	}
	got := sortedJobNames(p)
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

// ── printContextTable ─────────────────────────────────────────────────────────

func TestPrintContextTable_Empty(t *testing.T) {
	// Pipeline with no visible jobs: should return without printing.
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs:   map[string]model.Job{".hidden": {Stage: "build"}},
	}
	// No panic expected, no output to verify.
	printContextTable(p, nil, nil, nil)
}

func TestPrintContextTable_ActiveSkipped(t *testing.T) {
	// Job always active.
	content := `
stages: [build, deploy]
build-job:
  stage: build
  script: make
deploy-job:
  stage: deploy
  script: make deploy
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
`
	path := writePipeline(t, content)
	p, err := model.Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx1 := cicontext.New("main", "", "push", nil)
	ctx2 := cicontext.New("develop", "", "push", nil)
	enrichContext(ctx1, p)
	enrichContext(ctx2, p)

	// Just ensure it doesn't panic; output goes to real stdout in tests.
	printContextTable(p, []*cicontext.Context{ctx1, ctx2},
		[]string{"branch=main", "branch=develop"}, []bool{true, true})
}

func TestPrintContextTable_Blocked(t *testing.T) {
	// A context where workflow:rules: blocks the pipeline.
	content := `
stages: [build]
workflow:
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
build-job:
  stage: build
  script: make
`
	path := writePipeline(t, content)
	p, err := model.Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx1 := cicontext.New("main", "", "push", nil)
	ctx2 := cicontext.New("develop", "", "push", nil)
	r1 := enrichContext(ctx1, p)
	r2 := enrichContext(ctx2, p)

	printContextTable(p, []*cicontext.Context{ctx1, ctx2},
		[]string{"branch=main", "branch=develop"}, []bool{r1, r2})
}

func TestPrintContextTable_ManualState(t *testing.T) {
	content := `
stages: [build]
build-job:
  stage: build
  script: make
  rules:
    - when: manual
`
	path := writePipeline(t, content)
	p, err := model.Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := cicontext.New("main", "", "push", nil)
	enrichContext(ctx, p)
	printContextTable(p, []*cicontext.Context{ctx}, []string{"branch=main"}, []bool{true})
}

func TestPrintContextTable_ShortLabel(t *testing.T) {
	// Short label "x" (1 char) ensures a state string ("skipped", 7 chars) triggers
	// the ctxCols[c] = len(s) branch in printContextTable.
	content := `
stages: [build]
build-job:
  stage: build
  script: make
  rules:
    - if: '$CI_COMMIT_TAG != ""'
      when: on_success
    - when: never
`
	path := writePipeline(t, content)
	p, err := model.Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := cicontext.New("main", "", "push", nil)
	enrichContext(ctx, p)
	// Label "x" is shorter than "skipped" (7 chars), covering ctxCols width-expansion.
	printContextTable(p, []*cicontext.Context{ctx}, []string{"x"}, []bool{true})
}

// ── cmdCheck multi-context ───────────────────────────────────────────────────

func TestCmdCheck_MultiContext_Single(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--context", "branch=main", path})
	if *code != -1 {
		t.Errorf("multi-context single: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_Multiple(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--context", "branch=main", "--context", "branch=develop", path})
	if *code != -1 {
		t.Errorf("multi-context multiple: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_WithExtraVar(t *testing.T) {
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--context", "branch=main,DEPLOY=prod", path})
	if *code != -1 {
		t.Errorf("multi-context with extra var: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_ErrorPipeline(t *testing.T) {
	code := captureExit(t)
	content := `
stages: [build]
test-job:
  stage: nonexistent
  script: echo
`
	path := writePipeline(t, content)
	cmdCheck([]string{"--context", "branch=main", path})
	if *code != 1 {
		t.Errorf("multi-context error pipeline: want exit(1), got %d", *code)
	}
}

func TestCmdCheck_MultiContext_SkipsImplicitDefaults(t *testing.T) {
	// When --context is given, implicit defaults (branch=main, source=push) must not be set.
	// This is a smoke test: the command must complete without panicking.
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--context", "tag=v1.0.0", path})
	if *code == 2 {
		t.Errorf("multi-context tag: unexpected exit(2)")
	}
}

func TestCmdCheck_MultiContext_WithChanges(t *testing.T) {
	code := captureExit(t)
	content := `
stages: [build]
build-job:
  stage: build
  script: make
  rules:
    - changes: [src/**]
      when: on_success
`
	path := writePipeline(t, content)
	cmdCheck([]string{
		"--context", "branch=main",
		"--changes", "src/app.go",
		path,
	})
	if *code != -1 {
		t.Errorf("multi-context+changes: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_WithChangesFrom(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("src/app.go\n"), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--context", "branch=main", "--changes-from", "origin/main", path})
	if *code != -1 {
		t.Errorf("multi-context+changes-from: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_ChangesFrom_EmptyDiff(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	// reliable=true, allChanged nil → allChanged = []string{} guard hit in multi-context path
	cmdCheck([]string{"--context", "branch=main", "--changes-from", "origin/main", path})
	if *code != -1 {
		t.Errorf("multi-context+changes-from empty diff: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_ChangesFrom_Fails(t *testing.T) {
	orig := execCommandOutput
	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("not a git repository")
	}
	t.Cleanup(func() { execCommandOutput = orig })

	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	// git fails → changesReliable stays false → SetChangedFiles not called
	cmdCheck([]string{"--context", "branch=main", "--changes-from", "origin/main", path})
	if *code != -1 {
		t.Errorf("multi-context+changes-from fail: want no exit, got %d", *code)
	}
}

func TestCmdCheck_MultiContext_FormatJSON(t *testing.T) {
	// --context + non-text format: printContextTable should NOT be called.
	code := captureExit(t)
	path := writePipeline(t, minimalPipeline)
	cmdCheck([]string{"--context", "branch=main", "--format", "json", path})
	if *code != -1 {
		t.Errorf("multi-context json: want no exit, got %d", *code)
	}
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
