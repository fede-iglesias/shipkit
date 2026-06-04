# Changelog

All notable changes to this module are documented here. Format based on Keep a Changelog, semver via published Go module.

## [0.2.0] - 2026-06-04

### Added

- `RealFsAdapter.MkdirAll(ctx, path, perm)` delegates to `os.MkdirAll`.
- `RealFsAdapter.ReadFile(ctx, path)` delegates to `os.ReadFile`.
- `RealFsAdapter.AtomicWrite(ctx, path, data, perm)` writes via temp-then-rename
  in the same directory; creates a `.shipkit-atomic-*` temp file, writes, chmods,
  closes, then renames atomically.
- `NewHTTPBridgeWithBaseURL(baseURL)` constructor: returns an `HTTPBridgeAdapter`
  whose inner GitHub API base URL is overridden; used by cancha workflows to
  redirect API calls to a local testserver.
- `atomicWriteCommit` package-level seam (`var`): isolates the write/chmod/close/
  rename steps so error paths are unit-testable without real I/O failures.

### Changed

- `RealFsAdapter` now satisfies the expanded `ports.FsPort` interface including
  `MkdirAll`, `ReadFile`, and `AtomicWrite`.

## [0.1.0] - 2026-06-04

### Added

- Initial extraction from kt v0.1.3.
- Production implementations for all 11 port interfaces declared in
  `github.com/fede-iglesias/shipkit/ports`:
  `NewGitHubHTTP`, `NewRealFs`, `NewSigstoreCosign`, `NewRealSpawn`,
  `NewRealClock`, `NewPathsXDG`, `NewEnvOS`, `NewShellRcReal`,
  `NewCompletionCobra`, `NewAutostartReal`, `NewPromptTerm`.
- `RealFsAdapter` wraps `lifecycle/update/adapters.RealFsAdapter` and extends
  it with `CopyFile` and `RemoveDir`.
- Bridge adapters: `NewHTTPBridge`, `NewSpawnBridge` (nominal-type adapters
  between `lifecycle/update/ports` and `shipkit/ports`).
- `doc.go` with design rationale, production wiring pattern, and sigstoreRealVerify
  explanation.
- 100% statement coverage via stdlib testing.
