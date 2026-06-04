package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestAtomicWriteCloseError exercises the tmp.Close() failure path.
// We close the file manually before AtomicWrite's internal close,
// which causes the second close to fail.
// Since we can't inject that easily, we instead test the rename failure path
// by creating a scenario where the temp file can be written but rename fails.
func TestAtomicWriteRenameError(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("running as root - permission errors don't apply")
	}

	// Create a dir, make the target file a directory (can't rename file over dir).
	dir := t.TempDir()
	target := filepath.Join(dir, "output.md")
	// Make target a directory so rename will fail.
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	err := AtomicWrite(target, []byte("data"), 0o644)
	if err == nil {
		t.Error("expected error when rename target is a directory")
	}
}

// TestAtomicWriteCreateTempError triggers the CreateTemp failure via injection.
func TestAtomicWriteCreateTempError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")

	ops := atomicWriteDefaultOps
	ops.createTemp = func(dir, pattern string) (*os.File, error) {
		return nil, fmt.Errorf("injected createTemp error")
	}

	err := atomicWriteWith(path, []byte("data"), 0o644, ops)
	if err == nil {
		t.Error("expected error from injected createTemp failure")
	}
}

// TestMoveToArchiveRenameError exercises the os.Rename error in MoveToArchive
// via sequential (non-parallel) global injection.
// NOT parallel because it mutates a package-level variable.
func TestMoveToArchiveRenameErrorInjected(t *testing.T) {
	// NOT parallel - modifies global archiveRenameFn.
	orig := archiveRenameFn
	archiveRenameFn = func(src, dst string) error {
		return fmt.Errorf("injected rename error")
	}
	defer func() { archiveRenameFn = orig }()

	dir := t.TempDir()
	src := filepath.Join(dir, "source.md")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	archiveRoot := filepath.Join(dir, "archive")

	_, err := MoveToArchive(src, archiveRoot)
	if err == nil {
		t.Error("expected error from injected rename failure")
	}
}

// TestMoveToArchiveRenameError exercises the os.Rename error in MoveToArchive.
// We create a src file then make the archive dir a regular file to cause the rename to fail.
func TestMoveToArchiveRenameError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "source.md")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// archive "root" is actually a file, so Join(archiveRoot, base) would be
	// <file>/source.md which can't be renamed to.
	archiveFile := filepath.Join(dir, "archive-is-a-file")
	if err := os.WriteFile(archiveFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// EnsureParent(archiveFile/source.md) -> MkdirAll(archiveFile) fails because
	// archiveFile is a regular file.
	_, err := MoveToArchive(src, archiveFile)
	if err == nil {
		t.Error("expected error when archive path is a regular file")
	}
}
