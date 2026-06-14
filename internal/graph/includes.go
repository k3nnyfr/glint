package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/model"
)

// Includes returns a Mermaid flowchart of the full include dependency tree.
// Each node shows the jobs defined directly in that file, connected with
// dashed arrows (solid arrows represent the include hierarchy itself).
// It recurses into project:, component:, and local: includes to expose
// transitive dependencies. Includes that cannot be fetched are shown but not expanded.
func Includes(sourcePath string, rawIncludes []any, cfg fetcher.GitLabConfig) string {
	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     cfg,
		baseDir: filepath.Dir(sourcePath),
	}
	root := &treeNode{
		id:    "root",
		label: mermaidLabel(sourcePath),
		class: "main",
		jobs:  directJobs(sourcePath),
	}
	b.buildChildren(root, rawIncludes)
	return renderTree(root)
}

type treeNode struct {
	id       string
	label    string
	class    string
	children []*treeNode
	jobs     []string // job names defined directly in this file
}

// treeBuilder accumulates state while recursively traversing include entries.
type treeBuilder struct {
	counter int
	visited map[string]bool // prevents infinite loops on circular includes
	cfg     fetcher.GitLabConfig
	baseDir string // directory of the current pipeline file; changes for local includes
}

func (b *treeBuilder) nextID() string {
	b.counter++
	return fmt.Sprintf("inc%d", b.counter)
}

func (b *treeBuilder) buildChildren(parent *treeNode, rawIncludes []any) {
	for _, entry := range rawIncludes {
		parent.children = append(parent.children, b.parseEntry(entry)...)
	}
}

func (b *treeBuilder) parseEntry(entry any) []*treeNode {
	switch v := entry.(type) {
	case string:
		node := &treeNode{id: b.nextID(), label: "local: " + mermaidLabel(v), class: "local"}
		b.recurseLocal(node, v)
		return []*treeNode{node}
	case map[string]any:
		return b.parseMap(v)
	}
	return nil
}

func (b *treeBuilder) parseMap(m map[string]any) []*treeNode {
	if comp, ok := m["component"].(string); ok {
		node := &treeNode{
			id:    b.nextID(),
			label: "component:<br>" + mermaidLabel(comp),
			class: "component",
		}
		b.recurseComponent(node, comp)
		return []*treeNode{node}
	}
	if proj, ok := m["project"].(string); ok {
		ref, _ := m["ref"].(string)
		if ref == "" {
			ref = "HEAD"
		}
		files := includeFileList(m["file"])
		if len(files) == 0 {
			return []*treeNode{{
				id:    b.nextID(),
				label: fmt.Sprintf("project: %s @ %s", mermaidLabel(proj), ref),
				class: "project",
			}}
		}
		var nodes []*treeNode
		for _, f := range files {
			node := &treeNode{
				id:    b.nextID(),
				label: fmt.Sprintf("project: %s<br>%s @ %s", mermaidLabel(proj), mermaidLabel(f), ref),
				class: "project",
			}
			b.recurseProject(node, proj, f, ref)
			nodes = append(nodes, node)
		}
		return nodes
	}
	if local, ok := m["local"].(string); ok {
		node := &treeNode{id: b.nextID(), label: "local: " + mermaidLabel(local), class: "local"}
		b.recurseLocal(node, local)
		return []*treeNode{node}
	}
	if remote, ok := m["remote"].(string); ok {
		node := &treeNode{id: b.nextID(), label: "remote:<br>" + mermaidLabel(remote), class: "remote"}
		b.recurseRemote(node, remote)
		return []*treeNode{node}
	}
	if tmpl, ok := m["template"].(string); ok {
		return []*treeNode{{id: b.nextID(), label: "template:<br>" + mermaidLabel(tmpl), class: "template"}}
	}
	return nil
}

func (b *treeBuilder) recurseLocal(node *treeNode, path string) {
	absPath := filepath.Join(b.baseDir, path)
	key := "local:" + absPath
	if b.visited[key] {
		return
	}
	b.visited[key] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return
	}
	p, err := model.ParseBytes(data)
	if err != nil {
		return
	}
	node.jobs = jobNames(p)
	if len(p.Include) == 0 {
		return
	}
	orig := b.baseDir
	b.baseDir = filepath.Dir(absPath)
	b.buildChildren(node, p.Include)
	b.baseDir = orig
}

