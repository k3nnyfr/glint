package graph

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/model"
)

// ── mermaidLabel ──────────────────────────────────────────────────────────────

func TestMermaidLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{`has "quotes"`, "has 'quotes'"},
		{"has#hash", "has&#35;hash"},
		{`"quoted"#both`, "'quoted'&#35;both"},
	}
	for _, tc := range cases {
		got := mermaidLabel(tc.in)
		if got != tc.want {
			t.Errorf("mermaidLabel(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

// ── includeFileList ───────────────────────────────────────────────────────────

func TestIncludeFileList(t *testing.T) {
	if got := includeFileList("file.yml"); len(got) != 1 || got[0] != "file.yml" {
		t.Error("string form")
	}
	if got := includeFileList([]any{"a.yml", "b.yml"}); len(got) != 2 {
		t.Error("slice form")
	}
	if got := includeFileList([]any{"ok.yml", 42}); len(got) != 1 {
		t.Error("mixed slice drops non-strings")
	}
	if got := includeFileList(nil); got != nil {
		t.Error("nil should return nil")
	}
	if got := includeFileList(123); got != nil {
		t.Error("unknown type returns nil")
	}
}

// ── parseComponentRef ─────────────────────────────────────────────────────────

func TestParseComponentRef(t *testing.T) {
	cases := []struct {
		ref       string
		wantHost  string
		wantProj  string
		wantComp  string
		wantVer   string
		wantErr   bool
	}{
		{
			ref: "gitlab.com/group/project/component@v1.0",
			wantHost: "gitlab.com", wantProj: "group/project", wantComp: "component", wantVer: "v1.0",
		},
		{
			ref: "gitlab.com/a/b/c/d@main",
			wantHost: "gitlab.com", wantProj: "a/b/c", wantComp: "d", wantVer: "main",
		},
		{ref: "no-at-sign", wantErr: true},
		{ref: "no-at-sign@", wantErr: true},
		{ref: "host/comp@v1", wantErr: true}, // only 2 parts — need at least 3
	}
	for _, tc := range cases {
		host, proj, comp, ver, err := parseComponentRef(tc.ref)
		if tc.wantErr {
			if err == nil { t.Errorf("parseComponentRef(%q): expected error", tc.ref) }
			continue
		}
		if err != nil { t.Errorf("parseComponentRef(%q): %v", tc.ref, err) }
		if host != tc.wantHost { t.Errorf("host: got %q want %q", host, tc.wantHost) }
		if proj != tc.wantProj { t.Errorf("project: got %q want %q", proj, tc.wantProj) }
		if comp != tc.wantComp { t.Errorf("component: got %q want %q", comp, tc.wantComp) }
		if ver != tc.wantVer  { t.Errorf("version: got %q want %q", ver, tc.wantVer) }
	}
}

// ── jobNames ──────────────────────────────────────────────────────────────────

func TestJobNames_Empty(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}}
	if got := jobNames(p); got != nil {
		t.Errorf("empty pipeline: expected nil, got %v", got)
	}
}

// ── treeBuilder helpers ───────────────────────────────────────────────────────

func TestNextID(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}}
	if b.nextID() != "inc1" { t.Error("first ID") }
	if b.nextID() != "inc2" { t.Error("second ID") }
}

func TestParseEntry_String(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, baseDir: "/tmp"}
	nodes := b.parseEntry("some/local.yml")
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
	if !strings.Contains(nodes[0].label, "local:") { t.Error("expected local: label") }
}

func TestParseEntry_Unknown(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}}
	nodes := b.parseEntry(42) // unknown type
	if len(nodes) != 0 { t.Error("expected no nodes for unknown type") }
}

func TestParseMap_Template(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}}
	nodes := b.parseMap(map[string]any{"template": "Auto-DevOps"})
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
	if !strings.Contains(nodes[0].label, "template:") { t.Error("expected template: label") }
}

func TestParseMap_Remote(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	nodes := b.parseMap(map[string]any{"remote": "https://example.com/ci.yml"})
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
	if !strings.Contains(nodes[0].label, "remote:") { t.Error("expected remote: label") }
}

func TestParseMap_Project_NoFiles(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	nodes := b.parseMap(map[string]any{"project": "g/p"})
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
	if !strings.Contains(nodes[0].label, "project:") { t.Error("expected project: label") }
}

