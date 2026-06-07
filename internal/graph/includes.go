package graph

import (
	"fmt"
	"strings"
)

// Includes returns a Mermaid flowchart showing include file dependencies.
// sourcePath is the path to the main pipeline file; rawIncludes is Pipeline.Include.
func Includes(sourcePath string, rawIncludes []any) string {
	var sb strings.Builder
	w := func(s string) { sb.WriteString(s + "\n") }
	wf := func(f string, a ...any) { fmt.Fprintf(&sb, f+"\n", a...) }

	w("---")
	w("title: Include Dependencies")
	w("---")
	w("flowchart LR")
	w("    classDef main fill:#e24329,stroke:#c73a1e,color:#fff,font-weight:bold")
	w("    classDef project fill:#6b4fbb,stroke:#5a3fa0,color:#fff")
	w("    classDef component fill:#1aaa55,stroke:#148a43,color:#fff")
	w("    classDef local fill:#428fdc,stroke:#1068bf,color:#fff")
	w("    classDef remote fill:#868686,stroke:#686868,color:#fff")
	w("    classDef template fill:#fc6d26,stroke:#e56b1f,color:#fff")
	w("")
	wf("    root[\"%s\"]:::main", mermaidLabel(sourcePath))

	if len(rawIncludes) == 0 {
		return sb.String()
	}

	w("")

	counter := 0
	for _, entry := range rawIncludes {
		for _, n := range parseIncludeEntry(entry, &counter) {
			wf("    %s[\"%s\"]:::%s", n.id, n.label, n.class)
			wf("    root --> %s", n.id)
		}
	}

	return sb.String()
}

type incNode struct {
	id, label, class string
}

func parseIncludeEntry(entry any, counter *int) []incNode {
	newID := func() string {
		*counter++
		return fmt.Sprintf("inc%d", *counter)
	}
	switch v := entry.(type) {
	case string:
		return []incNode{{id: newID(), label: mermaidLabel(v), class: "local"}}
	case map[string]any:
		return parseIncludeMap(v, newID)
	}
	return nil
}

func parseIncludeMap(m map[string]any, newID func() string) []incNode {
	if comp, ok := m["component"].(string); ok {
		return []incNode{{
			id:    newID(),
			label: "component:<br>" + mermaidLabel(comp),
			class: "component",
		}}
	}
	if proj, ok := m["project"].(string); ok {
		ref, _ := m["ref"].(string)
		if ref == "" {
			ref = "HEAD"
		}
		files := includeFileList(m["file"])
		if len(files) == 0 {
			return []incNode{{
				id:    newID(),
				label: fmt.Sprintf("project: %s @ %s", mermaidLabel(proj), ref),
				class: "project",
			}}
		}
		var nodes []incNode
		for _, f := range files {
			nodes = append(nodes, incNode{
				id:    newID(),
				label: fmt.Sprintf("project: %s<br>%s @ %s", mermaidLabel(proj), mermaidLabel(f), ref),
				class: "project",
			})
		}
		return nodes
	}
	if local, ok := m["local"].(string); ok {
		return []incNode{{id: newID(), label: "local: " + mermaidLabel(local), class: "local"}}
	}
	if remote, ok := m["remote"].(string); ok {
		return []incNode{{id: newID(), label: "remote:<br>" + mermaidLabel(remote), class: "remote"}}
	}
	if tmpl, ok := m["template"].(string); ok {
		return []incNode{{id: newID(), label: "template:<br>" + mermaidLabel(tmpl), class: "template"}}
	}
	return nil
}

func includeFileList(v any) []string {
	switch f := v.(type) {
	case string:
		return []string{f}
	case []any:
		var out []string
		for _, item := range f {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// mermaidLabel escapes s for use inside a Mermaid double-quoted node label.
func mermaidLabel(s string) string {
	s = strings.ReplaceAll(s, `"`, `'`)
	s = strings.ReplaceAll(s, `#`, `&#35;`)
	return s
}
