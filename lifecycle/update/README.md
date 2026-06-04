# lifecycle/update

Self-update state machine for Go CLIs distributed via GitHub Releases with sigstore cosign verification.

## Overview

This package implements a full update lifecycle:

- snapshot current binary
- download latest release
- verify sigstore cosign bundle (embedded sigstore-go, no cosign binary required)
- atomic replace
- run data migrations
- health check
- rollback cascade on any failure

## Usage

```go
import (
    "github.com/fede-iglesias/shipkit/lifecycle/update"
    "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
)

// In cmd/myapp/main.go or init:
adapters.SetVerifyCore(sigstoreRealVerify) // wire from cmd layer
update.SetOrchestratorFactory(update.NewOrchestratorFactory())

cfg := update.Config{
    Repo:               "owner/myapp",
    TagPrefix:          "myapp-",
    BinaryPath:         binaryPath,
    DataRoot:           dataRoot,
    SnapshotDir:        filepath.Join(dataRoot, "snapshots"),
    HealthCheckTimeout: 5 * time.Second,
}

result, err := update.Run(ctx, cfg, update.RunOpts{})
```

## sigstore wiring

`sigstoreRealVerify` (TUF + Rekor network calls) must live in the consumer cmd
layer. The adapter returns `ErrCosignNotConfigured` until `SetVerifyCore` is
called. See `adapters/cosign_sigstore.go`.

## Ports

- `ports.HTTPPort` - GitHub Releases API
- `ports.FsPort` - filesystem (snapshot, restore, atomic replace, tar.gz extract)
- `ports.CosignPort` - sigstore bundle verification
- `ports.SpawnPort` - binary health check via subprocess
- `ports.ClockPort` - wall clock (injectable for tests)

## Adapters

- `adapters.NewGitHubHTTP()` - anonymous GitHub REST API client
- `adapters.NewRealFs()` - production filesystem adapter
- `adapters.NewSigstoreCosign()` - sigstore-go embedded adapter
- `adapters.NewRealSpawn()` - subprocess health check adapter
