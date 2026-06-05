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

// In cmd/myapp/main.go:
cos := adapters.NewSigstoreCosign()
cos.CertIdentityRegex = `https://github\.com/owner/myapp/.*`
cos.OIDCIssuer = "https://token.actions.githubusercontent.com"

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

As of v0.2.4 `NewSigstoreCosign` wires the real sigstore-go verifier (TUF +
Rekor) as the default. Consumers only need to set `CertIdentityRegex` and
`OIDCIssuer` for their repo; no `SetVerifyCore` call is required for the
adapter to function. `SetVerifyCore` is still exported so tests can inject a
stub that avoids network calls. See `adapters/cosign_sigstore.go` and
`adapters/cosign_sigstore_real.go`.

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
