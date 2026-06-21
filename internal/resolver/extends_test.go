package resolver

import (
	"fmt"
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// errorYAMLMarshaler implements yaml.Marshaler and always returns an error,
// allowing tests to trigger the yaml.Marshal failure path in Resolve.
type errorYAMLMarshaler struct{}

func (e errorYAMLMarshaler) MarshalYAML() (interface{}, error) {
	return nil, fmt.Errorf("forced marshal error for test")
}

// ── parseExtends ──────────────────────────────────────────────────────────────

func TestParseExtends(t *testing.T) {
	cases := []struct {
		name    string
		input   any
		want    []string
		wantErr bool
	}{
		{name: "nil", input: nil, want: nil},
		{name: "single string", input: "base", want: []string{"base"}},
		{name: "slice of strings", input: []any{"base1", "base2"}, want: []string{"base1", "base2"}},
		{name: "empty slice", input: []any{}, want: []string{}},
		{name: "invalid item in slice", input: []any{42}, wantErr: true},
		{name: "unexpected type", input: 123, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExtends(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got err=%v", tc.wantErr, err)
			}
			if !tc.wantErr {
				if len(got) != len(tc.want) {
					t.Fatalf("len mismatch: got %v want %v", got, tc.want)
				}
				for i := range got {
					if got[i] != tc.want[i] {
						t.Errorf("[%d] got %q want %q", i, got[i], tc.want[i])
					}
				}
			}
		})
	}
}

// ── topoSort ──────────────────────────────────────────────────────────────────

