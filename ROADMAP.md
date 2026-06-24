# Roadmap

This document tracks planned improvements to `glint`. Items are grouped by theme, roughly in priority order within each group. Nothing here is a commitment — the tool is experimental and the list will shift as real usage surfaces better priorities.

---

## Context simulation

Pass `--branch`, `--tag`, `--source`, or `--var` to `glint check` or `glint graph` to evaluate `rules:if:` expressions and `only`/`except` filters against a specific pipeline event.

- ~~**Single-context simulation**~~ — ✓ shipped v0.2.0; `--branch`, `--tag`, `--source`, `--var` flags on both subcommands; jobs classified as active / manual / skipped
- ~~**`workflow:rules:variables:` propagation**~~ — ✓ shipped post-v0.2.0; variables from the matching workflow rule entry injected into the evaluation context before job `rules:if:` expressions are evaluated
- ~~**Expression evaluator: multi-line `if:` values**~~ — ✓ shipped post-v0.2.0; newlines in block-scalar and folded YAML `if:` values treated as whitespace
- ~~**Expression evaluator: `${VAR}` syntax**~~ — ✓ shipped post-v0.2.0; `${CI_COMMIT_BRANCH}` equivalent to `$CI_COMMIT_BRANCH` everywhere
- ~~**Expression evaluator: regex flags**~~ — ✓ shipped post-v0.2.0; `/pattern/i`, `/pattern/m`, `/pattern/s` supported
- ~~**Expression evaluator: variable as regex RHS**~~ — ✓ shipped post-v0.2.0; `$BRANCH =~ $PATTERN` where `$PATTERN` holds a `/regex/` string evaluates correctly
- ~~**Expression evaluator: bare `true`/`false` and integer literals**~~ — ✓ shipped post-v0.2.0; `$FLAG == true`, `$COUNT == 4` compare as decimal strings matching GitLab CI behaviour
- ~~**Implicit default context**~~ — ✓ shipped v0.2.11; defaults to `--branch main --source push` when no context flag is given, so `rules:if:` is always evaluated
- ~~**`--list-vars` debug flag**~~ — ✓ shipped v0.2.11; prints sorted `KEY=VALUE` of all collected pipeline variables (root file + includes + workflow-rule union + effective context) to stderr
- ~~**Variable expansion**~~ — ✓ shipped v0.2.13; `$VAR`/`${VAR}` references within variable values expanded after all sources are merged; transitive chains resolved; visible in `--list-vars`
- ~~**Non-string scalar variables**~~ — ✓ shipped v0.2.13; `BUILD: true`, `RETRIES: 3` rendered and injected correctly instead of being silently dropped
- ~~**YAML `\/` escape in double-quoted strings**~~ — ✓ shipped v0.2.13; regex patterns like `/^us\//` in double-quoted `if:` blocks no longer cause a parse error
- ~~**Workflow rule strict evaluation**~~ — ✓ shipped v0.2.14; unparseable `if:` skips the rule instead of matching everything; prevents wrong variables being injected
- ~~**Single `=` operator**~~ — ✓ shipped v0.2.14; bare `=` accepted as alias for `==` in `rules:if:` expressions
- ~~**`rules:changes:` evaluation**~~ — ✓ shipped v0.2.21; `--changes PATH` and `--changes-from REF` flags; doublestar glob matching (`*` within segment, `**` across segments); permissive when no file list provided
- ~~**Multi-context simulation**~~ — ✓ shipped v0.2.22; `--context KEY=VALUE[,...]`; repeatable; prints a comparison table of `active`/`manual`/`skipped`/`blocked` per job across all contexts
- ~~**Context-scoped linting**~~ — ✓ shipped v0.2.23; jobs evaluated as `JobSkipped` in the supplied context are excluded from `needs:`/`dependencies:` cross-checks (GL027–GL031) to eliminate false-positive errors for conditionally-gated jobs

---

## Lint coverage

The current rule set covers the most common sources of broken pipelines. These are the gaps most likely to matter in practice.

