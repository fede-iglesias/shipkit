# shipkit-example

Reference consumer CLI for [shipkit](https://github.com/fede-iglesias/shipkit).

## What is this

`shipkit-example` is a minimal CLI that wires all five shipkit lifecycle verbs
using the production adapters from `github.com/fede-iglesias/shipkit/adapters`.
Use it as a copy-paste template when starting a new personal CLI.

## 30-second quickstart

```bash
go build ./cmd/shipkit-example
./shipkit-example --help
```

## Lifecycle verbs

| Verb | What it does |
|------|-------------|
| `install` | Creates XDG dirs, installs shell completions, writes marker JSON |
| `update` | Cosign-verified atomic self-update with rollback |
| `uninstall` | Removes binary, dirs, completions, and service units |
| `doctor` | Health checks: binary, XDG dirs, PATH, completions, network |
| `clean` | Prunes old snapshots and recovery manifests |

## Notes

- NOT released to `fede-iglesias/tools`. Per OQ C3 this is a sample consumer,
  not a shipped binary.
- The `sigstoreRealVerify` function in `cmd/shipkit-example/main.go` shows the
  exact wiring pattern for cosign verification in the cmd layer.
- Integration tests live in `integration_test.go`. Run with `go test ./...`
  (requires network for go build; skip with `go test -short ./...`).
