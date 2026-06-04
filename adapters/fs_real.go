package adapters

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
)

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
