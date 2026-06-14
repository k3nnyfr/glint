<p align="center">
  <img src="assets/glint-logo.png" alt="glint logo" width="220" />
</p>

<h1 align="center">glint</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"></a>
  <a href="CHANGELOG.md"><img src="https://img.shields.io/badge/release-v0.2.20-blue.svg" alt="Release"></a>
</p>

> **Disclaimer:** This tool was built through iterative AI-assisted development with [Claude](https://claude.ai). It is experimental, incomplete, and not intended for production use. Coverage of GitLab CI keywords is best-effort and may lag behind GitLab's evolving spec. Use it at your own discretion — no correctness guarantees are made. Contributions and bug reports are welcome.

A local tool to validate and lint `.gitlab-ci.yml` pipelines without needing a GitLab server.

## What it does

- **Lints** — 43 rules covering pipeline structure, keyword constraints, `needs:`/`dependencies:` graphs, expression reachability, and deprecations (GL001–GL043); run `glint explain <ID>` for any rule
- **Resolves includes** — local files, HTTPS URLs, GitLab project templates, and CI/CD Catalog components, with offline cache support
- **Simulates context** — `--branch`, `--tag`, `--source` flags evaluate `rules:if:` and `only`/`except` to show which jobs would be active, manual, or skipped
- **Multiple output formats** — `--format text` (default, ruff-style), `json`, `sarif` (GitHub Code Scanning / GitLab SAST), `junit`, `github` (PR annotations)
- **Project config** — `.glint.yml` for rule suppression, severity overrides, token/URL defaults; `# glint: ignore RULE` for per-job inline suppression
- **Graph visualization** — `glint graph` prints a terminal job tree; `glint graph pipeline` renders a GitLab CI-style SVG/PNG

See [FEATURES.md](FEATURES.md) for the complete feature reference and lint rules table, and [ROADMAP.md](ROADMAP.md) for planned improvements.

## Requirements

- Go 1.21 or later
- [Task](https://taskfile.dev) (optional, for development tasks)

## Installation

```bash
git clone https://git.k3nny.fr/glint
cd glint
go build -o glint ./cmd/glint/...
```

Or with Task:

```bash
task build
```

## Usage

```
glint [OPTIONS] <COMMAND>

Commands:
  check    Lint a pipeline file — exits 0 (clean) or 1 (errors found)
  graph    Visualise the pipeline as a job tree or Mermaid graph
  explain  Print description and fix for a lint rule
```

Run `glint <command> --help` for command-specific options and examples.

### `glint check`

```bash
glint check .gitlab-ci.yml
```

Exits `0` when no errors are found, `1` when at least one error is reported.

### Output formats

Pass `--format` to control the output. Plain text is the default.

```bash
# Default: ruff-style text (human-readable)
glint check .gitlab-ci.yml

# JSON — stable schema, machine-readable
glint check --format json .gitlab-ci.yml

# SARIF 2.1.0 — GitHub Code Scanning / GitLab SAST
glint check --format sarif .gitlab-ci.yml > glint.sarif

# JUnit XML — CI test-report artifact (GitLab: artifacts:reports:junit)
glint check --format junit .gitlab-ci.yml > glint-junit.xml

# GitHub Actions annotations — inline PR diff comments
glint check --format github .gitlab-ci.yml
```

In structured formats (`json`, `sarif`, `junit`, `github`) the summary line
(`OK: … no issues found` or `N finding(s): M error(s)`) is written to stderr
so stdout contains only the machine-readable payload.

**JSON schema (`schema_version: 1`):**
```json
{
  "schema_version": 1,
  "glint_version": "v0.2.18",
  "pipeline": ".gitlab-ci.yml",
  "findings": [
    {"rule":"GL004","severity":"error","file":".gitlab-ci.yml","line":14,
     "job":"deploy","message":"stage \"production\" is not defined in 'stages'"}
  ],
  "summary": {"total": 1, "errors": 1, "warnings": 0}
}
```

**GitHub annotation lines:**
```
::error file=.gitlab-ci.yml,line=14,title=GL004::job "deploy": stage "production" is not defined in 'stages'
```

### Project configuration (`.glint.yml`)

Place a `.glint.yml` file next to your pipeline (or anywhere in the directory tree up to the repository root) to configure glint for that project. glint searches upward from the pipeline file's directory, stopping at the first `.git` boundary.

```yaml
# .glint.yml

# Suppress specific rules entirely.
ignore:
  - GL007   # we still use only:/except:, migration in progress
  - GL032   # lots of dynamic variables injected by CI

# Override the severity of specific rules.
severity:
  GL004: warning  # demote stage errors to warnings during a migration
  GL035: error    # promote absolute-path warning to error for this project

# Extra stages that are valid but not declared in the pipeline YAML itself
# (e.g. injected by an include template we can't edit).
stages:
  - quality
  - security

# Default token — overridden by --token flag and GITLAB_TOKEN env.
token: glpat-xxxx

# Default GitLab instance URL.
url: https://gitlab.example.com

# Default cache directory for fetched remote includes.
cache_dir: ~/.cache/glint
```

**Priority chain for token and URL:** `--token`/`--gitlab-url` flags > `.glint.yml` values > `GITLAB_TOKEN`/`CI_SERVER_URL` environment variables.

### Inline suppression (`# glint: ignore`)

Suppress a finding for a specific job by placing a `# glint: ignore RULE` comment immediately before the job definition:

```yaml
# glint: ignore GL007
legacy-job:
  stage: build
  only:
    - main
  script: echo ok

# Multiple rules — comma- or space-separated:
# glint: ignore GL007, GL032
another-job:
  stage: build
  script: echo ok

# Suppress all rules for this job:
# glint: ignore all
noisy-job:
  stage: build
  script: echo ok
```

Inline suppressions are scoped to the single job they precede. They do not affect other jobs or pipeline-level findings. For project-wide suppression use `.glint.yml` `ignore:`.

### Remote project includes

Pipelines that include templates from other GitLab projects are supported.
Provide a token so `glint` can fetch them:

```bash
# personal access token (read_api scope)
GITLAB_TOKEN=glpat-xxxx glint check .gitlab-ci.yml

# CI/CD job token (when running inside a pipeline)
CI_JOB_TOKEN=$CI_JOB_TOKEN glint check .gitlab-ci.yml

# self-hosted GitLab
GITLAB_TOKEN=glpat-xxxx GITLAB_URL=https://gitlab.example.com glint check .gitlab-ci.yml

# or via flags
glint check --token glpat-xxxx --gitlab-url https://gitlab.example.com .gitlab-ci.yml
```

**Project includes** require a token; without one they are skipped with a
warning and the rest of the pipeline is linted as-is.

### Include cache and offline mode

Pass `--cache-dir` to cache fetched remote templates so repeated runs skip the network:

```bash
# First run: fetches and caches
glint check --cache-dir ~/.cache/glint .gitlab-ci.yml

# Subsequent runs: served from cache
glint check --cache-dir ~/.cache/glint .gitlab-ci.yml

# Fully offline (uses ~/.cache/glint automatically when --cache-dir is absent)
glint check --offline .gitlab-ci.yml
glint check --offline --cache-dir ~/.cache/glint .gitlab-ci.yml
```

Cache entries are keyed by SHA-256 of the full request URL/coordinates and stored as plain YAML files in the cache directory. There is currently no automatic expiry — delete the directory or individual entries to force a fresh fetch.

**Component includes** (`include: component: ...`) attempt the fetch
unauthenticated, so public [CI/CD Catalog](https://gitlab.com/explore/catalog)
components work without a token. A warning is emitted if the fetch fails.

Token resolution order (first non-empty wins):

| Source | Header used |
|--------|-------------|
| `--token` flag / `GITLAB_TOKEN` | `PRIVATE-TOKEN` |
| `CI_JOB_TOKEN` | `JOB-TOKEN` |
| `GITLAB_PRIVATE_TOKEN` | `PRIVATE-TOKEN` |

Instance URL resolution order: `--gitlab-url` flag → `CI_SERVER_URL` →
`GITLAB_URL` → `https://gitlab.com`

### Component reference format

```
<host>/<project-path>/<component-name>@<version>

gitlab.com/components/secret-detection/secret-detection@v0.1.0
gitlab.com/my-org/ci-catalog/lint@main
gitlab.example.com/platform/components/build@~latest
```

The component file is looked up in order:
1. `templates/<component-name>.yml` (single-file layout)
2. `templates/<component-name>/template.yml` (directory layout)

Component input parameters (`with:`) are not validated — they are resolved by
GitLab at runtime. Jobs in fetched components may use `$[[ inputs.xxx ]]`
placeholders in fields like `stage`; `glint` skips those fields rather
than producing false positive errors.

### `glint graph`

Visualise the pipeline. Without a mode word, prints a job tree and the include
dependency graph separated by `---`.

```bash
# Default: job tree + include dependency graph
glint graph .gitlab-ci.yml

# Job tree only (stages → jobs, like the tree command)
glint graph tree .gitlab-ci.yml

# Include dependency graph → Mermaid flowchart to stdout
glint graph includes .gitlab-ci.yml > includes.mmd

# GitLab-like pipeline layout → PNG (or SVG fallback) written to --out dir
glint graph pipeline .gitlab-ci.yml
# prints the output file path, e.g.: glint-out/pipeline-20260607-143022.png

# Mermaid to stdout + pipeline file path to stderr
glint graph all .gitlab-ci.yml > includes.mmd

# Custom output directory (pipeline mode)
glint graph pipeline --out /tmp/graphs .gitlab-ci.yml
```

**Job tree** (`graph tree`) — stages as branches, jobs as leaves. Jobs with
`when: manual`, `when: delayed`, or `trigger:` are annotated in brackets.

**Include graph** (`graph includes`) — [Mermaid](https://mermaid.js.org) flowchart written to stdout.
Pipe to a `.mmd` file or paste into [mermaid.live](https://mermaid.live).
One node per include entry, colour-coded by type:
- Orange (bold): the main pipeline file
- Purple: `project:` includes
- Green: `component:` includes
- Blue: `local:` includes
- Grey: `remote:` URL includes
- Light orange: GitLab-provided `template:` includes

**Pipeline graph** (`graph pipeline`) — GitLab CI-style SVG rendered to a timestamped file
in the `--out` directory (default: `glint-out/`). Converted to PNG automatically
when `rsvg-convert`, `inkscape`, or `magick` is available; falls back to SVG otherwise.
Jobs are colour-coded by type:
- Blue (`#1f75cb`): regular jobs
- Orange (`#fc6d26`): `when: manual` jobs
- Purple (`#6b4fbb`): `trigger:` jobs
- Amber (`#fca326`): `when: delayed` jobs

DAG mode (job-to-job Bézier arrows) activates automatically when any job has a `needs:` list.
Classic mode draws L-shaped or straight connectors between stage columns otherwise.

### `glint explain`

Print the documentation for a specific lint rule.

```bash
# Show description, example, and fix for GL007
glint explain GL007

# Case-insensitive: gl007 and GL007 are equivalent
glint explain gl007

# List all rules with their IDs and severity
glint explain
```

### Context simulation

Pass `--branch`, `--tag`, or `--source` to `glint check` to see which jobs
would run for a given pipeline event. The pipeline is still fully linted;
context output is printed first.

```bash
# What runs on a push to develop?
glint check --branch develop .gitlab-ci.yml

# What runs when a v1.2.0 tag is pushed?
glint check --tag v1.2.0 .gitlab-ci.yml

# Merge request pipeline
glint check --source merge_request_event .gitlab-ci.yml

# Arbitrary variable overrides (repeatable)
glint check --branch main --var DEPLOY_ENV=production .gitlab-ci.yml
```

**Evaluated:**
- `rules:if:` — full expression language: `==`, `!=`, `=~`, `!~`, `&&`, `||`, `!`, `()`, `$VAR`, string literals, `null`
- `only:` / `except:` — ref keywords (`branches`, `tags`, `merge_requests`, `schedules`, …), branch name globs (`feat/*`), and `/regex/` patterns
- Variable expansion — `$VAR` / `${VAR}` references within variable values are expanded after all sources are merged; use `--list-vars` to inspect the resolved values

**Not evaluated** (no git tree at lint time): `rules:changes:`, `rules:exists:`.
Rules without an `if:` clause always match.

**Predefined variables** set automatically by the shortcut flags:

| Flag | Variables populated |
|------|---------------------|
| `--branch <name>` | `CI_COMMIT_BRANCH`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE=push` |
| `--tag <name>` | `CI_COMMIT_TAG`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE=push` (clears `CI_COMMIT_BRANCH`) |
| `--source <event>` | `CI_PIPELINE_SOURCE` |
| `--var KEY=VALUE` | any variable; overrides shortcuts |

### Example output

```
# Clean pipeline (implicit default: --branch main --source push)
Context: branch=main, source=push

Active  (5): build, deploy-staging, test, ...

OK: .gitlab-ci.yml — no issues found (5 job(s), 3 stage(s))

# With --branch develop context
Context: branch=develop, source=push

Active  (3): build, deploy-staging, test
Skipped (2): deploy-prod, release-notes

OK: .gitlab-ci.yml — no issues found (5 job(s), 3 stage(s))

# With --tag v1.0.0 context
Context: tag=v1.0.0, source=push

Active  (4): build, deploy-prod, release-notes, test
Skipped (1): deploy-staging

OK: .gitlab-ci.yml — no issues found (5 job(s), 3 stage(s))

# Pipeline with issues
.gitlab-ci.yml:14: GL004 [error] job "deploy": stage "production" is not defined in 'stages'
.gitlab-ci.yml:22: GL027 [error] job "test": needs unknown job "build-app"
.gitlab-ci.yml:31: GL007 [warning] job "old-job": 'only'/'except' are deprecated; prefer 'rules'

3 finding(s): 2 error(s)
```

## Development

This project uses [Task](https://taskfile.dev) as a task runner.

```bash
task              # list available tasks
task build        # compile the binary
task test         # run Go unit tests
task lint-go      # run go vet
task validate     # run the binary against all testdata fixtures
task ci           # full check: vet → test → build → validate
task build-windows  # cross-compile for Windows x64 (requires a tagged commit → glint-<tag>.exe)
task build-linux    # cross-compile for Linux x64 (requires a tagged commit → glint-<tag>-linux-amd64)
task clean        # remove build artifacts
```

## Project structure

```
.
├── cmd/glint/     # CLI entrypoint
├── internal/
│   ├── cicontext/      # CI variable context, rules:if: evaluator, job reachability
│   ├── fetcher/        # GitLab API client (project include fetching)
│   ├── graph/          # Mermaid and SVG/PNG graph generators
│   ├── linter/         # lint rules and findings
│   ├── model/          # pipeline data structures and YAML parser
│   └── resolver/       # extends: resolution and project include merging
├── testdata/           # sample pipelines used for manual validation
├── Taskfile.yml
└── go.mod
```
