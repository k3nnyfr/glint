package graph

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"git.k3nny.fr/glint/internal/cicontext"
	"git.k3nny.fr/glint/internal/model"
)

// Layout constants (pixels) – tuned to resemble GitLab's full pipeline graph view.
const (
	chipW       = 178 // job chip width
	chipH       = 34  // job chip height
	chipGap     = 6   // vertical gap between chips in a column
	iconCX      = 14  // status circle: x offset from chip left edge to centre
	iconR       = 7   // status circle radius (14 px diameter)
	textLeft    = 27  // job name text: x offset from chip left edge
	labelH      = 30  // stage-name label area height (text + bottom gap)
	topPad      = 40  // outer top padding
	sidePad     = 40  // outer left / right padding
	stageGap    = 50  // horizontal gap between stage columns (connector space)
	subStageGap = 20  // horizontal gap between sub-columns within the same stage
	botPad      = 48  // outer bottom padding (legend lives here)
)

type svgPt struct{ x, y int }

// column represents one vertical stack of job chips in the SVG.
// A declared GitLab stage maps to one column unless jobs within it have
// same-stage needs:, in which case it splits into topological sub-columns.
type column struct {
	stage string   // declared GitLab stage name
	jobs  []string // job names in this column, topologically ordered
}

// computeColumns assigns jobs to SVG columns.
// When jobs within the same declared stage have needs: relationships between
// each other, the stage is split into topological sub-columns so that
// depended-upon jobs appear to the left of the jobs that depend on them.
func computeColumns(stages []string, byStage map[string][]string, jobs map[string]model.Job) []column {
	var cols []column
	for _, stage := range stages {
		stageJobs := byStage[stage]
		sort.Strings(stageJobs)
		if len(stageJobs) == 0 {
			continue
		}

		// Index same-stage membership.
		stageSet := make(map[string]bool, len(stageJobs))
		for _, j := range stageJobs {
			stageSet[j] = true
		}

		// Check whether any job has a same-stage need.
		hasIntraNeeds := false
		outer:
		for _, j := range stageJobs {
			for _, need := range jobs[j].Needs {
				if dep := needsJobName(need); dep != "" && stageSet[dep] {
					hasIntraNeeds = true
					break outer
				}
			}
		}

		if !hasIntraNeeds {
			cols = append(cols, column{stage: stage, jobs: stageJobs})
			continue
		}

		// Compute topological depth within the stage.
		depth := make(map[string]int, len(stageJobs))
		for _, j := range stageJobs {
			depth[j] = -1
		}
		inProgress := make(map[string]bool, len(stageJobs))
		var computeDepth func(j string) int
		computeDepth = func(j string) int {
			if depth[j] >= 0 {
				return depth[j]
			}
			if inProgress[j] {
				return 0 // cycle guard (cycles already rejected by GL029)
			}
			inProgress[j] = true
			maxDep := -1
			for _, need := range jobs[j].Needs {
				dep := needsJobName(need)
				if dep == "" || !stageSet[dep] {
					continue
				}
				d := computeDepth(dep)
				if d > maxDep {
					maxDep = d
				}
			}
			depth[j] = maxDep + 1
			return depth[j]
		}
		for _, j := range stageJobs {
			if depth[j] < 0 {
				computeDepth(j)
			}
		}

		// One sub-column per topological depth level.
		maxDepth := 0
		for _, j := range stageJobs {
			if depth[j] > maxDepth {
				maxDepth = depth[j]
			}
		}
		for l := 0; l <= maxDepth; l++ {
			var levelJobs []string
			for _, j := range stageJobs {
				if depth[j] == l {
					levelJobs = append(levelJobs, j)
				}
			}
			if len(levelJobs) > 0 {
				sort.Strings(levelJobs)
				cols = append(cols, column{stage: stage, jobs: levelJobs})
			}
		}
	}
	return cols
}

