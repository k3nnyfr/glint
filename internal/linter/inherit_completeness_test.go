package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestCheckInheritCompleteness(t *testing.T) {
	cases := []struct {
		name    string
		default_ *model.DefaultConfig
		inherit any // job.Inherit value
		wantHit bool
	}{
		{
			name:    "no inherit — no finding",
			inherit: nil,
			wantHit: false,
		},
		{
			name:    "inherit: default: false, no default block — GL043",
			inherit: map[string]any{"default": false},
			wantHit: true,
		},
		{
			name:    "inherit: default: true, no default block — GL043",
			inherit: map[string]any{"default": true},
			wantHit: true,
		},
		{
			name:    "inherit: default: [image], no default block — GL043",
			inherit: map[string]any{"default": []any{"image"}},
			wantHit: true,
		},
		{
			name:     "inherit: default: false, default block exists — no finding",
			default_: &model.DefaultConfig{Image: "node:20"},
			inherit:  map[string]any{"default": false},
			wantHit:  false,
		},
		{
			name:     "inherit: default: [image], image in default — no finding",
			default_: &model.DefaultConfig{Image: "node:20"},
			inherit:  map[string]any{"default": []any{"image"}},
			wantHit:  false,
		},
		{
			name:     "inherit: default: [before_script], before_script NOT in default — GL043",
			default_: &model.DefaultConfig{Image: "node:20"},
			inherit:  map[string]any{"default": []any{"before_script"}},
			wantHit:  true,
		},
		{
			name:     "inherit: default: [image, before_script], only image in default — GL043 for before_script",
			default_: &model.DefaultConfig{Image: "node:20"},
			inherit:  map[string]any{"default": []any{"image", "before_script"}},
			wantHit:  true,
		},
		{
			name:     "inherit: variables only — no default check",
			default_: nil,
			inherit:  map[string]any{"variables": false},
			wantHit:  false,
		},
		{
			name:     "inherit: unknown field in list — conservative, no finding",
			default_: &model.DefaultConfig{Image: "node:20"},
			inherit:  map[string]any{"default": []any{"nonexistent_field"}},
			wantHit:  false, // unknown fields return true from defaultBlockHasField
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := model.Job{Inherit: tc.inherit}
			p := &model.Pipeline{
				Default: tc.default_,
				Jobs:    map[string]model.Job{"test-job": job},
			}
			findings := checkInheritCompleteness(p)

			hit := false
			for _, f := range findings {
				if f.Rule == RuleInheritNoDefault {
					hit = true
				}
			}
			if hit != tc.wantHit {
				t.Errorf("GL043 hit=%v, want=%v; findings: %v", hit, tc.wantHit, findings)
			}
		})
	}
}

// TestCheckInheritCompleteness_BoolInherit verifies that a scalar (non-map)
// inherit value (e.g. inherit: true) is silently ignored.
func TestCheckInheritCompleteness_BoolInherit(t *testing.T) {
	job := model.Job{Inherit: true}
	p := &model.Pipeline{
		Default: nil,
		Jobs:    map[string]model.Job{"job": job},
	}
	if findings := checkInheritCompleteness(p); len(findings) != 0 {
		t.Errorf("bool inherit should produce no findings; got %v", findings)
	}
}

// TestCheckInheritCompleteness_ListItemNotString verifies that non-string items
// in the inherit: default: list are skipped.
func TestCheckInheritCompleteness_ListItemNotString(t *testing.T) {
	// The list contains 42 (int) and "image" (string). Only "image" should be checked.
	job := model.Job{Inherit: map[string]any{"default": []any{42, "image"}}}
	p := &model.Pipeline{
		Default: &model.DefaultConfig{Image: "node:20"},
		Jobs:    map[string]model.Job{"job": job},
	}
	// "image" IS set in default → no findings.
	if findings := checkInheritCompleteness(p); len(findings) != 0 {
		t.Errorf("non-string item should be skipped; got %v", findings)
	}
}

// TestDefaultBlockHasField_AllFields exercises every field branch in
// defaultBlockHasField, including the ones not covered by the main table test.
func TestDefaultBlockHasField_AllFields(t *testing.T) {
	cases := []struct {
		name  string
		d     *model.DefaultConfig
		field string
		want  bool
	}{
		{"after_script set", &model.DefaultConfig{AfterScript: []any{"echo"}}, "after_script", true},
		{"after_script nil", &model.DefaultConfig{}, "after_script", false},
		{"cache set", &model.DefaultConfig{Cache: map[string]any{"key": "main"}}, "cache", true},
		{"cache nil", &model.DefaultConfig{}, "cache", false},
		{"artifacts set", &model.DefaultConfig{Artifacts: map[string]any{"paths": []any{"dist/"}}}, "artifacts", true},
		{"artifacts nil", &model.DefaultConfig{}, "artifacts", false},
		{"retry set", &model.DefaultConfig{Retry: 2}, "retry", true},
		{"retry nil", &model.DefaultConfig{}, "retry", false},
		{"timeout set", &model.DefaultConfig{Timeout: "30m"}, "timeout", true},
		{"timeout empty", &model.DefaultConfig{}, "timeout", false},
		{"tags set", &model.DefaultConfig{Tags: []string{"docker"}}, "tags", true},
		{"tags empty", &model.DefaultConfig{}, "tags", false},
		{"unknown field", &model.DefaultConfig{}, "unknown_field", true}, // conservative
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := defaultBlockHasField(tc.d, tc.field)
			if got != tc.want {
				t.Errorf("defaultBlockHasField(%q) = %v, want %v", tc.field, got, tc.want)
			}
		})
	}
}

// TestPluralIs covers both branches of pluralIs.
func TestPluralIs(t *testing.T) {
	if pluralIs(1) != "this field is" {
		t.Errorf("pluralIs(1) = %q, want 'this field is'", pluralIs(1))
	}
	if pluralIs(2) != "these fields are" {
		t.Errorf("pluralIs(2) = %q, want 'these fields are'", pluralIs(2))
	}
}