- ~~**Variable reference validation (GL032)**~~ — ✓ shipped v0.2.11; warns when a `rules:if:` expression references `$VAR` / `${VAR}` not declared anywhere in pipeline YAML; predefined GitLab namespaces (`CI_*`, `GITLAB_*`, …) exempt; variables from included files are also considered
- ~~**`rules:if:` static reachability (GL033)**~~ — ✓ shipped v0.2.15; warns when every rule in a job's `rules:` block has `when: never`, making the job permanently excluded from any pipeline run; no `if:` evaluation required
- ~~**`services:` validation (GL034)**~~ — ✓ shipped v0.2.16; map form requires `name`; `alias` must be a valid DNS label
- ~~**`rules:changes` / `rules:exists` absolute path detection (GL035)**~~ — ✓ shipped v0.2.16; warns when a path starts with `/`; GitLab CI paths are always relative to the repo root
- ~~**`timeout` format validation (GL036)**~~ — ✓ shipped v0.2.16; validates job-level and `default.timeout` against recognised GitLab CI duration strings
- ~~**`id_tokens:` / `secrets:` required-key checks (GL037, GL038)**~~ — ✓ shipped v0.2.16; `id_tokens` entries must have `aud`; `secrets` entries must declare a provider
- ~~**`pages:publish` + `artifacts.paths` consistency (GL039)**~~ — ✓ shipped v0.2.16; warns when the publish directory is missing from `artifacts.paths`
- ~~**Duplicate stage names (GL040)**~~ — ✓ shipped v0.2.16; warns when a stage appears more than once in `stages:`
- ~~**`cache:key:files` must be exact paths (GL041)**~~ — ✓ shipped v0.2.16; warns when entries look like glob patterns
- ~~**Unreachable jobs**~~ — covered by GL033 (shipped v0.2.15); every-`when:never` rules block is statically dead
- ~~**`inherit:` completeness (GL043)**~~ — ✓ shipped v0.2.20; warns when `inherit: default:` is declared but there's no `default:` block, or list form names fields not set in `default:`

---

## Include resolution

- ~~**`include: local:`** full resolution~~ — ✓ shipped in v0.2.0; local files are read from disk, recursively resolved, and merged before linting
- ~~**`include: remote:`** (URL)~~ — ✓ shipped post-v0.2.0; plain HTTPS URLs are fetched (unauthenticated), parsed, and merged; sub-includes are resolved recursively; unreachable URLs emit `[WARNING]` and linting continues
- ~~**Recursive include depth limit**~~ — ✓ shipped v0.2.17; depth capped at 100 (matching GitLab); project/component includes now tracked in visited set to prevent cross-file cycles
- ~~**Offline mode / cache**~~ — ✓ shipped v0.2.17; `--cache-dir DIR` persists fetched templates; `--offline` serves from cache only; default cache dir (`~/.cache/glint`) used automatically with `--offline`
- ~~**`include: inputs:`**~~ — ✓ shipped v0.2.17; `$[[ inputs.KEY ]]` and `$[[ inputs.KEY | default(…) ]]` placeholders in fetched component YAML are substituted from the include's `with:` block before parsing

---

## Output formats — ✓ shipped v0.2.18

- ~~**JSON** (`--format json`)~~ — ✓ shipped v0.2.18; machine-readable findings with stable schema (version 1)
- ~~**SARIF** (`--format sarif`)~~ — ✓ shipped v0.2.18; SARIF 2.1.0; consumed natively by GitHub Code Scanning and GitLab SAST
- ~~**JUnit XML** (`--format junit`)~~ — ✓ shipped v0.2.18; lets CI pipelines publish lint results as a test report artifact
- ~~**GitHub annotation format** (`--format github`)~~ — ✓ shipped v0.2.18; emits `::error file=…,line=…,title=RULE::message` lines so findings appear as inline comments in PR diffs

---

## Pipeline graph improvements

The SVG renderer and terminal tree cover the basic layout. These would bring it closer to GitLab's full interactive view.

