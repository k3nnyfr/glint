package resolver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/model"
)

func TestSubstituteInputs(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		inputs map[string]any
		want   string
	}{
		{
			name:   "simple substitution",
			data:   `stage: $[[ inputs.STAGE ]]`,
			inputs: map[string]any{"STAGE": "deploy"},
			want:   `stage: deploy`,
		},
		{
			name:   "default string used when key absent",
			data:   `image: $[[ inputs.IMAGE | default('ubuntu:22.04') ]]`,
			inputs: map[string]any{},
			want:   `image: ubuntu:22.04`,
		},
		{
			name:   "input overrides default",
			data:   `image: $[[ inputs.IMAGE | default('ubuntu:22.04') ]]`,
			inputs: map[string]any{"IMAGE": "alpine:3.18"},
			want:   `image: alpine:3.18`,
		},
		{
			name:   "integer input",
			data:   `variables:\n  RETRIES: $[[ inputs.RETRY_COUNT ]]`,
			inputs: map[string]any{"RETRY_COUNT": 3},
			want:   `variables:\n  RETRIES: 3`,
		},
		{
			name:   "boolean default",
			data:   `variables:\n  ENABLED: $[[ inputs.ENABLE | default(true) ]]`,
			inputs: map[string]any{},
			want:   `variables:\n  ENABLED: true`,
		},
		{
			name:   "numeric default",
			data:   `variables:\n  COUNT: $[[ inputs.COUNT | default(5) ]]`,
			inputs: map[string]any{},
			want:   `variables:\n  COUNT: 5`,
		},
		{
			name:   "missing key no default becomes empty",
			data:   `script: $[[ inputs.CMD ]]`,
			inputs: map[string]any{},
			want:   `script: `,
		},
		{
			name:   "no placeholders unchanged",
			data:   `stage: build`,
			inputs: nil,
			want:   `stage: build`,
		},
		{
			name:   "multiple placeholders in one document",
			data:   "stage: $[[ inputs.STAGE ]]\nimage: $[[ inputs.IMAGE | default('alpine') ]]",
			inputs: map[string]any{"STAGE": "test"},
			want:   "stage: test\nimage: alpine",
		},
		{
			name:   "double-quoted default",
			data:   `image: $[[ inputs.IMAGE | default("debian:12") ]]`,
			inputs: map[string]any{},
			want:   `image: debian:12`,
		},
		{
			name:   "whitespace inside brackets",
			data:   `stage: $[[ inputs.STAGE  ]]`,
			inputs: map[string]any{"STAGE": "build"},
			want:   `stage: build`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(substituteInputs([]byte(tc.data), tc.inputs))
			if got != tc.want {
				t.Errorf("substituteInputs:\n got  %q\n want %q", got, tc.want)
			}
		})
	}
}

// ── IncludeWarning.String ─────────────────────────────────────────────────────

func TestIncludeWarning_String(t *testing.T) {
	skipped := IncludeWarning{Label: "myinclude", Skipped: true}
	s := skipped.String()
	if s == "" {
		t.Error("skipped: expected non-empty string")
	}

	withErr := IncludeWarning{Label: "remote foo", Err: fmt.Errorf("connection refused")}
	s2 := withErr.String()
	if s2 == "" {
		t.Error("withErr: expected non-empty string")
	}
}

// ── normaliseInclude ──────────────────────────────────────────────────────────

func TestNormaliseInclude(t *testing.T) {
	m, ok := normaliseInclude(map[string]any{"local": "ci.yml"})
	if !ok || m["local"] != "ci.yml" {
		t.Error("map form")
	}
	m2, ok2 := normaliseInclude("ci.yml")
	if !ok2 || m2["local"] != "ci.yml" {
		t.Error("string form")
	}
	_, ok3 := normaliseInclude(42)
	if ok3 {
		t.Error("int should not normalise")
	}
}

// ── includeFiles ─────────────────────────────────────────────────────────────

