package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// atomicWriteOps groups injectable I/O operations for AtomicWrite.
// Production code uses atomicWriteDefaultOps; tests can replace individual
// ops to exercise error branches without OS-level tricks.
type atomicWriteOps struct {
	createTemp func(dir, pattern string) (*os.File, error)
	write      func(f *os.File, data []byte) (int, error)
	closeFile  func(f *os.File) error
	rename     func(src, dst string) error
	remove     func(name string) error
}

// atomicWriteDefaultOps returns the production I/O operations.
var atomicWriteDefaultOps = atomicWriteOps{
	createTemp: os.CreateTemp,
	write:      func(f *os.File, data []byte) (int, error) { return f.Write(data) },
	closeFile:  func(f *os.File) error { return f.Close() },
	rename:     os.Rename,
	remove:     os.Remove,
}

// AtomicWrite writes data to path atomically: it creates a temp file in the
// same directory as path, writes data, then renames it to path. Parent
// directories are created if they do not exist.
//
// The write is atomic with respect to readers: they either see the old file or
// the fully written new file, never a partial write. The temp file and the
// destination must be on the same filesystem for the rename to be atomic.
//
// Returns a wrapped error if the parent directory cannot be created, the temp
// file cannot be written, or the rename fails.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	return atomicWriteWith(path, data, perm, atomicWriteDefaultOps)
}

// atomicWriteWith is the injectable implementation used by AtomicWrite and tests.
func atomicWriteWith(path string, data []byte, _ os.FileMode, ops atomicWriteOps) error {
	if err := EnsureParent(path); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := ops.createTemp(dir, ".shipkit-*.tmp")
	if err != nil {
		return fmt.Errorf("store/fs: create temp file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := ops.write(tmp, data); err != nil {
		_ = ops.closeFile(tmp)
		_ = ops.remove(tmpName)
		return fmt.Errorf("store/fs: write temp file: %w", err)
	}
	if err := ops.closeFile(tmp); err != nil {
		_ = ops.remove(tmpName)
		return fmt.Errorf("store/fs: close temp file: %w", err)
	}
	if err := ops.rename(tmpName, path); err != nil {
		_ = ops.remove(tmpName)
		return fmt.Errorf("store/fs: rename temp to %q: %w", path, err)
	}
	return nil
}

// mkdirAllFn is os.MkdirAll. Replaceable in non-parallel tests only.
var mkdirAllFn = os.MkdirAll

// EnsureParent creates all parent directories of path with permissions 0o755.
//
// Returns a wrapped error if os.MkdirAll fails (e.g., a path component exists
// as a regular file).
func EnsureParent(path string) error {
	parent := filepath.Dir(path)
	if err := mkdirAllFn(parent, 0o755); err != nil {
		return fmt.Errorf("store/fs: create parent dirs for %q: %w", path, err)
	}
	return nil
}

// walkDirFn is filepath.WalkDir. Replaceable in non-parallel tests only.
var walkDirFn = filepath.WalkDir

// WalkDir collects all files under dir whose names end with ext (e.g. ".md").
// Returns an empty slice (not an error) when dir does not exist.
//
// Returns:
//   - A slice of absolute paths matching ext, in filesystem traversal order.
//   - nil, nil when dir does not exist.
//   - A wrapped error for any other walk failure.
func WalkDir(dir, ext string) ([]string, error) {
	var paths []string

	err := walkDirFn(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip missing root gracefully.
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipAll
			}
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ext) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		// If root doesn't exist, WalkDir itself returns the error before
		// the callback runs. Handle that case here.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("store/fs: walk %q: %w", dir, err)
	}
	return paths, nil
}

// archiveRenameFn is os.Rename for archiving. Replaceable in tests.
var archiveRenameFn = os.Rename

// MoveToArchive moves srcPath to archiveRoot preserving the filename.
// The destination is archiveRoot/<filename>. Parent directories are
// created as needed.
//
// Returns:
//   - The new path on success.
//   - A wrapped error if EnsureParent or the rename fails.
//
// Note: MoveToArchive is a flat archive - files go directly under archiveRoot
// without preserving their original directory structure.
func MoveToArchive(srcPath, archiveRoot string) (string, error) {
	// Preserve just the filename for the archive destination.
	// This is a simple single-level archive - files go flat under archiveRoot.
	// The destination retains the base filename only.
	dst := filepath.Join(archiveRoot, filepath.Base(srcPath))

	if err := EnsureParent(dst); err != nil {
		return "", err
	}
	if err := archiveRenameFn(srcPath, dst); err != nil {
		return "", fmt.Errorf("store/fs: move %q to archive: %w", srcPath, err)
	}
	return dst, nil
}
