# glint — feature reference

This document describes every capability in the current release.
For planned work see [ROADMAP.md](ROADMAP.md).

---

## Lint rules

Every finding carries a stable rule ID (`GL001` – `GL045`) that can be used to
suppress, filter, or look up the check. Run `glint explain <ID>` for a
description, bad-YAML example, and fix.

### Pipeline-level

| ID | Sev | Rule |
|----|-----|------|
| GL001 | WARN | No `stages:` block — GitLab falls back to its built-in default stages |
| GL002 | ERR | `workflow.rules[*].when` must be `always` or `never` |
| GL036 | ERR | `default.timeout` is not a valid GitLab CI duration string |
| GL040 | WARN | A stage name appears more than once in `stages:` |
| GL045 | WARN | `include: remote:` uses plain `http://` — CI templates fetched unencrypted; prefer `https://` |

### Job structure

| ID | Sev | Rule |
|----|-----|------|
| GL003 | ERR | Job missing required `script:` (or `run:`) |
| GL004 | ERR | Job `stage:` references a stage not declared in `stages:` |
| GL005 | ERR | `only:` and `rules:` used together |
| GL006 | ERR | `except:` and `rules:` used together |
| GL007 | WARN | `only:` / `except:` used (deprecated; prefer `rules:`) |

### Keyword constraints

| ID | Sev | Rule |
|----|-----|------|
| GL008 | ERR | `when:` has an invalid value |
| GL009 | ERR | `when: delayed` without `start_in:` |
| GL010 | ERR | `start_in:` set but `when:` is not `delayed` |
| GL011 | ERR | `parallel:` integer not in range 2–200, or map form missing `matrix:` |
| GL012 | ERR | `retry:` integer not in range 0–2, or `retry.max` out of range |
| GL013 | ERR | `retry.when:` contains an unrecognised failure type |
| GL014 | ERR | `allow_failure:` is not a boolean or a map with `exit_codes:` |
| GL015 | ERR | `interruptible:` is not a boolean |
| GL016 | ERR | Trigger job also defines `script:` |
| GL017 | ERR | `trigger:` map missing `project:` or `include:` |
| GL018 | ERR | `coverage:` is not a regex wrapped in `/…/` |
| GL019 | ERR | `release:` missing required `tag_name:`, or is not a map |
| GL020 | ERR | `environment.url` set without `environment.name`, or invalid `action:` |
| GL021 | ERR | `artifacts.when` invalid, or `expose_as` set without `paths` |
| GL022 | WARN | `pages` job `artifacts.paths` does not include `public/` |
| GL023 | ERR | `cache.when` or `cache.policy` has an invalid value |
| GL024 | ERR | `rules[*].when` has an invalid value |
| GL025 | ERR | `image:` map form missing `name:` |
| GL026 | ERR | `inherit.default` / `inherit.variables` is not a boolean or list |
| GL034 | ERR | `services:` map form missing `name:`, or `alias:` is not a valid DNS label |
| GL036 | ERR | `timeout:` is not a valid GitLab CI duration string |
| GL037 | ERR | `id_tokens:` entry missing required `aud:` |
| GL038 | ERR | `secrets:` entry missing a provider key (`vault`, `gcp_secret_manager`, `azure_key_vault`) |
| GL039 | WARN | Job uses `pages:` keyword but `artifacts.paths` doesn't include the publish directory |
| GL041 | WARN | `cache.key.files` entry looks like a glob; must be an exact file path |

### Cross-job graph

| ID | Sev | Rule |
|----|-----|------|
| GL027 | ERR/WARN | `needs:` references a job that doesn't exist (WARN when `optional: true`) |
| GL028 | ERR | `needs:` references a job in a later stage |
| GL029 | ERR | Circular dependency in `needs:` graph |
| GL030 | ERR | `dependencies:` references a job that doesn't exist |
| GL031 | ERR | `dependencies:` references a job in the same or a later stage |
| GL044 | ERR/WARN | `rules:needs:` references a job that doesn't exist (WARN when `optional: true`) |

### Expression & reachability

