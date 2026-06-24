# glint — usage reference

Full examples and option descriptions for every command.
For the lint rules reference see [FEATURES.md](FEATURES.md).

---

## `glint check`

Lint a pipeline file. Exits `0` when no errors are found, `1` when at least
one error is reported (warnings alone do not fail).

```bash
glint check .gitlab-ci.yml
```

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

In structured formats (`json`, `sarif`, `junit`, `github`) the summary line is
written to stderr so stdout contains only the machine-readable payload.

**JSON schema (`schema_version: 1`):**
```json
{
  "schema_version": 1,
  "glint_version": "v0.2.20",
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

### Context simulation

Pass `--branch`, `--tag`, or `--source` to evaluate `rules:if:` and
`only`/`except` against a specific pipeline event. The pipeline is still fully
linted; the context summary is printed first. When no context flag is given,
glint defaults to `--branch main --source push`.

```bash
# What runs on a push to develop?
glint check --branch develop .gitlab-ci.yml

# What runs when a v1.2.0 tag is pushed?
glint check --tag v1.2.0 .gitlab-ci.yml

# Merge request pipeline
glint check --source merge_request_event .gitlab-ci.yml

# Arbitrary variable overrides (repeatable)
glint check --branch main --var DEPLOY_ENV=production .gitlab-ci.yml

# Debug variable resolution
glint check --list-vars .gitlab-ci.yml
```

**Predefined variables** set by the shortcut flags:

| Flag | Variables populated |
|------|---------------------|
| `--branch NAME` | `CI_COMMIT_BRANCH`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE=push` |
| `--tag NAME` | `CI_COMMIT_TAG`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE=push`; clears `CI_COMMIT_BRANCH` |
| `--source EVENT` | `CI_PIPELINE_SOURCE` |
| `--var KEY=VALUE` | any variable; overrides shortcuts; repeatable |

### Remote project includes

Provide a token so glint can fetch `include: project:` templates:

```bash
# Personal access token (read_api scope)
GITLAB_TOKEN=glpat-xxxx glint check .gitlab-ci.yml

# CI/CD job token (when running inside a pipeline)
CI_JOB_TOKEN=$CI_JOB_TOKEN glint check .gitlab-ci.yml

# Self-hosted GitLab
GITLAB_TOKEN=glpat-xxxx GITLAB_URL=https://gitlab.example.com glint check .gitlab-ci.yml

# Via flags (override env vars)
glint check --token glpat-xxxx --gitlab-url https://gitlab.example.com .gitlab-ci.yml
```

Project includes are skipped with a warning when no token is available; linting
continues with whatever is resolved.

**Token resolution order** (first non-empty wins):

| Source | Header |
|--------|--------|
| `--token` flag / `GITLAB_TOKEN` env | `PRIVATE-TOKEN` |
| `CI_JOB_TOKEN` env | `JOB-TOKEN` |
| `GITLAB_PRIVATE_TOKEN` env | `PRIVATE-TOKEN` |

**URL resolution order:** `--gitlab-url` flag → `CI_SERVER_URL` env → `GITLAB_URL` env → `https://gitlab.com`

### Include cache and offline mode

```bash
# Cache fetched templates to disk (keyed by SHA-256 of the request coordinates)
glint check --cache-dir ~/.cache/glint .gitlab-ci.yml

# Fully offline — serve from cache, warn on misses, make no network calls
glint check --offline .gitlab-ci.yml
glint check --offline --cache-dir /path/to/cache .gitlab-ci.yml
```

`--offline` without `--cache-dir` uses `~/.cache/glint` automatically.
There is no automatic cache expiry; delete the directory or individual entries
to force a re-fetch.

### Component reference format

`include: component:` references follow the format:

```
<host>/<project-path>/<component-name>@<version>

gitlab.com/components/secret-detection/secret-detection@v0.1.0
gitlab.com/my-org/ci-catalog/lint@main
gitlab.example.com/platform/components/build@~latest
```

The component file is looked up in order:
1. `templates/<component-name>.yml` (single-file layout)
2. `templates/<component-name>/template.yml` (directory layout)

Public Catalog components work without a token. `with:` input parameters are
substituted locally before parsing; they are not validated against the
component spec.

### Example output

```
# Clean pipeline
Context: branch=main, source=push

Active  (5): build, deploy-staging, test, lint, security-scan

OK: .gitlab-ci.yml — no issues found (5 job(s), 3 stage(s))

# With --branch develop
Context: branch=develop, source=push

Active  (3): build, deploy-staging, test
Skipped (2): deploy-prod, release-notes

OK: .gitlab-ci.yml — no issues found (5 job(s), 3 stage(s))

# Pipeline with issues
.gitlab-ci.yml:14: GL004 [error] job "deploy": stage "production" is not defined in 'stages'
.gitlab-ci.yml:22: GL027 [error] job "test": needs unknown job "build-app"
.gitlab-ci.yml:31: GL007 [warning] job "old-job": 'only'/'except' are deprecated; prefer 'rules'

3 finding(s): 2 error(s)
```