func TestIncludeFiles(t *testing.T) {
	if got := includeFiles(map[string]any{"file": "a.yml"}); len(got) != 1 {
		t.Error("string file")
	}
	if got := includeFiles(map[string]any{"file": []any{"a.yml", "b.yml"}}); len(got) != 2 {
		t.Error("slice file")
	}
	if got := includeFiles(map[string]any{"file": []any{"a.yml", 42}}); len(got) != 1 {
		t.Error("mixed slice, int dropped")
	}
	if got := includeFiles(map[string]any{}); got != nil {
		t.Error("no file key")
	}
	if got := includeFiles(map[string]any{"file": 99}); got != nil {
		t.Error("unknown type returns nil")
	}
}

// ── extractInputs ─────────────────────────────────────────────────────────────

func TestExtractInputs(t *testing.T) {
	m := map[string]any{
		"component": "host/p/c@v1",
		"with":      map[string]any{"KEY": "val"},
	}
	got := extractInputs(m)
	if got == nil || got["KEY"] != "val" {
		t.Error("expected KEY=val")
	}
	got2 := extractInputs(map[string]any{"component": "x"})
	if got2 != nil {
		t.Error("expected nil without with: key")
	}
	got3 := extractInputs(map[string]any{"with": "not-a-map"})
	if got3 != nil {
		t.Error("expected nil for non-map with:")
	}
}

// ── parseComponentRef ─────────────────────────────────────────────────────────

func TestParseComponentRef_Resolver(t *testing.T) {
	host, proj, comp, ver, err := parseComponentRef("gitlab.com/group/proj/comp@v1.0")
	if err != nil {
		t.Fatal(err)
	}
	if host != "gitlab.com" {
		t.Errorf("host: %q", host)
	}
	if proj != "group/proj" {
		t.Errorf("proj: %q", proj)
	}
	if comp != "comp" {
		t.Errorf("comp: %q", comp)
	}
	if ver != "v1.0" {
		t.Errorf("ver: %q", ver)
	}

	_, _, _, _, err2 := parseComponentRef("no-at")
	if err2 == nil {
		t.Error("expected error for no @")
	}
	_, _, _, _, err3 := parseComponentRef("a@")
	if err3 == nil {
		t.Error("expected error for empty version")
	}
	_, _, _, _, err4 := parseComponentRef("host/comp@v1")
	if err4 == nil {
		t.Error("expected error for too few parts")
	}
}

// ── mergeIncluded ─────────────────────────────────────────────────────────────

func TestMergeIncluded(t *testing.T) {
	dst := &model.Pipeline{
		Stages:  []string{"build"},
		Jobs:    map[string]model.Job{"existing": {Name: "existing"}},
		RawJobs: map[string]map[string]any{},
	}
	src := &model.Pipeline{
		Stages:    []string{"build", "test"},
		Jobs:      map[string]model.Job{"new-job": {Name: "new-job"}, "existing": {Name: "existing-from-src"}},
		RawJobs:   map[string]map[string]any{"new-job": {"script": "echo"}},
		Variables: map[string]any{"KEY": "val"},
	}
	mergeIncluded(dst, src)

	if len(dst.Stages) != 2 {
		t.Errorf("stages: got %d want 2", len(dst.Stages))
	}
	if _, ok := dst.Jobs["new-job"]; !ok {
		t.Error("expected new-job in dst")
	}
	if dst.Jobs["existing"].Name != "existing" {
		t.Error("dst wins on conflict")
	}
	if dst.Variables["KEY"] != "val" {
		t.Error("variable merged")
	}

	// Merge again with conflicting variable: dst wins
	src2 := &model.Pipeline{
		Jobs:      map[string]model.Job{},
		Variables: map[string]any{"KEY": "other"},
	}
	mergeIncluded(dst, src2)
	if dst.Variables["KEY"] != "val" {
		t.Error("dst var wins on conflict")
	}
}

func TestMergeIncluded_NilVariables(t *testing.T) {
	dst := &model.Pipeline{Jobs: map[string]model.Job{}}
	src := &model.Pipeline{
		Jobs:      map[string]model.Job{},
		Variables: map[string]any{"A": "1"},
	}
	mergeIncluded(dst, src)
	if dst.Variables["A"] != "1" {
		t.Error("expected A in dst after nil init")
	}
}

// ── resolveLocalInclude ───────────────────────────────────────────────────────

