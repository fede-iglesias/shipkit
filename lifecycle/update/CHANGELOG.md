# Changelog

## [0.2.0] - 2026-06-04

### Fixed

- `ExtractTarGz` now rejects path-traversal entries (`../foo` resolving outside
  `destDir`) and symlink / hardlink entries; returns `ErrTarballEntryEscapes`
  sentinel in either case (CVE-class fix per architectural review C2).

### Changed

- Forward state machine is now driven by a `map[State]stateHandler` populated
  from `Transitions()`; `Run` loop dispatches via the map. `IsForwardPath` and
  `IsTerminal` are consumed in production (architectural review C3).
- Rollback now persists the canonical recovery manifest via
  `github.com/fede-iglesias/shipkit/lifecycle/recovery` (filename
  `.shipkit.recovery-manifest.json`). Manifest fields: `Version`, `AppName`,
  `SnapshotPath`, `Steps`, `Cause`, `CreatedAt`. Written on both
  `KindRolledBack` and `KindFailedUnrecoverable` terminal paths.

### Removed

- Local `RecoveryManifest`, `RecoveryStep`, `RecoveryManifestFilename`,
  `PersistRecoveryManifest`, `LoadRecoveryManifest`, `ClearRecoveryManifest`
  types and functions; replaced by `lifecycle/recovery`.

### Added

- `require github.com/fede-iglesias/shipkit/lifecycle/recovery v0.1.0`.

## [0.1.0] - 2026-06-04

### Added

- Initial extraction from kt v0.1.3 `pkg/upgrade` into `shipkit/lifecycle/update`.
- Full state machine: 8-state forward path + cascade rollback path.
- Embedded sigstore-go cosign verification (no os/exec cosign binary).
- `SetVerifyCore` injection pattern - production wiring stays in consumer cmd layer.
- Atomic binary replace via temp-file + rename.
- Recovery manifest persisted to disk on unrecoverable rollback.
- Ports: `HTTPPort`, `FsPort`, `CosignPort`, `SpawnPort`, `ClockPort`.
- Adapters: `GitHubHTTPAdapter`, `RealFsAdapter`, `SigstoreCosignAdapter`, `RealSpawnAdapter`.
- `Config.Validate()`, `RunOpts`, `Kind` (7 constants), `Result`.
- `SetOrchestratorFactory` / `OrchestratorRunner` injection pattern.
- `PersistRecoveryManifest`, `LoadRecoveryManifest`, `ClearRecoveryManifest`.
- `ValidateTransitions`, `ValidateTransitionsTable` for state graph invariants.
- 100% test coverage with stdlib testing only (no testify, no ginkgo).