func TestParseMap_Project_WithRef(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	nodes := b.parseMap(map[string]any{
		"project": "g/p",
		"ref":     "main",
		"file":    "templates/ci.yml",
	})
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
}

func TestParseMap_Project_MultipleFiles(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	nodes := b.parseMap(map[string]any{
		"project": "g/p",
		"file":    []any{"a.yml", "b.yml"},
	})
	if len(nodes) != 2 { t.Fatalf("expected 2 nodes, got %d", len(nodes)) }
}

func TestParseMap_Component_WithDollar(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}}
	// Component ref with $ — skipped (dynamic, can't resolve)
	nodes := b.parseMap(map[string]any{"component": "${CI_SERVER_HOST}/group/project/comp@v1"})
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
	if nodes[0].jobs != nil { t.Error("jobs should be nil for unresolvable component") }
}

func TestParseMap_Component_BadRef(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	nodes := b.parseMap(map[string]any{"component": "no-at-sign"})
	if len(nodes) != 1 { t.Fatalf("expected 1 node, got %d", len(nodes)) }
}

func TestParseMap_Unknown(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}}
	nodes := b.parseMap(map[string]any{"unknown_key": "value"})
	if len(nodes) != 0 { t.Error("expected no nodes for unknown map keys") }
}

// ── recurseLocal ──────────────────────────────────────────────────────────────

func TestRecurseLocal_FileNotFound(t *testing.T) {
	b := &treeBuilder{visited: map[string]bool{}, baseDir: "/nonexistent"}
	node := &treeNode{id: "n1"}
	b.recurseLocal(node, "missing.yml")
	if node.jobs != nil { t.Error("jobs should remain nil on missing file") }
}

