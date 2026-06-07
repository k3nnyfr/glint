# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`gitlab-sim` is a CLI tool that validates and lints `.gitlab-ci.yml` pipelines locally, without a GitLab server. It resolves `extends:` inheritance, fetches remote `include: project:` templates and `include: component:` catalog entries, then runs a set of lint rules over the fully-merged pipeline.

**Goals:** catch misconfigured pipelines early in a developer's workflow, before pushing to GitLab. Eventual goal: produce a visual graph of the pipeline DAG.

**Non-goals:** full GitLab CI emulation, job execution, secret resolution, or runner management.

---

## Architecture

### Data flow

```
model.Parse(path)               // two-pass YAML → *Pipeline
  └─ resolver.ResolveIncludes   // fetch project: / component: includes, merge
       └─ resolver.Resolve       // resolve extends: inheritance chains
            └─ linter.Lint       // run all lint rules → []Finding
```

This order is intentional and must be preserved: includes must be merged before `extends:` resolution so that jobs defined in remote templates are available as base templates; `extends:` must be resolved before linting so that derived jobs carry their full merged definition.

### Package responsibilities

| Package | Role |
|---|---|
| `internal/model` | Data structures (`Pipeline`, `Job`, `Rule`, …) and YAML parser |
| `internal/linter` | Lint rules; each rule returns `[]Finding` |
| `internal/resolver` | `extends:` resolution and `include:` merging |
| `internal/fetcher` | GitLab REST API client (token auth, file fetch) |
| `internal/graph` | Mermaid graph generators (include dependencies, pipeline jobs) |
| `cmd/gitlab-sim` | CLI entrypoint, flag parsing, output formatting |

### Two-pass YAML parser (`internal/model/parser.go`)

`ParseBytes` runs two `yaml.Unmarshal` passes over the same input:

1. **Raw pass** — into `map[string]yaml.Node` to collect every top-level key without type assumptions.
2. **Typed pass** — into `*Pipeline` to decode reserved keys (`stages`, `variables`, `default`, `include`, `workflow`, `spec`) into typed structs.

Keys that survive the raw pass and are not in `ReservedKeys` are decoded individually as `Job` structs. This approach handles the open-ended job-name namespace without requiring a catch-all map in the typed struct.

**When adding a new top-level reserved key:** add it to `model.ReservedKeys` and add the corresponding typed field to `Pipeline`.

### `any` fields in `Job`

Many `Job` fields (e.g. `Artifacts`, `Cache`, `Image`, `Trigger`) are typed as `any` because GitLab CI allows both scalar and map forms. The linter type-asserts these at check time. Follow the existing pattern in `internal/linter/keywords.go`:

```go
switch v := job.Artifacts.(type) {
case map[string]any:
    // map form
case nil:
    // not set
}
```

Never change an `any` field to a concrete struct type unless the GitLab CI spec guarantees only one form — doing so will silently drop the other form during YAML decode.

---

## Adding a New Lint Rule

1. Add the check function to the appropriate file in `internal/linter/`:
   - **Per-job keyword constraint** → `keywords.go`, called from `checkJobKeywords`
   - **Cross-job graph rule** (needs, deps, extends) → dedicated file (`needs.go`, `dependencies.go`)
   - **Pipeline-level rule** → `linter.go`, called from `Lint`

2. Return `[]Finding` with the correct `Severity` (`Error` or `Warning`) and a `Job` field set when the finding is job-scoped, empty for pipeline-level.

3. Add a testdata fixture that triggers the new rule, and add it to the `validate` task in `Taskfile.yml`.

4. Document the rule in the lint rules table in `README.md`.

### Finding severity guide

| Severity | Use when |
|---|---|
| `Error` | The pipeline will definitely fail or behave incorrectly |
| `Warning` | Deprecated usage, best-practice deviation, or a condition that *may* be wrong |

---

## Adding a New Model Field

