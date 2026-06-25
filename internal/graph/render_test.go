package graph

import (
	"os"
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/model"
)

// ── svgEsc ────────────────────────────────────────────────────────────────────

func TestSvgEsc(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"<tag>", "&lt;tag&gt;"},
		{"a&b<c>d", "a&amp;b&lt;c&gt;d"},
	}
	for _, tc := range cases {
		if got := svgEsc(tc.in); got != tc.want {
			t.Errorf("svgEsc(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

// ── svgTrunc ──────────────────────────────────────────────────────────────────

func TestSvgTrunc(t *testing.T) {
	cases := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"short", 20, "short"},
		{"exactly20charslong!", 20, "exactly20charslong!"},
		{"this-is-a-very-long-job-name", 20, "this-is-a-very-long…"},
		{"αβγδεζηθικλμνξο", 5, "αβγδ…"},
	}
	for _, tc := range cases {
		got := svgTrunc(tc.s, tc.maxLen)
		if got != tc.want {
			t.Errorf("svgTrunc(%q, %d)=%q want %q", tc.s, tc.maxLen, got, tc.want)
		}
	}
}

// ── svgEmpty ──────────────────────────────────────────────────────────────────

func TestSvgEmpty(t *testing.T) {
	out := svgEmpty()
	if !strings.Contains(out, "<svg") { t.Error("expected <svg") }
	if !strings.Contains(out, "no jobs defined") { t.Error("expected 'no jobs defined'") }
}

// ── chipColor ─────────────────────────────────────────────────────────────────

func TestChipColor(t *testing.T) {
	if chipColor(model.Job{Trigger: "x"}) != "#6b4fbb" { t.Error("trigger color") }
	if chipColor(model.Job{When: "manual"}) != "#fc6d26" { t.Error("manual color") }
	if chipColor(model.Job{When: "delayed"}) != "#fca326" { t.Error("delayed color") }
	if chipColor(model.Job{When: "on_failure"}) != "#d9534f" { t.Error("on_failure color") }
	if chipColor(model.Job{}) != "#1f75cb" { t.Error("regular color") }
}

// ── imageString ───────────────────────────────────────────────────────────────

func TestImageString(t *testing.T) {
	if imageString("alpine") != "alpine" { t.Error("string form") }
	if imageString(map[string]any{"name": "golang:1.21"}) != "golang:1.21" { t.Error("map form") }
	if imageString(map[string]any{"other": "x"}) != "" { t.Error("map without name key") }
	if imageString(nil) != "" { t.Error("nil") }
	if imageString(42) != "" { t.Error("unexpected type") }
}

// ── drawChipIcon ──────────────────────────────────────────────────────────────

func TestDrawChipIcon(t *testing.T) {
	cases := []struct {
		job     model.Job
		wantTag string
	}{
		{model.Job{Trigger: "x"}, "<polyline"},
		{model.Job{When: "manual"}, "<polygon"},
		{model.Job{When: "delayed"}, "<circle"},
		{model.Job{When: "on_failure"}, "<line"},
		{model.Job{}, "<polyline"},
	}
	for _, tc := range cases {
		var sb strings.Builder
		drawChipIcon(&sb, tc.job, 10, 10)
		if !strings.Contains(sb.String(), tc.wantTag) {
			t.Errorf("drawChipIcon(%q): want %s, got:\n%s", tc.job.When, tc.wantTag, sb.String())
		}
	}
}

// ── pipelineSVG ───────────────────────────────────────────────────────────────

func TestPipelineSVG_Empty(t *testing.T) {
	svg := pipelineSVG(&model.Pipeline{}, nil)
	if !strings.Contains(svg, "no jobs defined") {
		t.Errorf("expected 'no jobs defined', got:\n%s", svg)
	}
}

func TestPipelineSVG_ClassicMode(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
			"test-job":  {Name: "test-job", Stage: "test"},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, "<svg") { t.Error("expected <svg") }
	if !strings.Contains(svg, "build-job") { t.Error("expected build-job") }
	if !strings.Contains(svg, "test-job") { t.Error("expected test-job") }
	// Classic mode bus-bar draws <line> elements for connectors
	if !strings.Contains(svg, "<line") {
		t.Error("expected <line> connector in classic mode")
	}
}

