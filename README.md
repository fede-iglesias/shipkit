# shipkit

Cobra lifecycle toolkit for personal Go CLIs (install / update / uninstall / doctor / clean).

[![Go Reference](https://pkg.go.dev/badge/github.com/fede-iglesias/shipkit.svg)](https://pkg.go.dev/github.com/fede-iglesias/shipkit)
[![CI](https://github.com/fede-iglesias/shipkit/actions/workflows/ci.yml/badge.svg)](https://github.com/fede-iglesias/shipkit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Status

v0.1.0 in progress. Multi-module mono-repo: each lifecycle verb and primitive is
its own Go module so consumers only take what they need. Individual modules reach
v0.1.0 after their extract batch lands. See CHANGELOG for progress.

## Install

Root module (wires all verbs):

```bash
go get github.com/fede-iglesias/shipkit@latest
```

Individual modules (after their respective batch lands in v0.1.0):

```bash
go get github.com/fede-iglesias/shipkit/lifecycle/update@latest
go get github.com/fede-iglesias/shipkit/lifecycle/install@latest
go get github.com/fede-iglesias/shipkit/frontmatter@latest
go get github.com/fede-iglesias/shipkit/store@latest
```

## Quickstart

Wire all five lifecycle verbs into a cobra root in one call:

```go
import (
    "github.com/fede-iglesias/shipkit"
    "github.com/spf13/cobra"
)

func main() {
    root := &cobra.Command{Use: "myapp"}
    cfg := shipkit.Config{
        AppName:   "myapp",
        Repo:      "your-org/tools",  // fede-iglesias/tools for personal CLIs
        TagPrefix: "myapp-",
        Version:   version,           // injected via -ldflags at build time
    }
    // RegisterLifecycle adds install / update / uninstall / doctor / clean subcommands.
    if err := shipkit.RegisterLifecycle(root, cfg); err != nil {
        log.Fatal(err)
    }
    root.Execute()
}
```

The full API (`RegisterLifecycle`, `Config`, per-verb getters, `Option` variants)
stabilizes when v0.1.0 lands after the extract batches complete.

## Architecture

shipkit is structured in two phases:

1. **Extract base**: primitive packages ported from an existing production CLI
   (`frontmatter`, `store`, `lifecycle/migrations`, `lifecycle/update`) with full
   test suites preserved.

2. **New verbs**: lifecycle verbs written from scratch (`install`, `uninstall`,
   `doctor`, `clean`) and the public `shipkit` root that wires them all via
   `RegisterLifecycle`.

All packages use the hexagonal ports pattern: each verb accepts IO interface
arguments (HTTP, FS, Cosign, Spawn, Paths, Env, etc.) so unit tests can swap in
fakes and achieve 100% statement coverage without network calls.

## Modules

| Module path | Purpose | Target tag |
|-------------|---------|------------|
| `github.com/fede-iglesias/shipkit` | Root: Config, RegisterLifecycle, wiring | `v0.1.0` |
| `github.com/fede-iglesias/shipkit/frontmatter` | YAML frontmatter round-trip | `frontmatter/v0.1.0` |
| `github.com/fede-iglesias/shipkit/store` | Atomic write, file lock, checksum | `store/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/migrations` | Ordered migration registry | `lifecycle/migrations/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/update` | Cosign-verified atomic self-update | `lifecycle/update/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/install` | Config dirs, completions, autostart | `lifecycle/install/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/uninstall` | Clean removal | `lifecycle/uninstall/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/doctor` | Health checks | `lifecycle/doctor/v0.1.0` |
| `github.com/fede-iglesias/shipkit/lifecycle/clean` | Snapshot and cache pruning | `lifecycle/clean/v0.1.0` |

## Distribution

shipkit packages publish via the standard Go module proxy (pkg.go.dev). Consumers
that build CLIs on top of shipkit ship their binaries via their own release
pipeline. See `kt` for the reference setup: goreleaser, cosign keyless signing,
and publishing to a separate public `tools` repo that hosts the install.sh and
release assets.

## License

[MIT](LICENSE)
