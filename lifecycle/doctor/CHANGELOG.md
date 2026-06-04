# Changelog

All notable changes to this module are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This module uses [semantic versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-06-04

### Changed

- `checkRecoveryManifest` now calls `recovery.Read` directly via
  `lifecycle/recovery`; `StatFileFunc` is no longer used for manifest
  detection. The check returns `StatusFail` when the manifest is present and
  parses, `StatusPass` when absent, and `StatusWarn` on any other read error.
- Manifest file path uses `recovery.Path(deps.DataRoot)` instead of a
  hardcoded string.

### Added

- `require github.com/fede-iglesias/shipkit/lifecycle/recovery v0.1.0`.

## [0.1.0] - 2026-06-04

### Added

- `Run` function that executes 13 health checks and returns a `Report`.
- Check inventory: `binary.in-path`, `binary.executable`, `binary.version`,
  `xdg.data-dir`, `xdg.config-dir`, `xdg.cache-dir`, `marker`, `completion.<shell>`,
  `autostart`, `recovery.manifest`, `network.github`, `network.cosign-tuf`,
  `network.update-feed`.
- `Options` struct with `Network`, `JSON`, and `Verbose` fields.
- `Deps` struct with injectable Func fields following the sigstoreRealVerify pattern
  (`StatExecutableFunc`, `StatDirFunc`, `StatFileFunc`, `ReadMarkerFunc`,
  `CheckNetworkGitHubFunc`, `CheckNetworkCosignTUFFunc`, `CheckNetworkUpdateFeedFunc`,
  `RunFunc`).
- `Report`, `Check`, `Summary`, `Status`, `CheckID` types with JSON tags.
- `ComputeSummary` helper to aggregate check counts.
- `ExitCode` helper: returns 0 (warn or better) or 1 (any failure).
- `FormatText` for human-readable `[PASS]/[WARN]/[FAIL]/[SKIP]` output.
- `NewCommand` cobra builder wiring `--network`, `--json`, `--verbose` flags.
- `ExitError` sentinel for cobra RunE non-zero exit signalling.
- Network checks gated behind `--network`; default is StatusSkipped.
- 100% statement coverage via stdlib testing.
