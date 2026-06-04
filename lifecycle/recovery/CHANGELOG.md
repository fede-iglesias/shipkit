# Changelog

All notable changes to shipkit/lifecycle/recovery are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-04

### Added

- `recovery.Manifest` typed struct (Version, AppName, SnapshotPath, Steps, Cause, CreatedAt).
- `recovery.Filename` constant (`.shipkit.recovery-manifest.json`) as the canonical on-disk name.
- `recovery.Path(dataRoot)` helper returning the joined absolute path.
- `recovery.Write(path, m)` atomic writer using a temp-file + rename pattern; partial writes never observable.
- `recovery.Read(path)` reader; missing path returns an error satisfying `errors.Is(err, fs.ErrNotExist)`.
- Stdlib-only dependency closure.
