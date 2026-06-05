# Changelog

## [0.2.1] - 2026-06-05

### Fixed

- B1+B4: `Result.From` is now populated from new `Config.CurrentVersion` field
  across every terminal Kind (KindCheckOnly, KindDryRun, KindOK, KindNoOp,
  KindRolledBack, KindFailedUnrecoverable). Callers that previously relied on
  the legacy blank `From` keep working: an empty `Config.CurrentVersion`
  preserves the old behavior.
- B2: `Orchestrator.rollback` is now nil-safe across `Clock`, `Manifest`,
  `Steps`, and inner Step pointers. `Run` short-circuits to
  `KindFailedUnrecoverable` when the failure happens before any snapshot
  state exists, instead of invoking rollback against an empty manifest and
  panicking on a nil dereference.
- B3: `--target-version` is now honored even when `--skip-verify` is set.
  Previously `resolveTargetVersion` queried `LatestRelease` regardless of
  `opts.Version`, causing the downstream asset download to install the
  latest release silently. Now the new `HTTPPort.GetReleaseByTag` is
  invoked when `opts.Version` is pinned, and a missing tag surfaces as
  `KindFailedUnrecoverable` with `Reason` containing
  `release v<X.Y.Z> not found in <repo>`.

### Added

- `Config.CurrentVersion string` field (json:"current_version,omitempty")
  declared by callers (e.g. `kt` via `version.Version`). Optional but
  required to populate `Result.From`.
- `HTTPPort.GetReleaseByTag(ctx, repo, tag) (Release, error)` method.
  `GitHubHTTPAdapter` implementation uses
  `GET /repos/{repo}/releases/tags/{tag}` (GitHub REST API anonymous lookup).

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
