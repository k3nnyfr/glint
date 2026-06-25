package model

import "testing"

// FuzzParseBytes ensures the YAML parser never panics on arbitrary input and
// that successful parses return a structurally sound Pipeline.
// Run with: go test -fuzz=FuzzParseBytes ./internal/model/
// Found failures are saved to testdata/fuzz/FuzzParseBytes/.
func FuzzParseBytes(f *testing.F) {
	// Seed corpus: representative inputs covering the main code paths in
	// ParseBytes, including the sanitizeYAMLEscapes pre-processing step.
	seeds := [][]byte{
		{},
		[]byte("null"),
		[]byte("stages: [build]\nbuild-job:\n  stage: build\n  script: echo ok\n"),
		[]byte(".base:\n  script: [make]\nchild:\n  extends: .base\n  stage: test\n"),
		[]byte("stages: [a, b]\njob-a:\n  stage: a\n  script: run\njob-b:\n  stage: b\n  needs: [job-a]\n  script: run\n"),
		[]byte("*undefined_anchor"),
		[]byte("- item1\n- item2\n"),
		[]byte("my-job: \"just a string\"\n"),
		[]byte("my-job:\n  stage: [build, test]\n"),
		[]byte("# glint: ignore GL007\nlegacy:\n  only: [main]\n  script: ok\n"),
		[]byte("workflow:\n  rules:\n    - if: '$CI_COMMIT_BRANCH == \"main\"'\n      when: always\njob:\n  script: echo ok\n"),
		[]byte("include:\n  - local: other.yml\njob:\n  script: echo ok\n"),
		[]byte("job:\n  script: echo ok\n  when: on_failure\n  rules:\n    - if: '$VAR =~ /^us\\//'\n"),
		[]byte("job:\n  stage: test\n  image:\n    name: golang:1.21\n    entrypoint: ['']\n  parallel:\n    matrix:\n      - PLATFORM: [linux, darwin]\n  script: go build\n"),
		[]byte("default:\n  retry: 2\n  timeout: 1h30m\nvariables:\n  ENV: production\nstages: [build, test, deploy]\n"),
		[]byte("&anchor\n  script: [echo ok]\njob:\n  <<: *anchor\n  stage: build\n"),
		[]byte("?"), // null/empty YAML key — must error, not produce an empty-named job
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := ParseBytes(data)
		if err != nil {
			return // errors are acceptable; panics are not
		}
		if p == nil {
			t.Fatal("ParseBytes returned nil pipeline with nil error")
		}
		for name := range p.Jobs {
			if name == "" {
				t.Fatal("ParseBytes produced a job with an empty name")
			}
		}
	})
}

// FuzzSanitizeYAMLEscapes ensures the escape sanitizer never panics and never
// produces output shorter than its input (it can only expand \/ to \\/).
func FuzzSanitizeYAMLEscapes(f *testing.F) {
	seeds := [][]byte{
		{},
		[]byte("stage: build"),
		[]byte(`if: "$CI_BRANCH =~ /^us\//"`),
		[]byte(`"pattern: /^us\//"`),
		[]byte(`'single quoted \/ unchanged'`),
		[]byte(`"\n\t\r"`),
		[]byte(`"nested \"quote\" inside"`),
		[]byte(`'it''s fine'`),
		[]byte(`"unclosed`),
		{'"', '\\'},      // double-quoted string ending with a lone backslash
		{'"', '\\', '/'}, // the exact sequence being rewritten
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		out := sanitizeYAMLEscapes(data)
		if len(out) < len(data) {
			t.Fatalf("sanitizeYAMLEscapes shrank output: input len=%d output len=%d\ninput: %q",
				len(data), len(out), data)
		}
	})
}