// RenderPipeline writes a PNG (or SVG fallback) file with a GitLab CI-style
// pipeline layout and returns the path to the generated file.
func RenderPipeline(p *model.Pipeline, outDir string, ctx *cicontext.Context) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %s: %w", outDir, err)
	}

	svg := pipelineSVG(p, ctx)

	ts := time.Now().Format("20060102-150405")
	svgPath := filepath.Join(outDir, "pipeline-"+ts+".svg")
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		return "", fmt.Errorf("writing SVG: %w", err)
	}

	pngPath := filepath.Join(outDir, "pipeline-"+ts+".png")
	if convertToPNG(svgPath, pngPath) {
		_ = os.Remove(svgPath)
		return pngPath, nil
	}
	return svgPath, nil
}

// RenderHTML writes a self-contained HTML file with the pipeline graph embedded
// inline with pan/zoom and a job-detail sidebar, then returns the output path.
func RenderHTML(p *model.Pipeline, outDir string, ctx *cicontext.Context) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %s: %w", outDir, err)
	}

	svg := pipelineSVG(p, ctx)
	html := htmlPage(svg)

	ts := time.Now().Format("20060102-150405")
	htmlPath := filepath.Join(outDir, "pipeline-"+ts+".html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		return "", fmt.Errorf("writing HTML: %w", err)
	}
	return htmlPath, nil
}

// convertToPNG tries rsvg-convert, Inkscape, magick, and convert (not on Windows).
func convertToPNG(svgPath, pngPath string) bool {
	candidates := [][]string{
		{"rsvg-convert", "--output", pngPath, svgPath},
		{"inkscape", "--export-filename=" + pngPath, svgPath},
		{"magick", svgPath, pngPath},
	}
	// `convert` is the legacy ImageMagick name; skip on Windows where the name
	// collides with the built-in FAT→NTFS converter.
	if runtime.GOOS != "windows" {
		candidates = append(candidates, []string{"convert", svgPath, pngPath})
	}
	for _, args := range candidates {
		if bin, err := exec.LookPath(args[0]); err == nil {
			if exec.Command(bin, args[1:]...).Run() == nil {
				return true
			}
		}
	}
	return false
}

