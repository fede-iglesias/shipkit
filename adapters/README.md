# shipkit/adapters

Production implementations of all 11 port interfaces declared in
`github.com/fede-iglesias/shipkit/ports`.

Import path: `github.com/fede-iglesias/shipkit/adapters`

pkg.go.dev: https://pkg.go.dev/github.com/fede-iglesias/shipkit/adapters

## When to use this package

Use `shipkit/adapters` when wiring a production CLI. It provides one concrete
adapter per port interface, ready to pass to `shipkit.RegisterLifecycle` or to
any per-verb `Deps` struct via `shipkit.WithXxxPort(...)`.

Do NOT use this package in unit tests - inject mock ports from
`shipkit/ports` instead.

## 11 adapters

| Constructor | Port | Backed by |
|-------------|------|-----------|
| `NewGitHubHTTP()` | `HTTPPort` | Anonymous GitHub REST API (net/http) |
| `NewRealFs()` | `FsPort` | os + io/fs + archive/tar + compress/gzip |
| `NewSigstoreCosign()` | `CosignPort` | sigstore-go embedded (no cosign binary) |
| `NewRealSpawn()` | `SpawnPort` | os/exec |
| `NewRealClock()` | `ClockPort` | time.Now() |
| `NewPathsXDG()` | `PathsPort` | os.UserHomeDir + XDG env vars |
| `NewEnvOS()` | `EnvPort` | os.Getenv + runtime.GOOS/GOARCH |
| `NewShellRcReal()` | `ShellRcPort` | RC file guarded-block insert/remove |
| `NewCompletionCobra()` | `CompletionPort` | cobra shell completion generator |
| `NewAutostartReal()` | `AutostartPort` | launchctl (darwin) / systemctl --user (linux) |
| `NewPromptTerm()` | `PromptPort` | bufio.Scanner on os.Stdin |

### Bridge adapters

`lifecycle/update/ports` defines structurally identical but nominally distinct
types (`Release`, `Asset`, `HealthResult`) to `shipkit/ports`. Two bridge
adapters convert between them so the update orchestrator can be wired via the
common `shipkit/ports` interfaces:

| Constructor | Wraps | Converts to |
|-------------|-------|-------------|
| `NewHTTPBridge()` | `NewGitHubHTTP()` | `shipkit/ports.HTTPPort` |
| `NewSpawnBridge()` | `NewRealSpawn()` | `shipkit/ports.SpawnPort` |

## 30-second quickstart

```go
import (
    "github.com/fede-iglesias/shipkit"
    "github.com/fede-iglesias/shipkit/adapters"
    "github.com/spf13/cobra"
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

    // The default path: RegisterLifecycle wires all production adapters automatically.
    if err := shipkit.RegisterLifecycle(root, cfg); err != nil {
        // handle
    }

    // Custom injection: swap one adapter, keep the rest at production defaults.
    if err := shipkit.RegisterLifecycle(root, cfg,
        shipkit.WithFsPort(adapters.NewRealFs()),
    ); err != nil {
        // handle
    }
}
```

## sigstoreRealVerify pattern

`NewSigstoreCosign()` returns an adapter that returns `ErrCosignNotConfigured`
until `SetVerifyCore` is called. Wire the real TUF + Rekor verification function
from your consumer `cmd/` layer:

```go
import "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"

// In cmd/myapp/main.go:
adapters.SetVerifyCore(sigstoreRealVerify)
```

This keeps `sigstore-go` TUF network calls out of `pkg/` scope, enabling 100%
unit test coverage without real network calls.

## Gotchas

- `NewAutostartReal()` returns `ErrAutostartUnsupported` on platforms without
  user-scope service management (Alpine OpenRC, NixOS variants). Handle via
  `errors.Is(err, autostart.ErrAutostartUnsupported)`.
- `NewGitHubHTTP()` and `NewHTTPBridge()` share the same underlying HTTP client
  but are distinct instances. Do not assume they share state.
- `NewRealFs().AtomicWrite` creates the temp file in the same directory as the
  destination. Cross-device writes fail at the rename step.
