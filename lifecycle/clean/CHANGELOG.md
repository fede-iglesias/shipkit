# Changelog

All notable changes to shipkit/lifecycle/clean are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-06-04

### Changed

- Recovery-manifest read now uses canonical `lifecycle/recovery.Read` and
  `recovery.Path`; `DefaultReadManifest` delegates to `recovery.Read` instead
  of parsing JSON locally.
- `RecoveryManifest` in `Deps.ReadManifestFunc` signature replaced by
  `*recovery.Manifest` from `lifecycle/recovery`; local `RecoveryManifest`
  struct removed.
- `collectSnapshots` uses `recovery.Path(dataDir)` for the manifest file path
  instead of a hardcoded string.

### Added

- `require github.com/fede-iglesias/shipkit/lifecycle/recovery v0.1.0`.

## [0.1.0] - 2026-06-04

### Added

- `Run(ctx, deps, opts)` - main entry point implementing the clean state machine (EnumerateCandidates, Confirm, RemoveLoop, ReportSummary).
- `Options` struct with scope flags: `Snapshots`, `Tmp`, `Cache`, `Logs`, `All`, plus `OlderThan`, `Keep`, `Yes`, `Print`.
- `Deps` struct with injected port interfaces: `FsPort`, `PathsPort`, `ClockPort`, `PromptPort`, and injectable list/manifest functions.
- `ErrNoScope` sentinel error returned when no scope flag is set (safe default: no accidental deletion).
- Recovery manifest protection: snapshots referenced by `.shipkit.recovery-manifest.json` are never deleted.
- Symlink escape prevention: entries whose resolved symlink destination escapes DataDir are skipped.
- `--keep N` override: newest N snapshots always preserved regardless of `--older-than`.
- `--older-than` duration filter defaulting to 720h (30 days).
- `--all` flag equivalent to `--snapshots --tmp --cache --logs`.
- `--print` dry-run mode: computes and returns candidates without removing anything.
- `NewCommand(deps)` cobra builder registering all flags.
- `DefaultListSnapshots`, `DefaultListTmp`, `DefaultListCache`, `DefaultListLogs` production OS adapter functions.
- `DefaultReadManifest` production OS adapter for `.shipkit.recovery-manifest.json`.
- `SnapshotEntry`, `TmpEntry`, `CacheEntry`, `LogEntry`, `CleanedItem`, `Result`, `RecoveryManifest` public types.
- `doc.go` package documentation with design rationale, usage example, and cross-references.
- `example_test.go` with `ExampleRun`, `ExampleRun_noScope`, `ExampleNewCommand`.
- 99.1% test coverage (remaining 0.9% is race-condition defensive code in OS adapter functions).