- ~~**Terminal job tree**~~ — ✓ shipped in v0.2.0 as `glint graph tree`; stages as branches, jobs as leaves, context-aware annotations
- ~~**`glint graph includes` shows jobs per file**~~ — ✓ shipped post-v0.2.0; each include node shows the jobs it defines as dashed-arrow rounded nodes in a distinct style
- **Multi-job connector accuracy** — draw one connector per job pair rather than one per stage pair in classic mode, so pipelines with uneven columns look correct
- **Job tooltip / detail panel** — embed a hidden `<title>` and `<desc>` per chip so SVG viewers show `stage`, `when`, `image`, and `needs` on hover
- **`when: on_failure` visual distinction** — dashed border or distinct icon for failure-path jobs
- **Blocked / skipped state colouring** — grey out jobs that are statically unreachable given known `rules:` conditions
- **Interactive HTML output** — self-contained `.html` file with pan/zoom and a job-detail sidebar; no external dependencies
- **Mermaid pipeline output** — keep `pipeline.go` but wire it up through `--graph pipeline --format mermaid` for users who want to paste into mermaid.live

---

## Findings quality — ✓ file and line numbers shipped post-v0.2.0; ruff-style format shipped v0.2.11

~~**File and line numbers on findings**~~ — ✓ shipped post-v0.2.0; every finding includes the source file and exact line of the job key. Works across local includes, remote project templates, and fetched component templates.

~~**Ruff-style output format**~~ — ✓ shipped v0.2.11; findings follow `file:line: RULEID [severity] message` matching the convention used by ruff and other modern linters.

**Remaining improvements**

- ~~**`needs: optional: true` false-positive errors**~~ — ✓ shipped post-v0.2.0; optional missing needs are downgraded to `[WARNING]`
- ~~**`extends:` jobs with missing script false errors**~~ — ✓ shipped post-v0.2.0; jobs using `extends:` that have no `script` after resolution emit `[WARNING]` (the script may come from an unfetchable remote base)
- ~~**`rules:if:` static reachability (GL042)**~~ — ✓ shipped v0.2.20; warns when all `rules:if:` conditions evaluate to false given declared variable values (only fires when all referenced vars are declared in YAML)

---

## CI / editor integration

- **GitLab CI template** — a `.gitlab-ci.yml` snippet that runs `glint` as a pipeline-validation job before the real pipeline executes; publishable to the GitLab CI/CD Catalog
- **GitHub Actions action** — `uses: k3nny/glint@v1` wrapper for repositories that mirror or manage GitLab pipelines from GitHub
- **Pre-commit hook** — entry for [pre-commit](https://pre-commit.com) so `glint` runs automatically on `git commit` when `.gitlab-ci.yml` changes
- **LSP server** — `glint lsp` mode exposing diagnostics over the Language Server Protocol; enables inline squiggles in VS Code, JetBrains, Neovim, etc. without a dedicated extension
- **VS Code extension** — thin wrapper around the LSP server with syntax highlighting for `.gitlab-ci.yml`

---

## Configuration — ✓ shipped v0.2.19

- ~~**`.glint.yml` config file**~~ — ✓ shipped v0.2.19; `ignore:`, `severity:`, `stages:`, `token:`, `url:`, `cache_dir:`; searched from the pipeline directory up to the git root
- ~~**Inline suppression comments**~~ — ✓ shipped v0.2.19; `# glint: ignore GL007` before a job definition; comma/space-separated rules; `# glint: ignore all` wildcard

---

## Reliability and developer experience

- ~~**Structured rule IDs**~~ — ✓ shipped post-v0.2.0; GL001–GL031 assigned; GL032 added v0.2.11; GL033 added v0.2.15; GL034–GL041 added v0.2.16; output formats (--format json/sarif/junit/github) added v0.2.18; GL042–GL043 added v0.2.20
- ~~**`glint explain <rule-id>`**~~ — ✓ shipped v0.2.20; prints rule description, rationale, bad-YAML example, and fix; `glint explain` (no arg) lists all rules
- ~~**Semantic versioning and first release**~~ — shipped as `v0.1.0` (2026-06-07)
- ~~**Subcommand CLI**~~ — shipped as `v0.2.0` (2026-06-11); `glint check` / `glint graph [mode]` with ruff-style `--help`
- **Changelog automation** — generate release notes from Conventional Commits via `git-cliff` or similar
- **Fuzz testing** — add a `go test -fuzz` target for the YAML parser to harden it against malformed input
