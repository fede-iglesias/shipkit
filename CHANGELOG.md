# Changelog

All notable changes to this project will be documented here.
The format is based on Keep a Changelog and this project adheres to Semantic Versioning
per published Go module.

## [Unreleased]

## store/v0.1.1 - 2026-06-05

Soporte para reconocer paths `knowledge/policies/git/` como type `policy-git`. Sin breaking changes.

### Added

- `KindFromPath`: nuevo case `policies/git/` que devuelve kind `"policy-git"`. Habilita a consumidores (ej. kt) registrar el type `policy-git` con archivos bajo `knowledge/policies/git/<slug>.md`.

## [0.2.2] - 2026-06-05

Hot-fix release. Retracts v0.2.1.

### Fixed

- `lifecycle/install`: dropped a no-op self-assignment (`deps.FS = deps.FS`)
  in `plan_test.go` that `go vet` flagged, causing the release workflow to
  fail for tag `lifecycle/install/v0.2.1`.
- root `go.mod` / `go.sum`: included entries for `lifecycle/install` v0.2.2
  and `lifecycle/uninstall` v0.1.1 by running `go mod tidy` after creating
  the submodule tag locally.

### Retracted

- `lifecycle/install` v0.2.1: vet self-assignment, use v0.2.2.
- root v0.2.1: missing go.sum entries, use v0.2.2.

### Changed (dependency graph)

- `lifecycle/install` v0.2.1 (retracted) to v0.2.2.
- `lifecycle/uninstall` stays at v0.1.1 (release was clean).
- Root v0.2.1 (retracted) to v0.2.2.

## [0.2.1] - 2026-06-05

Bugfix release for lifecycle verbs detected in kt cancha session.

### Fixed

- `lifecycle/uninstall`: walks up empty parent directories after removing
  completion scripts, bounded by `XDG_DATA_HOME` root (not the per-app data
  dir). The previous bound prevented walk-up beyond the app directory, so
  shared dirs like `$XDG_DATA_HOME/zsh/site-functions` were left orphaned
  after the app's own completion file was removed.
- `lifecycle/install --print` and `lifecycle/uninstall --print`: new `Plan`
  struct enumerates completion paths, shell RC blocks, marker file, and
  autostart info instead of only data dir and binary path.
- `example/shipkit-example` go.mod and go.sum tidied post v0.2.0 bump (CI
  build of v0.0.1 in cancha was failing with missing go.sum entries).

### Changed (dependency graph)

- `lifecycle/install` v0.2.0 to v0.2.1.
- `lifecycle/uninstall` v0.1.0 to v0.1.1.
- Root v0.2.0 to v0.2.1 (pins the above).

## [0.2.0] - 2026-06-04

### Added

- `WithMigrations(...migrations.Migration)` option: consumers can register
  update migrations on the orchestrator via `UpdateCmd`; migrations are applied
  in semver ascending order during an update run.
- `WithDoctorStatExecutable`, `WithDoctorStatDir`, `WithDoctorStatFile`,
  `WithDoctorReadMarker` options: override individual stat / read funcs wired
  by `DoctorCmd`; replaces the nil-fallback "not wired" behaviour with explicit
  os-backed defaults.
- `lifecycle/recovery v0.1.0` (new module): shared recovery-manifest contract
  used by update, clean, and doctor. Consumers gain a dependency on this module
  when importing any of the affected lifecycle verbs.

### Changed

- `DoctorCmd` now wires os-backed default implementations for
  `StatExecutableFunc`, `StatDirFunc`, `StatFileFunc`, and `ReadMarkerFunc`
  when the consumer does not supply overrides; no more nil-fallback silent skips
  in production runs.
- `UpdateCmd` now reads `SHIPKIT_RELEASES_BASE` and `SHIPKIT_SKIP_VERIFY` env
  vars; useful for cancha / integration-test workflows against a local
  testserver.

### Changed (dependency graph)

- Bumps to `ports v0.2.0`, `adapters v0.2.0`, `lifecycle/update v0.2.0`,
  `lifecycle/install v0.2.0`, `lifecycle/clean v0.2.0`, `lifecycle/doctor v0.2.0`.
- Adds `lifecycle/recovery v0.1.0` (new).

## [0.1.0] - 2026-06-04

### Added

