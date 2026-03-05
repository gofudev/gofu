# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `internal/pipeline` package — reusable build pipeline callable from both CLI and server

### Changed

- `gofu build` / `gofu run` delegate to `pipeline.Build()` instead of owning orchestration

## [v0.1.0]

TODO
