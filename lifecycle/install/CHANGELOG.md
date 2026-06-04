# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