func pipelineSVG(p *model.Pipeline, ctx *cicontext.Context) string {
	// Collect visible (non-template) job names in sorted order.
	var visible []string
	for name := range p.Jobs {
		if !strings.HasPrefix(name, ".") {
			visible = append(visible, name)
		}
	}
	sort.Strings(visible)

	if len(visible) == 0 {
		return svgEmpty()
	}

	// Precompute which jobs are skipped in the given context.
	skippedJobs := make(map[string]bool)
	if ctx != nil {
		for _, name := range visible {
			if cicontext.EvalJob(p.Jobs[name], ctx) == cicontext.JobSkipped {
				skippedJobs[name] = true
			}
		}
	}

	// Group by stage; fall back to "test" (GitLab default) when stage is unset.
	byStage := make(map[string][]string)
	for _, name := range visible {
		s := p.Jobs[name].Stage
		if s == "" {
			s = "test"
		}
		byStage[s] = append(byStage[s], name)
	}

	// Build ordered stage list: declared stages first, then undeclared extras.
	seen := make(map[string]bool)
	var stages []string
	for _, s := range p.Stages {
		if len(byStage[s]) > 0 && !seen[s] {
			stages = append(stages, s)
			seen[s] = true
		}
	}
	for _, name := range visible {
		s := p.Jobs[name].Stage
		if s == "" {
			s = "test"
		}
		if !seen[s] {
			stages = append(stages, s)
			seen[s] = true
		}
	}

	// ── Column layout ─────────────────────────────────────────────────────────
	// Split stages into sub-columns when intra-stage needs exist.
	cols := computeColumns(stages, byStage, p.Jobs)

	// X position of each column's left edge.
	colX := make([]int, len(cols))
	if len(cols) > 0 {
		colX[0] = sidePad
		for i := 1; i < len(cols); i++ {
			gap := stageGap
			if cols[i].stage == cols[i-1].stage {
				gap = subStageGap
			}
			colX[i] = colX[i-1] + chipW + gap
		}
	}

	// Compute SVG dimensions and per-job connector anchor points.
	maxColH := 0
	rightMid := make(map[string]svgPt)
	leftMid := make(map[string]svgPt)

	for ci, col := range cols {
		n := len(col.jobs)
		h := n*chipH + max(0, n-1)*chipGap
		if h > maxColH {
			maxColH = h
		}
		for j, name := range col.jobs {
			chy := topPad + labelH + j*(chipH+chipGap)
			rightMid[name] = svgPt{colX[ci] + chipW, chy + chipH/2}
			leftMid[name] = svgPt{colX[ci], chy + chipH/2}
		}
	}

	svgW := colX[len(cols)-1] + chipW + sidePad
	svgH := topPad + labelH + maxColH + botPad

	// DAG mode: any visible job with a needs: list triggers job-to-job arrows.
	dagMode := false
	for _, name := range visible {
		if len(p.Jobs[name].Needs) > 0 {
			dagMode = true
			break
		}
	}

	var sb strings.Builder
	w := func(s string) { sb.WriteString(s + "\n") }
	wf := func(f string, a ...any) { fmt.Fprintf(&sb, f+"\n", a...) }

	// ── Document ──────────────────────────────────────────────────────────────
	wf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		svgW, svgH, svgW, svgH)

	w(`  <defs>`)
	// Subtle drop shadow for job chips.
	w(`    <filter id="chip-shadow" x="-4%" y="-10%" width="108%" height="130%">`)
	w(`      <feDropShadow dx="0" dy="1" stdDeviation="1.5" flood-color="#000000" flood-opacity="0.07"/>`)
	w(`    </filter>`)
	w(`  </defs>`)

	// White page background.
	wf(`  <rect width="%d" height="%d" fill="#ffffff"/>`, svgW, svgH)

	// ── Stage headers ─────────────────────────────────────────────────────────
	// Each declared stage may span multiple sub-columns; draw one header per stage.
	drawnHeader := make(map[string]bool)
	for ci, col := range cols {
		if drawnHeader[col.stage] {
			continue
		}
		// Span all sub-columns that belong to this stage.
		x1 := colX[ci]
		x2 := colX[ci] + chipW
		for j := ci + 1; j < len(cols) && cols[j].stage == col.stage; j++ {
			x2 = colX[j] + chipW
		}
		centerX := (x1 + x2) / 2

		// Stage name – small, gray, uppercase.
		wf(`  <text x="%d" y="%d" text-anchor="middle" `+
			`font-family="'GitLab Sans','Segoe UI',-apple-system,BlinkMacSystemFont,sans-serif" `+
			`font-size="11" font-weight="600" letter-spacing="0.8" fill="#868686">%s</text>`,
			centerX, topPad+13, svgEsc(strings.ToUpper(col.stage)))

		// Separator line spanning all sub-columns.
		wf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#eaeaea" stroke-width="1"/>`,
			x1, topPad+21, x2, topPad+21)

		drawnHeader[col.stage] = true
	}

	// ── Connectors (drawn before chips so they appear behind job blocks) ───────
	const connStroke = "#dbdbdb"

	if dagMode {
		// Job-to-job bezier curves from needs:.
		for _, name := range visible {
			for _, need := range p.Jobs[name].Needs {
				dep := needsJobName(need)
				if dep == "" {
					continue
				}
				src, okS := rightMid[dep]
				dst, okD := leftMid[name]
				if !okS || !okD {
					continue
				}
				cpX := (src.x + dst.x) / 2
				wf(`  <path d="M%d,%d C%d,%d %d,%d %d,%d" stroke="%s" stroke-width="2" fill="none"/>`,
					src.x, src.y, cpX, src.y, cpX, dst.y, dst.x-7, dst.y, connStroke)
				// Small right-pointing triangle arrowhead.
				wf(`  <polygon points="%d,%d %d,%d %d,%d" fill="%s"/>`,
					dst.x-7, dst.y-4, dst.x, dst.y, dst.x-7, dst.y+4, connStroke)
			}
		}
	} else {
		// Classic: bus-bar connectors between adjacent stage columns.
		// Every job in stage[i] fans to a vertical bus at the midpoint gap,
		// then fans out to every job in stage[i+1].
		for ci := 0; ci < len(cols)-1; ci++ {
			x1 := colX[ci] + chipW    // right edge of current column
			x2 := colX[ci+1]          // left edge of next column
			midX := (x1 + x2) / 2

			srcJobs := cols[ci].jobs
			dstJobs := cols[ci+1].jobs

			// Collect all Y midpoints to span the vertical bus bar.
			var allYs []int
			srcY := make([]int, len(srcJobs))
			dstY := make([]int, len(dstJobs))
			for j, name := range srcJobs {
				srcY[j] = rightMid[name].y
				allYs = append(allYs, srcY[j])
			}
			for j, name := range dstJobs {
				dstY[j] = leftMid[name].y
				allYs = append(allYs, dstY[j])
			}
			sort.Ints(allYs)
			busMinY, busMaxY := allYs[0], allYs[len(allYs)-1]

			// Vertical bus bar at midX (only when there are multiple Y levels).
			if busMinY < busMaxY {
				wf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"/>`,
					midX, busMinY, midX, busMaxY, connStroke)
			}

			// Horizontal stubs from each source job to the bus.
			for _, y := range srcY {
				wf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"/>`,
					x1, y, midX, y, connStroke)
			}

			// Horizontal stubs from bus to each destination job, with arrowhead.
			for _, y := range dstY {
				wf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"/>`,
					midX, y, x2-7, y, connStroke)
				wf(`  <polygon points="%d,%d %d,%d %d,%d" fill="%s"/>`,
					x2-7, y-4, x2, y, x2-7, y+4, connStroke)
			}
		}
	}

	// ── Job chips (drawn after connectors so they appear on top) ─────────────
	for ci, col := range cols {
		for j, name := range col.jobs {
			job := p.Jobs[name]
			chy := topPad + labelH + j*(chipH+chipGap)
			isSkipped := skippedJobs[name]
			cx := colX[ci]

			// Build <desc> content: shown in the HTML sidebar and SVG viewer tooltips.
			desc := "stage: " + col.stage
			if job.When != "" {
				desc += "\nwhen: " + job.When
			}
			if img := imageString(job.Image); img != "" {
				desc += "\nimage: " + img
			}
			var needNames []string
			for _, n := range job.Needs {
				if s := needsJobName(n); s != "" {
					needNames = append(needNames, s)
				}
			}
			if len(needNames) > 0 {
				desc += "\nneeds: " + strings.Join(needNames, ", ")
			}
			if isSkipped {
				desc += "\nstate: skipped"
			}

			// data-job attribute enables JS click detection in the HTML output.
			wf(`  <g data-job="%s">`, svgEsc(name))
			wf(`    <title>%s</title>`, svgEsc(name))
			wf(`    <desc>%s</desc>`, svgEsc(desc))

			color := chipColor(job)
			if isSkipped {
				color = "#868686"
			}

			// Chip card (white, rounded, subtle border + shadow).
			// on_failure jobs get a dashed border to signal the failure path.
			dashAttr := ""
			if !isSkipped && job.When == "on_failure" {
				dashAttr = ` stroke-dasharray="4,3"`
			}
			wf(`    <rect x="%d" y="%d" width="%d" height="%d" rx="4" `+
				`fill="#ffffff" stroke="#dde1e7" stroke-width="1"%s filter="url(#chip-shadow)"/>`,
				cx, chy, chipW, chipH, dashAttr)

			// Colored status indicator circle.
			wf(`    <circle cx="%d" cy="%d" r="%d" fill="%s"/>`,
				cx+iconCX, chy+chipH/2, iconR, color)

			// Icon inside the circle (omitted for skipped — grey circle speaks for itself).
			if !isSkipped {
				drawChipIcon(&sb, job, cx+iconCX, chy+chipH/2)
			}

			// Job name text — dimmed for skipped jobs.
			textColor := "#303030"
			if isSkipped {
				textColor = "#868686"
			}
			wf(`    <text x="%d" y="%d" dominant-baseline="middle" `+
				`font-family="'GitLab Sans','Segoe UI',-apple-system,BlinkMacSystemFont,sans-serif" `+
				`font-size="13" fill="%s">%s</text>`,
				cx+textLeft, chy+chipH/2, textColor, svgEsc(svgTrunc(name, 20)))

			wf(`  </g>`)
		}
	}

	// ── Legend ────────────────────────────────────────────────────────────────
	legend := []struct{ color, label string }{
		{"#1f75cb", "regular"},
		{"#fc6d26", "manual"},
		{"#6b4fbb", "trigger"},
		{"#fca326", "delayed"},
		{"#d9534f", "on_failure"},
	}
	const legendItemW = 82
	legendY := topPad + labelH + maxColH + botPad/2
	lx0 := (svgW - len(legend)*legendItemW) / 2
	for k, item := range legend {
		lx := lx0 + k*legendItemW
		wf(`  <circle cx="%d" cy="%d" r="6" fill="%s"/>`, lx+6, legendY, item.color)
		wf(`  <text x="%d" y="%d" dominant-baseline="middle" `+
			`font-family="'GitLab Sans','Segoe UI',-apple-system,BlinkMacSystemFont,sans-serif" `+
			`font-size="11" fill="#868686">%s</text>`,
			lx+16, legendY, item.label)
	}

	w(`</svg>`)
	return sb.String()
}