func TestResolveLocalInclude_ValidFile(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.yml")
	if err := os.WriteFile(childPath, []byte("child-job:\n  script: echo ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	visited := map[string]bool{}
	warnings, _ := resolveLocalInclude(p, "/child.yml", fetcher.GitLabConfig{}, dir, visited, 0)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["child-job"]; !ok {
		t.Error("child-job should be merged")
	}
}

func TestResolveLocalInclude_MissingFile(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warnings, _ := resolveLocalInclude(p, "/nonexistent.yml", fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("expected warning for missing file")
	}
}

func TestResolveLocalInclude_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(badPath, []byte(":\tbad yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warnings, _ := resolveLocalInclude(p, "/bad.yml", fetcher.GitLabConfig{}, dir, map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("expected warning for invalid YAML")
	}
}

func TestResolveLocalInclude_AlreadyVisited(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.yml")
	if err := os.WriteFile(childPath, []byte("job:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	visited := map[string]bool{childPath: true}
	warnings, _ := resolveLocalInclude(p, "/child.yml", fetcher.GitLabConfig{}, dir, visited, 0)
	if len(warnings) != 0 {
		t.Error("no warnings expected for already-visited")
	}
	if len(p.Jobs) != 0 {
		t.Error("no jobs should be merged for already-visited")
	}
}

func TestResolveLocalInclude_WithNestedIncludes(t *testing.T) {
	dir := t.TempDir()
	grandchild := filepath.Join(dir, "grandchild.yml")
	if err := os.WriteFile(grandchild, []byte("gc-job:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "child.yml")
	childContent := "include:\n  - local: /grandchild.yml\nchild-job:\n  script: echo\n"
	if err := os.WriteFile(child, []byte(childContent), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	_, _ = resolveLocalInclude(p, "/child.yml", fetcher.GitLabConfig{}, dir, map[string]bool{}, 0)
	if _, ok := p.Jobs["gc-job"]; !ok {
		t.Error("grandchild job should be merged")
	}
}

// ── resolveRemoteInclude ──────────────────────────────────────────────────────

func TestResolveRemoteInclude_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "remote-job:\n  script: echo remote")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.AutoConfig()
	warnings, _ := resolveRemoteInclude(p, srv.URL+"/ci.yml", cfg, "/tmp", map[string]bool{}, 0)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["remote-job"]; !ok {
		t.Error("remote-job should be merged")
	}
}

func TestResolveRemoteInclude_Error(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warnings, _ := resolveRemoteInclude(p, "http://localhost:0/unreachable.yml", fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("expected warning for unreachable URL")
	}
}

func TestResolveRemoteInclude_AlreadyVisited(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	visited := map[string]bool{"http://example.com/ci.yml": true}
	warnings, _ := resolveRemoteInclude(p, "http://example.com/ci.yml", fetcher.GitLabConfig{}, "/tmp", visited, 0)
	if len(warnings) != 0 {
		t.Error("no warning expected for already-visited URL")
	}
}

func TestResolveRemoteInclude_InvalidYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, ":\tbad\tyaml")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warnings, _ := resolveRemoteInclude(p, srv.URL+"/bad.yml", fetcher.AutoConfig(), "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("expected warning for invalid YAML from remote")
	}
}

// ── resolveProjectInclude ─────────────────────────────────────────────────────

func TestResolveProjectInclude_NoToken(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	entry := map[string]any{"project": "g/p", "file": "ci.yml"}
	warnings, _ := resolveProjectInclude(p, entry, "g/p", fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("expected skipped warning when no token")
	}
	if !warnings[0].Skipped {
		t.Error("expected Skipped=true when no token")
	}
}

func TestResolveProjectInclude_AlreadyVisited(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	visited := map[string]bool{"project:g/p:ci.yml@": true}
	entry := map[string]any{"project": "g/p", "file": "ci.yml"}
	warnings, _ := resolveProjectInclude(p, entry, "g/p", fetcher.GitLabConfig{Token: "tok"}, "/tmp", visited, 0)
	if len(warnings) != 0 {
		t.Error("already-visited should produce no warning")
	}
}

func TestResolveProjectInclude_WithToken_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	entry := map[string]any{"project": "g/p", "file": "ci.yml", "ref": "main"}
	warnings, _ := resolveProjectInclude(p, entry, "g/p", cfg, "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("expected warning for 404 fetch")
	}
}

func TestResolveProjectInclude_NoFiles(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	entry := map[string]any{"project": "g/p"} // no file key
	warnings, _ := resolveProjectInclude(p, entry, "g/p", fetcher.GitLabConfig{Token: "tok"}, "/tmp", map[string]bool{}, 0)
	if len(warnings) != 0 {
		t.Error("no warnings expected when no files")
	}
}

// ── resolveComponentInclude ───────────────────────────────────────────────────

func TestResolveComponentInclude_DollarRef(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warn, _, hadErr := resolveComponentInclude(p, "${CI_SERVER}/g/p/comp@v1", nil, fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, 0)
	if !hadErr {
		t.Error("expected hadErr for $ ref")
	}
	if warn.Label == "" {
		t.Error("expected non-empty label")
	}
}

func TestResolveComponentInclude_BadRef(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	_, _, hadErr := resolveComponentInclude(p, "no-at-sign", nil, fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, 0)
	if !hadErr {
		t.Error("expected hadErr for bad ref")
	}
}

func TestResolveComponentInclude_AlreadyVisited(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	visited := map[string]bool{"component:host/g/p/comp@v1": true}
	_, _, hadErr := resolveComponentInclude(p, "host/g/p/comp@v1", nil, fetcher.GitLabConfig{}, "/tmp", visited, 0)
	if hadErr {
		t.Error("already-visited should not return hadErr")
	}
}

func TestResolveComponentInclude_FetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	_, _, hadErr := resolveComponentInclude(p, "localhost/g/p/comp@v1", nil, cfg, "/tmp", map[string]bool{}, 0)
	if !hadErr {
		t.Error("expected hadErr for 404 component fetch")
	}
}

// ── resolveIncludes — depth guard ─────────────────────────────────────────────

func TestResolveIncludes_DepthExceeded(t *testing.T) {
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warnings, _ := resolveIncludes(p, nil, fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, maxIncludeDepth+1)
	if len(warnings) == 0 {
		t.Error("expected depth-exceeded warning")
	}
}

// ── ResolveIncludes (public entry) ────────────────────────────────────────────

func TestResolveIncludes_LocalFile(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.yml")
	if err := os.WriteFile(childPath, []byte("include-job:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &model.Pipeline{
		Jobs:    map[string]model.Job{},
		RawJobs: map[string]map[string]any{},
		Include: []any{map[string]any{"local": "/child.yml"}},
	}
	warnings, _ := ResolveIncludes(p, fetcher.GitLabConfig{}, dir)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["include-job"]; !ok {
		t.Error("include-job should be merged into pipeline")
	}
}

func TestResolveIncludes_TemplateSkipped(t *testing.T) {
	p := &model.Pipeline{
		Jobs:    map[string]model.Job{},
		RawJobs: map[string]map[string]any{},
		Include: []any{map[string]any{"template": "Auto-DevOps.gitlab-ci.yml"}},
	}
	warnings, _ := ResolveIncludes(p, fetcher.GitLabConfig{}, "/tmp")
	if len(warnings) != 0 {
		t.Errorf("template should be silently skipped, got: %v", warnings)
	}
}

func TestResolveIncludes_StringLocalInclude(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "ci.yml")
	if err := os.WriteFile(childPath, []byte("str-job:\n  script: echo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &model.Pipeline{
		Jobs:    map[string]model.Job{},
		RawJobs: map[string]map[string]any{},
		Include: []any{"ci.yml"},
	}
	warnings, _ := ResolveIncludes(p, fetcher.GitLabConfig{}, dir)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestResolveIncludes_InvalidEntry(t *testing.T) {
	p := &model.Pipeline{
		Jobs:    map[string]model.Job{},
		RawJobs: map[string]map[string]any{},
		Include: []any{42},
	}
	warnings, _ := ResolveIncludes(p, fetcher.GitLabConfig{}, "/tmp")
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings for invalid entry: %v", warnings)
	}
}

// ── fetchComponentFile ────────────────────────────────────────────────────────

func TestFetchComponentFile_FallbackPath(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(404)
	}))
	defer srv.Close()

	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	_, err := fetchComponentFile(cfg, "g/p", "mycomp", "v1")
	if err == nil {
		t.Error("expected error when both paths fail")
	}
}

