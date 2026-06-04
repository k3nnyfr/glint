# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project does not yet use semantic versioning; entries are listed under `[Unreleased]` until a first release is tagged.

## [Unreleased]

### Added

- **Comprehensive keyword validation** — checks for all major GitLab CI YAML keywords based on the official docs:
  - `when` valid values: `on_success`, `on_failure`, `always`, `manual`, `delayed`, `never`
  - `start_in` only allowed when `when: delayed`; error if set without it
  - `parallel` must be integer 2–200 or map with `matrix` key
  - `retry` max value 0–2; `retry.when` failure types validated against the full enum
  - `allow_failure` must be boolean or `{exit_codes: ...}`
  - `interruptible` must be boolean
  - `trigger` jobs cannot have `script`; map form requires `project` or `include`
  - `coverage` must be a regex pattern wrapped in `/…/`
  - `release` requires `tag_name`
  - `environment.url` requires `environment.name`; `environment.action` validated
  - `artifacts.when` valid values; `expose_as` requires `paths`
  - `cache.when` and `cache.policy` valid values
  - `rules[*].when` validated per-rule
  - `image` map form requires `name`
  - `inherit.default` / `inherit.variables` must be boolean or list
  - `workflow.rules[*].when` restricted to `always` / `never`
  - Warning when `pages` job `artifacts.paths` does not include `public`
- **`dependencies:` validation** — referenced jobs must exist and must be in an earlier stage
- **`run:` keyword support** — recognised as alternative to `script:` (CI steps); no longer triggers "missing script" error
- **`spec:` reserved key** — top-level `spec:` is now recognised as a CI component header, not a job
- **New job model fields** — `interruptible`, `resource_group`, `start_in`, `run`
- **Testdata fixtures** — `keywords_valid.yml` (clean pipeline exercising every new check), `keywords_invalid.yml` (18 deliberate violations)

- **`extends:` resolution** — resolves single and chained template inheritance before linting; deep-merges base job fields into derived jobs (child scalars/lists win, maps are merged recursively); cycle detection via topological sort
- **`needs:` DAG validation** — checks referenced jobs exist, respect stage ordering, and contain no circular dependencies; handles both the `- job-name` shorthand and the `- job: name` map form; cross-pipeline needs (`pipeline:` key) are skipped
- **Hidden job support** — jobs named with a leading `.` are treated as reusable templates and exempted from the `script` requirement and other per-job checks
- **Core linter** — initial set of lint rules:
  - Missing `script` on non-trigger, non-template jobs (error)
  - Job `stage` not declared in `stages` (error)
  - `only`/`rules` or `except`/`rules` used together (error)
  - No `stages` block defined (warning)
  - Deprecated `only`/`except` usage (warning)
- **CLI** — `gitlab-sim <file>` exits 0 on clean pipelines, 1 on errors; prints findings with severity, job name, and message
- **YAML parser** — two-pass parse: reserved top-level keys (`stages`, `variables`, `default`, `include`, `workflow`) are decoded into typed structs; remaining keys are treated as job definitions
- **Taskfile** — `build`, `test`, `lint-go`, `validate`, `ci`, `clean` tasks via [Task](https://taskfile.dev)
- **Testdata fixtures** — `valid.yml`, `invalid.yml`, `extends.yml`, `needs.yml`, `needs_cycle.yml`
