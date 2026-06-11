package linter_test

import (
	"path/filepath"
	"testing"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/linter"
	"git.k3nny.fr/glint/internal/model"
	"git.k3nny.fr/glint/internal/resolver"
)

// TestSambaCI verifies that the Samba project's .gitlab-ci.yml (a real-world
// pipeline that is valid on GitLab) produces no Error findings.
// These files exercise local include resolution and multi-level extends chains.
func TestSambaCI(t *testing.T) {
	entryPoint := "../../testdata/samba/.gitlab-ci.yml"

	p, err := model.Parse(entryPoint)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	rootDir := filepath.Dir(filepath.Clean(entryPoint))
	incWarnings, _ := resolver.ResolveIncludes(p, fetcher.GitLabConfig{}, rootDir)
	for _, w := range incWarnings {
		t.Logf("include warning: %s", w)
	}

	extWarnings, err := resolver.Resolve(p)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	for _, w := range extWarnings {
		t.Logf("extends warning: job %q extends unknown %q", w.Job, w.Base)
	}

	findings := linter.Lint(p)

	for _, f := range findings {
		if f.Severity == linter.Error {
			t.Errorf("unexpected error finding on valid Samba CI: %s", f)
		}
	}
}

// TestSambaCIEntryFiles verifies all of the Samba entry-point files
// (files that can each act as the top-level CI file) lint without errors.
func TestSambaCIEntryFiles(t *testing.T) {
	entryPoints := []struct {
		name string
		path string
	}{
		{"default", "../../testdata/samba/.gitlab-ci.yml"},
		{"coverage", "../../testdata/samba/.gitlab-ci-coverage.yml"},
		{"private", "../../testdata/samba/.gitlab-ci-private.yml"},
	}

	for _, tc := range entryPoints {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p, err := model.Parse(tc.path)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			rootDir := filepath.Dir(filepath.Clean(tc.path))
			incWarnings, _ := resolver.ResolveIncludes(p, fetcher.GitLabConfig{}, rootDir)
			for _, w := range incWarnings {
				t.Logf("include warning: %s", w)
			}

			extWarnings, err := resolver.Resolve(p)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			for _, w := range extWarnings {
				t.Logf("extends warning: job %q extends unknown %q", w.Job, w.Base)
			}

			findings := linter.Lint(p)
			for _, f := range findings {
				if f.Severity == linter.Error {
					t.Errorf("unexpected error finding: %s", f)
				}
			}
		})
	}
}