func TestPipelineSVG_ClassicMode_MultiJob(t *testing.T) {
	// Two source jobs, two destination jobs → bus-bar with vertical segment
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-a": {Name: "build-a", Stage: "build"},
			"build-b": {Name: "build-b", Stage: "build"},
			"test-a":  {Name: "test-a", Stage: "test"},
			"test-b":  {Name: "test-b", Stage: "test"},
		},
	}
	svg := pipelineSVG(p, nil)
	// Bus bar: horizontal stubs from each src + dst, plus a vertical bus line
	// and an arrowhead polygon for each dst job.
	if !strings.Contains(svg, "<polygon") {
		t.Error("expected arrowhead <polygon> in multi-job classic mode")
	}
}

func TestPipelineSVG_DAGMode(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
			"test-job":  {Name: "test-job", Stage: "test", Needs: []any{"build-job"}},
		},
	}
	svg := pipelineSVG(p, nil)
	// DAG mode draws <path> bezier curves
	if !strings.Contains(svg, "<path") {
		t.Error("expected <path> connector in DAG mode")
	}
}

func TestPipelineSVG_DAGMode_StraightConnector(t *testing.T) {
	// Two stages at same height → straight connector in classic mode
	p := &model.Pipeline{
		Stages: []string{"a", "b"},
		Jobs: map[string]model.Job{
			"j1": {Name: "j1", Stage: "a"},
			"j2": {Name: "j2", Stage: "b"},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, "<svg") { t.Error("expected <svg") }
}

func TestPipelineSVG_AllJobTypes(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"deploy"},
		Jobs: map[string]model.Job{
			"manual-job":     {Name: "manual-job", Stage: "deploy", When: "manual"},
			"delayed-job":    {Name: "delayed-job", Stage: "deploy", When: "delayed"},
			"trigger-job":    {Name: "trigger-job", Stage: "deploy", Trigger: "other/project"},
			"onfailure-job":  {Name: "onfailure-job", Stage: "deploy", When: "on_failure"},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, "#fc6d26") { t.Error("expected manual color") }
	if !strings.Contains(svg, "#fca326") { t.Error("expected delayed color") }
	if !strings.Contains(svg, "#6b4fbb") { t.Error("expected trigger color") }
	if !strings.Contains(svg, "#d9534f") { t.Error("expected on_failure color") }
	// on_failure chips have a dashed border
	if !strings.Contains(svg, `stroke-dasharray="4,3"`) {
		t.Error("expected dashed border on on_failure chip")
	}
}

func TestPipelineSVG_JobNoStage(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"myjob": {Name: "myjob"},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, "myjob") { t.Error("expected myjob") }
}

func TestPipelineSVG_CrossPipelineNeed(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"j": {Name: "j", Stage: "build",
				Needs: []any{map[string]any{"pipeline": "other", "job": "x"}}},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, "<svg") { t.Error("expected <svg") }
}

func TestPipelineSVG_Tooltip(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"build-job": {
				Name:  "build-job",
				Stage: "build",
				When:  "on_success",
				Image: "golang:1.21",
				Needs: []any{"prep-job"},
			},
		},
	}
	svg := pipelineSVG(p, nil)
	// Tooltip <title> and <desc> should appear inside the <g data-job> wrapper
	if !strings.Contains(svg, `<title>build-job</title>`) {
		t.Error("expected <title>build-job</title>")
	}
	if !strings.Contains(svg, `<desc>`) {
		t.Error("expected <desc> element")
	}
	if !strings.Contains(svg, "image: golang:1.21") {
		t.Error("expected image in desc")
	}
	if !strings.Contains(svg, "needs: prep-job") {
		t.Error("expected needs in desc")
	}
	if !strings.Contains(svg, `data-job="build-job"`) {
		t.Error("expected data-job attribute")
	}
}

func TestPipelineSVG_SkippedJob(t *testing.T) {
	// A job with rules that never match in the given context should be greyed out.
	p := &model.Pipeline{
		Stages: []string{"deploy"},
		Jobs: map[string]model.Job{
			"deploy-job": {
				Name:   "deploy-job",
				Stage:  "deploy",
				Script: []any{"deploy.sh"},
				Rules: []model.Rule{
					{If: `$CI_COMMIT_TAG != ""`, When: "on_success"},
				},
			},
		},
	}
	// Branch context: CI_COMMIT_TAG is empty → rule doesn't match → job skipped
	ctx := cicontext.New("main", "", "push", nil)
	svg := pipelineSVG(p, ctx)

	// Skipped jobs use grey (#868686) instead of the regular blue
	if !strings.Contains(svg, `fill="#868686"`) {
		t.Error("expected grey fill for skipped job circle")
	}
	// Skipped state should appear in the tooltip desc
	if !strings.Contains(svg, "state: skipped") {
		t.Error("expected 'state: skipped' in tooltip desc")
	}
}

