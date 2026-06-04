# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-04

### Added

- Initial extract from kt v0.1.3 (`pkg/migrations`).
- `Migration` interface: `Version`, `Description`, `Apply`, `Revert` contract.
- `Registry` type with sorted-insert invariant maintained on each `Register` call.
- `New` constructor for empty `Registry`.
- `Pending` method: returns migrations strictly after `current` and up to `target`.
- `ApplyPending` method: runs pending migrations in ascending semver order, stops on first error.
- `Revert` method: undoes a range in reverse semver order, stops on first error.
- 100% statement coverage preserved from kt.
- `doc.go` with package-level comment, design rationale, usage example, and cross-references.
- `example_test.go` with `ExampleRegistry_Register`, `ExampleRegistry_ApplyPending`, `ExampleRegistry_Revert`.
- `README.md` with quickstart, common patterns, and gotchas.
