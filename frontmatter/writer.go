package frontmatter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeOps groups file I/O operations so they can be injected in tests.
// Production code uses defaultWriteOps; tests substitute individual fields
// to trigger error paths without touching the filesystem.
type writeOps struct {
	createTemp func(dir, pattern string) (*os.File, error)
	write      func(f *os.File, data []byte) (int, error)
	close      func(f *os.File) error
	remove     func(name string) error
	rename     func(src, dst string) error
}

// defaultWriteOps returns production write operations using os package primitives.
// The rename parameter is accepted separately because WriteFileWithRename exposes
// it as an injection point for tests; all other ops are always the real ones.
func defaultWriteOps(rename func(src, dst string) error) writeOps {
	return writeOps{
		createTemp: os.CreateTemp,
		write:      func(f *os.File, data []byte) (int, error) { return f.Write(data) },
		close:      func(f *os.File) error { return f.Close() },
		remove:     os.Remove,
		rename:     rename,
	}
}

// WriteFile atomically writes meta + body as a frontmatter document to path.
// It uses the standard os.Rename for atomicity.
//
// Atomicity: data is written to a temp file in the same directory as path,
// then renamed into place. This ensures no partial writes are visible to readers.
// On POSIX systems, rename within the same filesystem is atomic; on Windows,
// os.Rename is best-effort.
//
// Returns an error if marshaling fails, or if any file operation fails.
func WriteFile(path string, meta any, body []byte) error {
	return WriteFileWithRename(path, meta, body, os.Rename)
}

// WriteFileWithRename is the injectable version of WriteFile for testing.
// The rename parameter replaces os.Rename, enabling test isolation.
//
// Use WriteFile for production code. This function is exported for advanced
// consumers that need to substitute the rename operation (e.g. cross-device
// moves or test harnesses).
func WriteFileWithRename(path string, meta any, body []byte, rename func(src, dst string) error) error {
	return writeFile(path, meta, body, defaultWriteOps(rename))
}

// writeFile is the internal implementation with fully injectable operations.
// Used by WriteFileWithRename and testable via writeFileWithOps.
func writeFile(path string, meta any, body []byte, ops writeOps) error {
	data, err := Marshal(meta, body)
	if err != nil {
		return fmt.Errorf("frontmatter: marshal for write: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := ops.createTemp(dir, ".frontmatter-*.tmp")
	if err != nil {
		return fmt.Errorf("frontmatter: create temp file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Write data to temp file.
	if _, err := ops.write(tmp, data); err != nil {
		_ = ops.close(tmp)
		_ = ops.remove(tmpName)
		return fmt.Errorf("frontmatter: write temp file: %w", err)
	}
	if err := ops.close(tmp); err != nil {
		_ = ops.remove(tmpName)
		return fmt.Errorf("frontmatter: close temp file: %w", err)
	}

	// Atomic rename.
	if err := ops.rename(tmpName, path); err != nil {
		_ = ops.remove(tmpName)
		return fmt.Errorf("frontmatter: rename temp to %q: %w", path, err)
	}

	return nil
}

// ensure io is used (for the write function type).
var _ io.Writer = (*os.File)(nil)