**Extracted from kt v0.1.3** (battle-tested production code):
- `frontmatter` - YAML frontmatter parser and atomic writer; `ReadFile`, `WriteFile`, `Split`, `Unmarshal`, `Marshal`, `ReadFileInto`, `EnsureType`, `ErrNoFrontmatter`.
- `lifecycle/migrations` - ordered, semver-sorted migration registry; `New`, `Register`, `Pending`, `ApplyPending`, `Revert`.
- `store` - universal storage primitives; `AtomicWrite`, `BodyChecksum`, `Acquire`, `Lock.Release`, `PathFor`, `KindFromPath`.
- `lifecycle/update` - cosign-verified atomic self-update state machine; snapshot, download, sigstore-go verify, atomic replace, migrations, health check, rollback cascade. `sigstoreRealVerify` pattern preserved: `adapters.SetVerifyCore` called from consumer `cmd/` layer; `ErrCosignNotConfigured` returned when not wired.

**New from scratch** (TDD, 100% statement coverage):
- `lifecycle/install` - XDG dir setup, shell completions, shell RC guarded blocks, optional platform autostart (LaunchAgent / systemd-user). JSON marker for idempotency. Bash 3.2 darwin skip. `install.NewCommand`, `install.Run`, `install.Deps`, `install.Config`, `install.Options`, `install.Result`.
- `lifecycle/uninstall` - full removal: autostart, completions, RC blocks, XDG dirs, binary self-delete. Four `BinaryAction` outcomes, `--keep-data`, `--keep-config`, `--keep-cache`, `--print`. `uninstall.NewCommand`, `uninstall.Run`, `uninstall.Deps`, `uninstall.Options`, `uninstall.Result`.
- `lifecycle/doctor` - 13 health checks across binary, XDG dirs, PATH, completions, recovery manifest, autostart, optional network (GitHub, cosign TUF, update feed). `--network` gate, `--json` output. `doctor.NewCommand`, `doctor.Run`, `doctor.Deps`, `doctor.Options`, `doctor.Report`, `doctor.ExitCode`, `doctor.FormatText`.
- `lifecycle/clean` - scope-flagged artifact pruning (--snapshots, --tmp, --cache, --logs, --all) with recovery manifest protection, symlink escape prevention, `--keep N`, `--older-than`, `--print`. `clean.NewCommand`, `clean.Run`, `clean.Deps`, `clean.Options`, `clean.Result`.

**Supporting layer**:
- `ports` - 11 port interfaces: `HTTPPort`, `FsPort`, `CosignPort`, `SpawnPort`, `ClockPort`, `PathsPort`, `EnvPort`, `ShellRcPort`, `CompletionPort`, `AutostartPort`, `PromptPort`. Includes `MockXxx` helpers for each interface.
- `adapters` - 11 production implementations wired to real OS/network: `NewGitHubHTTP`, `NewRealFs`, `NewSigstoreCosign`, `NewRealSpawn`, `NewRealClock`, `NewPathsXDG`, `NewEnvOS`, `NewShellRcReal`, `NewCompletionCobra`, `NewAutostartReal`, `NewPromptTerm`. Plus 2 bridge adapters: `NewHTTPBridge`, `NewSpawnBridge` (nominal type adapter between `shipkit/ports` and `lifecycle/update/ports`).
- `shipkit` root - `Config`, `Config.WithDefaults`, `Config.Validate`, `ErrInvalidConfig`, `RegisterLifecycle`, `InstallCmd`, `UpdateCmd`, `UninstallCmd`, `DoctorCmd`, `CleanCmd`. `Option` DI: `WithoutInstall`, `WithoutUpdate`, `WithoutUninstall`, `WithoutDoctor`, `WithoutClean`, `WithHTTPPort`, `WithFsPort`, `WithCosignPort`, `WithSpawnPort`, `WithClockPort`, `WithPathsPort`, `WithEnvPort`, `WithShellRcPort`, `WithCompletionPort`, `WithAutostartPort`, `WithPromptPort`.

**Infrastructure**:
- Multi-module mono-repo: go.work (gitignored), CI matrix, release workflow, coverage gate (100% pkg, 85% cmd).
- 11 module tags: `frontmatter/v0.1.0`, `lifecycle/migrations/v0.1.0`, `store/v0.1.0`, `lifecycle/update/v0.1.0`, `ports/v0.1.0`, `lifecycle/install/v0.1.0`, `lifecycle/uninstall/v0.1.0`, `lifecycle/doctor/v0.1.0`, `lifecycle/clean/v0.1.0`, `adapters/v0.1.0`, `v0.1.0`.
- `kt v0.2.0` is the first downstream consumer via `shipkit.RegisterLifecycle`.

## Per-module changelogs

Each module also keeps its own `CHANGELOG.md` in its directory.
