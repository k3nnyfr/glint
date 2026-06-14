package model

import "testing"

func TestParseSuppressComment(t *testing.T) {
	cases := []struct {
		name        string
		head        string
		line        string
		wantRules   []string
		wantNil     bool
	}{
		{
			name:    "no comment",
			head:    "",
			line:    "",
			wantNil: true,
		},
		{
			name:      "single rule head comment",
			head:      "# glint: ignore GL007",
			wantRules: []string{"GL007"},
		},
		{
			name:      "multiple rules comma-separated",
			head:      "# glint: ignore GL007, GL032",
			wantRules: []string{"GL007", "GL032"},
		},
		{
			name:      "multiple rules space-separated",
			head:      "# glint: ignore GL007 GL032",
			wantRules: []string{"GL007", "GL032"},
		},
		{
			name:      "ignore all",
			head:      "# glint: ignore all",
			wantRules: []string{"*"},
		},
		{
			name:      "ignore all bare",
			head:      "# glint: ignore",
			wantRules: []string{"*"},
		},
		{
			name:      "lowercase rule ID is uppercased",
			head:      "# glint: ignore gl007",
			wantRules: []string{"GL007"},
		},
		{
			name:      "line comment",
			head:      "",
			line:      "# glint: ignore GL004",
			wantRules: []string{"GL004"},
		},
		{
			name:    "unrelated comment",
			head:    "# This job deploys to production",
			wantNil: true,
		},
		{
			name:    "partial match - not a glint directive",
			head:    "# hint: ignore this",
			wantNil: true,
		},
		{
			name:      "multi-line head comment with directive on second line",
			head:      "# Some description\n# glint: ignore GL007",
			wantRules: []string{"GL007"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSuppressComment(tc.head, tc.line)
			if tc.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tc.wantRules) {
				t.Fatalf("got %v, want %v", got, tc.wantRules)
			}
			for i, r := range tc.wantRules {
				if got[i] != r {
					t.Errorf("[%d] got %q, want %q", i, got[i], r)
				}
			}
		})
	}
}

func TestParseBytes_Suppressions(t *testing.T) {
	yaml := `
stages:
  - build

# glint: ignore GL007
deprecated-job:
  stage: build
  only:
    - main

normal-job:
  stage: build
  script: echo ok
`
	p, err := ParseBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if p.Suppressions == nil {
		t.Fatal("Suppressions is nil")
	}
	rules, ok := p.Suppressions["deprecated-job"]
	if !ok {
		t.Fatal("no suppression for deprecated-job")
	}
	if len(rules) != 1 || rules[0] != "GL007" {
		t.Errorf("rules = %v, want [GL007]", rules)
	}
	if _, ok := p.Suppressions["normal-job"]; ok {
		t.Error("normal-job should have no suppressions")
	}
}
