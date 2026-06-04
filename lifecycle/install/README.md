# shipkit/lifecycle/install

Package `install` provides the `install` lifecycle verb for shipkit-based CLIs.
It sets up user-scope state after the binary has been placed on disk by `install.sh`.

Import path: `github.com/fede-iglesias/shipkit/lifecycle/install`

pkg.go.dev: https://pkg.go.dev/github.com/fede-iglesias/shipkit/lifecycle/install

## When to use this package

Use this package when building a CLI that needs to set up XDG data/config/cache
directories, install shell completions, and optionally register an autostart
service unit - all from the user's perspective, without sudo.

The `install` verb is meant to be run once by the user after they have downloaded
the binary via `install.sh`. It is safe to re-run (idempotent via the JSON marker).

## 30-second quickstart

```go
import (
    "context"
    "github.com/fede-iglesias/shipkit/lifecycle/install"
    "github.com/fede-iglesias/shipkit/ports"
    "github.com/spf13/cobra"
)

func main() {
    deps := install.Deps{
        Cfg: install.Config{
            AppName: "myapp",
            Version: version, // injected via -ldflags
        },
        FS:         realFsAdapter,
        Paths:      realPathsAdapter,
        Env:        realEnvAdapter,
        ShellRc:    realShellRcAdapter,
        Completion: realCompletionAdapter,
        Autostart:  realAutostartAdapter,
        Prompt:     realPromptAdapter,
        Clock:      realClockAdapter,
    }
    root := &cobra.Command{Use: "myapp"}

    // Build the cobra subcommand and attach it to the root.
    installCmd, err := install.NewCommand(deps)
    if err != nil {
        log.Fatal(err)
    }
    root.AddCommand(installCmd)
}
```

In unit tests, replace every adapter with the mock ports from `shipkit/ports`:

```go
func TestMyInstall(t *testing.T) {
    deps := install.Deps{
        Cfg:        install.Config{AppName: "myapp", Version: "v0.1.0"},
        FS:         ports.NewMockFsPort(),
        Paths:      ports.NewMockPathsPort(),
        Env:        ports.NewMockEnvPort(),
        ShellRc:    ports.NewMockShellRcPort(),
        Completion: ports.NewMockCompletionPort(),
        Autostart:  ports.NewMockAutostartPort(),
        Prompt:     ports.NewMockPromptPort(),
        Clock:      ports.NewMockClockPort(time.Now()),
    }
    result, err := install.Run(t.Context(), deps, install.Options{}, &cobra.Command{Use: "myapp"})
    if err != nil {
        t.Fatal(err)
    }
    // assert on result.Marker, result.CompletionsWritten, result.Manifest...
}
```

## Common patterns

### Default install (autodetect shell)

```go
result, err := install.Run(ctx, deps, install.Options{}, rootCmd)
```

Detects the shell via `EnvPort.DetectShell` and installs completions accordingly.
Writes the marker to `{DataRoot}/.shipkit.installed`.

### Install with autostart

```go
// Requires Config.EnableAutostart = true.
result, err := install.Run(ctx, deps, install.Options{Autostart: true}, rootCmd)
```

Registers a platform autostart unit (LaunchAgent on darwin, systemd-user on linux)
via `AutostartPort`.

### Dry-run via --print

```go
result, err := install.Run(ctx, deps, install.Options{Print: true}, rootCmd)
```

Prints the plan to stderr. No files are written, no directories created.
`result.Manifest` will be empty.

## Gotchas

### Bash 3.2 on macOS

macOS ships Bash 3.2 (GPLv2). Programmable completion (`compinit`, `_myapp`)
requires Bash >= 4. The install verb detects this via `EnvPort.Get("BASH_VERSION")`
on darwin and skips bash completion installation, emitting a warning to stderr:

```
warning: skipping bash completions - darwin ships Bash 3.2.57(1)-release (requires >= 4)
  to get a modern bash: brew install bash
```

This is a skip, not an error. The rest of the install proceeds normally.

### Idempotency and --force

Without `--force`, a second `install` run on an already-installed system is a
no-op. `Result.AlreadyInstalled` will be `true` and no files are written.

Use `--force` to re-run all steps, overwriting completion files and the marker.

### Fish shell skips shellrc injection

Fish autoloads completion scripts from `~/.config/fish/completions/`. The install
verb installs the completion file there but does NOT inject a block into any RC
file. `ShellRcPort.EnsureBlock` is not called for fish.
