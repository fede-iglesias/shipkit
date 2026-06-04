package adapters

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewRealFs verifies that the constructor returns a non-nil adapter with
// all seams wired.
func TestNewRealFs(t *testing.T) {
	a := NewRealFs()
	if a == nil {
		t.Fatal("NewRealFs returned nil")
	}
	if a.RealFsAdapter == nil {
		t.Fatal("embedded RealFsAdapter is nil")
	}
	if a.CopyFileFn == nil {
		t.Error("CopyFileFn is nil")
	}
	if a.RemoveAllFn == nil {
		t.Error("RemoveAllFn is nil")
	}
}

// TestRealFsAdapter_CopyFile_HappyPath writes a source file and copies it to
// a destination using the real adapter.
func TestRealFsAdapter_CopyFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewRealFs()
	if err := a.CopyFile(context.Background(), src, dst, 0o644); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile dst: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("dst content = %q; want %q", string(got), "hello")
	}
}

// TestRealFsAdapter_CopyFile_ContextCancelled confirms that CopyFile respects
// a cancelled context.
func TestRealFsAdapter_CopyFile_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := NewRealFs()
	err := a.CopyFile(ctx, "src", "dst", 0o644)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled; got %v", err)
	}
}

// TestRealFsAdapter_CopyFile_SrcNotFound tests the error path when the source
// file does not exist.
func TestRealFsAdapter_CopyFile_SrcNotFound(t *testing.T) {
	a := NewRealFs()
	err := a.CopyFile(context.Background(), "/nonexistent/src", "/tmp/dst", 0o644)
	if err == nil {
		t.Fatal("want error; got nil")
	}
}

// TestRealFsAdapter_CopyFile_DstNotCreatable tests the error path when the
// destination path is not writable (injected CopyFileFn).
func TestRealFsAdapter_CopyFile_DstNotCreatable(t *testing.T) {
	sentinel := errors.New("create dst failed")
	a := &RealFsAdapter{
		RealFsAdapter: NewRealFs().RealFsAdapter,
		CopyFileFn: func(src, dst string, mode fs.FileMode) error {
			return sentinel
		},
		RemoveAllFn: os.RemoveAll,
	}
	err := a.CopyFile(context.Background(), "src", "dst", 0o644)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestRealFsAdapter_RemoveDir_HappyPath creates a directory tree and removes it.
func TestRealFsAdapter_RemoveDir_HappyPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	a := NewRealFs()
	if err := a.RemoveDir(context.Background(), sub); err != nil {
		t.Fatalf("RemoveDir: %v", err)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Errorf("directory still exists after RemoveDir")
	}
}

// TestRealFsAdapter_RemoveDir_Idempotent confirms that removing a non-existent
// directory returns nil (os.RemoveAll is idempotent).
func TestRealFsAdapter_RemoveDir_Idempotent(t *testing.T) {
	a := NewRealFs()
	if err := a.RemoveDir(context.Background(), "/nonexistent/path"); err != nil {
		t.Fatalf("RemoveDir on non-existent: %v", err)
	}
}

// TestRealFsAdapter_RemoveDir_ContextCancelled confirms that RemoveDir respects
// a cancelled context.
func TestRealFsAdapter_RemoveDir_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := NewRealFs()
	err := a.RemoveDir(ctx, "/some/dir")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled; got %v", err)
	}
}

// TestRealFsAdapter_RemoveDir_FnError tests the error path via injected seam.
func TestRealFsAdapter_RemoveDir_FnError(t *testing.T) {
	sentinel := errors.New("removeall failed")
	a := &RealFsAdapter{
		RealFsAdapter: NewRealFs().RealFsAdapter,
		CopyFileFn:    defaultCopyFile,
		RemoveAllFn:   func(string) error { return sentinel },
	}
	err := a.RemoveDir(context.Background(), "/some/dir")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestRealFsAdapter_MkdirAll_CreatesNestedDirs verifies that MkdirAll creates
// all intermediate directories under the given path.
func TestRealFsAdapter_MkdirAll_CreatesNestedDirs(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "a/b/c")

	a := NewRealFs()
	if err := a.MkdirAll(context.Background(), target, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat after MkdirAll: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", target)
	}
}

// TestRealFsAdapter_ReadFile_ReturnsContent writes a temp file and verifies
// ReadFile returns the exact bytes that were written.
func TestRealFsAdapter_ReadFile_ReturnsContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	want := []byte("hello readfile")

	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewRealFs()
	got, err := a.ReadFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q; want %q", got, want)
	}
}

// TestRealFsAdapter_ReadFile_NotFound verifies ReadFile returns error for missing file.
func TestRealFsAdapter_ReadFile_NotFound(t *testing.T) {
	a := NewRealFs()
	_, err := a.ReadFile(context.Background(), "/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("want error for missing file; got nil")
	}
}

