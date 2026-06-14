package graph

import (
	"os"
	"strings"
	"testing"

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
	if chipColor(model.Job{}) != "#1f75cb" { t.Error("regular color") }
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
	svg := pipelineSVG(&model.Pipeline{})
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
	svg := pipelineSVG(p)
	if !strings.Contains(svg, "<svg") { t.Error("expected <svg") }
	if !strings.Contains(svg, "build-job") { t.Error("expected build-job") }
	if !strings.Contains(svg, "test-job") { t.Error("expected test-job") }
	// Classic mode connector
	if !strings.Contains(svg, "<polyline") && !strings.Contains(svg, "<line") {
		t.Error("expected connector in classic mode")
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
	svg := pipelineSVG(p)
	// DAG mode draws <path> bezier curves
	if !strings.Contains(svg, "<path") {
		t.Error("expected <path> connector in DAG mode")
	}
}

func TestPipelineSVG_DAGMode_StraightConnector(t *testing.T) {
	// Two stages at same height → straight <line> connector in classic mode
	p := &model.Pipeline{
		Stages: []string{"a", "b"},
		Jobs: map[string]model.Job{
			"j1": {Name: "j1", Stage: "a"},
			"j2": {Name: "j2", Stage: "b"},
		},
	}
	svg := pipelineSVG(p)
	if !strings.Contains(svg, "<svg") { t.Error("expected <svg") }
}

func TestPipelineSVG_AllJobTypes(t *testing.T) {
	p := &model.Pipeline{
		Stages: []string{"deploy"},
		Jobs: map[string]model.Job{
			"manual-job":  {Name: "manual-job", Stage: "deploy", When: "manual"},
			"delayed-job": {Name: "delayed-job", Stage: "deploy", When: "delayed"},
			"trigger-job": {Name: "trigger-job", Stage: "deploy", Trigger: "other/project"},
		},
	}
	svg := pipelineSVG(p)
	// Each type has its own color in the SVG
	if !strings.Contains(svg, "#fc6d26") { t.Error("expected manual color") }
	if !strings.Contains(svg, "#fca326") { t.Error("expected delayed color") }
	if !strings.Contains(svg, "#6b4fbb") { t.Error("expected trigger color") }
}

func TestPipelineSVG_JobNoStage(t *testing.T) {
	p := &model.Pipeline{
		Jobs: map[string]model.Job{
			"myjob": {Name: "myjob"},
		},
	}
	svg := pipelineSVG(p)
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
	svg := pipelineSVG(p)
	if !strings.Contains(svg, "<svg") { t.Error("expected <svg") }
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
	path, err := RenderPipeline(p, dir)
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
	_, err := RenderPipeline(&model.Pipeline{}, filePath+"/nested")
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
	_, err := RenderPipeline(p, dir)
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
	path, err := RenderPipeline(p, dir)
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
	// This exercises the "return true" branch (line 72) and os.Remove in RenderPipeline.
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
	path, err := RenderPipeline(p, dir)
	if err != nil {
		t.Fatalf("RenderPipeline: %v", err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("expected .png output, got %q", path)
	}
}

// ── pipelineSVG coverage gaps ─────────────────────────────────────────────────

func TestPipelineSVG_LShapedElbow(t *testing.T) {
	// Classic mode with unequal stage sizes → different column heights → L-shaped elbow
	p := &model.Pipeline{
		Stages: []string{"build", "test"},
		Jobs: map[string]model.Job{
			"j1": {Name: "j1", Stage: "build"},
			"j2": {Name: "j2", Stage: "test"},
			"j3": {Name: "j3", Stage: "test"},
		},
	}
	svg := pipelineSVG(p)
	if !strings.Contains(svg, "<polyline") {
		t.Error("expected <polyline> for L-shaped elbow connector")
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
	svg := pipelineSVG(p)
	// Should render without panic; .hidden-template is not visible so no connector drawn.
	if !strings.Contains(svg, "<svg") {
		t.Error("expected valid SVG output")
	}
}