func TestRecurseLocal_ValidFile(t *testing.T) {
	dir := t.TempDir()
	ciPath := filepath.Join(dir, "ci.yml")
	if err := os.WriteFile(ciPath, []byte("job1:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &treeBuilder{visited: map[string]bool{}, baseDir: dir}
	node := &treeNode{id: "n1"}
	b.recurseLocal(node, "ci.yml")
	if len(node.jobs) == 0 { t.Error("expected jobs to be populated") }
}

func TestRecurseLocal_VisitedPrevents_Loop(t *testing.T) {
	dir := t.TempDir()
	ciPath := filepath.Join(dir, "ci.yml")
	if err := os.WriteFile(ciPath, []byte("job1:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &treeBuilder{
		visited: map[string]bool{"local:" + ciPath: true},
		baseDir: dir,
	}
	node := &treeNode{id: "n1"}
	b.recurseLocal(node, "ci.yml")
	if len(node.jobs) != 0 { t.Error("visited node should not be parsed again") }
}

func TestRecurseLocal_WithNestedIncludes(t *testing.T) {
	dir := t.TempDir()
	// parent.yml includes child.yml
	childPath := filepath.Join(dir, "child.yml")
	if err := os.WriteFile(childPath, []byte("child-job:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parentPath := filepath.Join(dir, "parent.yml")
	parentContent := "include:\n  - local: child.yml\nparent-job:\n  script: echo\n"
	if err := os.WriteFile(parentPath, []byte(parentContent), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &treeBuilder{visited: map[string]bool{}, baseDir: dir}
	node := &treeNode{id: "n1"}
	b.recurseLocal(node, "parent.yml")
	if len(node.children) == 0 { t.Error("expected children from nested include") }
}

func TestRecurseLocal_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	ciPath := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(ciPath, []byte(":\tbad\tyaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &treeBuilder{visited: map[string]bool{}, baseDir: dir}
	node := &treeNode{id: "n1"}
	b.recurseLocal(node, "bad.yml")
	if node.jobs != nil { t.Error("invalid YAML should not set jobs") }
}

// ── directJobs ────────────────────────────────────────────────────────────────

func TestDirectJobs_MissingFile(t *testing.T) {
	if jobs := directJobs("/nonexistent/ci.yml"); jobs != nil {
		t.Error("expected nil for missing file")
	}
}

func TestDirectJobs_ValidFile(t *testing.T) {
	dir := t.TempDir()
	ciPath := filepath.Join(dir, "ci.yml")
	if err := os.WriteFile(ciPath, []byte("build:\n  script: make\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs := directJobs(ciPath)
	if len(jobs) == 0 { t.Error("expected jobs from valid file") }
}

// ── renderTree ────────────────────────────────────────────────────────────────

func TestRenderTree_Basic(t *testing.T) {
	root := &treeNode{
		id:    "root",
		label: "main.yml",
		class: "main",
		jobs:  []string{"job-a", "job-b"},
		children: []*treeNode{
			{id: "inc1", label: "local: child.yml", class: "local"},
		},
	}
	out := renderTree(root)
	if !strings.Contains(out, "main.yml") { t.Error("expected root label") }
	if !strings.Contains(out, "job-a") { t.Error("expected job-a") }
	if !strings.Contains(out, "child.yml") { t.Error("expected child label") }
	if !strings.Contains(out, "root --> inc1") { t.Error("expected edge root→inc1") }
}

// ── Includes (integration) ────────────────────────────────────────────────────

func TestIncludes_LocalFile(t *testing.T) {
	dir := t.TempDir()

	childPath := filepath.Join(dir, "child.yml")
	if err := os.WriteFile(childPath, []byte("child-job:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(dir, "pipeline.yml")
	rawIncludes := []any{map[string]any{"local": "child.yml"}}

	out := Includes(sourcePath, rawIncludes, fetcher.GitLabConfig{})
	if !strings.Contains(out, "local:") { t.Error("expected local: node") }
}

func TestIncludes_NoIncludes(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "pipeline.yml")
	out := Includes(sourcePath, nil, fetcher.GitLabConfig{})
	if !strings.Contains(out, "flowchart") { t.Error("expected flowchart") }
}

// ── fetchComponentFile (graph package) ────────────────────────────────────────

func TestGraphFetchComponentFile_PrimarySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "comp-job:\n  script: echo")
	}))
	defer srv.Close()
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL}
	data, err := fetchComponentFile(cfg, "g/p", "mycomp", "v1")
	if err != nil {
		t.Fatalf("primary success: unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

// ── recurseRemote ─────────────────────────────────────────────────────────────

func TestRecurseRemote_AlreadyVisited(t *testing.T) {
	b := &treeBuilder{
		visited: map[string]bool{"remote:http://example.com/ci.yml": true},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseRemote(node, "http://example.com/ci.yml")
	// Visited: nothing should be fetched or added.
}

func TestRecurseRemote_ValidWithJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "remote-job:\n  script: echo")
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseRemote(node, srv.URL+"/ci.yml")
	if len(node.jobs) == 0 {
		t.Error("expected job names from remote YAML")
	}
}

func TestRecurseRemote_InvalidYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, ":\tbad\tyaml")
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseRemote(node, srv.URL+"/bad.yml")
	// ParseBytes fails → early return; no panic.
}

func TestRecurseRemote_WithSubIncludes(t *testing.T) {
	// The remote YAML includes a second URL (sub-includes path).
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if callCount == 1 {
			// First call: returns YAML with a remote sub-include.
			fmt.Fprintln(w, "include:\n  - remote: http://127.0.0.1:0/sub.yml\nparent-job:\n  script: echo")
		} else {
			// Sub-include fetch will fail (connection refused) — that's OK; we
			// just need the sub-include routing path to be executed.
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
		counter: 0,
	}
	node := &treeNode{id: "n1"}
	b.recurseRemote(node, srv.URL+"/ci.yml")
	// parent-job should be in node.jobs even if sub-include fails.
	if len(node.jobs) == 0 {
		t.Error("expected job names from remote YAML")
	}
}

// ── recurseComponent ─────────────────────────────────────────────────────────

// tlsHost returns a helper that creates a TLS test server, swaps http.DefaultTransport
// so the test client trusts the self-signed cert, and returns (close, host).
func tlsComponent(t *testing.T, handler http.HandlerFunc) (host string) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	orig := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = orig })
	return srv.URL[len("https://"):]
}

func TestRecurseComponent_AlreadyVisited(t *testing.T) {
	b := &treeBuilder{
		visited: map[string]bool{"component:gitlab.com/g/p/comp@v1": true},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, "gitlab.com/g/p/comp@v1")
	// Already visited: nothing happens.
}

func TestRecurseComponent_DollarRef(t *testing.T) {
	// ref containing $ → early return (line 196-197)
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, "$CI_SERVER/g/p/comp@v1")
	// no panic, node unchanged
}

