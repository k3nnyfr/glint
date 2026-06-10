package graph

import (
	"fmt"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

// Pipeline returns a Mermaid flowchart of pipeline jobs grouped by stage.
//
// Hidden template jobs (names starting with ".") are excluded.
// When any job has a needs: list, DAG mode is used and individual needs arrows
// are drawn. Otherwise, classic mode draws stage-ordering arrows between
// stage subgraphs.
func Pipeline(p *model.Pipeline) string {
	var sb strings.Builder
	w := func(s string) { sb.WriteString(s + "\n") }
	wf := func(f string, a ...any) { fmt.Fprintf(&sb, f+"\n", a...) }

	w("---")
	w("title: Pipeline")
	w("---")
	w("flowchart LR")
	w("    classDef regular fill:#428fdc,stroke:#1068bf,color:#fff")
	w("    classDef manual fill:#fc6d26,stroke:#e56b1f,color:#fff")
	w("    classDef trigger fill:#6b4fbb,stroke:#5a3fa0,color:#fff")
	w("    classDef delayed fill:#fca326,stroke:#d98a1e,color:#333")
	w("")

	// Collect visible (non-template) job names; sort for stable output.
	var visible []string
	for name := range p.Jobs {
		if !strings.HasPrefix(name, ".") {
			visible = append(visible, name)
		}
	}
	sort.Strings(visible)

	if len(visible) == 0 {
		w("    empty[\"no jobs defined\"]")
		return sb.String()
	}

	// Assign stable numeric IDs so graph output is deterministic.
	jobID := make(map[string]string, len(visible))
	for i, name := range visible {
		jobID[name] = fmt.Sprintf("j%d", i+1)
	}

	// Group jobs by stage; fall back to "test" (GitLab default) when unset.
	byStage := make(map[string][]string)
	for _, name := range visible {
		stage := p.Jobs[name].Stage
		if stage == "" {
			stage = "test"
		}
		byStage[stage] = append(byStage[stage], name)
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
		stage := p.Jobs[name].Stage
		if stage == "" {
			stage = "test"
		}
		if !seen[stage] {
			stages = append(stages, stage)
			seen[stage] = true
		}
	}

	// Emit one subgraph per stage.
	for _, stage := range stages {
		jobs := byStage[stage]
		if len(jobs) == 0 {
			continue
		}
		sort.Strings(jobs)
		wf("    subgraph %s[\"%s\"]", stageID(stage), stage)
		for _, name := range jobs {
			wf("        %s[\"%s\"]:::%s", jobID[name], mermaidLabel(name), jobClass(p.Jobs[name]))
		}
		w("    end")
		w("")
	}

	// DAG mode: at least one visible job has a needs: list.
	dagMode := false
	for _, name := range visible {
		if len(p.Jobs[name].Needs) > 0 {
			dagMode = true
			break
		}
	}

	if dagMode {
		w("    %% DAG: needs dependencies")
		for _, name := range visible {
			for _, need := range p.Jobs[name].Needs {
				dep := needsJobName(need)
				if dep == "" {
					continue
				}
				if depID, ok := jobID[dep]; ok {
					wf("    %s --> %s", depID, jobID[name])
				}
			}
		}
	} else if len(stages) > 1 {
		w("    %% Classic: stage ordering")
		ids := make([]string, len(stages))
		for i, s := range stages {
			ids[i] = stageID(s)
		}
		wf("    %s", strings.Join(ids, " --> "))
	}

	return sb.String()
}

func stageID(s string) string { return "stage_" + sanitizeID(s) }

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func jobClass(job model.Job) string {
	if job.Trigger != nil {
		return "trigger"
	}
	switch job.When {
	case "manual":
		return "manual"
	case "delayed":
		return "delayed"
	}
	return "regular"
}

func needsJobName(need any) string {
	switch v := need.(type) {
	case string:
		return v
	case map[string]any:
		if _, cross := v["pipeline"]; cross {
			return "" // cross-pipeline needs have no local counterpart
		}
		if j, ok := v["job"].(string); ok {
			return j
		}
	}
	return ""
}