func TestPipelineSVG_ContextNil(t *testing.T) {
	// nil context → no skipped jobs, regular colors
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"j": {Name: "j", Stage: "build"},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, `fill="#1f75cb"`) {
		t.Error("expected regular blue fill with nil context")
	}
}

// ── RenderPipeline ────────────────────────────────────────────────────────────

func TestRenderPipeline(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
		},
	}
	dir := t.TempDir()
	path, err := RenderPipeline(p, dir, nil)
	if err != nil {
		t.Fatalf("RenderPipeline: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("output file not found: %v", statErr)
	}
}

func TestRenderPipeline_InvalidDir(t *testing.T) {
	// Writing to a path under a file (not a dir) should error.
	dir := t.TempDir()
	filePath := dir + "/notadir"
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RenderPipeline(&model.Pipeline{}, filePath+"/nested", nil)
	if err == nil {
		t.Error("expected error writing to path under a file")
	}
}

func TestRenderPipeline_WriteFileFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping as root — file permissions don't apply")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs:   map[string]model.Job{"j": {Name: "j", Stage: "build"}},
	}
	_, err := RenderPipeline(p, dir, nil)
	if err == nil {
		t.Error("expected error when writing SVG to read-only directory")
	}
}

// ── convertToPNG ─────────────────────────────────────────────────────────────

func TestRenderPipeline_SVGFallback(t *testing.T) {
	// With no converters on PATH, RenderPipeline returns the SVG path (line 53).
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs:   map[string]model.Job{"j": {Name: "j", Stage: "build"}},
	}
	dir := t.TempDir()
	path, err := RenderPipeline(p, dir, nil)
	if err != nil {
		t.Fatalf("RenderPipeline: %v", err)
	}
	if !strings.HasSuffix(path, ".svg") {
		t.Errorf("expected .svg output when no converter available, got %q", path)
	}
}

func TestConvertToPNG_NoConverter(t *testing.T) {
	// In a test environment without rsvg-convert/inkscape/magick, returns false.
	// We can't assert false because the CI machine might have one of them,
	// but we can assert it doesn't panic.
	dir := t.TempDir()
	svgPath := dir + "/test.svg"
	pngPath := dir + "/test.png"
	_ = os.WriteFile(svgPath, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644)
	// result is either true (converter found) or false (no converter) — just no panic
	convertToPNG(svgPath, pngPath)
}

func TestConvertToPNG_FakeConverter(t *testing.T) {
	// Install a fake rsvg-convert that just creates the output file and exits 0.
	// This exercises the "return true" branch and os.Remove in RenderPipeline.
	binDir := t.TempDir()
	script := "#!/bin/sh\n# rsvg-convert --output <out> <in>\ncp \"$3\" \"$2\"\n"
	fakeBin := binDir + "/rsvg-convert"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+":"+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	svgDir := t.TempDir()
	svgPath := svgDir + "/test.svg"
	pngPath := svgDir + "/test.png"
	if err := os.WriteFile(svgPath, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok := convertToPNG(svgPath, pngPath)
	if !ok {
		t.Error("expected convertToPNG to return true with fake rsvg-convert")
	}
}

func TestRenderPipeline_PNGConversion(t *testing.T) {
	// Verify the PNG path is returned (and SVG removed) when converter succeeds.
	binDir := t.TempDir()
	script := "#!/bin/sh\ncp \"$3\" \"$2\"\n"
	if err := os.WriteFile(binDir+"/rsvg-convert", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+":"+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs:   map[string]model.Job{"j": {Name: "j", Stage: "build"}},
	}
	dir := t.TempDir()
	path, err := RenderPipeline(p, dir, nil)
	if err != nil {
		t.Fatalf("RenderPipeline: %v", err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("expected .png output, got %q", path)
	}
}

// ── pipelineSVG coverage gaps ─────────────────────────────────────────────────

func TestPipelineSVG_LShapedElbow(t *testing.T) {
	// Classic mode with unequal stage sizes → vertical bus bar required
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"j1": {Name: "j1", Stage: "build"},
			"j2": {Name: "j2", Stage: "test"},
			"j3": {Name: "j3", Stage: "test"},
		},
	}
	svg := pipelineSVG(p, nil)
	// The bus bar is a <line>; arrowheads are <polygon>
	if !strings.Contains(svg, "<polygon") {
		t.Error("expected <polygon> arrowhead in bus-bar connector")
	}
}

