package main

import (
	"testing"

	"git.k3nny.fr/glint/internal/config"
	"git.k3nny.fr/glint/internal/linter"
)

func TestApplyConfig(t *testing.T) {
	findings := []linter.Finding{
		{Severity: linter.Error, Rule: "GL004", Job: "deploy", File: "ci.yml", Line: 10, Message: "bad stage"},
		{Severity: linter.Warning, Rule: "GL007", Job: "old-job", File: "ci.yml", Line: 20, Message: "deprecated"},
		{Severity: linter.Warning, Rule: "GL032", Job: "check", File: "ci.yml", Line: 30, Message: "var ref"},
	}

	t.Run("no config — pass through", func(t *testing.T) {
		got := applyConfig(findings, config.Config{}, nil)
		if len(got) != 3 {
			t.Errorf("got %d findings, want 3", len(got))
		}
	})

	t.Run("ignore GL007", func(t *testing.T) {
		cfg := config.Config{Ignore: []string{"GL007"}}
		got := applyConfig(findings, cfg, nil)
		if len(got) != 2 {
			t.Fatalf("got %d findings, want 2", len(got))
		}
		for _, f := range got {
			if f.Rule == "GL007" {
				t.Error("GL007 should be suppressed")
			}
		}
	})

	t.Run("ignore case-insensitive", func(t *testing.T) {
		cfg := config.Config{Ignore: []string{"gl007"}}
		got := applyConfig(findings, cfg, nil)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
	})

	t.Run("severity demote error to warning", func(t *testing.T) {
		cfg := config.Config{Severity: map[string]string{"GL004": "warning"}}
		got := applyConfig(findings, cfg, nil)
		if len(got) != 3 {
			t.Fatalf("got %d findings, want 3", len(got))
		}
		if got[0].Severity != linter.Warning {
			t.Errorf("GL004 severity = %s, want WARNING", got[0].Severity)
		}
	})

	t.Run("severity promote warning to error", func(t *testing.T) {
		cfg := config.Config{Severity: map[string]string{"GL007": "error"}}
		got := applyConfig(findings, cfg, nil)
		if got[1].Severity != linter.Error {
			t.Errorf("GL007 severity = %s, want ERROR", got[1].Severity)
		}
	})

	t.Run("severity ignore is equivalent to ignore list", func(t *testing.T) {
		cfg := config.Config{Severity: map[string]string{"GL032": "ignore"}}
		got := applyConfig(findings, cfg, nil)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
		for _, f := range got {
			if f.Rule == "GL032" {
				t.Error("GL032 should be suppressed via severity=ignore")
			}
		}
	})

	t.Run("inline suppression by job", func(t *testing.T) {
		suppressions := map[string][]string{
			"old-job": {"GL007"},
		}
		got := applyConfig(findings, config.Config{}, suppressions)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
		for _, f := range got {
			if f.Job == "old-job" && f.Rule == "GL007" {
				t.Error("old-job GL007 should be suppressed")
			}
		}
	})

	t.Run("inline suppression wildcard", func(t *testing.T) {
		suppressions := map[string][]string{
			"old-job": {"*"},
		}
		got := applyConfig(findings, config.Config{}, suppressions)
		// old-job had GL007; should be gone
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
	})

	t.Run("pipeline-level findings not suppressed by job comment", func(t *testing.T) {
		pipelineFindings := []linter.Finding{
			{Severity: linter.Error, Rule: "GL001", Job: "", File: "ci.yml", Line: 0, Message: "no stages"},
		}
		suppressions := map[string][]string{
			"": {"GL001"}, // empty job key should not match pipeline-level
		}
		got := applyConfig(pipelineFindings, config.Config{}, suppressions)
		// Pipeline-level findings (f.Job == "") are never suppressed by job comments.
		if len(got) != 1 {
			t.Errorf("got %d, want 1 (pipeline-level finding must not be suppressed)", len(got))
		}
	})
}
