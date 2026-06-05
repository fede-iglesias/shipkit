# Changelog

## [0.2.3] - 2026-06-05

### Added

- Coverage-completion tests for branches the v0.2.2 cut left at 95-97%:
  realClock.NowUTC and realClock.Since, the default hostOS / hostArch
  lambdas, archAliases / osAliases / containsToken alias-table branches,
  rollback's nil-cause / nil-FS / nil-Migrator defensive paths,
  handleDownload's bundle-open error branch, the dispatch-loop short-circuit
  when snapshotID is empty, the OK-path To-fallback when health.Version is
  empty, and Restore's nil-ChmodFn fallback. No behavior change; coverage
  now satisfies the shipkit release gate (100% floor for library modules).

### Note

`v0.2.2` was tagged but did not produce a GitHub Release because the
coverage gate rejected the build at 95.2% on the update package. The fix
ships forward as `v0.2.3`. `go get
github.com/fede-iglesias/shipkit/lifecycle/update@v0.2.2` still works for
consumers (Go modules are immutable and the source is identical for the
bugfix surface area); upgrading to v0.2.3 only adds tests.

## [0.2.2] - 2026-06-05

### Fixed

- B5: `findAsset` now selects the release asset matching the running host's
  (`runtime.GOOS`, `runtime.GOARCH`) tuple instead of returning the first
  `.tar.gz` in the asset list. The matcher accepts case-insensitive aliases
  for OS (`darwin`/`macos`/`osx`, `windows`/`win`) and arch (`amd64`/`x86_64`,
  `arm64`/`aarch64`, etc.) and rejects companion files (`.bundle`,
  `.sbom.json`) even when their names contain the host tokens. Reproduced
  against the live `fede-iglesias/tools` `relay-v0.1.1` release: previously a
  darwin/arm64 host downloaded `relay_0.1.1_darwin_amd64.tar.gz` and failed
  cosign verify with a confusing "bundle not found" message; the new matcher
  returns `relay_0.1.1_darwin_arm64.tar.gz` and surfaces a clear
  "no .tar.gz asset matching <os>/<arch>" error if no asset matches.
- B6: `handleDownload` now fetches the cosign `.bundle` companion alongside
  the tarball when `Cfg.SkipVerify=false`. Previously only the tarball was
  downloaded, causing `Cosign.VerifyBundle` to fail with
  `cosign: bundle not found at "<tmpdir>/<asset>.tar.gz.bundle": no such file
  or directory`. The bundle URL is the tarball's `DownloadURL` with `.bundle`
  appended (goreleaser + cosign convention). `SkipVerify=true` preserves the
  legacy single-download fast path.
- B7: `RealFsAdapter.Restore` now re-applies the snapshot's mode bits via
  `Chmod` after the temp-rename swap. Previously `os.Create` produced a
  `0o666`-pre-umask file (typically `0o644` on macOS/Linux) and the rename
  inherited the temp file's mode, silently dropping the executable bit on
  rollback: a `0o755` binary came back as `0o644` and the user observed
  `permission denied` on the next invocation. When the snapshot has no
  execute bit (corrupt snapshot edge case), Restore falls back to `0o755`
  because a shipkit-managed CLI binary must remain executable.

### Added

- `hostOS`, `hostArch` package-level variables and `assetMatchesHost`,
  `containsToken` helpers in `orchestrator.go`. Tests use a `TestMain` plus
  `setHostForTest(t, goos, goarch)` to exercise the matcher under multiple
  simulated hosts without depending on the test runner's actual arch.
- `RealFsAdapter.ChmodFn` field (defaults to `os.Chmod` via `NewRealFs`).
  Used by `Restore` for B7; injectable for failure-path testing.

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
