# Changelog

All notable changes to shipkit/store are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-06-04

### Added

- Initial extract from kt v0.1.3 (universal primitives from pkg/store).
- `AtomicWrite`: crash-safe temp-file + rename write with automatic parent creation.
- `EnsureParent`: creates all parent directories for a given path.
- `WalkDir`: collect files by extension under a directory, ignoring missing roots.
- `MoveToArchive`: move a file to a flat archive directory preserving its name.
- `Acquire` / `Lock.Release`: advisory flock with configurable timeout and `ErrLockTimeout`.
- `BodyChecksum`: SHA-256 hex digest of normalized body content.
- `PathFor` / `KindFromPath`: resolve and reverse-map card file paths for the knowledge layout.
- `KnowledgeDir` / `ArchiveDir`: constants for the knowledge directory layout.
- `WithADRID`, `WithTaskID`, `WithMeetingID`: functional options for `PathFor`.
- 100% statement coverage preserved from kt source.
- `doc.go` package-level documentation.
- `example_test.go` with `ExampleAtomicWrite`, `ExampleBodyChecksum`, `ExampleAcquire`, `ExamplePathFor`.
