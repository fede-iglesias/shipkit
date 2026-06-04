package adapters

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