func TestRecurseComponent_BadRef(t *testing.T) {
	// ref with no @ → parseComponentRef fails (line 206-208)
	b := &treeBuilder{visited: map[string]bool{}, cfg: fetcher.GitLabConfig{}}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, "gitlab.com/g/p/mycomp-no-version")
	// no panic
}

func TestRecurseComponent_FetchError(t *testing.T) {
	host := tlsComponent(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, host+"/g/p/mycomp@v1")
	// 404 → fetch error → early return; no panic.
}

func TestRecurseComponent_InvalidYAML(t *testing.T) {
	host := tlsComponent(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, ":\tbad\tyaml")
	})
	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, host+"/g/p/mycomp@v1")
	// fetch succeeds but ParseBytes fails → early return; no panic.
}

func TestRecurseComponent_ValidYAML(t *testing.T) {
	host := tlsComponent(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "comp-job:\n  script: echo component")
	})
	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, host+"/g/p/mycomp@v1")
	if len(node.jobs) == 0 {
		t.Error("expected job names from component YAML")
	}
}

// ── recurseProject ────────────────────────────────────────────────────────────

func TestRecurseProject_AlreadyVisited(t *testing.T) {
	b := &treeBuilder{
		visited: map[string]bool{"project:g/p:ci.yml@main": true},
		cfg:     fetcher.GitLabConfig{Token: "tok"},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseProject(node, "g/p", "ci.yml", "main")
	// Already visited: nothing happens.
}

func TestRecurseProject_WithToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "proj-job:\n  script: echo project")
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseProject(node, "g/p", "ci.yml", "main")
	if len(node.jobs) == 0 {
		t.Error("expected job names from project YAML")
	}
}

func TestRecurseProject_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseProject(node, "g/p", "ci.yml", "main")
	// fetch fails → early return; no panic.
}

func TestRecurseProject_InvalidYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, ":\tbad\tyaml")
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseProject(node, "g/p", "ci.yml", "main")
	// ParseBytes fails → early return; no panic.
}

func TestRecurseProject_WithSubIncludes(t *testing.T) {
	// Project YAML includes sub-includes → covers recurseProject line 170 (buildChildren).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "include:\n  - remote: http://127.0.0.1:0/sub.yml\nproj-job:\n  script: echo")
	}))
	defer srv.Close()

	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseProject(node, "g/p", "ci.yml", "main")
	// proj-job should be populated; sub-include fetch fails (port 0) — that's fine.
	if len(node.jobs) == 0 {
		t.Error("expected job names from project YAML")
	}
}

func TestRecurseComponent_WithSubIncludes(t *testing.T) {
	// Component YAML includes sub-includes → covers recurseComponent line 221 (buildChildren).
	host := tlsComponent(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "include:\n  - remote: http://127.0.0.1:0/sub.yml\ncomp-job:\n  script: echo")
	})
	b := &treeBuilder{
		visited: map[string]bool{},
		cfg:     fetcher.GitLabConfig{},
		baseDir: "/tmp",
	}
	node := &treeNode{id: "n1"}
	b.recurseComponent(node, host+"/g/p/mycomp@v1")
	if len(node.jobs) == 0 {
		t.Error("expected job names from component YAML")
	}
}

// ── fetchComponentFile (fallback path) ───────────────────────────────────────

func TestGraphFetchComponentFile_FallbackSuccess(t *testing.T) {
	// Primary path (templates/comp.yml) returns 404; fallback (templates/comp/template.yml) returns 200.
	var reqCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		if reqCount == 1 {
			w.WriteHeader(404)
		} else {
			w.WriteHeader(200)
			fmt.Fprintln(w, "fallback-job:\n  script: echo")
		}
	}))
	defer srv.Close()
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL}
	data, err := fetchComponentFile(cfg, "g/p", "mycomp", "v1")
	if err != nil {
		t.Fatalf("fallback success: unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data from fallback path")
	}
}

// Ensure model import is used.
var _ = model.Job{}
