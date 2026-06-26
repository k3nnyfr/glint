<p align="center">
  <img src="assets/glint-logo.png" alt="glint logo" width="220" />
</p>

<h1 align="center">glint</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"></a>
  <a href="CHANGELOG.md"><img src="https://img.shields.io/badge/release-v0.4.1-blue.svg" alt="Release"></a>
</p>

> **Disclaimer:** This tool was built through iterative AI-assisted development with [Claude](https://claude.ai). It is experimental, incomplete, and not intended for production use. Coverage of GitLab CI keywords is best-effort and may lag behind GitLab's evolving spec. Use it at your own discretion — no correctness guarantees are made. Contributions and bug reports are welcome.

A local tool to validate and lint `.gitlab-ci.yml` pipelines without needing a GitLab server.

## What it does

- **Lints** — 45 rules covering pipeline structure, keyword constraints, `needs:`/`dependencies:` graphs, expression reachability, and deprecations (GL001–GL045); run `glint explain <ID>` for any rule
- **Resolves includes** — local files, HTTPS URLs, GitLab project templates, and CI/CD Catalog components, with offline cache support and HTTP proxy support (`--proxy` flag or `proxy:` in `.glint.yml`)
- **Renders merged pipeline** — `glint render` resolves all includes and `extends:` chains into a single flat YAML file, matching what GitLab CI actually processes
- **Simulates context** — `--branch`, `--tag`, `--source` flags evaluate `rules:if:` and `only`/`except` to show which jobs would be active, manual, or skipped; `--context branch=main --context branch=develop` prints a multi-column comparison table across multiple contexts in one run
- **Multiple output formats** — `--format text` (default, colorized and column-aligned), `json`, `sarif` (GitHub Code Scanning / GitLab SAST), `junit`, `github` (PR annotations); exits `2` on errors, `10` on warnings only
- **Project config** — `.glint.yml` for rule suppression, severity overrides, token/URL/proxy defaults; `# glint: ignore RULE` for per-job inline suppression; `--no-warn` flag to suppress all warnings
- **Graph visualization** — `glint graph` prints a terminal job tree (default); `glint graph includes` emits a Mermaid include-dependency graph; `glint graph pipeline` renders a GitLab CI-style SVG/PNG; `--format mermaid` or `--format html` for alternative pipeline output; `--no-skipped` hides jobs that would not run in the given context
- **LSP server** — `glint lsp` starts a Language Server Protocol server over stdin/stdout; connect with any LSP client to get inline diagnostics (rule ID as code, error/warning severity) in VS Code, Neovim, Emacs, JetBrains, etc.
- **VS Code extension** — `editors/vscode/` wraps the LSP server; inline squiggles for every glint rule directly in the editor

See [FEATURES.md](FEATURES.md) for the complete feature reference and lint rules table, and [ROADMAP.md](ROADMAP.md) for planned improvements.

## Installation

See [INSTALL.md](INSTALL.md) for all options: pre-built binaries (Linux amd64/arm64, macOS Intel/Apple Silicon, Windows), Homebrew tap, and building from source.

Quick start (Linux/macOS, building from source):

```bash
git clone https://git.k3nny.fr/k3nny/glint
cd glint
go build -o glint ./cmd/glint/...
sudo mv glint /usr/local/bin/
```

Homebrew:

```bash
brew tap k3nny/glint https://github.com/k3nny/homebrew-glint
brew install glint
```

## Requirements

Go 1.21 or later (when building from source). Pre-built binaries have no runtime dependencies.

## Usage

```
glint [OPTIONS] <COMMAND>

Commands:
  check    Lint a pipeline file — exits 0 (clean), 2 (errors), or 10 (warnings only)
  render   Resolve all includes and extends into a single merged YAML file
  graph    Visualise the pipeline as a job tree or Mermaid graph
  explain  Show description and fix for a lint rule (e.g. glint explain GL007)
  lsp      Start a Language Server Protocol server (stdin/stdout)
```

Run `glint <command> --help` for all flags. See [FEATURES.md](FEATURES.md) for the
complete feature reference.

## Integrations

### Pre-commit hook

Add to `.pre-commit-config.yaml` in your repository to run glint automatically whenever `.gitlab-ci.yml` changes:

```yaml
repos:
  - repo: https://git.k3nny.fr/k3nny/glint
    rev: v0.3.0
    hooks:
      - id: glint
```

Requires [pre-commit](https://pre-commit.com) and Go 1.21+. On first run, pre-commit builds glint from source automatically.

### GitLab CI component

Copy [`templates/check.yml`](templates/check.yml) into your repository and include it as a local file, or publish this repository to a GitLab CI/CD Catalog and reference it as a component:

```yaml
# As a local include (copy templates/check.yml to your repo first):
include:
  - local: .gitlab/glint-check.yml

# As a Catalog component (after publishing to a GitLab instance):
include:
  - component: $CI_SERVER_FQDN/k3nny/glint/check@v0.3.0
    inputs:
      stage: validate          # optional, default: validate
      allow_failure: true      # optional, default: false
```

The component downloads the glint Linux binary, runs `glint check`, and respects all inputs defined in the `spec:` block.

### GitHub Actions

Copy [`action.yml`](action.yml) from this repository, or mirror this repo to GitHub as `k3nny/glint` and reference it directly:

```yaml
- uses: k3nny/glint@v0.3.0
  with:
    file: .gitlab-ci.yml   # optional, default: .gitlab-ci.yml
    args: '--format sarif' # optional
```

The action downloads the glint Linux binary into `$RUNNER_TEMP` and runs `glint check`. Only Linux runners are supported (matches the available release binary).

### VS Code extension

Clone this repository and load the extension from `editors/vscode/`:

```bash
cd editors/vscode
npm install          # install dependencies (once)
npm run compile      # compile TypeScript → out/
```

Then in VS Code: **Run → Start Debugging** (F5) — this opens an Extension Development Host with glint diagnostics active for any `.gitlab-ci.yml` you open.

Make sure `glint` is on your `PATH`, or set `glint.executablePath` in VS Code settings to the full path of the binary.

To package a `.vsix` for local installation:

```bash
task ext-package     # produces glint-X.Y.Z.vsix
code --install-extension glint-X.Y.Z.vsix
```

## Development

This project uses [Task](https://taskfile.dev) as a task runner.

```bash
task              # list available tasks
task build        # compile the binary
task test         # run Go unit tests
task lint-go      # run go vet
task validate     # run the binary against all testdata fixtures
task ci           # full check: vet → test → build → validate
task fuzz         # run fuzz tests for the YAML parser (Ctrl-C to stop; FUZZ_TIME=60s to set duration)
task changelog    # regenerate CHANGELOG.md from git history via git-cliff
task changelog-next  # preview unreleased section (dry-run, no file written)
task ext-install     # install VS Code extension npm dependencies
task ext-compile     # compile the VS Code extension TypeScript source
task ext-package     # package the VS Code extension as a .vsix
task build-linux-amd64   # cross-compile for Linux x86-64 (requires a tagged commit)
task build-linux-arm64   # cross-compile for Linux ARM64 (requires a tagged commit)
task build-darwin-amd64  # cross-compile for macOS Intel (requires a tagged commit)
task build-darwin-arm64  # cross-compile for macOS Apple Silicon (requires a tagged commit)
task build-windows       # cross-compile for Windows x86-64 (requires a tagged commit)
task build-release       # build all platform binaries at once (requires a tagged commit)
task clean        # remove build artifacts
```

**Optional tools:**

- [git-cliff](https://git-cliff.org) — changelog generator used by `task changelog`. Install with `brew install git-cliff` or `cargo install git-cliff`.

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