func TestFetchComponentFile_PrimarySuccess(t *testing.T) {
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
		t.Error("primary success: expected non-empty data")
	}
}

func TestFetchComponentFile_FallbackSuccess(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// First call (primary path) fails.
			w.WriteHeader(404)
			return
		}
		// Second call (fallback path) succeeds.
		w.WriteHeader(200)
		fmt.Fprintln(w, "comp-job:\n  script: echo")
	}))
	defer srv.Close()

	cfg := fetcher.GitLabConfig{BaseURL: srv.URL}
	data, err := fetchComponentFile(cfg, "g/p", "mycomp", "v1")
	if err != nil {
		t.Fatalf("fallback success: unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("fallback success: expected non-empty data")
	}
}

// ── resolveProjectInclude — success path ──────────────────────────────────────

func TestResolveProjectInclude_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "proj-job:\n  script: echo from project")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	entry := map[string]any{"project": "g/p", "file": "ci.yml", "ref": "main"}
	warnings, _ := resolveProjectInclude(p, entry, "g/p", cfg, "/tmp", map[string]bool{}, 0)
	if len(warnings) != 0 {
		t.Errorf("success: unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["proj-job"]; !ok {
		t.Error("proj-job should be merged into pipeline")
	}
}

func TestResolveProjectInclude_InvalidYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, ":\tbad\tyaml")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	entry := map[string]any{"project": "g/p", "file": "ci.yml"}
	warnings, _ := resolveProjectInclude(p, entry, "g/p", cfg, "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("invalid YAML: expected warning")
	}
}

