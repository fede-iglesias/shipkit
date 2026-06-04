package ports

import (
	"context"
	"io/fs"
)

// FsPort abstracts all filesystem operations used by the shipkit lifecycle verbs.
//
// This port extends the lifecycle/update/ports.FsPort with two additional
// methods required by install, uninstall, and clean: CopyFile and RemoveDir.
// The five original methods (Snapshot, Restore, AtomicReplace, ExtractTarGz,
// WriteFile) are structural mirrors of the update-scoped port.
//
// Implementations must be safe for concurrent use and must treat each operation
// as atomic from the caller's perspective. Path-traversal attacks must be
// rejected by implementations that expose directory enumeration.
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

	// CopyFile copies the file at src to dst, applying the given permission
	// mode. Intermediate directories under dst are NOT created automatically;
	// the caller must ensure the destination directory exists. The operation is
	// NOT atomic: a partial dst may exist if the call fails mid-write.
	//
	// mode is the permission bits applied to dst (e.g. 0o755 for executables).
	CopyFile(ctx context.Context, src, dst string, mode fs.FileMode) error

	// RemoveDir removes dir and all of its contents recursively. It is
	// equivalent to os.RemoveAll. Returning nil if dir does not exist is
	// acceptable (idempotent).
	RemoveDir(ctx context.Context, dir string) error
}

// MockFsPort is a test double for FsPort. It records calls and returns the
// values set on its exported Func fields. Use NewMockFsPort for safe defaults.
type MockFsPort struct {
	SnapshotFunc      func(ctx context.Context, src, snapshotDir string) (string, error)
	RestoreFunc       func(ctx context.Context, snapshotID, dst string) error
	AtomicReplaceFunc func(ctx context.Context, target, newFile string) error
	ExtractTarGzFunc  func(ctx context.Context, archive, destDir string) error
	CopyFileFunc      func(ctx context.Context, src, dst string, mode fs.FileMode) error
	RemoveDirFunc     func(ctx context.Context, dir string) error

	SnapshotCalls      [][2]string
	RestoreCalls       [][2]string
	AtomicReplaceCalls [][2]string
	ExtractTarGzCalls  [][2]string
	CopyFileCalls      []struct{ Src, Dst string; Mode fs.FileMode }
	RemoveDirCalls     []string
}

// NewMockFsPort returns a MockFsPort with safe zero-value defaults.
func NewMockFsPort() *MockFsPort { return &MockFsPort{} }

// Snapshot implements FsPort.
func (m *MockFsPort) Snapshot(ctx context.Context, src, snapshotDir string) (string, error) {
	m.SnapshotCalls = append(m.SnapshotCalls, [2]string{src, snapshotDir})
	if m.SnapshotFunc != nil {
		return m.SnapshotFunc(ctx, src, snapshotDir)
	}
	return "", nil
}

// Restore implements FsPort.
func (m *MockFsPort) Restore(ctx context.Context, snapshotID, dst string) error {
	m.RestoreCalls = append(m.RestoreCalls, [2]string{snapshotID, dst})
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, snapshotID, dst)
	}
	return nil
}

// AtomicReplace implements FsPort.
func (m *MockFsPort) AtomicReplace(ctx context.Context, target, newFile string) error {
	m.AtomicReplaceCalls = append(m.AtomicReplaceCalls, [2]string{target, newFile})
	if m.AtomicReplaceFunc != nil {
		return m.AtomicReplaceFunc(ctx, target, newFile)
	}
	return nil
}

// ExtractTarGz implements FsPort.
func (m *MockFsPort) ExtractTarGz(ctx context.Context, archive, destDir string) error {
	m.ExtractTarGzCalls = append(m.ExtractTarGzCalls, [2]string{archive, destDir})
	if m.ExtractTarGzFunc != nil {
		return m.ExtractTarGzFunc(ctx, archive, destDir)
	}
	return nil
}

// CopyFile implements FsPort.
func (m *MockFsPort) CopyFile(ctx context.Context, src, dst string, mode fs.FileMode) error {
	m.CopyFileCalls = append(m.CopyFileCalls, struct {
		Src, Dst string
		Mode     fs.FileMode
	}{src, dst, mode})
	if m.CopyFileFunc != nil {
		return m.CopyFileFunc(ctx, src, dst, mode)
	}
	return nil
}

// RemoveDir implements FsPort.
func (m *MockFsPort) RemoveDir(ctx context.Context, dir string) error {
	m.RemoveDirCalls = append(m.RemoveDirCalls, dir)
	if m.RemoveDirFunc != nil {
		return m.RemoveDirFunc(ctx, dir)
	}
	return nil
}
