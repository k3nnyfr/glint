package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

func TestCheckInsecureRemoteInclude(t *testing.T) {
	tests := []struct {
		name    string
		include []any
		wantGL  string // expected rule ID, empty = no findings
	}{
		{
			name:    "http URL triggers GL045",
			include: []any{map[string]any{"remote": "http://example.com/ci.yml"}},
			wantGL:  RuleInsecureRemoteInclude,
		},
		{
			name:    "https URL is clean",
			include: []any{map[string]any{"remote": "https://example.com/ci.yml"}},
		},
		{
			name:    "local include ignored",
			include: []any{map[string]any{"local": "/templates/ci.yml"}},
		},
		{
			name:    "string include ignored",
			include: []any{"/templates/ci.yml"},
		},
		{
			name:    "no includes",
			include: nil,
		},
		{
			name: "mixed: http fires, https does not",
			include: []any{
				map[string]any{"remote": "https://ok.example.com/ci.yml"},
				map[string]any{"remote": "http://bad.example.com/ci.yml"},
			},
			wantGL: RuleInsecureRemoteInclude,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &model.Pipeline{
				Include: tc.include,
				Jobs:    map[string]model.Job{},
			}
			findings := checkInsecureRemoteInclude(p)
			if tc.wantGL == "" {
				if len(findings) != 0 {
					t.Errorf("expected no findings, got: %v", findings)
				}
				return
			}
			found := false
			for _, f := range findings {
				if f.Rule == tc.wantGL {
					found = true
					if f.Severity != Warning {
						t.Errorf("severity = %v; want Warning", f.Severity)
					}
				}
			}
			if !found {
				t.Errorf("expected finding %s, got: %v", tc.wantGL, findings)
			}
		})
	}
}