func TestTopoSort(t *testing.T) {
	t.Run("simple chain", func(t *testing.T) {
		// a → b (b must come before a)
		graph := map[string][]string{"a": {"b"}, "b": {}}
		order, err := topoSort(graph)
		if err != nil {
			t.Fatal(err)
		}
		// b must appear before a
		pos := func(s string) int {
			for i, v := range order { if v == s { return i } }
			return -1
		}
		if pos("b") > pos("a") {
			t.Errorf("b should come before a, got %v", order)
		}
	})

	t.Run("no deps", func(t *testing.T) {
		graph := map[string][]string{"a": {}, "b": {}}
		order, err := topoSort(graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 {
			t.Fatalf("expected 2 items, got %v", order)
		}
	})

	t.Run("cycle detected", func(t *testing.T) {
		graph := map[string][]string{"a": {"b"}, "b": {"a"}}
		_, err := topoSort(graph)
		if err == nil {
			t.Fatal("expected cycle error")
		}
	})

	t.Run("already visited node skipped", func(t *testing.T) {
		// diamond: a→b, a→c, b→d, c→d
		graph := map[string][]string{
			"a": {"b", "c"},
			"b": {"d"},
			"c": {"d"},
			"d": {},
		}
		order, err := topoSort(graph)
		if err != nil {
			t.Fatal(err)
		}
		// d must come first (or at least before b, c, a)
		pos := func(s string) int {
			for i, v := range order { if v == s { return i } }
			return -1
		}
		if pos("d") > pos("b") || pos("d") > pos("c") {
			t.Errorf("d should come before b and c, got %v", order)
		}
	})
}

// ── deepMerge ─────────────────────────────────────────────────────────────────

func TestDeepMerge(t *testing.T) {
	t.Run("override wins on scalar", func(t *testing.T) {
		base := map[string]any{"a": "base", "b": "keep"}
		over := map[string]any{"a": "new"}
		got := deepMerge(base, over)
		if got["a"] != "new" || got["b"] != "keep" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("nested maps merged recursively", func(t *testing.T) {
		base := map[string]any{"m": map[string]any{"x": 1, "y": 2}}
		over := map[string]any{"m": map[string]any{"y": 99, "z": 3}}
		got := deepMerge(base, over)
		m := got["m"].(map[string]any)
		if m["x"] != 1 || m["y"] != 99 || m["z"] != 3 {
			t.Errorf("nested merge wrong: %v", m)
		}
	})

	t.Run("override scalar wins over base map", func(t *testing.T) {
		base := map[string]any{"m": map[string]any{"x": 1}}
		over := map[string]any{"m": "scalar"}
		got := deepMerge(base, over)
		if got["m"] != "scalar" {
			t.Errorf("expected scalar to win, got %v", got["m"])
		}
	})

	t.Run("base map + override scalar - base is map, override is scalar", func(t *testing.T) {
		base := map[string]any{"k": map[string]any{"a": 1}}
		over := map[string]any{"k": "replaced"}
		got := deepMerge(base, over)
		if got["k"] != "replaced" {
			t.Errorf("got %v", got["k"])
		}
	})

	t.Run("empty maps", func(t *testing.T) {
		got := deepMerge(map[string]any{}, map[string]any{})
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// ── Resolve ───────────────────────────────────────────────────────────────────

func buildPipeline(rawJobs map[string]map[string]any) *model.Pipeline {
	p := &model.Pipeline{
		Jobs:    make(map[string]model.Job),
		RawJobs: rawJobs,
	}
	for name, raw := range rawJobs {
		ext := raw["extends"]
		p.Jobs[name] = model.Job{Name: name, Extends: ext}
	}
	return p
}

func TestResolve(t *testing.T) {
	t.Run("no extends — noop", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			"build": {"script": []any{"make"}},
		})
		warnings, err := Resolve(p)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("unexpected: err=%v warnings=%v", err, warnings)
		}
	})

	t.Run("single extends merges fields", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			".base":  {"image": "alpine", "script": []any{"echo base"}},
			"child": {"extends": ".base", "stage": "test"},
		})
		warnings, err := Resolve(p)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("unexpected: err=%v warnings=%v", err, warnings)
		}
		child := p.Jobs["child"]
		if child.Stage != "test" {
			t.Errorf("stage not preserved: %q", child.Stage)
		}
	})

	t.Run("unknown base produces warning, not error", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			"child": {"extends": ".missing", "script": []any{"echo x"}},
		})
		warnings, err := Resolve(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warnings) != 1 || warnings[0].Job != "child" || warnings[0].Base != ".missing" {
			t.Errorf("expected warning about .missing, got %v", warnings)
		}
	})

	t.Run("multi-extend list", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			".a":    {"image": "alpine"},
			".b":    {"tags": []any{"docker"}},
			"child": {"extends": []any{".a", ".b"}, "script": []any{"echo x"}},
		})
		warnings, err := Resolve(p)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("unexpected: %v %v", err, warnings)
		}
	})

	t.Run("cycle returns error", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			"a": {"extends": "b"},
			"b": {"extends": "a"},
		})
		_, err := Resolve(p)
		if err == nil {
			t.Fatal("expected cycle error")
		}
	})

	t.Run("file and line preserved after merge", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			".base":  {"script": []any{"echo base"}},
			"child": {"extends": ".base"},
		})
		p.Jobs["child"] = model.Job{Name: "child", Extends: ".base", File: "ci.yml", Line: 10}
		_, err := Resolve(p)
		if err != nil {
			t.Fatal(err)
		}
		if p.Jobs["child"].File != "ci.yml" || p.Jobs["child"].Line != 10 {
			t.Errorf("file/line lost after merge: %+v", p.Jobs["child"])
		}
	})

	t.Run("parseExtends invalid type returns error", func(t *testing.T) {
		p := buildPipeline(map[string]map[string]any{
			"job": {"extends": 123},
		})
		p.Jobs["job"] = model.Job{Name: "job", Extends: 123}
		_, err := Resolve(p)
		if err == nil {
			t.Fatal("expected error for invalid extends type")
		}
	})

	t.Run("yaml.Marshal fails — errorYAMLMarshaler injected into merged map", func(t *testing.T) {
		// Inject a value whose MarshalYAML() returns an error so yaml.Marshal
		// fails at extends.go:74-77.
		p := buildPipeline(map[string]map[string]any{
			".base":  {"script": []any{"echo"}},
			"child": {"extends": ".base", "bad": errorYAMLMarshaler{}},
		})
		_, err := Resolve(p)
		if err == nil {
			t.Fatal("expected error when yaml.Marshal fails")
		}
	})

	t.Run("yaml.Unmarshal fails — map injected where []string expected", func(t *testing.T) {
		// Marshal succeeds (maps are valid YAML), but Unmarshal into model.Job
		// fails because Dependencies []string cannot hold a mapping.
		p := buildPipeline(map[string]map[string]any{
			".base": {
				"dependencies": map[string]any{"invalid": "not-a-list"},
			},
			"child": {"extends": ".base", "stage": "build"},
		})
		_, err := Resolve(p)
		if err == nil {
			t.Fatal("expected error when yaml.Unmarshal fails on incompatible type")
		}
	})
}