func TestPipelineSVG_DAGNeedNotInPositionMap(t *testing.T) {
	// DAG mode: job needs a template job (starts with .) → dep not in position maps → continue
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"j": {Name: "j", Stage: "build", Needs: []any{".hidden-template"}},
			".hidden-template": {Name: ".hidden-template", Stage: "build"},
		},
	}
	svg := pipelineSVG(p, nil)
	// Should render without panic; .hidden-template is not visible so no connector drawn.
	if !strings.Contains(svg, "<svg") {
		t.Error("expected valid SVG output")
	}
}

// ── RenderHTML ────────────────────────────────────────────────────────────────

func TestRenderHTML(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"build-job": {Name: "build-job", Stage: "build"},
		},
	}
	dir := t.TempDir()
	path, err := RenderHTML(p, dir, nil)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	if !strings.HasSuffix(path, ".html") {
		t.Errorf("expected .html output, got %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	html := string(data)
	if !strings.Contains(html, "<!DOCTYPE html>") { t.Error("expected DOCTYPE") }
	if !strings.Contains(html, "<svg") { t.Error("expected embedded <svg") }
	if !strings.Contains(html, "build-job") { t.Error("expected job name in HTML") }
	if !strings.Contains(html, "applyTransform") { t.Error("expected JS pan/zoom function") }
	if !strings.Contains(html, "sidebar") { t.Error("expected sidebar element") }
}

func TestRenderHTML_InvalidDir(t *testing.T) {
	dir := t.TempDir()
	filePath := dir + "/notadir"
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RenderHTML(&model.Pipeline{}, filePath+"/nested", nil)
	if err == nil {
		t.Error("expected error writing to path under a file")
	}
}

func TestRenderHTML_WriteFileFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping as root — file permissions don't apply")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs:   map[string]model.Job{"j": {Name: "j", Stage: "build"}},
	}
	_, err := RenderHTML(p, dir, nil)
	if err == nil {
		t.Error("expected error when writing HTML to read-only directory")
	}
}

// ── computeColumns ────────────────────────────────────────────────────────────

func TestComputeColumns_NoIntraNeeds(t *testing.T) {
	// No intra-stage needs → one column per stage.
	stages := []string{"build", "test"}
	byStage := map[string][]string{
		"build": {"build-a", "build-b"},
		"test":  {"test-a"},
	}
	jobs := map[string]model.Job{
		"build-a": {Name: "build-a", Stage: "build"},
		"build-b": {Name: "build-b", Stage: "build"},
		"test-a":  {Name: "test-a", Stage: "test"},
	}
	cols := computeColumns(stages, byStage, jobs)
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns (one per stage), got %d", len(cols))
	}
	if cols[0].stage != "build" || len(cols[0].jobs) != 2 {
		t.Errorf("col[0]: got stage=%q jobs=%v", cols[0].stage, cols[0].jobs)
	}
	if cols[1].stage != "test" || len(cols[1].jobs) != 1 {
		t.Errorf("col[1]: got stage=%q jobs=%v", cols[1].stage, cols[1].jobs)
	}
}

func TestComputeColumns_IntraNeeds(t *testing.T) {
	// job-b depends on job-a in the same stage → split into 2 sub-columns.
	stages := []string{"build"}
	byStage := map[string][]string{
		"build": {"job-a", "job-b"},
	}
	jobs := map[string]model.Job{
		"job-a": {Name: "job-a", Stage: "build"},
		"job-b": {Name: "job-b", Stage: "build", Needs: []any{"job-a"}},
	}
	cols := computeColumns(stages, byStage, jobs)
	if len(cols) != 2 {
		t.Fatalf("expected 2 sub-columns, got %d", len(cols))
	}
	// Both belong to the same stage.
	if cols[0].stage != "build" || cols[1].stage != "build" {
		t.Error("both sub-columns should be in stage 'build'")
	}
	// job-a has no same-stage deps → depth 0 → first sub-column.
	if len(cols[0].jobs) != 1 || cols[0].jobs[0] != "job-a" {
		t.Errorf("col[0] should contain job-a, got %v", cols[0].jobs)
	}
	// job-b depends on job-a → depth 1 → second sub-column.
	if len(cols[1].jobs) != 1 || cols[1].jobs[0] != "job-b" {
		t.Errorf("col[1] should contain job-b, got %v", cols[1].jobs)
	}
}