| ID | Sev | Rule |
|----|-----|------|
| GL032 | WARN | `rules:if:` references `$VAR` not declared in any `variables:` block (may be false-positive for project-setting variables) |
| GL033 | WARN | Every rule in `rules:` has `when: never` — job permanently excluded (provable without evaluating `if:` expressions) |
| GL035 | WARN | `rules:changes` / `rules:exists` path is absolute — absolute paths never match in GitLab CI |
| GL042 | WARN | Every `rules:if:` condition evaluates to false given the declared variable values — job statically unreachable (only fires when all referenced variables are declared in YAML) |

### Inheritance

| ID | Sev | Rule |
|----|-----|------|
| GL043 | WARN | `inherit: default:` declared but the pipeline has no `default:` block (dead declaration), or the list form names fields not set in `default:` |

### Hidden jobs

Jobs whose name starts with `.` are reusable templates; most rules are skipped for them. This matches GitLab CI's own behaviour.

---

## Include resolution

| Capability | Notes |
|------------|-------|
| `include: local:` | Read from disk, recursively merged before linting |
| `include: remote:` | HTTPS URL fetched unauthenticated; unreachable URLs produce a warning, linting continues |
| `include: project:` | Fetched from the GitLab REST API using the configured token; skipped with a warning when no token is available |
| `include: component:` | Fetched from the GitLab CI/CD Catalog; public components work without a token |
| `include: inputs:` | `$[[ inputs.KEY ]]` and `$[[ inputs.KEY \| default(…) ]]` placeholders substituted from the `with:` block before parsing |
| Depth limit | Include chains capped at 100 levels (matches GitLab); circular cross-file references detected via visited-set tracking |
| Offline mode | `--offline` serves all remote includes from the cache; missing entries produce a warning |
| Include cache | `--cache-dir DIR` (or `~/.cache/glint` by default with `--offline`) persists fetched templates; keyed by SHA-256 of the request coordinates |

**Token resolution order** (first non-empty wins):

| Source | Header |
|--------|--------|
| `--token` flag / `GITLAB_TOKEN` env | `PRIVATE-TOKEN` |
| `CI_JOB_TOKEN` env | `JOB-TOKEN` |
| `GITLAB_PRIVATE_TOKEN` env | `PRIVATE-TOKEN` |

**URL resolution order:** `--gitlab-url` flag → `CI_SERVER_URL` env → `GITLAB_URL` env → `https://gitlab.com`

---

## Pipeline rendering (`glint render`)

`glint render <PIPELINE>` resolves all `include:` and `extends:` chains and
writes the fully flattened pipeline to a single YAML file. This is what GitLab
CI processes server-side.

```bash
glint render .gitlab-ci.yml                        # writes rendered.gitlab-ci.yml
glint render --output merged.yml .gitlab-ci.yml    # custom output path
glint render --output - .gitlab-ci.yml | yq .      # stream to stdout
glint render --offline --cache-dir ~/.cache/glint .gitlab-ci.yml
```

**Output order:** `stages`, `variables`, `default`, `workflow`, template jobs
(`.name`, alphabetical), then regular jobs (alphabetical). The `include:` key
(consumed by resolution) and `extends:` keys (merged into each job) are
stripped. All other fields are preserved verbatim.

Accepts the same network flags as `glint check`: `--token`, `--gitlab-url`,
`--cache-dir`, `--offline`, `--proxy`. When writing to a file (not stdout)
a summary line is printed to stderr: `rendered: <file> (N job(s), M stage(s))`.

---

## Context simulation

Pass `--branch`, `--tag`, `--source`, or `--var` to evaluate `rules:if:` and
`only`/`except` filters against a specific pipeline event. When no context flag
is given glint defaults to `--branch main --source push` so that `rules:if:`
expressions are always evaluated.

**Evaluated at lint time:**

- `rules:if:` — full expression language: `==`, `!=`, `=~`, `!~`, `&&`, `||`, `!`, `(…)`, `$VAR`/`${VAR}`, string literals, `null`, regex flags (`/pat/i`)
- `only:` / `except:` — ref keywords, branch-name globs, and `/regex/` patterns
- `workflow:rules:` — evaluated to determine whether the pipeline would run; matching rule's `variables:` are injected before job evaluation
- `rules:changes:` — path-glob patterns evaluated against the supplied changed-file list (see `--changes` / `--changes-from` flags); `*` matches within a segment, `**` crosses `/` boundaries; the extended `{paths: [...], compare_to: ...}` form is supported; without changed-file data the condition is always treated as matching (permissive)
- Variable expansion — `$VAR` / `${VAR}` references in variable values expanded after all sources merge; transitive chains resolved (up to 10 passes)

