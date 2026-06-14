# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org).

## [0.2.20] - 2026-06-14

### Added

- **`glint explain <RULE>`** — new subcommand that prints the description, rationale, a bad-YAML example, and the corrected fix for any rule ID (GL001–GL043). `glint explain` with no argument lists all rules with ID, severity, and title. Rule IDs are case-insensitive (`gl007` and `GL007` are equivalent).

- **`rules:if:` evaluated reachability (GL042)** — warns when every `rules:if:` condition in a job evaluates to false using the values of variables declared in the pipeline YAML, meaning the job can never be included in a pipeline run. The check is conservative: it only fires when all variables referenced in the `if:` expressions are explicitly declared in the YAML; predefined `CI_*` / `GITLAB_*` variables and undeclared variables are treated as unknown (may have any runtime value) to avoid false positives.

- **`inherit:` completeness (GL043)** — warns when a job declares `inherit: default:` (boolean or list form) but the pipeline has no `default:` block, making the declaration a no-op. Also warns when the list form names fields (e.g. `[image, before_script]`) that are not actually set in the `default:` block — those list entries have no effect.

## [0.2.19] - 2026-06-14

### Added

- **`.glint.yml` project config file** — glint now searches for a `.glint.yml` file starting from the pipeline file's directory and walking up to the first `.git` boundary. Supported keys:
  - `ignore: [GL007, GL032]` — suppress rules globally for the project.
  - `severity: {GL004: warning}` — override rule severity (`error`, `warning`, or `ignore`). `ignore` is equivalent to listing the rule in `ignore:`.
  - `stages: [quality]` — declare extra stage names that are valid beyond those in the pipeline's own `stages:` block; jobs in these stages are not flagged by GL004.
  - `token: glpat-xxx` — default GitLab personal access token (lower priority than the `--token` flag and `GITLAB_TOKEN` env var).
  - `url: https://gitlab.example.com` — default GitLab instance URL.
  - `cache_dir: ~/.cache/glint` — default cache directory for fetched remote includes.

- **Inline suppression comments** — a `# glint: ignore RULE` comment placed immediately before a job definition suppresses that rule for the specific job. Multiple rules can be comma- or space-separated (`# glint: ignore GL007, GL032`). Use `# glint: ignore all` to suppress every finding for the job. Suppressions are scoped to the annotated job; pipeline-level findings are unaffected.

## [0.2.18] - 2026-06-14

### Added

- **`--format json`** — `glint check` can now emit a structured JSON report instead of plain text. The schema (version `1`) includes `glint_version`, `pipeline`, a `findings` array (each finding has `rule`, `severity`, `file`, `line`, `job`, `message`), and a `summary` block (`total`, `errors`, `warnings`). An empty findings array is `[]`, not `null`. In this mode the human-readable summary line is written to stderr so stdout contains only the JSON payload.

- **`--format sarif`** — emits a SARIF 2.1.0 JSON document (schema `https://json.schemastore.org/sarif-2.1.0.json`). The `runs[0].tool.driver` lists every unique rule ID found in findings; each `result` carries `ruleId`, `level` (`error`/`warning`), `message.text` (including the job name prefix), and `locations[0].physicalLocation` with `artifactLocation.uri` and `region.startLine` (when available). Pipeline-level findings without a file have no `locations` entry. This format is consumed natively by GitHub Code Scanning and GitLab SAST.

- **`--format junit`** — emits a JUnit XML document compatible with CI test-report artifact parsers (GitLab: `artifacts:reports:junit`; GitHub: upload-artifact + test-reporter). Each finding becomes a `<testcase>` with a `<failure>` child whose `message` and `type` attributes carry the finding details. A clean pipeline produces a single passing `<testcase>` with no `<failure>` element.

- **`--format github`** — emits GitHub Actions workflow-command annotation lines (`::error file=…,line=…,title=RULE::message` / `::warning …`). GitHub CI renders these as inline comments on the relevant file in pull requests. Pipeline-level findings without a file omit the `file=` parameter.

- **Format validation** — an unknown `--format` value exits with code 2 and a helpful error message listing the valid formats.

## [0.2.17] - 2026-06-14

### Added

