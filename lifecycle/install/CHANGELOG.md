# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.2] - 2026-06-05

Hot-fix release. Retracts v0.2.1.

### Fixed

- Dropped a no-op self-assignment (`deps.FS = deps.FS`) in `plan_test.go`
  that `go vet` flagged, causing the release workflow to fail for tag
  `lifecycle/install/v0.2.1`. The line had no runtime effect; removing it
  unblocks the release pipeline.

### Retracted

- v0.2.1: vet self-assignment, use v0.2.2.

## [0.2.1] - 2026-06-05

### Fixed

- `Run` with `Options.Print` now emits a `Plan` that enumerates completion
  paths (bash, fish, zsh), shell RC blocks (with the fpath + compinit lines
  the consumer will append), marker file path, and autostart info. The prior
  print path showed only data dir and binary path, which hid relevant install
  side-effects from `--print -y` callers in cancha and consumer smokes.

## [0.2.0] - 2026-06-04

### Changed

- All filesystem I/O in `install.go` now routes through `ports.FsPort`:
  `deps.FS.MkdirAll` replaces `os.MkdirAll`, `deps.FS.ReadFile` replaces
  `os.ReadFile`, and `deps.FS.AtomicWrite` replaces `os.CreateTemp` + `os.Rename`.
- `readMarker` accepts `ctx` and `deps` instead of reading the file directly;
  internally calls `deps.FS.ReadFile`.

### Removed

- Internal `atomicWriteBytes` / `atomicWriteBytesHooked` helpers and their
  associated `atomicWriteHooks` struct; replaced by `deps.FS.AtomicWrite`.

## [0.1.0] - 2026-06-04

### Added

- `Run`: linear state machine (Plan, CreateDirs, EmitCompletions, EnsureShellHooks,
  InstallAutostart, WriteMarker) for first-install and idempotent re-install.
- `Options`: `Force`, `Autostart`, `Completions`, `Print`, `Yes`, `Stderr` flags.
- `Deps`: injectable port struct (FsPort, PathsPort, EnvPort, ShellRcPort,
  CompletionPort, AutostartPort, PromptPort, ClockPort).
- `Config`: application-level config (AppName, BinaryName, Version, EnableAutostart,
  AutostartLabel, AutostartArgs).
- `InstallMarker`: JSON marker struct written to `{DataRoot}/.shipkit.installed`
  with `app`, `version_installed`, `installed_at`, `bin_path`, `completions`,
  `autostart` fields.
- `Result`: outcome struct with `Marker`, `AlreadyInstalled`, `PathEnsured`,
  `CompletionsWritten`, `AutostartInstalled`, `Manifest`.
- `NewCommand`: cobra builder wiring `--force`, `--autostart`, `--completions`,
  `--print`, `-y/--yes` flags.
- Bash 3.2 skip on darwin: detects `BASH_VERSION` starting with "3." and emits
  a "brew install bash" warning to stderr instead of installing a broken completion.
- Atomic writes via temp-then-rename for all files (completions and marker).
- 100% statement coverage via stdlib testing, no testify or ginkgo.
