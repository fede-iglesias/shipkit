package adapters

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// testShellRcAdapter returns a ShellRcRealAdapter backed by a real temp dir.
// The returned dir is accessible for post-call assertions.
func testShellRcAdapter(t *testing.T) (*ShellRcRealAdapter, string) {
	t.Helper()
	dir := t.TempDir()
	return NewShellRcReal(), dir
}

// TestNewShellRcReal verifies the constructor returns a non-nil adapter.
func TestNewShellRcReal(t *testing.T) {
	a := NewShellRcReal()
	if a == nil {
		t.Fatal("NewShellRcReal returned nil")
	}
}

// TestShellRcReal_EnsureBlock_NewFile verifies that EnsureBlock creates a new
// file and writes the block when the file does not exist.
func TestShellRcReal_EnsureBlock_NewFile(t *testing.T) {
	a, dir := testShellRcAdapter(t)
	rc := filepath.Join(dir, ".zshrc")

	res, err := a.EnsureBlock(rc, "fpath", "fpath=(/some/path $fpath)")
	if err != nil {
		t.Fatalf("EnsureBlock: %v", err)
	}
	if !res.Written {
		t.Errorf("result.Written = false; want true")
	}

	content, _ := os.ReadFile(rc)
	if string(content) == "" {
		t.Error("rc file is empty after EnsureBlock")
	}
	if !containsString(string(content), "# >>> shipkit:fpath >>>") {
		t.Error("open marker not found in rc file")
	}
	if !containsString(string(content), "fpath=(/some/path $fpath)") {
		t.Error("block content not found in rc file")
	}
	if !containsString(string(content), "# <<< shipkit:fpath <<<") {
		t.Error("close marker not found in rc file")
	}
}

// TestShellRcReal_EnsureBlock_Unchanged verifies that a second call with the
// same content returns Unchanged.
func TestShellRcReal_EnsureBlock_Unchanged(t *testing.T) {
	a, dir := testShellRcAdapter(t)
	rc := filepath.Join(dir, ".zshrc")

	content := "fpath=(/some/path $fpath)"
	if _, err := a.EnsureBlock(rc, "fpath", content); err != nil {
		t.Fatalf("first EnsureBlock: %v", err)
	}

	res, err := a.EnsureBlock(rc, "fpath", content)
	if err != nil {
		t.Fatalf("second EnsureBlock: %v", err)
	}
	if !res.Unchanged {
		t.Errorf("result.Unchanged = false; want true (content identical)")
	}
}

// TestShellRcReal_EnsureBlock_Updated verifies that a call with changed content
// returns Updated.
func TestShellRcReal_EnsureBlock_Updated(t *testing.T) {
	a, dir := testShellRcAdapter(t)
	rc := filepath.Join(dir, ".zshrc")

	if _, err := a.EnsureBlock(rc, "fpath", "old content"); err != nil {
		t.Fatalf("first EnsureBlock: %v", err)
	}

	res, err := a.EnsureBlock(rc, "fpath", "new content")
	if err != nil {
		t.Fatalf("second EnsureBlock: %v", err)
	}
	if !res.Updated {
		t.Errorf("result.Updated = false; want true")
	}

	got, _ := os.ReadFile(rc)
	if !containsString(string(got), "new content") {
		t.Error("updated content not in file")
	}
}

// TestShellRcReal_EnsureBlock_ReadError exercises the read error path.
func TestShellRcReal_EnsureBlock_ReadError(t *testing.T) {
	sentinel := errors.New("read fail")
	a := &ShellRcRealAdapter{
		ReadFileFn: func(name string) ([]byte, error) {
			if name != "rc" {
				return nil, nil
			}
			return nil, sentinel
		},
		WriteFileFn: os.WriteFile,
		RenameFn:    os.Rename,
	}
	_, err := a.EnsureBlock("rc", "id", "content")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestShellRcReal_EnsureBlock_WriteError exercises the write error path.
func TestShellRcReal_EnsureBlock_WriteError(t *testing.T) {
	sentinel := errors.New("write fail")
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return nil, os.ErrNotExist },
		WriteFileFn: func(string, []byte, os.FileMode) error { return sentinel },
		RenameFn:    os.Rename,
	}
	_, err := a.EnsureBlock("/tmp/test.rc", "id", "content")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestShellRcReal_EnsureBlock_RenameError exercises the rename error path.