func TestComputeColumns_EmptyStageSkipped(t *testing.T) {
	// Stages with no jobs are silently skipped.
	stages := []string{"build", "empty", "test"}
	byStage := map[string][]string{
		"build": {"j1"},
		"test":  {"j2"},
	}
	jobs := map[string]model.Job{
		"j1": {Name: "j1", Stage: "build"},
		"j2": {Name: "j2", Stage: "test"},
	}
	cols := computeColumns(stages, byStage, jobs)
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns (empty stage skipped), got %d", len(cols))
	}
}

func TestComputeColumns_CrossStageNeedIgnored(t *testing.T) {
	// job-b has needs: for both job-a (same stage) and prev-job (different stage).
	// The cross-stage need must be ignored when computing topological depth.
	stages := []string{"build"}
	byStage := map[string][]string{
		"build": {"job-a", "job-b"},
	}
	jobs := map[string]model.Job{
		"job-a": {Name: "job-a", Stage: "build"},
		"job-b": {Name: "job-b", Stage: "build", Needs: []any{"job-a", "prev-job"}},
		"prev-job": {Name: "prev-job", Stage: "prepare"},
	}
	cols := computeColumns(stages, byStage, jobs)
	// Still split into 2 sub-columns; the cross-stage need is ignored.
	if len(cols) != 2 {
		t.Fatalf("expected 2 sub-columns, got %d", len(cols))
	}
	if cols[0].jobs[0] != "job-a" {
		t.Errorf("expected job-a in first sub-column, got %v", cols[0].jobs)
	}
	if cols[1].jobs[0] != "job-b" {
		t.Errorf("expected job-b in second sub-column, got %v", cols[1].jobs)
	}
}

func TestComputeColumns_CycleGuard(t *testing.T) {
	// Intra-stage cycle must not hang (cycle guard returns depth 0).
	stages := []string{"build"}
	byStage := map[string][]string{
		"build": {"a", "b"},
	}
	jobs := map[string]model.Job{
		"a": {Name: "a", Stage: "build", Needs: []any{"b"}},
		"b": {Name: "b", Stage: "build", Needs: []any{"a"}},
	}
	cols := computeColumns(stages, byStage, jobs)
	// Should return without hanging; exact column count is implementation-defined.
	if len(cols) == 0 {
		t.Error("expected at least one column")
	}
}

func TestPipelineSVG_IntraStageDAG(t *testing.T) {
	// job-b depends on job-a in the same stage → SVG should render both,
	// placed as sub-columns (sub-stage gap between them).
	p := &model.Pipeline{
		Stages: []string{"build"},
		Jobs: map[string]model.Job{
			"job-a": {Name: "job-a", Stage: "build"},
			"job-b": {Name: "job-b", Stage: "build", Needs: []any{"job-a"}},
		},
	}
	svg := pipelineSVG(p, nil)
	if !strings.Contains(svg, "job-a") { t.Error("expected job-a in SVG") }
	if !strings.Contains(svg, "job-b") { t.Error("expected job-b in SVG") }
	// DAG mode: bezier connectors between the two jobs.
	if !strings.Contains(svg, "<path") {
		t.Error("expected bezier <path> connector between intra-stage jobs")
	}
}

// ── htmlPage ──────────────────────────────────────────────────────────────────

func TestHtmlPage(t *testing.T) {
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50"><rect/></svg>`
	html := htmlPage(svgContent)
	if !strings.Contains(html, "<!DOCTYPE html>") { t.Error("expected DOCTYPE") }
	if !strings.Contains(html, svgContent) { t.Error("expected SVG embedded in HTML") }
	if !strings.Contains(html, "id=\"viewport\"") { t.Error("expected viewport element") }
	if !strings.Contains(html, "id=\"sidebar\"") { t.Error("expected sidebar element") }
	if !strings.Contains(html, "applyTransform") { t.Error("expected JS transform function") }
}