func (b *treeBuilder) recurseProject(node *treeNode, project, filePath, ref string) {
	key := fmt.Sprintf("project:%s:%s@%s", project, filePath, ref)
	if b.visited[key] || !b.cfg.HasToken() || filePath == "" {
		return
	}
	b.visited[key] = true

	data, err := b.cfg.FetchFile(project, filePath, ref)
	if err != nil {
		return
	}
	p, err := model.ParseBytes(data)
	if err != nil {
		return
	}
	node.jobs = jobNames(p)
	if len(p.Include) == 0 {
		return
	}
	b.buildChildren(node, p.Include)
}

func (b *treeBuilder) recurseRemote(node *treeNode, rawURL string) {
	key := "remote:" + rawURL
	if b.visited[key] {
		return
	}
	b.visited[key] = true

	data, err := b.cfg.FetchURL(rawURL)
	if err != nil {
		return
	}
	p, err := model.ParseBytes(data)
	if err != nil {
		return
	}
	node.jobs = jobNames(p)
	if len(p.Include) == 0 {
		return
	}
	b.buildChildren(node, p.Include)
}

func (b *treeBuilder) recurseComponent(node *treeNode, ref string) {
	if strings.ContainsRune(ref, '$') {
		return
	}
	key := "component:" + ref
	if b.visited[key] {
		return
	}
	b.visited[key] = true

	host, project, component, version, err := parseComponentRef(ref)
	if err != nil {
		return
	}
	data, err := fetchComponentFile(b.cfg.ForHost(host), project, component, version)
	if err != nil {
		return
	}
	p, err := model.ParseBytes(data)
	if err != nil {
		return
	}
	node.jobs = jobNames(p)
	if len(p.Include) == 0 {
		return
	}
	b.buildChildren(node, p.Include)
}

func renderTree(root *treeNode) string {
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
	w("    classDef job fill:#f5f5ff,stroke:#7175a0,color:#333")
	w("")

	jobCounter := 0
	var emit func(n *treeNode)
	emit = func(n *treeNode) {
		wf("    %s[\"%s\"]:::%s", n.id, n.label, n.class)
		for _, jobName := range n.jobs {
			jobCounter++
			jobID := fmt.Sprintf("job%d", jobCounter)
			wf("    %s(\"%s\"):::job", jobID, mermaidLabel(jobName))
			wf("    %s -.-> %s", n.id, jobID)
		}
		for _, child := range n.children {
			emit(child)
			wf("    %s --> %s", n.id, child.id)
		}
	}
	emit(root)

	return sb.String()
}

// directJobs parses a single pipeline file (without include resolution) and
// returns the sorted list of job names defined directly in it.
func directJobs(path string) []string {
	p, err := model.Parse(path)
	if err != nil {
		return nil
	}
	return jobNames(p)
}

// jobNames returns a sorted slice of all job names defined in p.
func jobNames(p *model.Pipeline) []string {
	if len(p.Jobs) == 0 {
		return nil
	}
	names := make([]string, 0, len(p.Jobs))
	for name := range p.Jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// parseComponentRef parses a CI/CD component reference of the form
// <host>/<project-path>/<component-name>@<version>.
func parseComponentRef(ref string) (host, project, component, version string, err error) {
	atIdx := strings.LastIndex(ref, "@")
	if atIdx < 0 || atIdx == len(ref)-1 {
		err = fmt.Errorf("component reference %q must include a version", ref)
		return
	}
	version = ref[atIdx+1:]
	path := ref[:atIdx]
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		err = fmt.Errorf("component reference %q must be <host>/<project>/<component>@<version>", ref)
		return
	}
	host = parts[0]
	component = parts[len(parts)-1]
	project = strings.Join(parts[1:len(parts)-1], "/")
	return
}

// fetchComponentFile fetches a component's template YAML, trying the single-file
// layout first and the directory layout as fallback.
func fetchComponentFile(cfg fetcher.GitLabConfig, project, component, version string) ([]byte, error) {
	if data, err := cfg.FetchFile(project, "templates/"+component+".yml", version); err == nil {
		return data, nil
	}
	data, err := cfg.FetchFile(project, "templates/"+component+"/template.yml", version)
	if err != nil {
		return nil, fmt.Errorf("component %s/%s@%s not found", project, component, version)
	}
	return data, nil
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
