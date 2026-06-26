# Roadmap

This document tracks planned improvements to `glint`. Items are grouped by theme, roughly in priority order within each group. Nothing here is a commitment — the tool is experimental and the list will shift as real usage surfaces better priorities.

---

## Context simulation

Pass `--branch`, `--tag`, `--source`, or `--var` to `glint check` or `glint graph` to evaluate `rules:if:` expressions and `only`/`except` filters against a specific pipeline event.

- [x] **Single-context simulation** — shipped v0.2.0; `--branch`, `--tag`, `--source`, `--var` flags on both subcommands; jobs classified as active / manual / skipped
- [x] **`workflow:rules:variables:` propagation** — shipped post-v0.2.0; variables from the matching workflow rule entry injected into the evaluation context before job `rules:if:` expressions are evaluated
- [x] **Expression evaluator: multi-line `if:` values** — shipped post-v0.2.0; newlines in block-scalar and folded YAML `if:` values treated as whitespace
- [x] **Expression evaluator: `${VAR}` syntax** — shipped post-v0.2.0; `${CI_COMMIT_BRANCH}` equivalent to `$CI_COMMIT_BRANCH` everywhere
- [x] **Expression evaluator: regex flags** — shipped post-v0.2.0; `/pattern/i`, `/pattern/m`, `/pattern/s` supported
- [x] **Expression evaluator: variable as regex RHS** — shipped post-v0.2.0; `$BRANCH =~ $PATTERN` where `$PATTERN` holds a `/regex/` string evaluates correctly
- [x] **Expression evaluator: bare `true`/`false` and integer literals** — shipped post-v0.2.0; `$FLAG == true`, `$COUNT == 4` compare as decimal strings matching GitLab CI behaviour
- [x] **Implicit default context** — shipped v0.2.11; defaults to `--branch main --source push` when no context flag is given, so `rules:if:` is always evaluated
- [x] **`--list-vars` debug flag** — shipped v0.2.11; prints sorted `KEY=VALUE` of all collected pipeline variables (root file + includes + workflow-rule union + effective context) to stderr
- [x] **Variable expansion** — shipped v0.2.13; `$VAR`/`${VAR}` references within variable values expanded after all sources are merged; transitive chains resolved; visible in `--list-vars`
- [x] **Non-string scalar variables** — shipped v0.2.13; `BUILD: true`, `RETRIES: 3` rendered and injected correctly instead of being silently dropped
- [x] **YAML `\/` escape in double-quoted strings** — shipped v0.2.13; regex patterns like `/^us\//` in double-quoted `if:` blocks no longer cause a parse error
- [x] **Workflow rule strict evaluation** — shipped v0.2.14; unparseable `if:` skips the rule instead of matching everything; prevents wrong variables being injected
- [x] **Single `=` operator** — shipped v0.2.14; bare `=` accepted as alias for `==` in `rules:if:` expressions
- [x] **`rules:changes:` evaluation** — shipped v0.2.21; `--changes PATH` and `--changes-from REF` flags; doublestar glob matching (`*` within segment, `**` across segments); permissive when no file list provided
- [x] **Multi-context simulation** — shipped v0.2.22; `--context KEY=VALUE[,...]`; repeatable; prints a comparison table of `active`/`manual`/`skipped`/`blocked` per job across all contexts
- [x] **Context-scoped linting** — shipped v0.2.23; jobs evaluated as `JobSkipped` in the supplied context are excluded from `needs:`/`dependencies:` cross-checks (GL027–GL031) to eliminate false-positive errors for conditionally-gated jobs

---

## Lint coverage

The current rule set covers the most common sources of broken pipelines. These are the gaps most likely to matter in practice.

- [x] **Variable reference validation (GL032)** — shipped v0.2.11; warns when a `rules:if:` expression references `$VAR` / `${VAR}` not declared anywhere in pipeline YAML; predefined GitLab namespaces (`CI_*`, `GITLAB_*`, …) exempt; variables from included files are also considered
- [x] **`rules:if:` static reachability (GL033)** — shipped v0.2.15; warns when every rule in a job's `rules:` block has `when: never`, making the job permanently excluded from any pipeline run; no `if:` evaluation required
- [x] **`services:` validation (GL034)** — shipped v0.2.16; map form requires `name`; `alias` must be a valid DNS label
- [x] **`rules:changes` / `rules:exists` absolute path detection (GL035)** — shipped v0.2.16; warns when a path starts with `/`; GitLab CI paths are always relative to the repo root
- [x] **`timeout` format validation (GL036)** — shipped v0.2.16; validates job-level and `default.timeout` against recognised GitLab CI duration strings
- [x] **`id_tokens:` / `secrets:` required-key checks (GL037, GL038)** — shipped v0.2.16; `id_tokens` entries must have `aud`; `secrets` entries must declare a provider
- [x] **`pages:publish` + `artifacts.paths` consistency (GL039)** — shipped v0.2.16; warns when the publish directory is missing from `artifacts.paths`
- [x] **Duplicate stage names (GL040)** — shipped v0.2.16; warns when a stage appears more than once in `stages:`
- [x] **`cache:key:files` must be exact paths (GL041)** — shipped v0.2.16; warns when entries look like glob patterns
- [x] **Unreachable jobs** — covered by GL033 (shipped v0.2.15); every-`when:never` rules block is statically dead
- [x] **`inherit:` completeness (GL043)** — shipped v0.2.20; warns when `inherit: default:` is declared but there's no `default:` block, or list form names fields not set in `default:`