---

## `glint graph`

Visualise the pipeline. Without a mode word, prints a job tree and the include
dependency graph separated by `---`.

```bash
# Default: job tree + include dependency graph
glint graph .gitlab-ci.yml

# Job tree only
glint graph tree .gitlab-ci.yml

# Job tree with context (shows active/manual/skipped annotations)
glint graph tree --branch develop .gitlab-ci.yml

# Include dependency graph → Mermaid flowchart to stdout
glint graph includes .gitlab-ci.yml > includes.mmd

# GitLab-like pipeline layout → PNG/SVG written to --out dir
glint graph pipeline .gitlab-ci.yml
# prints the output path, e.g.: glint-out/pipeline-20260614-143022.png

# Pipeline graph with skipped-job colouring (grey out context-unreachable jobs)
glint graph pipeline --branch main .gitlab-ci.yml

# Interactive HTML with pan/zoom and job-detail sidebar
glint graph pipeline --format html .gitlab-ci.yml

# Mermaid flowchart of the pipeline to stdout
glint graph pipeline --format mermaid .gitlab-ci.yml

# Mermaid includes graph to stdout + pipeline SVG file path to stderr
glint graph all .gitlab-ci.yml > includes.mmd

# Custom output directory
glint graph pipeline --out /tmp/graphs .gitlab-ci.yml
```

**Job tree** — stages as branches, jobs as leaves; annotated with `[manual]`,
`[delayed]`, `[trigger]` where applicable. Context flags apply the same
evaluation as `glint check`.

**Include graph** — [Mermaid](https://mermaid.js.org) flowchart; pipe to `.mmd`
or paste into [mermaid.live](https://mermaid.live). Nodes are colour-coded by
include type: orange (main file), purple (project), green (component), blue
(local), grey (remote URL), light orange (GitLab template).

**Pipeline graph** — GitLab CI-style SVG rendered to a timestamped file.
Converted to PNG when `rsvg-convert`, `inkscape`, or `magick` is available.

**Pipeline graph formats** (`--format`):

| Flag | Output |
|------|--------|
| `svg` (default) | Write SVG/PNG to `--out` |
| `html` | Write self-contained HTML to `--out` — inline SVG with mouse pan/zoom, drag, and a job-detail sidebar that opens on chip click |
| `mermaid` | Print Mermaid flowchart to stdout |

**Visual chip styles:**

| Type | Colour | Icon | Border |
|------|--------|------|--------|
| regular | blue `#1f75cb` | checkmark | solid |
| manual | orange `#fc6d26` | play triangle | solid |
| trigger | purple `#6b4fbb` | chevron | solid |
| delayed | yellow `#fca326` | clock | solid |
| on_failure | red `#d9534f` | X mark | dashed |
| skipped (context) | grey `#868686` | none | solid |

Pass context flags (`--branch`, `--tag`, `--source`, `--var`) to grey out jobs
that would be skipped in the given pipeline event.
DAG mode (Bézier arrows between jobs) activates automatically when any job has
a `needs:` list; classic mode uses stage-column connectors otherwise.

---

## `glint explain`

Print the documentation for a specific lint rule.

```bash
# Description, example, and fix for GL007
glint explain GL007

# Case-insensitive
glint explain gl007

# List all rules with ID, severity, and title
glint explain
```

---

## Project configuration (`.glint.yml`)

Place a `.glint.yml` anywhere in the directory tree from the pipeline file up
to the first `.git` boundary. glint searches upward and uses the first file it
finds.

```yaml
# Suppress rules globally for this project.
ignore:
  - GL007   # migrating from only:/except:
  - GL032   # dynamic variables injected by CI

# Override rule severity: error | warning | ignore
# 'ignore' is equivalent to listing the rule under ignore:.
severity:
  GL004: warning   # demote during a stage migration
  GL035: error     # promote to hard error for this project

# Extra stage names valid beyond those declared in the pipeline YAML
# (e.g. injected by an include template you don't own).
stages:
  - quality
  - security

# Default token — lower priority than --token and GITLAB_TOKEN.
token: glpat-xxxx

# Default GitLab instance URL.
url: https://gitlab.example.com

# Default cache directory for fetched remote includes.
cache_dir: ~/.cache/glint
```

**Priority chain:** `--token`/`--gitlab-url` flags > `.glint.yml` > environment variables.

---

## Inline suppression (`# glint: ignore`)

Suppress a finding for a specific job by placing a comment immediately before
the job definition in the pipeline YAML:

```yaml
# glint: ignore GL007
legacy-job:
  stage: build
  only: [main]
  script: echo ok

# Multiple rules — comma- or space-separated:
# glint: ignore GL007, GL032
other-job:
  stage: build
  script: echo ok

# Suppress every rule for this job:
# glint: ignore all
noisy-job:
  stage: build
  script: echo ok
```

Suppressions are scoped to the single job they precede and do not affect other
jobs or pipeline-level findings. For project-wide suppression, use `.glint.yml`
`ignore:` instead.
