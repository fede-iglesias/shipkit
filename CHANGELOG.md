# Changelog

All notable changes to this project will be documented here.
The format is based on Keep a Changelog and this project adheres to Semantic Versioning
per published Go module.

## [Unreleased]

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
