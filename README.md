# gitlab-sim

A local tool to validate and lint `.gitlab-ci.yml` pipelines without needing a GitLab server.

## Features

- **YAML validation** — detects malformed pipeline files early
- **Stage validation** — every job's `stage` must be declared in `stages`
- **`extends:` resolution** — resolves single and multi-level template inheritance before linting, so derived jobs are evaluated against their fully merged definition
- **`needs:` DAG validation** — checks that `needs:` references exist, respect stage ordering, and contain no circular dependencies
- **Deprecation warnings** — flags `only`/`except` usage in favour of `rules`

## Requirements

- Go 1.21 or later
- [Task](https://taskfile.dev) (optional, for development tasks)

## Installation

```bash
git clone https://git.k3nny.fr/gitlab-sim
cd gitlab-sim
go build -o gitlab-sim ./cmd/gitlab-sim/...
```

Or with Task:

```bash
task build
```

## Usage

```bash
gitlab-sim <pipeline.yml>
```

Exits `0` when no errors are found, `1` when at least one error is reported.

### Example output

```
# Clean pipeline
OK: .gitlab-ci.yml — no issues found (5 jobs, 3 stages)

# Pipeline with issues
[ERROR] job "deploy": stage "production" is not defined in 'stages'
[ERROR] job "test": needs unknown job "build-app"
[WARNING] job "old-job": 'only'/'except' are deprecated; prefer 'rules'

3 finding(s): 2 error(s)
```

## Lint rules

### Pipeline-level

| Severity | Rule |
|----------|------|
| ERROR | `workflow.rules[*].when` is not `always` or `never` |
| WARNING | No `stages` defined (GitLab falls back to default stages) |

### Job-level — structure

| Severity | Rule |
|----------|------|
| ERROR | Job is missing required `script` (or `run`) — non-trigger, non-template jobs |
| ERROR | Job references a `stage` not declared in `stages` |
| ERROR | `only` and `rules` used together on the same job |
| ERROR | `except` and `rules` used together on the same job |
| WARNING | `only`/`except` used (deprecated, prefer `rules`) |

### Job-level — keyword constraints

| Severity | Rule |
|----------|------|
| ERROR | `when` is not one of `on_success`, `on_failure`, `always`, `manual`, `delayed`, `never` |
| ERROR | `when: delayed` without `start_in` |
| ERROR | `start_in` set but `when` is not `delayed` |
| ERROR | `parallel` integer not in range 2–200 |
| ERROR | `parallel` map form missing `matrix` key |
| ERROR | `retry` integer not in range 0–2 |
| ERROR | `retry.max` not in range 0–2 |
| ERROR | `retry.when` contains an invalid failure type |
| ERROR | `allow_failure` is not a boolean or a map with `exit_codes` |
| ERROR | `interruptible` is not a boolean |
| ERROR | `trigger` job also has `script` |
| ERROR | `trigger` map missing `project` or `include` |
| ERROR | `coverage` is not a regex pattern wrapped in `/` |
| ERROR | `release` missing required `tag_name` |
| ERROR | `environment.url` set without `environment.name` |
| ERROR | `environment.action` is not one of `start`, `stop`, `prepare`, `verify`, `access` |
| ERROR | `artifacts.when` is not `on_success`, `on_failure`, or `always` |
| ERROR | `artifacts.expose_as` set without `artifacts.paths` |
| ERROR | `cache.when` is not `on_success`, `on_failure`, or `always` |
| ERROR | `cache.policy` is not `pull`, `push`, or `pull-push` |
| ERROR | `rules[*].when` is not one of the valid `when` values |
| ERROR | `image` map form missing `name` key |
| ERROR | `inherit.default` / `inherit.variables` is not a boolean or list |
| WARNING | `pages` job `artifacts.paths` does not include `public` |

### Cross-job graph

| Severity | Rule |
|----------|------|
| ERROR | `needs:` references a job that does not exist |
| ERROR | `needs:` references a job in a later stage |
| ERROR | Circular dependency detected in `needs:` graph |
| ERROR | `dependencies:` references a job that does not exist |
| ERROR | `dependencies:` references a job in the same or a later stage |
| ERROR | `extends:` references an unknown job |
| ERROR | Cycle detected in `extends:` graph |

### Hidden jobs (templates)

Jobs whose name starts with `.` are treated as reusable templates and skipped for most rules. This matches GitLab's own behaviour.

## Development

This project uses [Task](https://taskfile.dev) as a task runner.

```bash
task              # list available tasks
task build        # compile the binary
task test         # run Go unit tests
task lint-go      # run go vet
task validate     # run the binary against all testdata fixtures
task ci           # full check: vet → test → build → validate
task clean        # remove build artifacts
```

## Project structure

```
.
├── cmd/gitlab-sim/     # CLI entrypoint
├── internal/
│   ├── linter/         # lint rules and findings
│   ├── model/          # pipeline data structures and YAML parser
│   └── resolver/       # extends: resolution
├── testdata/           # sample pipelines used for manual validation
├── Taskfile.yml
└── go.mod
```
