# shipkit/lifecycle/clean

The `clean` package implements the `clean` lifecycle verb for shipkit-powered CLIs. It removes stale artifacts (snapshots, tmp work dirs, cache entries, log files) from a CLI's data directory with explicit user confirmation and strong safety defaults.

## When to use this package

Use `lifecycle/clean` when:

- You want to expose a `clean` subcommand in your CLI
- You need to reclaim disk space occupied by old update snapshots
- You want to clean up tmp or cache directories left behind by failed or completed operations

## 30-second quick start

```go
package main

import (
    "os"

    "github.com/fede-iglesias/shipkit/adapters"
    "github.com/fede-iglesias/shipkit/lifecycle/clean"
    "github.com/spf13/cobra"
)

func main() {
    deps := clean.Deps{
        AppName:           "myapp",
        FS:                adapters.NewRealFs(),
        Paths:             adapters.NewPathsXDG(),
        Clock:             adapters.NewRealClock(),
        Prompt:            adapters.NewPromptTerm(),
        ListSnapshotsFunc: clean.DefaultListSnapshots,
        ListTmpFunc:       clean.DefaultListTmp,
        ListCacheFunc:     clean.DefaultListCache,
        ListLogsFunc:      clean.DefaultListLogs,
        ReadManifestFunc:  clean.DefaultReadManifest,
    }

    root := &cobra.Command{Use: "myapp"}
    root.AddCommand(clean.NewCommand(deps))

    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

## Common patterns

### Clean all old snapshots keeping the newest 3

```go
result, err := clean.Run(ctx, deps, clean.Options{
    Snapshots: true,
    Keep:      3,
    Yes:       true,
})
```

### Dry-run to see what would be cleaned

```go
result, err := clean.Run(ctx, deps, clean.Options{
    Snapshots: true,
    All:       true,
    OlderThan: 7 * 24 * time.Hour,
    Print:     true, // no changes made
})
for _, item := range result.Items {
    fmt.Printf("[%s] %s\n", item.Category, item.Path)
}
```

### Interactive clean (prompts for confirmation)

```go
result, err := clean.Run(ctx, deps, clean.Options{
    Snapshots: true,
    // Yes not set - will prompt the user
})
```

### Clean all scopes

```go
result, err := clean.Run(ctx, deps, clean.Options{
    All:       true,
    OlderThan: 30 * 24 * time.Hour,
    Yes:       true,
})
fmt.Printf("Reclaimed %d bytes\n", result.Reclaimed)
```

### Wire into cobra as a subcommand

```go
root.AddCommand(clean.NewCommand(deps))
// Exposes: myapp clean --snapshots --keep 3 -y
```

## Gotchas and edge cases

- **No flags = exit 1**: Calling `clean` with no scope flag (--snapshots, --tmp, --cache, --logs, --all) returns `ErrNoScope` and prints help. This prevents accidental mass deletion.
- **Recovery manifest protection**: If `.shipkit.recovery-manifest.json` exists in DataDir and references a snapshot, that snapshot is NEVER deleted regardless of age or `--keep`.
- **Symlink escape prevention**: Any snapshot entry whose path resolves to a symlink pointing outside DataDir is refused and left in place.
- **--keep overrides --older-than**: When `--keep N` is set, the newest N snapshots are preserved even if their age exceeds `--older-than`. This ensures rollback is always available.
- **Default retention**: `--older-than` defaults to 720h (30 days) when not specified.
- **Idempotent**: Running clean with the same flags when there is nothing to clean returns exit 0 with no error.

## Godoc

See [pkg.go.dev/github.com/fede-iglesias/shipkit/lifecycle/clean](https://pkg.go.dev/github.com/fede-iglesias/shipkit/lifecycle/clean).