**Not evaluated** (no git tree at lint time): `rules:exists:`.

**Predefined variables** set by shortcut flags:

| Flag | Variables populated |
|------|---------------------|
| `--branch NAME` | `CI_COMMIT_BRANCH`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE=push` |
| `--tag NAME` | `CI_COMMIT_TAG`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE=push`; clears `CI_COMMIT_BRANCH` |
| `--source EVENT` | `CI_PIPELINE_SOURCE` |
| `--var KEY=VALUE` | any variable; overrides shortcuts; repeatable |

Use `--list-vars` to print the resolved variable table to stderr.

**Changed-file flags** (for `rules:changes:` evaluation):

| Flag | Effect |
|------|--------|
| `--changes PATH` | Mark PATH as changed; repeatable |
| `--changes-from REF` | Run `git diff --name-only REF` to auto-detect changed files |

**Context-scoped linting** (single-context mode):

When a context is given, jobs evaluated as `skipped` are excluded from `needs:` and `dependencies:` cross-job checks (GL027–GL031). This eliminates false positives for jobs that are intentionally gated to specific pipeline events:

```yaml
deploy-job:
  needs: [build-job]   # GL027 suppressed on branch pipelines; only checked on tag pipelines
  rules:
    - if: '$CI_COMMIT_TAG != ""'
```

**Multi-context comparison** (`--context`):

Pass `--context KEY=VALUE[,...]` (repeatable) instead of `--branch`/`--tag`/`--source` to evaluate every job across multiple contexts simultaneously. Each `--context` flag defines one column; glint prints a table showing `active`, `manual`, `skipped`, or `blocked` per job per context.

Known context keys: `branch`, `tag`, `source` (case-insensitive). Any other `KEY=VALUE` pair is injected as a CI variable override. The `--changes`/`--changes-from` file list is shared across all contexts.

```
glint check --context branch=main --context branch=develop .gitlab-ci.yml

Context comparison:

JOB         branch=main  branch=develop
----------  -----------  -------------
build-job   active       active
deploy-job  active       skipped
test-job    active       active
```

---

## Output formats (`glint check`)

Pass `--format` to `glint check`. In structured formats the summary line is
written to stderr so stdout contains only the machine-readable payload.

| Format | Flag | Description |
|--------|------|-------------|
| Text (default) | `--format text` | Four aligned columns (location, rule, severity, message); `error` in bold red, `warning` in bold orange; colors auto-detected (suppressed when `NO_COLOR` is set or stdout is not a terminal) |
| JSON | `--format json` | Stable schema (version 1); `findings` array + `summary` block |
| SARIF 2.1.0 | `--format sarif` | Consumed by GitHub Code Scanning and GitLab SAST |
| JUnit XML | `--format junit` | CI test-report artifact (`artifacts:reports:junit`) |
| GitHub annotations | `--format github` | `::error file=…,line=…,title=RULE::message` inline PR comments |

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | No findings (clean pipeline) |
| `2` | One or more error findings |
| `10` | One or more warning findings, no errors |

Use `--no-warn` to suppress all warnings; a pipeline with only warnings then exits `0`.

**JSON schema (`schema_version: 1`):**
```json
{
  "schema_version": 1,
  "glint_version": "v0.3.0",
  "pipeline": ".gitlab-ci.yml",
  "findings": [
    {"rule":"GL004","severity":"error","file":".gitlab-ci.yml","line":14,"column":1,
     "job":"deploy","message":"stage \"production\" is not defined in 'stages'"}
  ],
  "summary": {"total": 1, "errors": 1, "warnings": 0}
}
```

---

## Configuration

### `.glint.yml` project config file

Searched from the pipeline file's directory upward to the first `.git` boundary.

