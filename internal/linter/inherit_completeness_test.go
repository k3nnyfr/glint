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