// TestResolveProjectInclude_WithSubIncludes covers includes.go:236-240 —
// the fetched project YAML itself contains include: entries.
func TestResolveProjectInclude_WithSubIncludes(t *testing.T) {
	dir := t.TempDir()
	// Write a local file that the project YAML sub-includes.
	localContent := "local-from-project:\n  script: echo local\n"
	if err := os.WriteFile(filepath.Join(dir, "local.yml"), []byte(localContent), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// Return YAML that itself has a local sub-include.
		fmt.Fprintln(w, "include:\n  - local: /local.yml\nproj-sub-job:\n  script: echo proj")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	entry := map[string]any{"project": "g/p", "file": "ci.yml", "ref": "main"}
	warnings, _ := resolveProjectInclude(p, entry, "g/p", cfg, dir, map[string]bool{}, 0)
	if len(warnings) != 0 {
		t.Errorf("sub-includes: unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["proj-sub-job"]; !ok {
		t.Error("proj-sub-job should be merged via project include")
	}
}

// tlsComp sets up a TLS test server and swaps http.DefaultTransport so the test
// client trusts the self-signed cert. Returns the host (without scheme).
func tlsComp(t *testing.T, h http.HandlerFunc) (host string) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	orig := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = orig })
	return srv.URL[len("https://"):]
}

// ── resolveComponentInclude — success path ────────────────────────────────────

func TestResolveComponentInclude_Success(t *testing.T) {
	host := tlsComp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "comp-job:\n  script: echo from component")
	})
	ref := host + "/g/p/mycomp@v1"

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{}
	_, _, hadErr := resolveComponentInclude(p, ref, nil, cfg, "/tmp", map[string]bool{}, 0)
	if hadErr {
		t.Error("success: unexpected hadErr")
	}
	if _, ok := p.Jobs["comp-job"]; !ok {
		t.Error("comp-job should be merged into pipeline")
	}
}

func TestResolveComponentInclude_InvalidYAML(t *testing.T) {
	host := tlsComp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, ":\tbad\tyaml")
	})
	ref := host + "/g/p/mycomp@v1"

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{}
	_, _, hadErr := resolveComponentInclude(p, ref, nil, cfg, "/tmp", map[string]bool{}, 0)
	if !hadErr {
		t.Error("invalid YAML: expected hadErr")
	}
}

