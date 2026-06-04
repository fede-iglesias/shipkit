package frontmatter

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteFile_MarshalError covers the Marshal failure path inside writeFile.
// goccy/go-yaml rarely fails, but we inject a custom marshal-like scenario:
// we use a type that triggers a marshal error.
func TestWriteFile_MarshalError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")

	// Use a type with a channel: goccy/go-yaml may or may not error on this.
	// If it doesn't error, skip - the path is simply unreachable with this library.
	type badMeta struct {
		Ch chan int `yaml:"ch"`
	}
	err := WriteFile(path, badMeta{Ch: make(chan int)}, []byte("body"))
	if err == nil {
		t.Skip("goccy/go-yaml marshals channels without error; marshal error path not reachable")
	}
}

// TestWriteFile_WriteError covers the write error path via injected write ops.
func TestWriteFile_WriteError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	meta := struct {
		Type string `yaml:"type"`
	}{Type: "test"}

	writeErr := errors.New("write failed")
	ops := defaultWriteOps(os.Rename)
	ops.write = func(f *os.File, data []byte) (int, error) {
		return 0, writeErr
	}

	err := writeFile(path, meta, []byte("body\n"), ops)
	if err == nil {
		t.Fatal("expected error from write failure")
	}
	if !errors.Is(err, writeErr) {
		t.Errorf("want writeErr in chain, got %v", err)
	}
}

// TestWriteFile_CloseError covers the close error path via injected write ops.
func TestWriteFile_CloseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	meta := struct {
		Type string `yaml:"type"`
	}{Type: "test"}

	closeErr := errors.New("close failed")
	ops := defaultWriteOps(os.Rename)
	ops.close = func(f *os.File) error {
		_ = f.Close() // close for real to avoid leaks
		return closeErr
	}

	err := writeFile(path, meta, []byte("body\n"), ops)
	if err == nil {
		t.Fatal("expected error from close failure")
	}
	if !errors.Is(err, closeErr) {
		t.Errorf("want closeErr in chain, got %v", err)
	}
}

// TestWriteFile_CleanupOnRenameError verifies temp file is removed when rename fails.
func TestWriteFile_CleanupOnRenameError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	meta := struct {
		Type string `yaml:"type"`
	}{Type: "test"}

	renameErr := errors.New("rename failed")
	err := WriteFileWithRename(path, meta, []byte("body\n"), func(src, dst string) error {
		return renameErr
	})
	if err == nil {
		t.Fatal("expected error from failing rename")
	}
	if !errors.Is(err, renameErr) {
		t.Errorf("want renameErr in chain, got %v", err)
	}

	// The temp file should be cleaned up.
	entries, err2 := os.ReadDir(dir)
	if err2 != nil {
		t.Fatal(err2)
	}
	// path itself should not exist (rename failed).
	// No other files (temp cleaned up) should remain.
	for _, e := range entries {
		if e.Name() != filepath.Base(path) {
			t.Errorf("temp file not cleaned up: %q still in dir", e.Name())
		}
	}
}