// drawChipIcon writes an SVG symbol inside the status circle to help identify
// the job type at a glance.
func drawChipIcon(sb *strings.Builder, job model.Job, cx, cy int) {
	if job.Trigger != nil {
		// Right-pointing chevron for trigger jobs.
		fmt.Fprintf(sb, "    <polyline points=\"%d,%d %d,%d %d,%d\" "+
			"stroke=\"#fff\" stroke-width=\"1.5\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\"/>\n",
			cx-3, cy-3, cx+3, cy, cx-3, cy+3)
		return
	}
	switch job.When {
	case "manual":
		// Filled play triangle.
		fmt.Fprintf(sb, "    <polygon points=\"%d,%d %d,%d %d,%d\" fill=\"#fff\"/>\n",
			cx-3, cy-4, cx+5, cy, cx-3, cy+4)
	case "delayed":
		// Clock: circle + hands.
		fmt.Fprintf(sb, "    <circle cx=\"%d\" cy=\"%d\" r=\"5\" stroke=\"#fff\" stroke-width=\"1.2\" fill=\"none\"/>\n", cx, cy)
		fmt.Fprintf(sb, "    <line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#fff\" stroke-width=\"1.2\" stroke-linecap=\"round\"/>\n",
			cx, cy, cx, cy-3)
		fmt.Fprintf(sb, "    <line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#fff\" stroke-width=\"1.2\" stroke-linecap=\"round\"/>\n",
			cx, cy, cx+2, cy+1)
	case "on_failure":
		// X mark for failure-path jobs.
		fmt.Fprintf(sb, "    <line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#fff\" stroke-width=\"1.5\" stroke-linecap=\"round\"/>\n",
			cx-3, cy-3, cx+3, cy+3)
		fmt.Fprintf(sb, "    <line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#fff\" stroke-width=\"1.5\" stroke-linecap=\"round\"/>\n",
			cx+3, cy-3, cx-3, cy+3)
	default:
		// Regular job: small white checkmark.
		fmt.Fprintf(sb, "    <polyline points=\"%d,%d %d,%d %d,%d\" "+
			"stroke=\"#fff\" stroke-width=\"1.5\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\"/>\n",
			cx-3, cy, cx-1, cy+3, cx+4, cy-3)
	}
}

