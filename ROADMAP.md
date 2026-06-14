# Roadmap

This document tracks planned improvements to `glint`. Items are grouped by theme, roughly in priority order within each group. Nothing here is a commitment — the tool is experimental and the list will shift as real usage surfaces better priorities.

---

## Context-aware validation — ✓ single-context shipped in v0.2.0; expression evaluator hardened post-v0.2.0; implicit defaults and --list-vars shipped v0.2.11; variable expansion and scalar handling shipped v0.2.13; workflow evaluation and output fixes shipped v0.2.14

Single-context simulation is fully implemented. Pass `--branch`, `--tag`, `--source`, or `--var` to either `glint check` or `glint graph`; jobs are evaluated and shown as active / manual / skipped.

```bash
# shipped: single-context simulation
glint check --branch develop .gitlab-ci.yml
glint check --tag v1.2.0 .gitlab-ci.yml
glint check --source merge_request_event --var CI_MERGE_REQUEST_TARGET_BRANCH_NAME=main .gitlab-ci.yml
glint graph tree --branch main .gitlab-ci.yml   # tree annotated with [skipped] / [manual]
```

**Shipped post-v0.2.0 (unreleased)**

- ✓ **`workflow:rules:variables:` propagation** — variables defined on the matching `workflow:rules:` entry are injected into the evaluation context before job `rules:if:` expressions are evaluated. Pipeline-level `variables:` defaults are also available. Priority chain (highest wins): `--var` > shortcuts > workflow-rule vars > pipeline defaults.
- ✓ **Expression evaluator: multi-line expressions** — newlines in block-scalar and folded YAML `if:` values are now treated as whitespace; `||` / `&&` on a continuation line evaluate correctly.
- ✓ **Expression evaluator: `${VAR}` curly-brace syntax** — `${CI_COMMIT_BRANCH}` is equivalent to `$CI_COMMIT_BRANCH` everywhere.
- ✓ **Expression evaluator: regex flags** — `/pattern/i`, `/pattern/m`, `/pattern/s` are now supported; `i` maps to `(?i)` in Go's regexp.
- ✓ **Expression evaluator: variable as regex RHS** — `$BRANCH =~ $PATTERN` where `$PATTERN` holds a `/regex/` string is evaluated correctly.
- ✓ **Expression evaluator: bare `true` / `false` keywords** — treated as the strings `"true"` / `"false"` matching GitLab CI's own behaviour; `$GATEWAY_ENABLED == true` now evaluates correctly.
- ✓ **Expression evaluator: integer literals** — `$COUNT == 4`, `$ENABLED == 1`, `$DISABLED == 0` compare as decimal strings.

~~**Implicit default context**~~ — ✓ shipped v0.2.11; `glint check` and `glint graph` default to `--branch main --source push` when no context flag is given, so `rules:if:` expressions are always evaluated out of the box.

~~**`--list-vars` debug flag**~~ — ✓ shipped v0.2.11; prints sorted `KEY=VALUE` of all collected variables (pipeline YAML + included files + workflow-rule union + effective context) to stderr.

**Shipped in v0.2.13**

- ✓ **Variable expansion** — `$VAR` / `${VAR}` references within variable values are expanded after all sources are merged; transitive chains resolve over multiple passes; visible in `--list-vars` effective-context output.
- ✓ **Non-string scalar variables** — `BUILD: true`, `RETRIES: 3` and similar bare boolean/integer values now render correctly in `--list-vars` and are injected into the evaluation context as string equivalents; previously shown as `(complex)` and silently dropped.
- ✓ **YAML `\/` escape in double-quoted strings** — regex patterns like `/^us\//` in double-quoted `if:` blocks no longer cause a parse error; the raw bytes are preprocessed before YAML unmarshalling.

**Shipped in v0.2.14**

- ✓ **Workflow rule strict evaluation** — workflow `rules:if:` now uses strict mode (parse failure → skip rule, not match); fixes premature matching that blocked later rules and injected wrong variables.
- ✓ **Single `=` operator** — `=` is now accepted as an alias for `==` in `rules:if:` expressions, matching common user intent.
- ✓ **Source location through `extends:` resolution** — `File` and `Line` are now preserved when a job is rebuilt via extends, so findings reference the correct source location.
- ✓ **Sorted findings output** — findings are sorted by `(File, Line, Rule)`; same-file issues group together in line order.
- ✓ **Consistent warning format** — all warnings use ruff-style `path: [warning] message` format.
- ✓ **`--version` / `-v` flag** — prints compiled version; version also shown at the top of every `--help` output.

**Remaining work**

- **Multi-context simulation** — run multiple contexts in one invocation and print a comparison table:
  ```bash
  glint check --context branch=main --context branch=develop --context tag=v1.0.0 .gitlab-ci.yml
  ```
- **Context-scoped linting** — skip `needs:`/`dependencies:` cross-checks for jobs that are statically unreachable in the given context
- **`rules:changes:` evaluation** — path glob evaluation against the local git tree

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
- **`inherit:` completeness** — flag when a job overrides a default field that would require `inherit: default: false` to suppress

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
- **`rules:if:` static reachability** — report when a job's entire `rules:` block can never evaluate to `when: on_success` given the declared pipeline variables (pure static, no context required)

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

- ~~**Structured rule IDs**~~ — ✓ shipped post-v0.2.0; GL001–GL031 assigned; GL032 added v0.2.11; GL033 added v0.2.15; GL034–GL041 added v0.2.16; output formats (--format json/sarif/junit/github) added v0.2.18
- **`--explain <rule-id>`** — print the rule description, rationale, and an example fix
- ~~**Semantic versioning and first release**~~ — shipped as `v0.1.0` (2026-06-07)
- ~~**Subcommand CLI**~~ — shipped as `v0.2.0` (2026-06-11); `glint check` / `glint graph [mode]` with ruff-style `--help`
- **Changelog automation** — generate release notes from Conventional Commits via `git-cliff` or similar
- **Fuzz testing** — add a `go test -fuzz` target for the YAML parser to harden it against malformed input
