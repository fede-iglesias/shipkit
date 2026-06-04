# Changelog

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
