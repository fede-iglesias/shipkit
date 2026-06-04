# lifecycle/recovery

Shared on-disk recovery manifest format for shipkit lifecycle modules.

## Overview

When a self-update fails and rollback cannot complete cleanly, lifecycle/update
writes a small JSON manifest at the well-known path under the application's XDG
data root. Two downstream consumers read it:

- lifecycle/clean: protects the referenced snapshot from deletion.
- lifecycle/doctor: surfaces a pending recovery to the operator.

## Import path

```
github.com/fede-iglesias/shipkit/lifecycle/recovery
```

## API

- `recovery.Filename` constant: canonical filename (`.shipkit.recovery-manifest.json`).
- `recovery.Path(dataRoot)`: returns the joined absolute path under a data root.
- `recovery.Manifest`: typed struct describing the on-disk JSON shape.
- `recovery.Write(path, m)`: atomic writer (temp+rename), used by lifecycle/update.
- `recovery.Read(path)`: reader; missing file returns an error satisfying `errors.Is(err, fs.ErrNotExist)`.

## Usage

```go
import (
    "time"

    "github.com/fede-iglesias/shipkit/lifecycle/recovery"
)

m := recovery.Manifest{
    Version:      1,
    AppName:      "myapp",
    SnapshotPath: "/var/lib/myapp/snapshots/snap-2026-06-01",
    Steps:        []string{"pre-update", "snapshot", "download"},
    Cause:        "verify failed",
    CreatedAt:    time.Now().UTC(),
}
if err := recovery.Write(recovery.Path(dataRoot), m); err != nil {
    return err
}

got, err := recovery.Read(recovery.Path(dataRoot))
```

## Atomicity

`Write` creates the manifest via a temp file in the parent directory and renames
it into place. Readers either see no file at all or the fully serialised JSON.

## Dependencies

Stdlib only.
