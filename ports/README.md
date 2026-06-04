# shipkit/ports

Port interfaces (Hexagonal Architecture) for the shipkit lifecycle verbs.

## When to use this package

Import `shipkit/ports` when you are:

- Authoring a custom shipkit lifecycle verb and need to declare your dependency
  surface via a `Deps` struct.
- Writing unit tests for a verb and need the `MockXxx` helpers to inject fakes
  without real I/O.
- Implementing a custom adapter (e.g. a mock HTTP server, a VFS) that satisfies
  one of the 11 interfaces.

Do NOT import this package to call OS functions directly - use `shipkit/adapters`
for production wiring.

## 11 port interfaces

| Port | Purpose | Used by |
|------|---------|---------|
| `HTTPPort` | Outbound HTTP: release discovery + asset download | update, doctor |
| `FsPort` | Filesystem: snapshot, restore, atomic replace, extract, copy, rmdir | install, update, uninstall, clean |
| `CosignPort` | sigstore bundle verification (embedded, no os/exec) | update |
| `SpawnPort` | Binary health check after install | update, doctor |
| `ClockPort` | Injectable time source for deterministic tests | all verbs |
| `PathsPort` | XDG dirs, home dir, binary path, PATH inspection | install, uninstall, doctor |
| `EnvPort` | Env var access, shell/OS/arch detection | install, doctor |
| `ShellRcPort` | Guarded block insert/remove in shell RC files | install, uninstall |
| `CompletionPort` | Shell completion script generation + path resolution | install, uninstall |
| `AutostartPort` | Platform service management (LaunchAgent / systemd-user) | install, uninstall, doctor |
| `PromptPort` | Interactive y/n confirmation for destructive ops | uninstall, clean |

## 30-second example

```go
package install

import (
    "context"

    "github.com/fede-iglesias/shipkit/ports"
)

type Deps struct {
    Paths  ports.PathsPort
    Env    ports.EnvPort
    ShellRc ports.ShellRcPort
    Clock  ports.ClockPort
}

func Run(ctx context.Context, deps Deps) error {
    dataDir, err := deps.Paths.DataDir("myapp")
    if err != nil {
        return err
    }
    shell := deps.Env.DetectShell()
    _, err = deps.ShellRc.EnsureBlock("~/.zshrc", "myapp:fpath", "fpath+=("+dataDir+")")
    return err
}
```

In tests, inject mocks:

```go
func TestRun_happy(t *testing.T) {
    deps := install.Deps{
        Paths:   ports.NewMockPathsPort(),
        Env:     ports.NewMockEnvPort(),
        ShellRc: ports.NewMockShellRcPort(),
        Clock:   ports.NewMockClockPort(time.Now()),
    }
    if err := install.Run(context.Background(), deps); err != nil {
        t.Fatal(err)
    }
}
```

## Gotchas

- `CompletionPort.EmitCompletion` and `CompletionPort.CompletionPath` require
  `*cobra.Command`. This package therefore imports `github.com/spf13/cobra`.
  This is intentional: shipkit is a cobra-app builder toolkit. If cobra is not
  acceptable in your build, do not import `shipkit/ports/completion.go` directly
  - create a wrapper package that removes the cobra coupling (not recommended).

- Bash 3.2 on macOS (the system default) does not support bash completion v2.
  The install verb detects this via `EnvPort.DetectOS()` + bash version and
  skips bash completion with a warning. `CompletionPort` itself does not enforce
  this constraint; the decision lives in the install verb.

- `AutostartPort.Install` returns `ErrAutostartUnsupported` on platforms that
  lack user-scope service management (Alpine OpenRC, NixOS variants). The doctor
  verb surfaces this as a warning.

- `ShellRcPort.EnsureBlock` is idempotent: calling it twice with the same content
  returns `EnsureResult{Unchanged: true}` on the second call. Callers must NOT
  assume a write happened just because no error was returned.

## godoc

https://pkg.go.dev/github.com/fede-iglesias/shipkit/ports
