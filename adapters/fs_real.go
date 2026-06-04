package adapters

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
)

// atomicWriteCommit is a seam for tests: it receives the open temp file and
// performs the write, chmod, close, and rename steps. Production code MUST NOT
// replace this var; it exists solely to make the error paths unit-testable.
var atomicWriteCommit = func(tmp *os.File, data []byte, perm fs.FileMode, finalPath string) error {
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("atomic write: write: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("atomic write: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: close: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		cleanup()
		return fmt.Errorf("atomic write: rename: %w", err)
	}
	return nil
}

// RealFsAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.FsPort]. It wraps
// [lifecycle/update/adapters.RealFsAdapter] (which covers Snapshot, Restore,
// AtomicReplace, ExtractTarGz) and adds the two additional methods required
// by the install/uninstall/clean verbs: CopyFile and RemoveDir.
//
// All seam fields inherited from the embedded type remain injectable for
// failure-path testing. CopyFile and RemoveDir expose their own injectable
// functions (CopyFileFn, RemoveAllFn) following the same seam pattern.
type RealFsAdapter struct {
	// Embed the update adapter to inherit Snapshot, Restore, AtomicReplace,
	// ExtractTarGz without duplicating their implementations.
	*updateadapters.RealFsAdapter

	// CopyFileFn performs the copy in CopyFile. Defaults to a real
	// open-create-copy using os primitives. Injectable for error-path tests.
	CopyFileFn func(src, dst string, mode fs.FileMode) error

	// RemoveAllFn removes a directory tree in RemoveDir. Defaults to
	// os.RemoveAll. Injectable for error-path tests.
	RemoveAllFn func(path string) error
}

// NewRealFs returns a RealFsAdapter with all seams wired to real os/stdlib
// functions. This is the constructor consumers must use in production wiring.
func NewRealFs() *RealFsAdapter {
	return &RealFsAdapter{
		RealFsAdapter: updateadapters.NewRealFs(),
		CopyFileFn:    defaultCopyFile,
		RemoveAllFn:   os.RemoveAll,
	}
}

// defaultCopyFile reads the full content of src and writes it to dst with the
// given permission mode. Uses os.ReadFile + os.WriteFile which limits the
// distinct error paths to open-failure and write-failure, both of which are
// covered by unit tests.
//
// It is NOT atomic with respect to other writers: a partial dst may exist
// if the write fails mid-stream. The caller (install verb) is responsible for
// ensuring the destination directory exists before calling.
func defaultCopyFile(src, dst string, mode fs.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("copy file: read src %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return fmt.Errorf("copy file: write dst %s: %w", dst, err)
	}
	return nil
}

// CopyFile copies the file at src to dst, applying the given permission mode.
// The destination directory must already exist. The operation is not atomic.
//
// Returns an error if src cannot be opened, dst cannot be created, or the
// copy fails mid-stream.
func (a *RealFsAdapter) CopyFile(ctx context.Context, src, dst string, mode fs.FileMode) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return a.CopyFileFn(src, dst, mode)
}

// RemoveDir removes dir and all of its contents recursively. Returns nil if
// dir does not exist (idempotent). Delegates to RemoveAllFn, which defaults
// to os.RemoveAll.
func (a *RealFsAdapter) RemoveDir(ctx context.Context, dir string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return a.RemoveAllFn(dir)
}

// MkdirAll implements ports.FsPort. It is equivalent to os.MkdirAll.
// The ctx is reserved for future cancellation but unused for short-lived FS calls.
func (a *RealFsAdapter) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ReadFile implements ports.FsPort. It is equivalent to os.ReadFile.
func (a *RealFsAdapter) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// AtomicWrite implements ports.FsPort. It writes data to path using a
// temp-then-rename pattern: creates a temp file in the same directory,
// writes data, chmods to perm, closes, then renames to path atomically.
// Returns an error if the parent directory does not exist.
func (a *RealFsAdapter) AtomicWrite(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".shipkit-atomic-*")
	if err != nil {
		return fmt.Errorf("atomic write: create temp: %w", err)
	}
	return atomicWriteCommit(tmp, data, perm, path)
}
