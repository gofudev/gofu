# Gofu

Safe, compiled language for SaaS integrations. Restricted Go subset → static analysis → native binary.

Gofu lets you write Go-like code that compiles to safe, sandboxed native binaries. It restricts the Go language to a safe subset — no arbitrary imports, no unsafe operations, no network access outside approved patterns — then compiles through standard Go tooling.

## Install

```bash
go install gofu.dev/gofu/cmd/gofu@latest
```

Or build from source:

```bash
git clone https://github.com/gofudev/gofu.git
cd gofu
make build
```

## Usage

```bash
# Initialize a new module
gofu init mymodule

# Build a module
gofu build

# Run a function
echo '{"name": "World"}' | gofu run Greet 3>&1

# Run tests
gofu test
```

## Status

Pre-release (`v0.1.x`). API and language subset may change.

## License

Business Source License 1.1 — free for non-commercial and open-source use. Commercial use requires a paid license. See [LICENSE](LICENSE) for details.