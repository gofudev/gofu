# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `pipeline` package — public API (`pipeline.Build`) for running the build pipeline programmatically [#1](https://github.com/gofudev/gofu/pull/1), [#2](https://github.com/gofudev/gofu/pull/2)
- `pipeline.Analyze` and `pipeline.AnalyzeSource` — extract runnable metadata (name, params, returns) from source without building [#2]([https:](https://github.com/gofudev/gofu/pull/4)

### Fixed

- Capture `go build` stderr output in compilation error messages [#3](https://github.com/gofudev/gofu/pull/3)

## [v0.1.0]

### Added

- `gofu` CLI with `init`, `build`, `run`, and `test` commands
- `internal/analyzer` — static analysis enforcing the Gofu restricted subset (no goroutines, no channels, no `init`, allowlisted stdlib only)
- `internal/codegen` — code generation for compiled Gofu binaries
- `internal/cli` — command implementations with go.mod management
- `//gofu:allow` and `//gofu:block` directives for fine-grained analysis control
- `//gofu:runnable` directive for marking exported entry-point functions
- `//gofu:secret` directive for declaring required credentials
- `modules/credentials` — credential access API for Gofu programs
- `modules/http` — HTTP client module with SSRF protection
- End-to-end test suite (`tests/e2e/`)
- CI pipeline via GitHub Actions
- `scripts/publish.sh` for releasing versioned tags

### Fixed

- Version detection to match only root-level tags (not module tags)
