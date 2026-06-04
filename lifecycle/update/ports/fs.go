package ports

import "context"

// FsPort abstracts all filesystem operations used by the update process.
// Implementations must be safe for concurrent use and must treat each
// operation as atomic from the caller's perspective.
type FsPort interface {
	// Snapshot copies the file at src into snapshotDir, tagging it with a
	// timestamp-based identifier, and returns the resulting snapshot ID.
	// The ID is an opaque string that can later be passed to Restore.
	Snapshot(ctx context.Context, src, snapshotDir string) (string, error)

	// Restore copies the snapshot identified by snapshotID back to dst
	// atomically. The destination is replaced via a rename so that any
	// process already holding an open file descriptor to dst continues to
	// use the old inode.
	Restore(ctx context.Context, snapshotID, dst string) error

	// AtomicReplace renames newFile to target atomically using an OS-level
	// rename (both paths must be on the same filesystem). Any process holding
	// an open descriptor to target continues to use the old inode.
	AtomicReplace(ctx context.Context, target, newFile string) error

	// ExtractTarGz decompresses and extracts the .tar.gz archive at archive
	// into destDir, creating it if necessary. The implementation must reject
	// path-traversal entries (entries whose cleaned path escapes destDir).
	ExtractTarGz(ctx context.Context, archive, destDir string) error
}