---

## Include resolution

- [x] **`include: local:` full resolution** — shipped v0.2.0; local files are read from disk, recursively resolved, and merged before linting
- [x] **`include: remote:` (URL)** — shipped post-v0.2.0; plain HTTPS URLs are fetched (unauthenticated), parsed, and merged; sub-includes are resolved recursively; unreachable URLs emit `[WARNING]` and linting continues
- [x] **Recursive include depth limit** — shipped v0.2.17; depth capped at 100 (matching GitLab); project/component includes now tracked in visited set to prevent cross-file cycles
- [x] **Offline mode / cache** — shipped v0.2.17; `--cache-dir DIR` persists fetched templates; `--offline` serves from cache only; default cache dir (`~/.cache/glint`) used automatically with `--offline`
- [x] **`include: inputs:`** — shipped v0.2.17; `$[[ inputs.KEY ]]` and `$[[ inputs.KEY | default(…) ]]` placeholders in fetched component YAML are substituted from the include's `with:` block before parsing

---

## Output formats

- [x] **JSON** (`--format json`) — shipped v0.2.18; machine-readable findings with stable schema (version 1)
- [x] **SARIF** (`--format sarif`) — shipped v0.2.18; SARIF 2.1.0; consumed natively by GitHub Code Scanning and GitLab SAST
- [x] **JUnit XML** (`--format junit`) — shipped v0.2.18; lets CI pipelines publish lint results as a test report artifact
- [x] **GitHub annotation format** (`--format github`) — shipped v0.2.18; emits `::error file=…,line=…,title=RULE::message` lines so findings appear as inline comments in PR diffs

---

## Pipeline graph improvements

The SVG renderer and terminal tree cover the basic layout. These would bring it closer to GitLab's full interactive view.

- [x] **Terminal job tree** — shipped v0.2.0 as `glint graph tree`; stages as branches, jobs as leaves, context-aware annotations
- [x] **`glint graph includes` shows jobs per file** — shipped post-v0.2.0; each include node shows the jobs it defines as dashed-arrow rounded nodes in a distinct style
- [x] **Multi-job connector accuracy** — shipped v0.2.25; classic mode uses a bus-bar pattern (vertical rail at the midpoint + horizontal stubs per job) instead of a single center-to-center line, so uneven columns look correct
- [x] **Job tooltip / detail panel** — shipped v0.2.25; each chip is wrapped in `<g data-job="…"><title>…</title><desc>…</desc>` — SVG viewers show stage, when, image, and needs on hover; HTML output uses the data for the sidebar
- [x] **`when: on_failure` visual distinction** — shipped v0.2.25; dashed chip border + X-mark icon + red circle (`#d9534f`); legend entry added; Mermaid `on_failure` class wired
- [x] **Blocked / skipped state colouring** — shipped v0.2.25; `glint graph pipeline` accepts context flags (`--branch`, `--tag`, etc.); jobs evaluated as skipped are greyed out (`#868686`) with dimmed text and no icon
- [x] **Interactive HTML output** — shipped v0.2.25; `glint graph pipeline --format html` writes a self-contained `.html` file with mouse pan/zoom and a click-to-open job-detail sidebar; no external dependencies
- [x] **Mermaid pipeline output** — shipped v0.2.25; `glint graph pipeline --format mermaid` prints a Mermaid flowchart to stdout (paste into mermaid.live)
- [x] **Same-stage job ordering** — shipped v0.2.26; jobs within a stage that have `needs:` between each other are placed in topological sub-columns (left-to-right by depth); stage header spans all sub-columns
- [x] **Graph links rendered behind job chips** — shipped v0.2.26; SVG connectors (Bézier curves and bus-bar stubs) are drawn before job chips so lines pass behind rectangles

---

## Findings quality

- [x] **File and line numbers on findings** — shipped post-v0.2.0; every finding includes the source file and exact line of the job key; works across local includes, remote project templates, and fetched component templates
- [x] **Ruff-style output format** — shipped v0.2.11; findings follow `file:line: RULEID [severity] message` matching the convention used by ruff and other modern linters
- [x] **`needs: optional: true` false-positive errors** — shipped post-v0.2.0; optional missing needs are downgraded to `[WARNING]`
- [x] **`extends:` jobs with missing script false errors** — shipped post-v0.2.0; jobs using `extends:` that have no `script` after resolution emit `[WARNING]` (the script may come from an unfetchable remote base)
- [x] **`rules:if:` static reachability (GL042)** — shipped v0.2.20; warns when all `rules:if:` conditions evaluate to false given declared variable values (only fires when all referenced vars are declared in YAML)

