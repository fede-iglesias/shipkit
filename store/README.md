# shipkit/store

Universal storage primitives: atomic file writes, advisory locks, content checksums, and safe path resolution for the knowledge card layout.

## When to use

Use this package when your shipkit consumer needs any of:

- Writing files safely (no partial writes visible to readers)
- Computing content-addressable checksums to detect changes
- Serializing concurrent access to a shared resource via advisory locks
- Resolving or reverse-mapping file paths for knowledge card kinds

## 30-second quickstart

```go
import (
    "github.com/fede-iglesias/shipkit/store"
    "time"
)

// 1. Write a file atomically.
err := store.AtomicWrite("/project/knowledge/plans/q3.md", []byte("# Q3\n"), 0o644)

// 2. Check whether the body changed before writing.
newBody := []byte("updated content\n")
if store.BodyChecksum(newBody) != store.BodyChecksum(oldBody) {
    _ = store.AtomicWrite(path, newBody, 0o644)
}

// 3. Protect a critical section with an advisory lock.
lk, err := store.Acquire("/project/.locks/plans.lock", 5*time.Second)
if err != nil { /* handle ErrLockTimeout */ }
defer lk.Release()
// ... critical section ...
```

## Patterns

### Atomic write + checksum verify

Write only when content changes to avoid spurious file system churn:

```go
existing, _ := os.ReadFile(path)
if store.BodyChecksum(existing) == store.BodyChecksum(newData) {
    return nil // nothing changed
}
return store.AtomicWrite(path, newData, 0o644)
```

### Advisory lock around critical section

Coordinate concurrent writers on the same file:

```go
lk, err := store.Acquire(path+".lock", 5*time.Second)
if err != nil {
    return fmt.Errorf("acquire lock: %w", err)
}
defer lk.Release()

// Safe to read-modify-write path here.
```

### Path validation against kind layout

Resolve a card path from kind + slug and reverse-map a filesystem path back to its kind:

```go
p, err := store.PathFor("/project", "adr", "use-postgres", store.WithADRID("ADR-0042"))
// p = /project/knowledge/decisions/0042-use-postgres.md

kind, err := store.KindFromPath("knowledge/decisions/0042-use-postgres.md")
// kind = "adr"
```

## Gotchas

- **Lock is advisory**: only callers that use Acquire participate in mutual exclusion. Processes that open the same file directly bypass the lock entirely.
- **AtomicWrite requires same-filesystem temp**: the temp file is created in the same directory as the destination. Cross-device writes will fail at the rename step.
- **BodyChecksum normalizes trailing whitespace**: two bodies that differ only in trailing spaces/newlines produce the same digest. This is intentional for document equality checks.
- **mkdirAllFn and walkDirFn are package-level vars**: they exist for test injection only. Do not reassign them in production code - the package does not protect concurrent access to these globals.

## API reference

[pkg.go.dev/github.com/fede-iglesias/shipkit/store](https://pkg.go.dev/github.com/fede-iglesias/shipkit/store)
