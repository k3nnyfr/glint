# Installing glint

## Requirements

- Go 1.21 or later (for building from source)
- Any 64-bit Linux, macOS, or Windows system (for pre-built binaries)

---

## Option 1 — Build from source

```bash
git clone https://git.k3nny.fr/k3nny/glint
cd glint
go build -o glint ./cmd/glint/...
sudo mv glint /usr/local/bin/
```

Or with [Task](https://taskfile.dev):

```bash
task build
sudo mv glint /usr/local/bin/
```

---

## Option 2 — Download a pre-built binary

Pre-built binaries are attached to each [release](https://git.k3nny.fr/k3nny/glint/releases).

| Platform | File |
|----------|------|
| Linux x86-64 | `glint-vX.Y.Z-linux-amd64` |
| Linux ARM64 | `glint-vX.Y.Z-linux-arm64` |
| macOS Intel | `glint-vX.Y.Z-darwin-amd64` |
| macOS Apple Silicon | `glint-vX.Y.Z-darwin-arm64` |
| Windows x86-64 | `glint-vX.Y.Z-windows-amd64.exe` |

### Linux (amd64)

```bash
VERSION=v0.4.0
curl -Lo glint https://git.k3nny.fr/k3nny/glint/releases/download/${VERSION}/glint-${VERSION}-linux-amd64
chmod +x glint
sudo mv glint /usr/local/bin/
```

### Linux (ARM64 — Raspberry Pi 4, AWS Graviton, …)

```bash
VERSION=v0.4.0
curl -Lo glint https://git.k3nny.fr/k3nny/glint/releases/download/${VERSION}/glint-${VERSION}-linux-arm64
chmod +x glint
sudo mv glint /usr/local/bin/
```

### macOS (Apple Silicon — M1/M2/M3)

```bash
VERSION=v0.4.0
curl -Lo glint https://git.k3nny.fr/k3nny/glint/releases/download/${VERSION}/glint-${VERSION}-darwin-arm64
chmod +x glint
sudo mv glint /usr/local/bin/
```

### macOS (Intel)

```bash
VERSION=v0.4.0
curl -Lo glint https://git.k3nny.fr/k3nny/glint/releases/download/${VERSION}/glint-${VERSION}-darwin-amd64
chmod +x glint
sudo mv glint /usr/local/bin/
```

### Verify the installation

```bash
glint --version
```

---

## Option 3 — Homebrew (macOS and Linux)

A Homebrew tap is available at `k3nny/glint`.

> **First-time setup:** create a GitHub repository named `homebrew-glint`
> under your account and copy [`Formula/glint.rb`](Formula/glint.rb) into it.
> Users then install via the tap as shown below.

```bash
brew tap k3nny/glint https://github.com/k3nny/homebrew-glint
brew install glint
```

To upgrade:

```bash
brew upgrade glint
```

The formula builds glint from source using Go, which Homebrew provides
automatically as a build dependency. No pre-built binary download is needed.

### Updating the formula on a new release

After tagging a new release, update `Formula/glint.rb`:

1. Compute the tarball checksum:
   ```bash
   curl -sL https://git.k3nny.fr/k3nny/glint/archive/vX.Y.Z.tar.gz | sha256sum
   ```
2. Update `url` and `sha256` in the formula.
3. Commit and push to `homebrew-glint`.

---

## Option 4 — Go install

If you already have Go 1.21+:

```bash
go install git.k3nny.fr/k3nny/glint/cmd/glint@latest
```

The binary is placed in `$(go env GOPATH)/bin/`. Add that directory to your
`PATH` if it is not already there:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

---

## Integrations

For editor and CI integrations (pre-commit hook, GitLab CI component, GitHub
Actions, VS Code extension) see [README.md](README.md#integrations).
