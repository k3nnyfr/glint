<p align="center">
  <img src="assets/glint-logo.png" alt="glint logo" width="220" />
</p>

<h1 align="center">glint</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"></a>
  <a href="CHANGELOG.md"><img src="https://img.shields.io/badge/release-v0.2.23-blue.svg" alt="Release"></a>
</p>

> **Disclaimer:** This tool was built through iterative AI-assisted development with [Claude](https://claude.ai). It is experimental, incomplete, and not intended for production use. Coverage of GitLab CI keywords is best-effort and may lag behind GitLab's evolving spec. Use it at your own discretion — no correctness guarantees are made. Contributions and bug reports are welcome.

A local tool to validate and lint `.gitlab-ci.yml` pipelines without needing a GitLab server.

## What it does

- **Lints** — 43 rules covering pipeline structure, keyword constraints, `needs:`/`dependencies:` graphs, expression reachability, and deprecations (GL001–GL043); run `glint explain <ID>` for any rule
- **Resolves includes** — local files, HTTPS URLs, GitLab project templates, and CI/CD Catalog components, with offline cache support
- **Simulates context** — `--branch`, `--tag`, `--source` flags evaluate `rules:if:` and `only`/`except` to show which jobs would be active, manual, or skipped; `--context branch=main --context branch=develop` prints a multi-column comparison table across multiple contexts in one run
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

Run `glint <command> --help` for all flags. See [USAGE.md](USAGE.md) for full
examples covering output formats, context simulation, remote includes, cache,
graph modes, and project configuration.

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
