# Roadmap

This document tracks planned improvements to `glint`. Items are grouped by theme, roughly in priority order within each group. Nothing here is a commitment — the tool is experimental and the list will shift as real usage surfaces better priorities.

---

## Context-aware validation

Pipelines in Git Flow, Trunk-Based Development, or any branching strategy are rarely uniform: jobs activate or skip based on `$CI_COMMIT_BRANCH`, `$CI_COMMIT_TAG`, `$CI_PIPELINE_SOURCE`, and similar runtime variables. Today `glint` validates structure but cannot tell which jobs are actually reachable for a given context.

The plan is to make the execution context injectable so the linter can evaluate `rules:if:` / `only` / `except` conditions and report per-context reachability.

**CLI surface**

```bash
# Push to develop
glint --branch develop .gitlab-ci.yml

# Tag push (v1.2.0) — sets CI_COMMIT_TAG and clears CI_COMMIT_BRANCH
glint --tag v1.2.0 .gitlab-ci.yml

# Merge request pipeline
glint --source merge_request_event \
           --var CI_MERGE_REQUEST_TARGET_BRANCH_NAME=main \
           .gitlab-ci.yml

# Explicit variable overrides for anything not covered by the shortcuts
glint --var CI_COMMIT_BRANCH=feat/my-feature \
           --var CI_ENVIRONMENT_NAME=staging \
           .gitlab-ci.yml

# Simulate multiple contexts in one run (print per-context job tables)
glint --context branch=main \
           --context branch=develop \
           --context tag=v1.0.0 \
           .gitlab-ci.yml
```

**What context injection enables**

- Each job is resolved to **active** / **manual** / **skipped** for the given context
- Warn when the entire pipeline would produce zero runnable jobs (common mistake when a `workflow:rules:` block is too restrictive)
- Lint only the active job subset — skip `needs:` / `dependencies:` cross-checks for jobs that never co-execute in that context
- `--context` multi-simulation: print a table showing which jobs activate per context, making it easy to audit Git Flow rules across branches and tags at once

**Expression evaluator scope**

GitLab's `rules:if:` expression language will be implemented incrementally:

| Priority | Operators / features |
|----------|----------------------|
| 1 (MVP)  | `==`, `!=`, `null` check, `&&`, `\|\|`, `!`, parentheses |
| 2        | Regex match `=~` / `!~` with `/pattern/` literals |
| 3        | `$CI_COMMIT_BRANCH =~ /^feat\//`, anchored patterns |
| 4        | `only: branches / tags / merge_requests` shorthand mapping |
| 5        | `changes:` path glob evaluation against a real or mock file tree |

Predefined variables populated automatically from `--branch` / `--tag` / `--source` shortcuts: `CI_COMMIT_BRANCH`, `CI_COMMIT_TAG`, `CI_COMMIT_REF_NAME`, `CI_COMMIT_REF_SLUG`, `CI_PIPELINE_SOURCE`, `CI_DEFAULT_BRANCH` (defaulting to `main`).

---

## Lint coverage

The current rule set covers the most common sources of broken pipelines. These are the gaps most likely to matter in practice.

- **Variable reference validation** — warn when a job references `$VAR` (or `${VAR}`) that is not declared anywhere in `variables:`, `default.variables`, or the job itself
- **`services:` validation** — map form requires `name`; `alias` must be a valid DNS label
- **`rules:changes` / `rules:exists`** — warn on glob patterns that can never match (e.g. absolute paths, double `**` on unsupported versions)
- **`timeout` format** — must be a duration string GitLab understands (`1h 30m`, `90 minutes`, etc.)
- **`id_tokens:` / `secrets:`** — presence and required-key checks
- **`pages:publish`** — validate that the path is consistent with `artifacts.paths`
- **`inherit:` completeness** — flag when a job overrides a default field that would require `inherit: default: false` to suppress
- **Unreachable jobs** — detect jobs that can never run because every `rules:` branch evaluates to `never` (static analysis only, no variable expansion)
- **Duplicate stage names** — GitLab silently merges them; warn to avoid confusion
- **`cache:key:files`** — must be a list of paths, not a glob

