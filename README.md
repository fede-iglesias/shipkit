# shipkit

Cobra lifecycle toolkit for personal Go CLIs (install / update / uninstall / doctor / clean).

[![Go Reference](https://pkg.go.dev/badge/github.com/fede-iglesias/shipkit.svg)](https://pkg.go.dev/github.com/fede-iglesias/shipkit)
[![CI](https://github.com/fede-iglesias/shipkit/actions/workflows/ci.yml/badge.svg)](https://github.com/fede-iglesias/shipkit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Status

v0.1.0 shipped. Multi-module mono-repo: 11 tags total, each lifecycle verb and
primitive is its own Go module so consumers only take what they need.

## Install

```bash
go get github.com/fede-iglesias/shipkit@v0.1.0
```

The root module declares dependencies on all sub-modules, so one `go get`
transitively pulls every sub-package.

Individual modules (for granular adoption):

```bash
go get github.com/fede-iglesias/shipkit/lifecycle/update@v0.1.0
go get github.com/fede-iglesias/shipkit/lifecycle/install@v0.1.0
go get github.com/fede-iglesias/shipkit/frontmatter@v0.1.0
go get github.com/fede-iglesias/shipkit/store@v0.1.0
```

Note: use `@v0.1.0`, not `@subdir/v0.1.0`. Go module proxy resolves
multi-module mono-repos via the tag prefix, not the subdir path.

## Quickstart

Wire all five lifecycle verbs into a cobra root in one call:

```go
import (
    "github.com/spf13/cobra"
    "github.com/fede-iglesias/shipkit"
)

func main() {
    root := &cobra.Command{Use: "myapp"}
    cfg := shipkit.Config{
        AppName:    "myapp",
        BinaryName: "myapp",
        Repo:       "owner/tools",
        TagPrefix:  "myapp-",
        Version:    "0.1.0",
        BinaryPath: "/usr/local/bin/myapp",
    }.WithDefaults()
    if err := shipkit.RegisterLifecycle(root, cfg); err != nil {
        // handle
    }
    root.Execute()
}
```

`RegisterLifecycle` adds `install`, `update`, `uninstall`, `doctor`, and
`clean` as cobra subcommands. Pass `shipkit.Option` variants to inject custom
ports or disable individual verbs:

```go
// Disable uninstall and inject a custom filesystem adapter in tests.
err := shipkit.RegisterLifecycle(root, cfg,
    shipkit.WithoutUninstall(),
    shipkit.WithFsPort(myFakeFsPort),
)
```

## Architecture

shipkit is structured in two layers:

**Foundation** - primitive packages extracted from a production CLI:
- `frontmatter` - YAML frontmatter round-trip
- `store` - atomic write, file lock, checksum
- `lifecycle/migrations` - ordered migration registry
- `lifecycle/update` - cosign-verified atomic self-update with rollback

**New verbs** - lifecycle commands written from scratch with TDD:
- `lifecycle/install` - config dirs, completions, shell hooks, optional autostart
- `lifecycle/uninstall` - clean removal with --keep-data and --print options
- `lifecycle/doctor` - 13 health checks, --network gate, JSON output
- `lifecycle/clean` - snapshot and cache pruning with recovery manifest protection

**Supporting layer**:
- `ports` - 11 port interfaces (HTTPPort, FsPort, CosignPort, SpawnPort, ClockPort, PathsPort, EnvPort, ShellRcPort, CompletionPort, AutostartPort, PromptPort)
- `adapters` - 11 production implementations + 2 bridge adapters for nominal type compatibility between shipkit/ports and lifecycle/update/ports

All packages follow the hexagonal ports pattern: every verb accepts IO interface
arguments so unit tests inject fakes and reach 100% statement coverage without
network calls.

Network-bound functions (cosign TUF, GitHub release queries) follow the
`sigstoreRealVerify` pattern: the adapter returns `ErrNotConfigured` by default;
the consumer CLI wires the real implementation in its `cmd/` layer.

## Modules

| Module path | Purpose | Tag |
|-------------|---------|-----|
| `github.com/fede-iglesias/shipkit` | Config, RegisterLifecycle, 5 verb getters, Option DI | `v0.1.0` |
| `github.com/fede-iglesias/shipkit/frontmatter` | YAML frontmatter round-trip | `frontmatter/v0.1.0` |
| `github.com/fede-iglesias/shipkit/store` | Atomic write, file lock, checksum | `store/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/migrations` | Ordered migration registry | `lifecycle/migrations/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/update` | Cosign-verified atomic self-update | `lifecycle/update/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/install` | Config dirs, completions, autostart | `lifecycle/install/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/uninstall` | Clean removal | `lifecycle/uninstall/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/doctor` | 13 health checks, JSON output | `lifecycle/doctor/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/clean` | Snapshot and cache pruning | `lifecycle/clean/v0.1.0` |
| `github.com/fede-iglesias/shipkit/ports` | 11 port interfaces | `ports/v0.1.0` |
| `github.com/fede-iglesias/shipkit/adapters` | 11 production adapters + 2 bridges | `adapters/v0.1.0` |

## Distribution

shipkit packages publish via the standard Go module proxy (pkg.go.dev). Consumers
that build CLIs on top of shipkit ship their own binaries via their own release
pipeline. See `kt` for the reference setup: goreleaser + cosign keyless signing +
a separate public `tools` repo that hosts `install.sh` and release assets.
The `kt upgrade` command is the first consumer of `lifecycle/update`.

## License

[MIT](LICENSE)
