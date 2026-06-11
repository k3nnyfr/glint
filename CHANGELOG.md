# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org).

## [Unreleased]

### Added

- **File and line numbers on findings** — every finding now includes the source file and line where the job is defined, e.g. `[ERROR] job "deploy" (src/deploy.yml:14): …`. For jobs that come from local or fetched includes the file reflects the include source. Pipeline-level findings (workflow rules, missing stages) reference the root pipeline file.

- **`glint graph includes` shows jobs per file** — each node in the Mermaid include dependency graph now shows the jobs defined directly in that file. Jobs are rendered as rounded nodes (`(name)`) in a distinct light-purple style, connected with dashed arrows (`-.->`) to distinguish ownership from the include hierarchy (solid `-->` arrows). The root pipeline file always shows its direct jobs; local and fetched project/component nodes show theirs when the file can be read.

### Fixed

- **`extends:` jobs with missing script downgraded to warning** — a job that declares `extends:` but has no `script` after resolution now emits `[WARNING]` instead of `[ERROR]`. The script may legitimately come from a base job in a remote include that could not be fetched at lint time (e.g. no token configured).

- **Variable map form now parses correctly** — `variables:` entries that use the extended `{value, description, options}` form (GitLab CI 13.7+) no longer cause `yaml: cannot unmarshal !!map into string`. Both `Pipeline.Variables` and per-job `Variables` now accept either plain strings or map-form declarations.
- **`default.image` map form now parses correctly** — `default: image: {name: ..., pull_policy: ...}` used to cause `yaml: cannot unmarshal !!map into string`; `DefaultConfig.Image` is now typed as `any` to match `Job.Image`.
- **`default.before_script` / `default.after_script` now accept both list and scalar forms** — previously `DefaultConfig.BeforeScript` and `DefaultConfig.AfterScript` were `[]string`, causing a parse error when the field was written as a block scalar string. They are now typed as `any` to match the corresponding `Job` fields.
- **`rules.changes` / `rules.exists` map form now parses correctly** — extended `changes: {paths: [...], compare_to: "..."}` syntax (GitLab CI 15.3+) used to cause `yaml: cannot unmarshal !!map into []string`.

## [0.2.0] - 2026-06-11

### Added