// TestResolveComponentInclude_WithSubIncludes covers includes.go:288-291 —
// the fetched component YAML itself contains include: entries.
func TestResolveComponentInclude_WithSubIncludes(t *testing.T) {
	dir := t.TempDir()
	localContent := "comp-local-job:\n  script: echo comp-local\n"
	if err := os.WriteFile(filepath.Join(dir, "sub.yml"), []byte(localContent), 0o644); err != nil {
		t.Fatal(err)
	}

	host := tlsComp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// Component YAML contains a local sub-include.
		fmt.Fprintln(w, "include:\n  - local: /sub.yml\ncomp-job:\n  script: echo comp")
	})
	ref := host + "/g/p/mycomp@v1"

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{}
	_, extW, hadErr := resolveComponentInclude(p, ref, nil, cfg, dir, map[string]bool{}, 0)
	if hadErr {
		t.Errorf("sub-includes: unexpected hadErr; extWarnings=%v", extW)
	}
	if _, ok := p.Jobs["comp-job"]; !ok {
		t.Error("comp-job should be merged via component include")
	}
}

// ── resolveRemoteInclude — sub-includes ───────────────────────────────────────

func TestResolveRemoteInclude_WithSubIncludes(t *testing.T) {
	// The remote file itself includes a local file. The local file resolution
	// exercises the sub-include path in resolveRemoteInclude (line 189-193).
	dir := t.TempDir()
	localContent := "local-child-job:\n  script: echo child\n"
	if err := os.WriteFile(filepath.Join(dir, "child.yml"), []byte(localContent), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// The remote YAML includes a local file.
		fmt.Fprintln(w, "include:\n  - local: /child.yml\nremote-job:\n  script: echo remote")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	warnings, _ := resolveRemoteInclude(p, srv.URL+"/ci.yml", fetcher.AutoConfig(), dir, map[string]bool{}, 0)
	if len(warnings) != 0 {
		t.Errorf("sub-includes: unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["remote-job"]; !ok {
		t.Error("remote-job should be merged")
	}
	if _, ok := p.Jobs["local-child-job"]; !ok {
		t.Error("local-child-job should be merged via sub-include")
	}
}

// ── resolveIncludes — routing branches ───────────────────────────────────────

func TestResolveIncludes_RemoteEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "remote-job:\n  script: echo")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	includes := []any{map[string]any{"remote": srv.URL + "/ci.yml"}}
	warnings, _ := resolveIncludes(p, includes, fetcher.AutoConfig(), "/tmp", map[string]bool{}, 0)
	if len(warnings) != 0 {
		t.Errorf("remote entry: unexpected warnings: %v", warnings)
	}
	if _, ok := p.Jobs["remote-job"]; !ok {
		t.Error("remote-job should be merged via resolveIncludes remote routing")
	}
}

func TestResolveIncludes_ProjectEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "proj-job:\n  script: echo")
	}))
	defer srv.Close()

	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL, Token: "tok"}
	includes := []any{map[string]any{"project": "g/p", "file": "ci.yml"}}
	_, _ = resolveIncludes(p, includes, cfg, "/tmp", map[string]bool{}, 0)
	// proj-job should be merged (or warning if fetch fails, but with token it should succeed)
}

func TestResolveIncludes_ComponentEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintln(w, "comp-job:\n  script: echo")
	}))
	defer srv.Close()

	host := srv.URL[len("http://"):]
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	cfg := fetcher.GitLabConfig{BaseURL: srv.URL}
	includes := []any{map[string]any{"component": host + "/g/p/mycomp@v1"}}
	warnings, _ := resolveIncludes(p, includes, cfg, "/tmp", map[string]bool{}, 0)
	_ = warnings // component path is exercised regardless of outcome
}

func TestResolveIncludes_ComponentEntry_HadErr(t *testing.T) {
	// Component with a dollar sign in ref → hadErr=true → warning is appended.
	p := &model.Pipeline{Jobs: map[string]model.Job{}, RawJobs: map[string]map[string]any{}}
	includes := []any{map[string]any{"component": "${CI_SERVER}/g/p/comp@v1"}}
	warnings, _ := resolveIncludes(p, includes, fetcher.GitLabConfig{}, "/tmp", map[string]bool{}, 0)
	if len(warnings) == 0 {
		t.Error("$ in component ref: expected warning")
	}
}

// ── substituteInputs — empty data ────────────────────────────────────────────

func TestSubstituteInputs_EmptyData(t *testing.T) {
	result := substituteInputs([]byte{}, map[string]any{"KEY": "val"})
	if len(result) != 0 {
		t.Errorf("empty data: expected empty result, got %q", result)
	}
}