func TestShellRcReal_EnsureBlock_RenameError(t *testing.T) {
	sentinel := errors.New("rename fail")
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return nil, os.ErrNotExist },
		WriteFileFn: func(string, []byte, os.FileMode) error { return nil },
		RenameFn:    func(string, string) error { return sentinel },
	}
	_, err := a.EnsureBlock("/tmp/test.rc", "id", "content")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestShellRcReal_RemoveBlock_HappyPath verifies block removal.
func TestShellRcReal_RemoveBlock_HappyPath(t *testing.T) {
	a, dir := testShellRcAdapter(t)
	rc := filepath.Join(dir, ".zshrc")

	if _, err := a.EnsureBlock(rc, "fpath", "some content"); err != nil {
		t.Fatalf("EnsureBlock: %v", err)
	}

	res, err := a.RemoveBlock(rc, "fpath")
	if err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}
	if !res.Removed {
		t.Errorf("result.Removed = false; want true")
	}

	got, _ := os.ReadFile(rc)
	if containsString(string(got), "shipkit:fpath") {
		t.Error("block markers still present after RemoveBlock")
	}
}

// TestShellRcReal_RemoveBlock_NotFound verifies idempotent removal.
func TestShellRcReal_RemoveBlock_NotFound(t *testing.T) {
	a, dir := testShellRcAdapter(t)
	rc := filepath.Join(dir, ".zshrc")

	// File exists but has no block.
	if err := os.WriteFile(rc, []byte("# plain rc"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := a.RemoveBlock(rc, "fpath")
	if err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}
	if !res.NotFound {
		t.Errorf("result.NotFound = false; want true")
	}
}

// TestShellRcReal_RemoveBlock_FileNotExist verifies idempotent removal when
// the file does not exist.
func TestShellRcReal_RemoveBlock_FileNotExist(t *testing.T) {
	a := NewShellRcReal()
	res, err := a.RemoveBlock("/nonexistent/.zshrc", "fpath")
	if err != nil {
		t.Fatalf("RemoveBlock on missing file: %v", err)
	}
	if !res.NotFound {
		t.Errorf("result.NotFound = false; want true")
	}
}

// TestShellRcReal_RemoveBlock_ReadError exercises the read error path.
func TestShellRcReal_RemoveBlock_ReadError(t *testing.T) {
	sentinel := errors.New("read fail")
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return nil, sentinel },
		WriteFileFn: os.WriteFile,
		RenameFn:    os.Rename,
	}
	_, err := a.RemoveBlock("rc", "id")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestShellRcReal_RemoveBlock_WriteError exercises the write error path.
func TestShellRcReal_RemoveBlock_WriteError(t *testing.T) {
	sentinel := errors.New("write fail")
	// Prepare file content with a block already present.
	blockContent := "\n# >>> shipkit:id >>>\ncontent\n# <<< shipkit:id <<<\n"
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return []byte(blockContent), nil },
		WriteFileFn: func(string, []byte, os.FileMode) error { return sentinel },
		RenameFn:    os.Rename,
	}
	_, err := a.RemoveBlock("rc", "id")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestShellRcReal_RemoveBlock_RenameError exercises the rename error path.
func TestShellRcReal_RemoveBlock_RenameError(t *testing.T) {
	sentinel := errors.New("rename fail")
	blockContent := "\n# >>> shipkit:id >>>\ncontent\n# <<< shipkit:id <<<\n"
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return []byte(blockContent), nil },
		WriteFileFn: func(string, []byte, os.FileMode) error { return nil },
		RenameFn:    func(string, string) error { return sentinel },
	}
	_, err := a.RemoveBlock("rc", "id")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestOpenCloseMarkers verifies the marker format strings.
func TestOpenCloseMarkers(t *testing.T) {
	if openMarker("fpath") != "# >>> shipkit:fpath >>>" {
		t.Errorf("openMarker unexpected: %q", openMarker("fpath"))
	}
	if closeMarker("fpath") != "# <<< shipkit:fpath <<<" {
		t.Errorf("closeMarker unexpected: %q", closeMarker("fpath"))
	}
}

// TestShellRcPort_Compliance verifies that ShellRcRealAdapter satisfies
// ports.ShellRcPort at compile time. The blank assignment below fails
// compilation if the interface is not satisfied.
func TestShellRcPort_Compliance(t *testing.T) {
	var _ ports.ShellRcPort = NewShellRcReal()
}

// containsString is a helper used in tests.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
