# shipkit/lifecycle/uninstall

Remove a shipkit-powered CLI application and all of its user-state from the
machine.

## When to use

Use this package to wire an `uninstall` subcommand into your CLI. It handles:
the autostart service, shell completion files, RC-file guarded blocks, XDG
data/config/cache directories, and the binary itself. All behaviour is
controlled via injected ports - no hard-coded paths, no OS calls in test scope.

## 30-second quickstart

```go
import (
    "context"
    "os"

    "github.com/fede-iglesias/shipkit/lifecycle/uninstall"
    "github.com/fede-iglesias/shipkit/ports"
)

// Wire production deps.
deps := uninstall.Deps{
    AppName:    "myapp",
    BinPath:    "/usr/local/bin/myapp",
    FS:         realFsPort,
    Paths:      xdgPathsPort,
    ShellRc:    realShellRcPort,
    Completion: cobraCompletionPort,
    Autostart:  realAutostartPort,
    Prompt:     termPromptPort,
    // RemoveBinaryFunc lives in cmd layer (sigstoreRealVerify pattern).
    RemoveBinaryFunc: func(p string) error { return os.Remove(p) },
    // Schedule a clean exit after binary self-delete.
    ScheduledExitFunc: func() {
        go func() {
            time.Sleep(200 * time.Millisecond)
            os.Exit(0)
        }()
    },
}

result, err := uninstall.Run(ctx, deps, uninstall.Options{}, rootCmd)
```

For cobra wiring use `NewCommand`:

```go
root.AddCommand(uninstall.NewCommand(deps, root))
```

## Common patterns

### Default: full uninstall with confirmation

```go
result, err := uninstall.Run(ctx, deps, uninstall.Options{}, root)
// Prompts "This will remove myapp ... Proceed? [y/N]"
// On confirm: tears down autostart, completions, RC blocks, dirs, binary.
```

### --keep-data: preserve user data

```go
result, err := uninstall.Run(ctx, deps, uninstall.Options{KeepData: true}, root)
// XDG data dir is skipped; config, cache, completions, binary still removed.
```

### --print: dry-run preview

```go
result, err := uninstall.Run(ctx, deps, uninstall.Options{Print: true}, root)
// No prompt, no mutations. result.Removed lists what would be deleted.
fmt.Println(result.Removed)
```

## Gotchas

**Binary self-delete on Unix:**
`os.Remove` on a running binary on Linux/darwin succeeds immediately - the
kernel releases the inode when the process exits. Wire `RemoveBinaryFunc` in
your cmd layer:

```go
deps.RemoveBinaryFunc = func(p string) error { return os.Remove(p) }
```

If the binary is owned by root (installed via sudo), `os.Remove` fails with
EPERM. The result is `BinaryDeleteRequested` with a `sudo rm` hint in
`NextSteps`. The package never calls sudo itself.

**ScheduledExitFunc vs BinaryDeletedNow:**
When the binary removes itself, the process must exit cleanly to release the
inode. Wire `ScheduledExitFunc` to schedule an `os.Exit` after `Run` returns,
giving the caller time to flush logs. If you don't wire it, the binary is
marked `BinaryDeletedNow` but no exit is forced - you are responsible for
exiting.

**Linux NFS volumes:**
`os.Remove` may return EBUSY on NFS mounts. The result falls back to
`BinaryDeleteRequested` with the manual-removal hint.

**RemoveBinaryFunc is nil by default:**
If you don't wire `RemoveBinaryFunc`, the binary is never touched and
`BinaryAction` is `BinaryKept`. This is the safe default for library consumers
who wire production adapters lazily.

## pkg.go.dev

https://pkg.go.dev/github.com/fede-iglesias/shipkit/lifecycle/uninstall