- **Subcommand CLI** — reworked interface inspired by [ruff](https://docs.astral.sh/ruff/):
  - `glint check <file>` — lint a pipeline (replaces bare `glint <file>`)
  - `glint graph [mode] <file>` — visualise the pipeline (replaces `--graph` flag)
  - Graph modes: no-arg (tree + includes), `tree`, `includes`, `pipeline`, `all`
  - Per-command `--help` with ruff-style layout: `Arguments:`, `Options:` (flag declaration on its own line, description below), `[env: ...]` / `[default: ...]` / `[possible values: ...]` metadata, `Examples:` section

- **`glint graph tree`** — jobs displayed as a terminal directory tree grouped by stage (like the `tree` command); job-type annotations (`[manual]`, `[delayed]`, `[trigger]`) when no context is set; evaluated-state annotations (`[skipped]`, `[manual]`) when a context is provided via `--branch` / `--tag` / `--source`

- **Context flags on `glint graph`** — `--branch`, `--tag`, `--source`, `--var` are now available on `glint graph` as well as `glint check`

- **Local include resolution** — `include: local:` entries are now read from disk, recursively resolved, and merged into the pipeline before linting; enables cross-file `extends:` and `needs:` validation for multi-file pipelines

- **Cross-platform release builds** — two Taskfile tasks for tagged release binaries:
  - `task build-windows` — cross-compiles for Windows x64; output: `glint-<tag>.exe`
  - `task build-linux` — cross-compiles for Linux x64; output: `glint-<tag>-linux-amd64`
  - Both tasks require an exact git tag on the current commit

### Fixed

- **`extends:` unknown base no longer fatal** — when a base job referenced by `extends:` does not exist, glint now emits a resolver warning and skips extends resolution for that job rather than aborting with exit code 2; linting continues on the job's own fields
- **`script: |` (block scalar) support** — jobs using a multiline block scalar for `script:`, `before_script:`, or `after_script:` are now parsed correctly; previously caused false-positive "missing script" errors

### Changed

- **`glint <file>` removed** — use `glint check <file>`
- **`--graph <mode>` removed** — replaced by `glint graph [mode]`
- **`--graph-out` renamed to `--out`** — now a flag on `glint graph` (`glint graph pipeline --out <dir>`)

## [0.1.0] - 2026-06-07

### Added

- **Context simulation (`--branch` / `--tag` / `--source` / `--var`)** — show which jobs would be active, manual, or skipped for a specific pipeline event without leaving the terminal:
  - `--branch <name>` — simulates a branch push; populates `CI_COMMIT_BRANCH`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, and defaults `CI_PIPELINE_SOURCE` to `push`
  - `--tag <name>` — simulates a tag push; populates `CI_COMMIT_TAG`, clears `CI_COMMIT_BRANCH`
  - `--source <event>` — sets `CI_PIPELINE_SOURCE` explicitly (`merge_request_event`, `schedule`, `web`, `api`, `pipeline`, …)
  - `--var KEY=VALUE` — sets any CI variable; repeatable; overrides shortcut values
  - Evaluates `rules:if:` expressions (`==`, `!=`, `=~`, `!~`, `&&`, `||`, `!`, parentheses, `$VAR`, string literals, `null`)
  - Evaluates `only:`/`except:` ref keywords (`branches`, `tags`, `merge_requests`, `schedules`, `pushes`, `web`, `api`, `pipelines`), branch name globs (`feat/*`), and `/regex/` patterns
  - Evaluates `workflow:rules:` — warns when the pipeline itself would not start for the given context
  - `rules:changes:` and `rules:exists:` are not evaluated (no git tree at lint time); rules without `if:` always match
  - Linting runs in full regardless of context; context output is printed before findings
  - New `internal/cicontext` package (`context.go`, `eval.go`, `reachability.go`); no new external dependencies
  - 33 unit tests covering the expression evaluator; 5 fixture runs in `task validate`

- **Graph output (`--graph`)** — visualises the pipeline instead of running lint rules:
  - `--graph includes` — [Mermaid](https://mermaid.js.org) flowchart of include dependencies written to stdout; one node per `include:` entry (project, component, local, remote, template), colour-coded by type; pipe to a `.mmd` file or paste into [mermaid.live](https://mermaid.live)
  - `--graph pipeline` — GitLab CI-style SVG/PNG pipeline graph written to a timestamped file in `--graph-out` (default: `glint-out/`); jobs rendered as white chip cards with a coloured status indicator (blue: regular, orange: manual, purple: trigger, amber: delayed); DAG mode draws job-to-job Bézier arrows when any job has `needs:`, classic mode draws L-shaped connectors between stage columns; converted to PNG automatically when `rsvg-convert`, `inkscape`, or `magick` is available
  - `--graph all` — include Mermaid to stdout, pipeline file path to stderr
  - New `internal/graph` package (`includes.go`, `pipeline.go`, `render.go`); no new external dependencies

- **CI/CD catalog component resolution** — resolves `include: component:` references from the GitLab CI/CD Catalog:
  - Reference format: `<host>/<project-path>/<component-name>@<version>` (host determines which GitLab instance is queried)
  - Tries single-file layout (`templates/<name>.yml`) then directory layout (`templates/<name>/template.yml`) automatically
  - Public catalog components are fetched without authentication (no token required)
  - References containing CI variables (e.g. `$CI_SERVER_FQDN`) are skipped with a warning — they cannot be resolved at lint time
  - Jobs imported from a component may use `$[[ inputs.xxx ]]` input placeholders in stage names; the stage validation check is skipped for those values rather than producing false positives
- **Remote project include resolution** — fetches `include: project:` templates from the GitLab REST API before linting; jobs from remote templates are merged into the pipeline so `extends:`, `needs:`, and `dependencies:` references can be validated across file boundaries
  - Token auto-discovery: `GITLAB_TOKEN` (→ `PRIVATE-TOKEN` header) → `CI_JOB_TOKEN` (→ `JOB-TOKEN` header) → `GITLAB_PRIVATE_TOKEN`
  - Instance URL auto-discovery: `--gitlab-url` flag → `CI_SERVER_URL` → `GITLAB_URL` → `https://gitlab.com`
  - `--token` and `--gitlab-url` CLI flags for explicit overrides
  - `file:` accepts both string and list-of-strings forms
  - Project includes require a token; they are skipped with a `WARNING` when none is configured
  - Component includes attempt the fetch unauthenticated first; a `WARNING` is emitted only on failure
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
- **CLI** — `glint <file>` exits 0 on clean pipelines, 1 on errors; prints findings with severity, job name, and message
- **YAML parser** — two-pass parse: reserved top-level keys (`stages`, `variables`, `default`, `include`, `workflow`) are decoded into typed structs; remaining keys are treated as job definitions
- **Taskfile** — `build`, `test`, `lint-go`, `validate`, `ci`, `clean` tasks via [Task](https://taskfile.dev)
- **Testdata fixtures** — `valid.yml`, `invalid.yml`, `extends.yml`, `needs.yml`, `needs_cycle.yml`