---

## Include resolution

- **`include: local:`** full resolution — parse and merge locally-referenced YAML files the same way remote project includes are handled; enables cross-file `extends:` and `needs:` validation for monorepo setups
- **`include: remote:`** (URL) — fetch and merge plain HTTP/HTTPS URLs (no auth required)
- **Recursive include depth limit** — guard against include cycles across files
- **Offline mode / cache** — persist fetched remote templates to a local cache directory; `--offline` flag to skip network calls and use only cached copies
- **`include: inputs:`** — substitute CI component input values into fetched templates before merging, so component-scoped jobs get their correct `stage:` and keyword values

---

## Output formats

Right now the only output is plain-text findings. Structured output enables integration with other tools.

- **JSON** (`--format json`) — machine-readable findings with file, job, severity, rule ID, and message; stable schema
- **SARIF** (`--format sarif`) — [Static Analysis Results Interchange Format](https://sarifweb.azurewebsites.net); consumed natively by GitHub Code Scanning and GitLab SAST
- **JUnit XML** (`--format junit`) — lets CI pipelines publish lint results as a test report artifact
- **GitHub / GitLab annotation format** — emit `::error file=…,line=…::message` lines so findings appear as inline comments in PR diffs

---

## Pipeline graph improvements

The SVG renderer covers the basic layout. These would bring it closer to GitLab's full interactive view.

- **Multi-job connector accuracy** — draw one connector per job pair rather than one per stage pair in classic mode, so pipelines with uneven columns look correct
- **Job tooltip / detail panel** — embed a hidden `<title>` and `<desc>` per chip so SVG viewers show `stage`, `when`, `image`, and `needs` on hover
- **`when: on_failure` visual distinction** — dashed border or distinct icon for failure-path jobs
- **Blocked / skipped state colouring** — grey out jobs that are statically unreachable given known `rules:` conditions
- **Interactive HTML output** — self-contained `.html` file with pan/zoom and a job-detail sidebar; no external dependencies
- **Mermaid pipeline output** — keep `pipeline.go` but wire it up through `--graph pipeline --format mermaid` for users who want to paste into mermaid.live

---

## CI / editor integration

- **GitLab CI template** — a `.gitlab-ci.yml` snippet that runs `glint` as a pipeline-validation job before the real pipeline executes; publishable to the GitLab CI/CD Catalog
- **GitHub Actions action** — `uses: k3nny/glint@v1` wrapper for repositories that mirror or manage GitLab pipelines from GitHub
- **Pre-commit hook** — entry for [pre-commit](https://pre-commit.com) so `glint` runs automatically on `git commit` when `.gitlab-ci.yml` changes
- **LSP server** — `glint lsp` mode exposing diagnostics over the Language Server Protocol; enables inline squiggles in VS Code, JetBrains, Neovim, etc. without a dedicated extension
- **VS Code extension** — thin wrapper around the LSP server with syntax highlighting for `.gitlab-ci.yml`

---

## Configuration

- **`.glint.yml` config file** — project-level configuration for:
  - Rule suppression by rule ID (e.g. `ignore: [no-only, missing-stages]`)
  - Severity overrides (demote specific errors to warnings)
  - Custom `stages` allowlist for projects that use a non-standard default set
  - Token and URL defaults so flags are not needed in every invocation
- **Inline suppression comments** — `# glint: ignore next-line <rule-id>` in the pipeline YAML

---

## Reliability and developer experience

- **Structured rule IDs** — assign a stable short ID to every rule (e.g. `GS001`) so suppression, documentation, and SARIF output are stable across versions
- **`--explain <rule-id>`** — print the rule description, rationale, and an example fix
- ~~**Semantic versioning and first release**~~ — shipped as `v0.1.0` (2026-06-07)
- **Changelog automation** — generate release notes from Conventional Commits via `git-cliff` or similar
- **Fuzz testing** — add a `go test -fuzz` target for the YAML parser to harden it against malformed input