// TestRealFsAdapter_AtomicWrite_WritesContentAndChmod verifies that AtomicWrite
// writes the expected content to the path and applies the given permission mode.
func TestRealFsAdapter_AtomicWrite_WritesContentAndChmod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")
	want := []byte("atomic content")

	a := NewRealFs()
	if err := a.AtomicWrite(context.Background(), path, want, 0o755); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after AtomicWrite: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q; want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o; want 0755", info.Mode().Perm())
	}
}

// TestRealFsAdapter_AtomicWrite_LeavesNoTempOnSuccess verifies that no
// .shipkit-atomic-* temp files remain after a successful atomic write.
func TestRealFsAdapter_AtomicWrite_LeavesNoTempOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	a := NewRealFs()
	if err := a.AtomicWrite(context.Background(), path, []byte("data"), 0o644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".shipkit-atomic-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) > 0 {
		t.Errorf("temp files remain after successful write: %v", matches)
	}
}

// TestRealFsAdapter_AtomicWrite_BadDir verifies that AtomicWrite returns an
// error when the parent directory does not exist.
func TestRealFsAdapter_AtomicWrite_BadDir(t *testing.T) {
	a := NewRealFs()
	err := a.AtomicWrite(context.Background(), "/nonexistent/dir/file.txt", []byte("x"), 0o644)
	if err == nil {
		t.Fatal("want error writing to nonexistent dir; got nil")
	}
}

// TestDefaultCopyFile_ReadError tests the error branch when src open fails.
func TestDefaultCopyFile_ReadError(t *testing.T) {
	err := defaultCopyFile("/nonexistent/file", "/tmp/dst", 0o644)
	if err == nil {
		t.Fatal("want error; got nil")
	}
}

// TestDefaultCopyFile_WriteError tests the error branch when dst creation fails.
func TestDefaultCopyFile_WriteError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Try to write to a non-existent subdirectory.
	err := defaultCopyFile(src, "/nonexistent-dir/dst.txt", 0o644)
	if err == nil {
		t.Fatal("want error writing to non-existent dir; got nil")
	}
}

// TestAtomicWriteCommit_WriteFails covers the write-error branch of atomicWriteCommit.
// Strategy: create a temp file, close it, then pass it to atomicWriteCommit.
// Writes to a closed *os.File return an error on all supported platforms.
func TestAtomicWriteCommit_WriteFails(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".shipkit-atomic-*")
	if err != nil {
		t.Fatal(err)
	}
	// Pre-close so Write fails.
	f.Close()

	dest := filepath.Join(dir, "out.txt")
	got := atomicWriteCommit(f, []byte("x"), 0o644, dest)
	if got == nil {
		t.Fatal("want error from atomicWriteCommit with closed file; got nil")
	}
	if !strings.Contains(got.Error(), "write") {
		t.Errorf("want error mentioning 'write', got %q", got.Error())
	}
	// Temp file must be cleaned up.
	if _, statErr := os.Stat(f.Name()); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after error cleanup", f.Name())
	}
}

// TestAtomicWriteCommit_RenameFails covers the rename-error branch of atomicWriteCommit.
// Strategy: pre-create a directory at the destination path so os.Rename fails with EISDIR.
func TestAtomicWriteCommit_RenameFails(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".shipkit-atomic-*")
	if err != nil {
		t.Fatal(err)
	}
	// Pre-create a directory at the destination so Rename fails.
	dest := filepath.Join(dir, "dest-dir")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	got := atomicWriteCommit(f, []byte("payload"), 0o644, dest)
	if got == nil {
		t.Fatal("want error from atomicWriteCommit with dir-as-dest; got nil")
	}
	if !strings.Contains(got.Error(), "rename") {
		t.Errorf("want error mentioning 'rename', got %q", got.Error())
	}
	// Temp file must be cleaned up.
	if _, statErr := os.Stat(f.Name()); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after error cleanup", f.Name())
	}
}

// TestRealFsAdapter_AtomicWrite_CommitSeam verifies that replacing atomicWriteCommit
// redirects through the seam, confirming the seam is actually called.
func TestRealFsAdapter_AtomicWrite_CommitSeam(t *testing.T) {
	sentinel := errors.New("injected commit error")
	orig := atomicWriteCommit
	atomicWriteCommit = func(_ *os.File, _ []byte, _ fs.FileMode, _ string) error {
		return sentinel
	}
	t.Cleanup(func() { atomicWriteCommit = orig })

	dir := t.TempDir()
	a := NewRealFs()
	err := a.AtomicWrite(context.Background(), filepath.Join(dir, "out.txt"), []byte("x"), 0o644)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error via seam; got %v", err)
	}
}