```yaml
# Suppress rules globally for this project.
ignore:
  - GL007   # migrating from only:/except:
  - GL032   # dynamic variables injected by CI

# Override rule severity (error | warning | ignore).
# 'ignore' is equivalent to listing the rule in ignore:.
severity:
  GL004: warning   # demote during a stage migration
  GL035: error     # promote to hard error for this project

# Extra stage names valid beyond those in the pipeline's own stages: block.
stages:
  - quality
  - security

# Default GitLab token (lower priority than --token and GITLAB_TOKEN).
token: glpat-xxxx

# Default GitLab instance URL.
url: https://gitlab.example.com

# Default cache directory.
cache_dir: ~/.cache/glint

# HTTP proxy for remote includes and GitLab API calls.
# Overrides HTTP_PROXY / HTTPS_PROXY env vars when set.
# Leave empty to use system proxy settings.
proxy: http://proxy.example.com:8080
```

**Priority chain:** `--token`/`--gitlab-url`/`--proxy` flags > `.glint.yml` > environment variables.

### Inline suppression (`# glint: ignore`)

Suppress a finding for a specific job by placing a comment immediately before it:

```yaml
# glint: ignore GL007
legacy-job:
  only: [main]
  script: echo ok

# Multiple rules (comma- or space-separated):
# glint: ignore GL007, GL032
other-job:
  script: echo ok

# Suppress every rule for this job:
# glint: ignore all
noisy-job:
  script: echo ok
```

Suppressions are scoped to the single job they precede and do not affect other
jobs or pipeline-level findings.

---

## Graph visualization (`glint graph`)

| Mode | Output |
|------|--------|
| `tree` (default) | Terminal job tree: stages as branches, jobs as leaves; annotated with `[manual]`, `[delayed]`, `[trigger]` where applicable |
| `includes` | Mermaid flowchart to stdout; colour-coded nodes by include type (local, remote, project, component, template) |
| `pipeline` | GitLab CI-style SVG/PNG written to `--out` directory (default: `glint-out/`); converted to PNG when `rsvg-convert`, `inkscape`, or `magick` is available |
| `all` | `includes` to stdout + `pipeline` file path to stderr |

**`glint graph pipeline --format <FORMAT>`**

| Format | Effect |
|--------|--------|
| `svg` (default) | Write GitLab CI-style SVG/PNG to `--out` |
| `mermaid` | Print Mermaid flowchart to stdout (paste into [mermaid.live](https://mermaid.live)) |
| `html` | Write self-contained HTML to `--out` with mouse pan/zoom and a click-to-open job-detail sidebar |

**Context flags** (`--branch`, `--tag`, `--source`, `--var`, `--changes`, `--changes-from`) work on all modes and annotate or colour jobs based on their evaluated state. Use `--no-skipped` to remove jobs that would not run in the given context from all output entirely (tree, SVG, HTML, and Mermaid).

**Visual distinctions in SVG and HTML output:**

- **Regular** — blue circle with checkmark
- **Manual** — orange circle with play triangle
- **Trigger** — purple circle with chevron
- **Delayed** — yellow circle with clock
- **`when: on_failure`** — red circle (`#d9534f`) with X mark; dashed chip border
- **Skipped** (with context flags, without `--no-skipped`) — grey circle, dimmed job name

In DAG pipelines (any job has `needs:`) the pipeline graph uses job-to-job
Bézier connectors. In classic mode a bus-bar pattern (vertical rail + per-job
stubs) accurately connects every job across adjacent stages.

Each chip carries a `<title>` and `<desc>` with stage, when, image, and needs —
shown as a tooltip in SVG viewers and as a sidebar panel in HTML output.

---

## Developer tools

| Tool | Description |
|------|-------------|
| `glint render <PIPELINE>` | Resolve all includes and extends; write the fully merged pipeline to a single YAML file. See [Pipeline rendering](#pipeline-rendering-glint-render) above. |
| `glint explain <RULE>` | Print description, rationale, bad-YAML example, and fix for a rule. Case-insensitive (`gl007` = `GL007`). |
| `glint explain` | List all rules with ID, severity, and title. |
| `--no-warn` | (`glint check`) Discard all warning findings before output and exit-code calculation. |
| `--no-skipped` | (`glint graph`) Remove skipped jobs from graph output entirely. |
| `--list-vars` | (`glint check`, `glint graph`) Print all resolved pipeline variables to stderr before continuing. |
| `--version` / `-v` | Print the compiled version string. |