- **Recursive include depth limit** — `resolveIncludes` now accepts a `depth` counter and returns a warning when the nesting depth exceeds 100 (matching GitLab's documented limit). This guards against pathologically deep include chains.

- **Cycle detection for project and component includes** — `project:` and `component:` includes were not previously tracked in the `visited` map (only `local:` and `remote:` were). They are now registered before recursing into their sub-includes, preventing cross-file include cycles from causing infinite loops.

- **`include: inputs:` substitution** — when a `component:` include entry has a `with:` block, all `$[[ inputs.KEY ]]` and `$[[ inputs.KEY | default('…') ]]` placeholders in the fetched template YAML are substituted with the corresponding values before parsing. Supported default value forms: `'single quoted'`, `"double quoted"`, bare booleans (`true`/`false`), and bare integers. Missing keys without a default become empty strings. This replaces the previous behaviour of leaving `$[[…]]` tokens in the YAML, which caused false-positive lint findings.

- **Include cache (`--cache-dir`)** — pass `--cache-dir DIR` to `glint check` or `glint graph` to persist fetched remote templates (both `project:` and `component:` includes and `remote:` URLs) to a local directory. Cache entries are keyed by the SHA-256 of the full request coordinates (base URL + project + path + ref). The directory is created automatically on first use.

- **Offline mode (`--offline`)** — pass `--offline` to skip all network calls. Remote includes not present in the cache are surfaced as warnings (same UX as "no token"). When `--offline` is set without an explicit `--cache-dir`, the default platform cache directory (`$XDG_CACHE_HOME/glint` or `~/.cache/glint`) is used automatically.

## [0.2.16] - 2026-06-14

### Added

- **`services:` validation (GL034)** — the map form of a service entry requires a `name` key; emits an `ERROR` when absent. The optional `alias` field must be a valid DNS label (letters, digits, hyphens, and dots; must start and end with an alphanumeric character); invalid aliases emit an `ERROR`.

- **`rules:changes` / `rules:exists` absolute path detection (GL035)** — emits a `WARNING` when a path in `rules:changes:` or `rules:exists:` starts with `/`. GitLab CI evaluates these paths relative to the repository root, so absolute paths can never match. Applies to both the list form and the `{paths: …}` map form.

- **`timeout:` format validation (GL036)** — emits an `ERROR` when a job's `timeout:` (or the pipeline-level `default.timeout:`) is not a valid GitLab CI duration string. Valid formats: `30m`, `1h 30m`, `90 minutes`, `2 hours 30 minutes`, `1 day`, etc.

- **`id_tokens:` `aud` validation (GL037)** — emits an `ERROR` for each OIDC token entry in `id_tokens:` that is missing the required `aud` key. GitLab returns an API error at pipeline start when `aud` is absent.

- **`secrets:` provider validation (GL038)** — emits an `ERROR` for each secret entry in `secrets:` that does not declare a provider key (`vault`, `gcp_secret_manager`, or `azure_key_vault`).

- **`pages:` keyword + `artifacts.paths` consistency (GL039)** — emits a `WARNING` when a job uses the `pages:` keyword but `artifacts.paths` does not include the publish directory (default `public`, or the value of `pages.publish`). GitLab Pages will not deploy unless the publish directory is listed as an artifact.

- **Duplicate stage names (GL040)** — emits a `WARNING` when a stage name appears more than once in the top-level `stages:` list. GitLab silently merges duplicate stage entries, which can produce confusing pipeline ordering.

- **`cache.key.files` glob detection (GL041)** — emits a `WARNING` when an entry in `cache.key.files` contains glob metacharacters (`*`, `?`, `[`). The `files` field requires exact file paths; GitLab does not expand globs there.

## [0.2.15] - 2026-06-13

### Added

- **Static reachability check (GL033)** — warns when every rule in a job's `rules:` block has an explicit `when: never`, making the job permanently excluded from any pipeline run. This is a purely static claim: no matter which `if:` condition evaluates to true, the outcome is always "skip"; and if no rule matches, the implicit fallback is also skip. No expression evaluation or context is required. The finding is a `WARNING` (may be intentional as a "disabled job" pattern). Only jobs where every rule has the literal `when: never` value are flagged; rules with no `when:` (default `on_success`), `when: manual`, `when: always`, or `when: on_failure` are not.

## [0.2.14] - 2026-06-13

### Added

- **`--version` / `-v` flag** — `glint --version`, `glint -v`, and `glint version` all print the compiled version string (e.g. `glint v0.2.14`). The version is also shown at the top of every `--help` output (global, `check --help`, `graph --help`). The version is injected at build time via `-ldflags "-X main.version=..."` using `git describe --tags --always --dirty`.

- **Sorted findings output** — `Lint` now returns findings sorted by `(File, Line, Rule)`. All issues from the same source file appear together in ascending line order; pipeline-level findings with no file location sort first. Previously findings were emitted in map-iteration order (non-deterministic).

### Fixed

- **Warning format consistency** — include-resolution warnings, extends-chain warnings, and the workflow non-start warning now use the same ruff-style `path: [warning] message` format as lint findings instead of the old `[WARNING] …` prefix with no file context.

- **Workflow rule permissive evaluation** — workflow `rules:if:` expressions are now evaluated in strict mode: an expression that cannot be fully parsed returns `false` (skip this rule, try the next) instead of `true` (match everything). Previously, a complex or partially-unsupported condition on the first workflow rule would match every context, blocking all subsequent rules and injecting the wrong variables. Job rules retain permissive evaluation (`true` on parse failure) to avoid silently dropping jobs.

- **Single `=` operator in `rules:if:`** — a bare `=` not followed by `=` or `~` is now accepted as an alias for `==`. This is a common mistake in GitLab CI YAML; previously it caused a parse failure and triggered the permissive fallback.

- **Source location lost through `extends:` resolution** — when a job was resolved via `extends:`, the merged definition was re-encoded and re-decoded as a fresh `model.Job` struct, which does not carry `File` or `Line` (they are not YAML keys). Those fields are now explicitly copied back from the original job before replacing it in `p.Jobs`, so extended jobs report the correct source file and line number in findings.

## [0.2.13] - 2026-06-12

### Added

- **Variable expansion (`$VAR` / `${VAR}`)** — variable values that reference other variables are now expanded in the effective context after all sources are merged. Transitive chains (`A=$B`, `B=$C`) are resolved over up to ten passes; circular references are left as-is. The expanded values are visible in `--list-vars` output under "Effective context variables" and are used when evaluating `rules:if:` expressions.

- **Non-string scalar variables (`bool`, `int`, `float64`)** — variables declared with bare `true`/`false` or integer values (e.g. `BUILD: true`, `RETRIES: 3`) are now handled correctly in all variable processing paths. Previously they were rendered as `(complex)` in `--list-vars` output and silently dropped from the effective context; they now render and inject as their string equivalents (`"true"`, `"3"`), matching GitLab CI's own behaviour where all variable values are strings.

### Fixed

- **YAML `\/` escape in double-quoted strings** — regex patterns containing `\/` (escaped forward-slash) in double-quoted `if:` expressions (e.g. `$CI_COMMIT_BRANCH =~ /^us\//`) caused a YAML parse error (`found unknown escape character`) with `gopkg.in/yaml.v3`, which does not implement this YAML 1.2 escape. The parser now preprocesses the raw bytes before unmarshalling: inside double-quoted strings, `\/` is rewritten to `\\/`, which `yaml.v3` parses as a literal backslash followed by a slash — preserving the regex intent.

## [0.2.11] - 2026-06-12

### Added

- **Ruff-style finding output** — findings now follow the `file:line: RULEID [severity] message` format (e.g. `.gitlab-ci.yml:14: GL004 [error] job "deploy": stage "production" is not defined in 'stages'`), matching the output convention used by [ruff](https://docs.astral.sh/ruff/) and other modern linters. Job-scoped findings prefix the message with `job "name": `; pipeline-level findings omit the job prefix. Severity is lowercase inside brackets (`[error]`, `[warning]`).

- **Implicit default context (`--branch main --source push`)** — when `glint check` or `glint graph` is invoked without any of `--branch`, `--tag`, `--source`, or `--var`, the context now defaults to `--branch main --source push` so that `rules:if:` expressions are always evaluated. Previously the context was empty and no rule evaluation occurred. Any explicit context flag bypasses the defaults entirely.

- **`--list-vars` debug flag** — available on both `glint check` and `glint graph`; prints all pipeline-level variables collected from the root file and every included file (sorted `KEY=VALUE`) to stderr, then continues normally. When a context is active, also prints the effective merged variable set (pipeline defaults + workflow-rule variables + CLI flags). Useful for diagnosing GL032 false positives.

- **Included-file variables now visible to all lint rules** — `variables:` blocks declared in included files (local, remote, project, and component includes) are now merged into the pipeline's variable namespace before linting. This eliminates false-positive GL032 warnings for variables declared in shared CI templates. Root-pipeline variables take precedence over included-file variables when the same key appears in both (matching GitLab's own override behaviour).

- **Variable reference validation (GL032)** — glint now warns when a `rules:if:` expression references a variable (`$VAR` or `${VAR}`) that is not declared anywhere in the pipeline YAML: pipeline-level `variables:`, the job's own `variables:`, or any `workflow:rules:variables:` block. Predefined GitLab CI variable namespaces (`CI_*`, `GITLAB_*`, `FF_*`, `RUNNER_*`, `TRIGGER_*`, `CHAT_*`) are exempt. Because variables can also be set in GitLab CI/CD project settings (invisible to glint), the finding is a `[WARNING]` rather than an error. Each undeclared variable is reported at most once per job to keep the output concise.

- **Structured rule IDs** — every finding now carries a stable `GL###` identifier (e.g. `GL003`) that appears in the output alongside the location and message: `.gitlab-ci.yml:14: GL003 [error] job "deploy": missing required field 'script'`. IDs are assigned per check function across 31 rules (GL001–GL031) and are stable across versions. The `linter.Finding` struct exposes the ID as a `Rule string` field for programmatic consumers. The README lint rules table is updated with ID columns.

- **`workflow:rules:variables:` now propagate to job rule evaluation** — when a `workflow:rules:` entry matches, any `variables:` it defines are injected into the evaluation context so job `rules:if:` expressions can reference them. Pipeline-level `variables:` are also available as defaults (lower priority). Variable priority order, highest first: `--var` CLI overrides → `--branch`/`--tag`/`--source` shortcuts → workflow-rule variables → pipeline-level variable defaults. This means `$DEPLOY_TARGET == "production"` in a job rule correctly evaluates when a workflow rule sets `DEPLOY_TARGET: production` for the matching branch. The `glint graph tree` command benefits from the same enrichment.

- **`rules:if:` expression evaluator improvements** — six correctness fixes to the GitLab CI expression parser:
  - **Multi-line expressions** — newlines (`\n`, `\r`) are now treated as whitespace between tokens, so block-scalar `if:` values (e.g. `if: | ...`) and folded YAML scalars with `||`/`&&` on a continuation line are parsed correctly instead of falling back to permissive `true`.
  - **`${VAR}` curly-brace variable syntax** — `${CI_COMMIT_BRANCH}` is now equivalent to `$CI_COMMIT_BRANCH` everywhere a value is expected.
  - **Regex flags** — `/pattern/i`, `/pattern/m`, `/pattern/s` are now honoured; the `i` flag (case-insensitive) is translated to Go's `(?i)` prefix before compiling. Unknown flags are silently ignored.
  - **Variable as regex RHS** — `$BRANCH =~ $PATTERN` where `$PATTERN` holds a `/regex/[flags]` string is now evaluated by extracting and compiling the pattern from the variable's value; if the value is empty or does not look like a regex literal the expression falls back to permissive `true`.
  - **`true` / `false` keywords** — bare `true` and `false` (without quotes) are now recognised as the string values `"true"` and `"false"`, matching GitLab CI's own behaviour. `$GATEWAY_ENABLED == true` and `$FEATURE_FLAG == false` now evaluate correctly.
  - **Integer literals** — bare integers (e.g. `$PARALLEL == 4`, `$ENABLED == 1`, `$DISABLED == 0`) are now parsed as their decimal string representations and compared accordingly.

- **File and line numbers on findings** — every finding now includes the source file and line where the job is defined, e.g. `[ERROR] job "deploy" (src/deploy.yml:14): …`. For jobs that come from local or fetched includes the file reflects the include source. Pipeline-level findings (workflow rules, missing stages) reference the root pipeline file.

- **`glint graph includes` shows jobs per file** — each node in the Mermaid include dependency graph now shows the jobs defined directly in that file. Jobs are rendered as rounded nodes (`(name)`) in a distinct light-purple style, connected with dashed arrows (`-.->`) to distinguish ownership from the include hierarchy (solid `-->` arrows). The root pipeline file always shows its direct jobs; local and fetched project/component nodes show theirs when the file can be read.

### Fixed

- **`include: remote:` URL includes are now fetched and merged** — glint fetches plain HTTPS URLs in `include: remote:` entries (no authentication), parses the resulting YAML, merges its jobs into the pipeline, and recursively resolves any sub-includes the remote file itself declares. Unreachable or unparseable URLs emit a `[WARNING]` and lint continues on the rest of the pipeline. The `glint graph includes` command now expands remote nodes with their jobs and sub-include tree, matching the behaviour of local and project includes.

- **`needs: optional: true` downgraded to warning** — a `needs:` entry that carries `optional: true` and references a job not present in the pipeline now emits `[WARNING]` instead of `[ERROR]`. GitLab CI silently skips such dependencies at runtime (the job is absent when its include was not triggered), so the finding was a false positive. Non-optional missing needs remain errors. Optional missing deps are also excluded from the cycle-detection graph.

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
