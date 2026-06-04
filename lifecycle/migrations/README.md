# migrations

Ordered, versioned migration registry for shipkit consumers.

## When to use

Use this package when your CLI tool needs to transform user data (files,
config, directory structure) as it upgrades from one version to another.
Each migration is a named, semver-tagged, idempotent operation that can be
applied forward or reverted backward.

## 30-second quickstart

```go
import (
    "context"
    "fmt"
    "github.com/fede-iglesias/shipkit/lifecycle/migrations"
)

// Step 1: implement the Migration interface.
type renameConfig struct{}

func (m *renameConfig) Version() string                         { return "0.2.0" }
func (m *renameConfig) Description() string                     { return "rename config.yaml to shipkit.yaml" }
func (m *renameConfig) Apply(_ context.Context, root string) error {
    // rename root/config.yaml -> root/shipkit.yaml (idempotent)
    return nil
}
func (m *renameConfig) Revert(_ context.Context, root string) error {
    // rename root/shipkit.yaml -> root/config.yaml (idempotent)
    return nil
}

// Step 2: register and apply.
func main() {
    r := migrations.New()
    r.Register(&renameConfig{})

    count, err := r.ApplyPending(context.Background(), "/home/user/.myapp", "0.1.0", "0.2.0")
    if err != nil {
        panic(err)
    }
    fmt.Printf("applied %d migration(s)\n", count)
}
```

## Common patterns

### Pattern 1: register multiple migrations in any order

```go
r := migrations.New()
// Register in any order - registry always maintains semver ascending sort.
r.Register(&migration030{})
r.Register(&migration010{})
r.Register(&migration020{})

pending, _ := r.Pending("", "0.3.0")
// pending[0].Version() == "0.1.0"
// pending[1].Version() == "0.2.0"
// pending[2].Version() == "0.3.0"
```

### Pattern 2: apply a contiguous range

```go
// Only apply migrations strictly after "0.1.5" and up to "0.3.0".
count, err := r.ApplyPending(ctx, dataRoot, "0.1.5", "0.3.0")
```

### Pattern 3: revert a range on rollback

```go
// Undo everything from 0.3.0 back to (but not including) 0.1.0.
// Reverts in reverse semver order: 0.3.0, 0.2.0, 0.1.1, etc.
err := r.Revert(ctx, dataRoot, "0.3.0", "0.1.0")
```

## Gotchas

- **Semver lex order vs numeric**: migrations are sorted numerically by
  MAJOR.MINOR.PATCH segments. "0.10.0" sorts after "0.9.0" (correct). Do
  NOT rely on lexicographic order.

- **Idempotency contract**: every Apply and Revert implementation MUST be
  idempotent. The registry does not track what has been applied; callers
  (typically the update verb) are responsible for passing the correct
  `current` version to Pending/ApplyPending/Revert.

- **Apply stops on first error**: if a migration returns an error, ApplyPending
  stops and returns the partial count. Migrations applied before the failure
  are NOT automatically reverted. The caller must handle rollback explicitly
  via Revert if needed.

## pkg.go.dev

https://pkg.go.dev/github.com/fede-iglesias/shipkit/lifecycle/migrations
