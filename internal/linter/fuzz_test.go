package linter

import (
	"testing"

	"git.k3nny.fr/glint/internal/model"
)

// FuzzLint ensures that the full Parse → Lint pipeline never panics on
// arbitrary YAML input. Lint rules type-assert model fields extensively;
// this fuzzer drives those assertions against malformed-but-parseable YAML.
// Run with: go test -fuzz=FuzzLint ./internal/linter/
func FuzzLint(f *testing.F) {
	seeds := []string{
		// Minimal valid pipeline
		"stages: [build]\njob:\n  stage: build\n  script: echo ok\n",
		// Manual / delayed / trigger / on_failure job types
		"job:\n  script: ok\n  when: manual\n",
		"job:\n  script: ok\n  when: delayed\n  start_in: 5 minutes\n",
		"job:\n  trigger:\n    project: group/repo\n",
		"job:\n  script: ok\n  when: on_failure\n",
		// needs: and dependencies:
		"stages: [a,b]\na:\n  stage: a\n  script: ok\nb:\n  stage: b\n  needs: [a]\n  script: ok\n",
		"stages: [a,b]\na:\n  stage: a\n  script: ok\nb:\n  stage: b\n  dependencies: [a]\n  script: ok\n",
		// rules:
		"job:\n  script: ok\n  rules:\n    - if: '$CI_COMMIT_BRANCH == \"main\"'\n      when: on_success\n",
		// parallel matrix
		"job:\n  script: ok\n  parallel:\n    matrix:\n      - ARCH: [amd64, arm64]\n",
		// image as string / map
		"job:\n  script: ok\n  image: golang:1.21\n",
		"job:\n  script: ok\n  image:\n    name: golang:1.21\n    entrypoint: ['']\n",
		// artifacts / cache with both string and map when:
		"job:\n  script: ok\n  artifacts:\n    when: on_success\n    paths: [dist/]\n",
		"job:\n  script: ok\n  cache:\n    key: $CI_COMMIT_REF_SLUG\n    paths: [vendor/]\n",
		// environment / release / coverage
		"job:\n  script: ok\n  environment:\n    name: production\n    url: https://example.com\n",
		"job:\n  script: ok\n  release:\n    tag_name: $CI_COMMIT_TAG\n    description: Release\n",
		"job:\n  script: ok\n  coverage: '/^TOTAL.*?(\\d+%)$/'\n",
		// retry / timeout
		"job:\n  script: ok\n  retry: 2\n",
		"job:\n  script: ok\n  timeout: 2h30m\n",
		// workflow
		"workflow:\n  rules:\n    - if: '$CI_COMMIT_BRANCH'\n      when: always\njob:\n  script: ok\n",
		// extends
		".base:\n  script: ok\nchild:\n  extends: .base\n  stage: test\n",
		// id_tokens / secrets
		"job:\n  script: ok\n  id_tokens:\n    TOKEN:\n      aud: https://example.com\n",
		// services
		"job:\n  script: ok\n  services:\n    - name: postgres:14\n      alias: db\n",
		// pages job
		"pages:\n  script: make docs\n  artifacts:\n    paths: [public/]\n",
		// inherit
		"default:\n  retry: 1\njob:\n  script: ok\n  inherit:\n    default: false\n",
		// allow_failure
		"job:\n  script: ok\n  allow_failure:\n    exit_codes: [1, 2]\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := model.ParseBytes(data)
		if err != nil {
			return
		}
		// Lint must never panic regardless of pipeline content.
		_ = Lint(p, nil)
	})
}
