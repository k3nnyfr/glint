package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"git.k3nny.fr/glint/internal/linter"
)

var testFindings = []linter.Finding{
	{
		Severity: linter.Error,
		Rule:     "GL004",
		Job:      "deploy",
		File:     ".gitlab-ci.yml",
		Line:     14,
		Message:  `stage "production" is not defined in 'stages'`,
	},
	{
		Severity: linter.Warning,
		Rule:     "GL007",
		Job:      "old-job",
		File:     ".gitlab-ci.yml",
		Line:     31,
		Message:  "'only'/'except' are deprecated; prefer 'rules'",
	},
}

func TestWriteJSON(t *testing.T) {
	t.Run("findings", func(t *testing.T) {
		var buf bytes.Buffer
		writeJSON(&buf, testFindings, ".gitlab-ci.yml")

		var got jsonReport
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got.SchemaVersion != 1 {
			t.Errorf("schema_version = %d, want 1", got.SchemaVersion)
		}
		if len(got.Findings) != 2 {
			t.Fatalf("got %d findings, want 2", len(got.Findings))
		}
		if got.Summary.Errors != 1 || got.Summary.Warnings != 1 {
			t.Errorf("summary = %+v, want {2,1,1}", got.Summary)
		}
		if got.Findings[0].Rule != "GL004" || got.Findings[0].Severity != "error" {
			t.Errorf("finding[0] = %+v", got.Findings[0])
		}
		if got.Findings[0].Job != "deploy" || got.Findings[0].Line != 14 {
			t.Errorf("finding[0] job/line = %s/%d", got.Findings[0].Job, got.Findings[0].Line)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		writeJSON(&buf, nil, "ci.yml")

		var got jsonReport
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got.Findings == nil {
			t.Error("findings must be [] not null")
		}
		if got.Summary.Total != 0 {
			t.Errorf("total = %d, want 0", got.Summary.Total)
		}
	})
}

func TestWriteSARIF(t *testing.T) {
	t.Run("findings", func(t *testing.T) {
		var buf bytes.Buffer
		writeSARIF(&buf, testFindings, ".gitlab-ci.yml")

		var got sarifLog
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if got.Version != "2.1.0" {
			t.Errorf("version = %q, want 2.1.0", got.Version)
		}
		if len(got.Runs) != 1 {
			t.Fatalf("runs count = %d, want 1", len(got.Runs))
		}
		run := got.Runs[0]
		if run.Tool.Driver.Name != "glint" {
			t.Errorf("driver.name = %q", run.Tool.Driver.Name)
		}
		if len(run.Results) != 2 {
			t.Fatalf("results count = %d, want 2", len(run.Results))
		}
		r0 := run.Results[0]
		if r0.RuleID != "GL004" || r0.Level != "error" {
			t.Errorf("result[0] ruleId/level = %s/%s", r0.RuleID, r0.Level)
		}
		if len(r0.Locations) != 1 {
			t.Fatalf("result[0] locations count = %d, want 1", len(r0.Locations))
		}
		loc := r0.Locations[0].PhysicalLocation
		if loc.ArtifactLocation.URI != ".gitlab-ci.yml" {
			t.Errorf("artifactLocation.uri = %q", loc.ArtifactLocation.URI)
		}
		if loc.Region == nil || loc.Region.StartLine != 14 {
			t.Errorf("region = %v", loc.Region)
		}
		// Message should include job name prefix.
		if !strings.Contains(r0.Message.Text, `job "deploy"`) {
			t.Errorf("message missing job prefix: %q", r0.Message.Text)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		writeSARIF(&buf, nil, "ci.yml")

		var got sarifLog
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		run := got.Runs[0]
		if run.Results == nil {
			t.Error("results must be [] not null")
		}
		if run.Tool.Driver.Rules == nil {
			t.Error("rules must be [] not null")
		}
	})
}

func TestWriteJUnit(t *testing.T) {
	t.Run("findings", func(t *testing.T) {
		var buf bytes.Buffer
		writeJUnit(&buf, testFindings, ".gitlab-ci.yml")

		if !strings.HasPrefix(buf.String(), "<?xml") {
			t.Error("output does not start with XML declaration")
		}

		var got junitTestsuites
		if err := xml.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid XML: %v\noutput:\n%s", err, buf.String())
		}
		if got.Failures != 2 {
			t.Errorf("failures = %d, want 2", got.Failures)
		}
		if len(got.Suites) != 1 {
			t.Fatalf("suites count = %d, want 1", len(got.Suites))
		}
		suite := got.Suites[0]
		if len(suite.Cases) != 2 {
			t.Fatalf("test cases count = %d, want 2", len(suite.Cases))
		}
		if suite.Cases[0].Name != "GL004" {
			t.Errorf("case[0].name = %q", suite.Cases[0].Name)
		}
		if suite.Cases[0].Failure == nil {
			t.Error("case[0] has no failure element")
		}
	})

	t.Run("empty_is_passing", func(t *testing.T) {
		var buf bytes.Buffer
		writeJUnit(&buf, nil, "ci.yml")

		var got junitTestsuites
		if err := xml.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid XML: %v", err)
		}
		if got.Failures != 0 {
			t.Errorf("failures = %d, want 0", got.Failures)
		}
		suite := got.Suites[0]
		if len(suite.Cases) != 1 || suite.Cases[0].Failure != nil {
			t.Error("expected exactly one passing testcase when no findings")
		}
	})
}

func TestWriteGitHub(t *testing.T) {
	cases := []struct {
		name     string
		findings []linter.Finding
		wantLine string
	}{
		{
			name:     "error with file and line",
			findings: []linter.Finding{testFindings[0]},
			wantLine: "::error file=.gitlab-ci.yml,line=14,title=GL004::job \"deploy\": stage \"production\" is not defined in 'stages'",
		},
		{
			name:     "warning",
			findings: []linter.Finding{testFindings[1]},
			wantLine: "::warning file=.gitlab-ci.yml,line=31,title=GL007::job \"old-job\": 'only'/'except' are deprecated; prefer 'rules'",
		},
		{
			name: "no file or line",
			findings: []linter.Finding{{
				Severity: linter.Error,
				Rule:     "GL001",
				Message:  "no stages defined",
			}},
			wantLine: "::error title=GL001::no stages defined",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeGitHub(&buf, tc.findings)
			got := strings.TrimRight(buf.String(), "\n")
			if got != tc.wantLine {
				t.Errorf("\ngot:  %s\nwant: %s", got, tc.wantLine)
			}
		})
	}
}

func TestCountSeverities(t *testing.T) {
	errs, warns := countSeverities(testFindings)
	if errs != 1 || warns != 1 {
		t.Errorf("countSeverities = (%d, %d), want (1, 1)", errs, warns)
	}
}
