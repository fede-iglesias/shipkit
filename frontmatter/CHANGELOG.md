# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-04

### Added

- Initial extract from kt v0.1.3 (`pkg/frontmatter`).
- `Split`: separates YAML frontmatter from body; normalizes CRLF and trailing newlines.
- `Unmarshal`: YAML deserialization via `goccy/go-yaml` with field-order preservation.
- `Marshal`: serializes metadata + body into a complete frontmatter document.
- `ReadFile`: reads a file and returns `map[string]any` metadata and body bytes.
- `ReadFileInto`: reads a file and unmarshals metadata directly into a typed struct.
- `WriteFile`: atomic write via write-temp-then-rename strategy.
- `WriteFileWithRename`: injectable rename for testing or cross-device scenarios.
- `EnsureType`: injects a `type` key into YAML only when absent.
- 100% statement coverage preserved from kt source.
- `doc.go`, `README.md`, example tests (`ExampleSplit`, `ExampleMarshal`, `ExampleWriteFile`, `ExampleEnsureType`).
