# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
