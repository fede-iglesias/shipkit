# Changelog

All notable changes to shipkit/ports are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-06-04

### Added

- 11 port interfaces for the shipkit lifecycle verbs.
- 5 kt-shaped ports (structural mirrors of lifecycle/update/ports): `HTTPPort`,
  `FsPort` (extended with `CopyFile` and `RemoveDir`), `CosignPort`, `SpawnPort`,
  `ClockPort`.
- 6 new ports required by install, uninstall, doctor, clean: `PathsPort`,
  `EnvPort`, `ShellRcPort`, `CompletionPort`, `AutostartPort`, `PromptPort`.
- `ShellKind` type with `ShellBash`, `ShellZsh`, `ShellFish`, `ShellUnknown` constants.
- `EnsureResult` and `RemoveResult` value types for `ShellRcPort`.
- `AutostartUnit` and `AutostartStatus` value types for `AutostartPort`.
- `HealthResult` value type for `SpawnPort`.
- `Release` and `Asset` value types for `HTTPPort`.
- Mock helpers (`MockXxx` + `NewMockXxx`) for all 11 ports, enabling consumer
  unit tests with zero real I/O.
- `doc.go` with Hexagonal Architecture context, design rationale, and usage example.
- `example_test.go` with five consumer-pattern examples.
- `cobra v1.10.2` dependency (intentional: `CompletionPort.EmitCompletion` accepts
  `*cobra.Command` per OQ A2 resolution).
