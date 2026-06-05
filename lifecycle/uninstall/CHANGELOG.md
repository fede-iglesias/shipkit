# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2026-06-05

### Fixed

- Completion cleanup now walks up empty parent directories after removing the
  app's completion files, bounded by `XDG_DATA_HOME` root rather than the
  per-app data dir. Previously, shared parent dirs like
  `$XDG_DATA_HOME/zsh/site-functions` were left orphaned because the walk-up
  bound stopped at the app dir before it could climb to the shared parent.
- `Run` with `Options.Print` now emits a `Plan` that enumerates completion
  paths, shell RC blocks, marker file, and autostart label. The prior print
  path showed only data dir and binary path, hiding relevant teardown
  side-effects from `--print -y` callers in cancha and consumer smokes.

## [0.1.0] - 2026-06-04

### Added

- `Run`: linear teardown state machine (confirm, stop autostart, remove
  completions, remove RC blocks, remove XDG dirs, binary action).
- `Options`: `KeepData`, `KeepConfig`, `KeepBinary`, `Yes`, `Print`.
- `Deps`: injectable ports (FsPort, PathsPort, ShellRcPort, CompletionPort,
  AutostartPort, PromptPort) plus `RemoveBinaryFunc` and `ScheduledExitFunc`
  for binary self-deletion (sigstoreRealVerify-style consumer wiring).
- `Result`: `Stopped`, `Removed`, `Skipped`, `BinaryAction`, `NextSteps`.
- `BinaryAction` constants: `BinaryDeletedNow`, `BinaryScheduledExit`,
  `BinaryKept`, `BinaryDeleteRequested`.
- `NewCommand`: cobra builder registering `--keep-data`, `--keep-config`,
  `--keep-binary`, `-y/--yes`, `--print` flags.
- 100% statement coverage via stdlib testing only.
- `doc.go`, `example_test.go`, `README.md` per shipkit in-code docs standard.
