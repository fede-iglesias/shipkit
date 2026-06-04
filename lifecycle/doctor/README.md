# shipkit/lifecycle/doctor

Read-only health inspection for shipkit-powered CLIs.

## When to use this package

Import this package when you want to add a `doctor` subcommand to your CLI that
inspects the installation health and reports pass/warn/fail/skipped outcomes for
each component, without mutating any state.

## Quick start

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/fede-iglesias/shipkit/lifecycle/doctor"
    "github.com/fede-iglesias/shipkit/ports"
)

func main() {
    deps := doctor.Deps{
        AppName:  "myapp",
        BinPath:  "/usr/local/bin/myapp",
        Version:  "0.1.0",
        DataRoot: os.ExpandEnv("$HOME/.local/share/myapp"),
        // ... wire remaining fields and Func fields
    }
    report, err := doctor.Run(context.Background(), deps, doctor.Options{})
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(2)
    }
    fmt.Print(doctor.FormatText(report, false))
    os.Exit(doctor.ExitCode(report))
}
```

## Cobra integration (30-second pattern)

```go
root.AddCommand(doctor.NewCommand(deps))
// Registers: doctor [--network] [--json] [--verbose]
```

## Common patterns

### Wiring filesystem functions in the consumer cmd layer

```go
deps.StatExecutableFunc = func(path string) (bool, error) {
    info, err := os.Stat(path)
    if err != nil { return false, err }
    return info.Mode()&0111 != 0, nil
}
deps.StatDirFunc = func(path string) (bool, error) {
    _, err := os.Stat(path)
    if os.IsNotExist(err) { return false, nil }
    return err == nil, err
}
deps.StatFileFunc = func(path string) (bool, error) {
    _, err := os.Stat(path)
    if os.IsNotExist(err) { return false, nil }
    return err == nil, err
}
deps.ReadMarkerFunc = func(path string) (string, error) {
    data, err := os.ReadFile(path)
    return string(data), err
}
```

### Enabling network checks

```go
report, _ := doctor.Run(ctx, deps, doctor.Options{Network: true})
```

Network checks use the `CheckNetworkGitHubFunc`, `CheckNetworkCosignTUFFunc`, and
`CheckNetworkUpdateFeedFunc` fields on Deps. Wire them in the consumer cmd layer
using HTTPPort and the app's Repo/TagPrefix values.

### JSON output

```go
opts := doctor.Options{JSON: true}
report, _ := doctor.Run(ctx, deps, opts)
data, _ := json.Marshal(report)
// {"checks":[...],"summary":{"pass":9,"warn":0,"fail":0,"skipped":3,"ok":true}}
```

### Interpreting exit codes

```go
os.Exit(doctor.ExitCode(report))
// 0: no failures (warnings are OK)
// 1: at least one check failed
```

## Gotchas and edge cases

- **Stat functions are nil by default.** When not wired, affected checks return
  StatusWarn instead of failing. This is intentional: it prevents panics in
  consumers that do not need every check.

- **Network checks are opt-in.** They require `--network` or `Options.Network: true`
  and their corresponding `CheckNetwork*Func` fields. Without them, the checks appear
  in the report as StatusSkipped.

- **Recovery manifest means update failure.** If `.shipkit.recovery-manifest.json`
  exists in DataRoot, the recovery.manifest check fails. Run `myapp update` to retry
  the update or `myapp install --force` to recover.

- **Autostart not enabled.** When `AutostartLabel` is empty, the autostart check
  reports "not enabled in this app" with StatusPass. This is correct: the app did
  not configure autostart, so no service is expected to be running.

## Godoc

https://pkg.go.dev/github.com/fede-iglesias/shipkit/lifecycle/doctor