func chipColor(job model.Job) string {
	if job.Trigger != nil {
		return "#6b4fbb"
	}
	switch job.When {
	case "manual":
		return "#fc6d26"
	case "delayed":
		return "#fca326"
	case "on_failure":
		return "#d9534f"
	}
	return "#1f75cb"
}

// imageString extracts the image name from a job Image field (string or map form).
func imageString(img any) string {
	switch v := img.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return ""
}

func svgEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func svgTrunc(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

func svgEmpty() string {
	const w, h = 300, 80
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`+
			`<rect width="%d" height="%d" fill="#fff"/>`+
			`<text x="%d" y="%d" text-anchor="middle" dominant-baseline="middle" `+
			`font-family="sans-serif" font-size="13" fill="#868686">no jobs defined</text>`+
			`</svg>`,
		w, h, w, h, w/2, h/2)
}

// htmlPage wraps an SVG pipeline graph in a self-contained HTML page with
// mouse pan/zoom and a job-detail sidebar that appears on chip click.
func htmlPage(svgContent string) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Pipeline Graph</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'Segoe UI', Arial, sans-serif;
  background: #f5f5f5;
  display: flex;
  height: 100vh;
  overflow: hidden;
}
#sidebar {
  width: 260px;
  flex-shrink: 0;
  background: #fff;
  border-right: 1px solid #e0e0e0;
  display: none;
  flex-direction: column;
  overflow-y: auto;
}
#sidebar.open { display: flex; }
#sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: .6rem .8rem;
  border-bottom: 1px solid #e0e0e0;
  background: #fafafa;
  flex-shrink: 0;
}
#sidebar-header h3 {
  font-size: .9rem;
  color: #303030;
  word-break: break-all;
}
#close-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  color: #868686;
  padding: .2rem .4rem;
  margin-left: .4rem;
  flex-shrink: 0;
}
#close-btn:hover { color: #303030; }
#sidebar-body { padding: .8rem; flex: 1; }
#sidebar-body pre {
  font-size: .82rem;
  color: #555;
  white-space: pre-wrap;
  line-height: 1.6;
}
#viewport {
  flex: 1;
  overflow: hidden;
  cursor: grab;
  position: relative;
  background: #f5f5f5;
}
#viewport.dragging { cursor: grabbing; }
#svg-wrap {
  display: inline-block;
  transform-origin: 0 0;
  padding: 20px;
}
g[data-job] { cursor: pointer; }
g[data-job]:hover > rect { stroke: #1f75cb; stroke-width: 2; }
</style>
</head>
<body>
<div id="sidebar">
  <div id="sidebar-header">
    <h3 id="sidebar-title">Job details</h3>
    <button id="close-btn" title="Close">&#x2715;</button>
  </div>
  <div id="sidebar-body">
    <pre id="sidebar-details"></pre>
  </div>
</div>
<div id="viewport">
  <div id="svg-wrap">
`)
	sb.WriteString(svgContent)
	sb.WriteString(`
  </div>
</div>
<script>
(function() {
  'use strict';
  var vp = document.getElementById('viewport');
  var wrap = document.getElementById('svg-wrap');
  var sidebar = document.getElementById('sidebar');
  var titleEl = document.getElementById('sidebar-title');
  var detailsEl = document.getElementById('sidebar-details');
  var tx = 0, ty = 0, scale = 1;

  function applyTransform() {
    wrap.style.transform = 'translate(' + tx + 'px,' + ty + 'px) scale(' + scale + ')';
  }

  // Zoom toward the cursor position.
  vp.addEventListener('wheel', function(e) {
    e.preventDefault();
    var factor = e.deltaY < 0 ? 1.1 : 1 / 1.1;
    var r = vp.getBoundingClientRect();
    var mx = e.clientX - r.left;
    var my = e.clientY - r.top;
    tx = mx - (mx - tx) * factor;
    ty = my - (my - ty) * factor;
    scale *= factor;
    applyTransform();
  }, { passive: false });

  // Drag to pan (skip if clicking on a job chip).
  var dragging = false, ox = 0, oy = 0;
  vp.addEventListener('mousedown', function(e) {
    if (e.target.closest('g[data-job]')) return;
    dragging = true;
    ox = e.clientX - tx;
    oy = e.clientY - ty;
    vp.classList.add('dragging');
  });
  window.addEventListener('mousemove', function(e) {
    if (!dragging) return;
    tx = e.clientX - ox;
    ty = e.clientY - oy;
    applyTransform();
  });
  window.addEventListener('mouseup', function() {
    dragging = false;
    vp.classList.remove('dragging');
  });

  // Double-click on the background to reset the view.
  vp.addEventListener('dblclick', function(e) {
    if (e.target.closest('g[data-job]')) return;
    tx = 0; ty = 0; scale = 1;
    applyTransform();
  });

  // Click a job chip to show its details in the sidebar.
  document.addEventListener('click', function(e) {
    var g = e.target.closest('g[data-job]');
    if (!g) return;
    var jobName = g.dataset.job;
    var descEl = g.querySelector('desc');
    var desc = descEl ? descEl.textContent : '';
    titleEl.textContent = jobName;
    detailsEl.textContent = desc;
    sidebar.classList.add('open');
  });

  // Close the sidebar.
  document.getElementById('close-btn').addEventListener('click', function() {
    sidebar.classList.remove('open');
  });
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') sidebar.classList.remove('open');
  });
})();
</script>
</body>
</html>
`)
	return sb.String()
}
