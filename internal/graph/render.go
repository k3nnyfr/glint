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

	"git.k3nny.fr/glint/internal/model"
)

// Layout constants (pixels) – tuned to resemble GitLab's full pipeline graph view.
const (
	chipW     = 178 // job chip width
	chipH     = 34  // job chip height
	chipGap   = 6   // vertical gap between chips in a column
	iconCX    = 14  // status circle: x offset from chip left edge to circle centre
	iconR     = 7   // status circle radius (14 px diameter)
	textLeft  = 27  // job name text: x offset from chip left edge
	labelH    = 30  // stage-name label area height (text + bottom gap)
	topPad    = 40  // outer top padding
	sidePad   = 40  // outer left / right padding
	stageGap  = 50  // horizontal gap between stage columns (connector space)
	botPad    = 48  // outer bottom padding (legend lives here)
)

type svgPt struct{ x, y int }

// RenderPipeline writes a PNG (or SVG fallback) file with a GitLab CI-style
// pipeline layout and returns the path to the generated file.
func RenderPipeline(p *model.Pipeline, outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %s: %w", outDir, err)
	}

	svg := pipelineSVG(p)

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

func pipelineSVG(p *model.Pipeline) string {
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

	// Compute per-stage column heights and job anchor points for connectors.
	colH := make([]int, len(stages))   // height of the chip stack (no label)
	colCtY := make([]int, len(stages)) // y centre of the chip stack
	maxColH := 0
	rightMid := make(map[string]svgPt)
	leftMid := make(map[string]svgPt)

	for i, stage := range stages {
		jobs := byStage[stage]
		sort.Strings(jobs)
		n := len(jobs)
		h := n*chipH + max(0, n-1)*chipGap
		colH[i] = h
		if h > maxColH {
			maxColH = h
		}
		cx := sidePad + i*(chipW+stageGap)
		for j, name := range jobs {
			chy := topPad + labelH + j*(chipH+chipGap)
			rightMid[name] = svgPt{cx + chipW, chy + chipH/2}
			leftMid[name] = svgPt{cx, chy + chipH/2}
		}
		colCtY[i] = topPad + labelH + h/2
	}

	svgW := sidePad*2 + len(stages)*chipW + max(0, len(stages)-1)*stageGap
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

	// ── Stage columns ─────────────────────────────────────────────────────────
	for i, stage := range stages {
		cx := sidePad + i*(chipW+stageGap)
		jobs := byStage[stage]
		sort.Strings(jobs)

		// Stage name – small, gray, uppercase, centered above the chip stack.
		wf(`  <text x="%d" y="%d" text-anchor="middle" `+
			`font-family="'GitLab Sans','Segoe UI',-apple-system,BlinkMacSystemFont,sans-serif" `+
			`font-size="11" font-weight="600" letter-spacing="0.8" fill="#868686">%s</text>`,
			cx+chipW/2, topPad+13, svgEsc(strings.ToUpper(stage)))

		// Subtle separator line below stage name.
		wf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#eaeaea" stroke-width="1"/>`,
			cx, topPad+21, cx+chipW, topPad+21)

		// Job chips.
		for j, name := range jobs {
			job := p.Jobs[name]
			chy := topPad + labelH + j*(chipH+chipGap)
			color := chipColor(job)

			// Chip card (white, rounded, subtle border + shadow).
			wf(`  <rect x="%d" y="%d" width="%d" height="%d" rx="4" `+
				`fill="#ffffff" stroke="#dde1e7" stroke-width="1" filter="url(#chip-shadow)"/>`,
				cx, chy, chipW, chipH)

			// Colored status indicator circle.
			wf(`  <circle cx="%d" cy="%d" r="%d" fill="%s"/>`,
				cx+iconCX, chy+chipH/2, iconR, color)

			// Icon symbol inside the circle.
			drawChipIcon(&sb, job, cx+iconCX, chy+chipH/2)

			// Job name text.
			wf(`  <text x="%d" y="%d" dominant-baseline="middle" `+
				`font-family="'GitLab Sans','Segoe UI',-apple-system,BlinkMacSystemFont,sans-serif" `+
				`font-size="13" fill="#303030">%s</text>`,
				cx+textLeft, chy+chipH/2, svgEsc(svgTrunc(name, 20)))
		}
	}

	// ── Connectors ────────────────────────────────────────────────────────────
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
		// Classic: one connector per adjacent stage pair.
		for i := 0; i < len(stages)-1; i++ {
			x1 := sidePad + i*(chipW+stageGap) + chipW
			x2 := sidePad + (i+1)*(chipW+stageGap)
			y1 := colCtY[i]
			y2 := colCtY[i+1]
			midX := (x1 + x2) / 2

			if y1 == y2 {
				// Straight horizontal line.
				wf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"/>`,
					x1, y1, x2-7, y2, connStroke)
			} else {
				// L-shaped elbow: right → vertical → right.
				wf(`  <polyline points="%d,%d %d,%d %d,%d %d,%d" `+
					`stroke="%s" stroke-width="2" fill="none" stroke-linejoin="round"/>`,
					x1, y1, midX, y1, midX, y2, x2-7, y2, connStroke)
			}
			// Arrowhead at the destination.
			wf(`  <polygon points="%d,%d %d,%d %d,%d" fill="%s"/>`,
				x2-7, y2-4, x2, y2, x2-7, y2+4, connStroke)
		}
	}

	// ── Legend ────────────────────────────────────────────────────────────────
	legend := []struct{ color, label string }{
		{"#1f75cb", "regular"},
		{"#fc6d26", "manual"},
		{"#6b4fbb", "trigger"},
		{"#fca326", "delayed"},
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
		fmt.Fprintf(sb, "  <polyline points=\"%d,%d %d,%d %d,%d\" "+
			"stroke=\"#fff\" stroke-width=\"1.5\" fill=\"none\" stroke-linecap=\"round\" stroke-linejoin=\"round\"/>\n",
			cx-3, cy-3, cx+3, cy, cx-3, cy+3)
		return
	}
	switch job.When {
	case "manual":
		// Filled play triangle.
		fmt.Fprintf(sb, "  <polygon points=\"%d,%d %d,%d %d,%d\" fill=\"#fff\"/>\n",
			cx-3, cy-4, cx+5, cy, cx-3, cy+4)
	case "delayed":
		// Clock hands: vertical + horizontal.
		fmt.Fprintf(sb, "  <circle cx=\"%d\" cy=\"%d\" r=\"5\" stroke=\"#fff\" stroke-width=\"1.2\" fill=\"none\"/>\n", cx, cy)
		fmt.Fprintf(sb, "  <line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#fff\" stroke-width=\"1.2\" stroke-linecap=\"round\"/>\n",
			cx, cy, cx, cy-3)
		fmt.Fprintf(sb, "  <line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#fff\" stroke-width=\"1.2\" stroke-linecap=\"round\"/>\n",
			cx, cy, cx+2, cy+1)
	default:
		// Regular job: small white checkmark outline.
		fmt.Fprintf(sb, "  <polyline points=\"%d,%d %d,%d %d,%d\" "+
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
	}
	return "#1f75cb"
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