1. Add the field to `model.Job` with the correct `yaml:` tag.
2. If the field has constrained values, add a validity map to `internal/linter/keywords.go` (e.g. `validJobWhen`).
3. Add the check to `checkJobKeywords` and call it from there.
4. Add both a valid and an invalid fixture case to `testdata/keywords_valid.yml` / `testdata/keywords_invalid.yml`.

---

## Go Code Style

- **Format:** `gofmt` (enforced via `go vet` in the `ci` task). Never commit unformatted code.
- **Imports:** group stdlib / external / internal with a blank line between groups.
- **Errors:** always wrap with context using `fmt.Errorf("doing X: %w", err)`. Use `%w` (not `%s`) to preserve the error chain.
- **No panics** in library code (`internal/`). Panics are acceptable only in `main()` for truly unrecoverable startup failures — prefer returning errors.
- **No global mutable state.** All configuration flows through function parameters or structs (see `fetcher.GitLabConfig`).
- **Exported symbols** must have a doc comment. Unexported helpers do not need one unless the logic is non-obvious.
- **Table-driven tests** are the default style for unit tests. Group cases in a `[]struct{ name, input, want }` slice and range over them.
- **Dependencies:** prefer the standard library. The only current external dependency is `gopkg.in/yaml.v3`; keep it that way unless there is a compelling reason.
- `go.sum` must always be committed alongside `go.mod`.

---

## Commit Message Guidelines

**IMPORTANT: This project uses [Conventional Commits](https://www.conventionalcommits.org/) format.**

All commit messages must follow this format:
```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to build process or auxiliary tools
- `perf`: Performance improvements
- `style`: Code style changes (formatting, missing semicolons, etc.)

**Scopes:**
- `linter`: changes to lint rules (`internal/linter/`)
- `model`: data structures or YAML parser (`internal/model/`)
- `resolver`: extends/include resolution (`internal/resolver/`)
- `fetcher`: GitLab API client (`internal/fetcher/`)
- `graph`: Mermaid graph generators (`internal/graph/`)
- `cli`: CLI entrypoint (`cmd/gitlab-sim/`)
- `testdata`: fixture files only
- `docs`: README, CHANGELOG, or other documentation
- `build`: Taskfile, go.mod, go.sum, CI config
- `claude`: CLAUDE.md changes

**Breaking Changes:**
Add `!` after type/scope for breaking changes (e.g. changed CLI flags, removed output fields):
```
feat(cli)!: rename --token to --api-token
```

**Note:** Always include a scope in parentheses, even for documentation changes.

---

## Development Workflow

```bash
task ci          # full check: vet → test → build → validate (run before every commit)
task build       # compile the binary
task test        # run Go unit tests
task lint-go     # go vet only
task validate    # run the binary against all testdata fixtures
task clean       # remove build artifacts
```

`task ci` must pass before any commit is created.

### Testdata fixtures

Every fixture in `testdata/` is run by `task validate`. Files whose expected behaviour is *clean* (exit 0) are listed with `ignore_error: false`; files that are expected to produce errors (exit 1) use `ignore_error: true`. Keep both categories; do not use `ignore_error: true` for a fixture that is supposed to be clean.

---

## Environment Variables (for manual testing)

| Variable | Purpose |
|---|---|
| `GITLAB_TOKEN` | Personal access token (`read_api` scope) |
| `CI_JOB_TOKEN` | CI/CD job token (when running inside a pipeline) |
| `GITLAB_PRIVATE_TOKEN` | Legacy PAT name (lowest priority) |
| `CI_SERVER_URL` | GitLab instance URL (takes precedence over `GITLAB_URL`) |
| `GITLAB_URL` | GitLab instance URL fallback |

Token resolution order (first non-empty wins): `GITLAB_TOKEN` → `CI_JOB_TOKEN` → `GITLAB_PRIVATE_TOKEN`.
URL resolution order: `--gitlab-url` flag → `CI_SERVER_URL` → `GITLAB_URL` → `https://gitlab.com`.