---

## CI / editor integration

- [x] **GitLab CI template** — shipped v0.2.28; `templates/check.yml` is a GitLab CI/CD Catalog component with `spec:` inputs for stage, file, version, allow_failure, and extra args; also usable as a plain local/remote include
- [x] **GitHub Actions action** — shipped v0.2.28; `action.yml` composite action downloads the glint Linux binary and runs `glint check`; mirror to GitHub as `k3nny/glint` to reference as `uses: k3nny/glint@v0.2.28`
- [x] **Pre-commit hook** — shipped v0.2.28; `.pre-commit-hooks.yaml` defines `language: golang` hook; pre-commit builds glint from source on first run and re-runs on staged `.gitlab-ci.yml` changes
- [x] **LSP server** — shipped v0.2.29; `glint lsp` runs a JSON-RPC 2.0 LSP server over stdin/stdout; `textDocument/didOpen`, `didChange`, `didSave`, `didClose` all publish diagnostics; rule IDs appear as the diagnostic `code`; include resolution is best-effort using env-var token and default cache dir
- [x] **VS Code extension** — shipped v0.2.30; `editors/vscode/` TypeScript extension wraps `glint lsp` via `vscode-languageclient`; activates on YAML files and restricts LSP processing to `**/.gitlab-ci.yml`; configurable binary path via `glint.executablePath`; build with `task ext-compile`, package with `task ext-package`

---

## Configuration

- [x] **`.glint.yml` config file** — shipped v0.2.19; `ignore:`, `severity:`, `stages:`, `token:`, `url:`, `cache_dir:`; searched from the pipeline directory up to the git root
- [x] **Inline suppression comments** — shipped v0.2.19; `# glint: ignore GL007` before a job definition; comma/space-separated rules; `# glint: ignore all` wildcard
- [x] **Proxy support** — shipped v0.2.31; `--proxy` flag on `check`, `graph`, and `lsp` subcommands; also configurable via `proxy:` in `.glint.yml`; overrides `HTTP_PROXY` / `HTTPS_PROXY` env vars when set

---

## Security & hardening

- [x] **Path traversal guard for local includes** — shipped v0.2.31; `include: local:` paths containing `../../` or similar sequences are rejected instead of reading files outside the repository root
- [x] **HTTP timeout on remote fetches** — shipped v0.2.31; all HTTP calls via the fetcher use a 30-second timeout to prevent indefinite hangs on slow or unresponsive servers
- [x] **Unbounded response size cap** — shipped v0.2.31; remote include and GitLab API responses are capped at 10 MiB; responses exceeding the limit are rejected to prevent memory exhaustion
- [x] **Cache file permissions** — shipped v0.2.31; cache directories are created with mode `0700` and cache files with mode `0600`; previously used world-readable `0755`/`0644`
- [x] **GL045: HTTP remote include warning** — shipped v0.2.31; warns when `include: remote:` uses a plain `http://` URL; CI templates fetched unencrypted are at risk of tampering
- [x] **LSP Content-Length DoS cap** — shipped v0.2.31; `glint lsp` rejects messages with `Content-Length` exceeding 64 MiB to prevent memory exhaustion from a malicious client

---

## Reliability and developer experience

- [x] **Structured rule IDs** — shipped post-v0.2.0; GL001–GL031 assigned; GL032 added v0.2.11; GL033 added v0.2.15; GL034–GL041 added v0.2.16; output formats added v0.2.18; GL042–GL043 added v0.2.20; GL044 added v0.2.24; graph improvements shipped v0.2.25–v0.2.26
- [x] **`glint explain <rule-id>`** — shipped v0.2.20; prints rule description, rationale, bad-YAML example, and fix; `glint explain` (no arg) lists all rules
- [x] **Semantic versioning and first release** — shipped v0.1.0 (2026-06-07)
- [x] **Subcommand CLI** — shipped v0.2.0 (2026-06-11); `glint check` / `glint graph [mode]` with ruff-style `--help`
- [x] **Changelog automation** — shipped v0.2.27; `cliff.toml` configures git-cliff to produce Keep-a-Changelog–compatible release notes from Conventional Commits; `task changelog` regenerates `CHANGELOG.md`, `task changelog-next` previews unreleased entries
- [x] **Fuzz testing** — shipped v0.2.27; `FuzzParseBytes` and `FuzzSanitizeYAMLEscapes` in `internal/model/fuzz_test.go`; seeds run as regular tests in CI; `task fuzz` runs them continuously (default 30 s)
