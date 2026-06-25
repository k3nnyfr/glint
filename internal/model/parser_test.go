package model

import (
	"os"
	"path/filepath"
	"testing"
)

// ── Parse (file) ──────────────────────────────────────────────────────────────

func TestParse(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		content := `
stages: [build]
build-job:
  stage: build
  script: [make]
`
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitlab-ci.yml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := Parse(path)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if p.SourceFile != path {
			t.Errorf("SourceFile: got %q want %q", p.SourceFile, path)
		}
		if _, ok := p.Jobs["build-job"]; !ok {
			t.Error("build-job not found")
		}
		if p.Jobs["build-job"].File != path {
			t.Errorf("job File not set: %q", p.Jobs["build-job"].File)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := Parse("/nonexistent/path/ci.yml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

// ── ParseBytes edge cases ─────────────────────────────────────────────────────

func TestParseBytes_EdgeCases(t *testing.T) {
	// Empty YAML: doc.Kind is not DocumentNode → return empty Pipeline.
	p, err := ParseBytes([]byte(""))
	if err != nil {
		t.Fatalf("empty YAML: unexpected error: %v", err)
	}
	if p == nil || len(p.Jobs) != 0 {
		t.Error("empty YAML: expected empty pipeline")
	}

	// YAML null: second pass succeeds, doc root is ScalarNode → return empty Pipeline (line 51-53).
	p2, err := ParseBytes([]byte("null"))
	if err != nil {
		t.Fatalf("null YAML: unexpected error: %v", err)
	}
	if p2 == nil || len(p2.Jobs) != 0 {
		t.Error("null YAML: expected empty pipeline")
	}

	// Invalid YAML (undefined alias): first-pass Unmarshal fails (line 34-36).
	_, err = ParseBytes([]byte("*undefined_anchor"))
	if err == nil {
		t.Error("undefined alias: expected error from ParseBytes")
	}

	// Sequence YAML: second-pass Unmarshal fails (cannot decode !!seq into Pipeline).
	_, err = ParseBytes([]byte("- item1\n- item2\n"))
	if err == nil {
		t.Error("sequence YAML: expected error from ParseBytes")
	}

	// Job value that cannot be decoded as map[string]any (scalar job value).
	_, err = ParseBytes([]byte("my-job: \"just a string\"\n"))
	if err == nil {
		t.Error("scalar job value: expected error from ParseBytes")
	}

	// Job with wrong field type: rawMap decode succeeds, Job decode fails (line 79-81).
	_, err = ParseBytes([]byte("my-job:\n  stage: [build, test]\n"))
	if err == nil {
		t.Error("wrong field type: expected error from ParseBytes (stage must be string)")
	}

	// Null/empty YAML key (e.g. bare "?"): job name cannot be empty.
	_, err = ParseBytes([]byte("?"))
	if err == nil {
		t.Error("null key: expected error from ParseBytes (job name cannot be empty)")
	}
}

// TestParse_ParseBytesError exercises the Parse → ParseBytes error path (line 18).
func TestParse_ParseBytesError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(path, []byte("my-job: \"scalar\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Parse(path)
	if err == nil {
		t.Error("scalar job value: expected error from Parse")
	}
}

// TestParseSuppressComment_NonIgnore ensures that a "glint:" directive that is
// not "ignore" is silently skipped.
func TestParseSuppressComment_NonIgnore(t *testing.T) {
	result := parseSuppressComment("# glint: refresh GL001", "")
	if result != nil {
		t.Errorf("non-ignore glint directive should return nil, got %v", result)
	}
}

// ── sanitizeYAMLEscapes ───────────────────────────────────────────────────────

func TestSanitizeYAMLEscapes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no double quotes — unchanged",
			input: "stage: build",
			want:  "stage: build",
		},
		{
			name:  "double-quoted string without backslash",
			input: `if: "$CI_BRANCH == \"main\""`,
			want:  `if: "$CI_BRANCH == \"main\""`,
		},
		{
			// \/ inside double-quoted YAML string becomes \\/ (yaml.v3 does not recognise \/).
			name:  "slash escape rewritten in larger context",
			input: `if: "$CI_BRANCH =~ /^us\//"`,
			want:  `if: "$CI_BRANCH =~ /^us\\//"`,
		},
		{
			name:  "backslash-slash inside double quotes rewritten",
			input: `"pattern: /^us\//"`  ,
			want:  `"pattern: /^us\\//"`,
		},
		{
			name:  "single-quoted string unchanged",
			input: `'hello \/ world'`,
			want:  `'hello \/ world'`,
		},
		{
			name:  "escaped single quote inside single-quoted string",
			input: `'it''s fine'`,
			want:  `'it''s fine'`,
		},
		{
			name:  "other backslash escapes unchanged",
			input: `"\n\t\r"`,
			want:  `"\n\t\r"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(sanitizeYAMLEscapes([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}
