package recovery

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteCommit_WriteFails covers the write-error branch of writeCommit.
// Strategy: pre-close the temp file so tmp.Write returns an error, then call
// writeCommit with the closed descriptor. The cleanup func must remove the temp
// file so no leftover remains after the error.
func TestWriteCommit_WriteFails(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".shipkit-recovery-*")
	if err != nil {
		t.Fatal(err)
	}
	// Pre-close so Write fails.
	f.Close()

	dest := filepath.Join(dir, "out.json")
	got := writeCommit(f, []byte(`{"version":1}`), dest)
	if got == nil {
		t.Fatal("want error from writeCommit with closed file; got nil")
	}
	if !strings.Contains(got.Error(), "write") {
		t.Errorf("want error mentioning 'write', got %q", got.Error())
	}
	// Temp file must be cleaned up after the error.
	if _, statErr := os.Stat(f.Name()); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after error cleanup", f.Name())
	}
}

// TestWriteCommit_ChmodFails covers the chmod-error branch of writeCommit.
// Strategy: replace the writeChmod seam with an error-returning func so the
// branch is exercised without OS-level file descriptor tricks.
func TestWriteCommit_ChmodFails(t *testing.T) {
	injected := errors.New("injected chmod error")
	orig := writeChmod
	writeChmod = func(_ *os.File, _ os.FileMode) error { return injected }
	t.Cleanup(func() { writeChmod = orig })

	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".shipkit-recovery-*")
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out.json")
	got := writeCommit(f, []byte(`{"version":1}`), dest)
	if got == nil {
		t.Fatal("want error from writeCommit with injected chmod failure; got nil")
	}
	if !strings.Contains(got.Error(), "chmod") {
		t.Errorf("want error mentioning 'chmod', got %q", got.Error())
	}
	// Temp file must be cleaned up after the error.
	if _, statErr := os.Stat(f.Name()); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after chmod-error cleanup", f.Name())
	}
}

// TestWriteCommit_CloseFails covers the close-error branch of writeCommit.
// Strategy: replace the writeClose seam with an error-returning func.
func TestWriteCommit_CloseFails(t *testing.T) {
	injected := errors.New("injected close error")
	orig := writeClose
	writeClose = func(_ *os.File) error { return injected }
	t.Cleanup(func() { writeClose = orig })

	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".shipkit-recovery-*")
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "out.json")
	got := writeCommit(f, []byte(`{"version":1}`), dest)
	if got == nil {
		t.Fatal("want error from writeCommit with injected close failure; got nil")
	}
	if !strings.Contains(got.Error(), "close") {
		t.Errorf("want error mentioning 'close', got %q", got.Error())
	}
	// Temp file must be cleaned up after the error.
	if _, statErr := os.Stat(f.Name()); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after close-error cleanup", f.Name())
	}
}

// TestWriteCommit_RenameFails covers the rename-error branch of writeCommit.
// Strategy: pre-create a directory at the destination path so os.Rename fails
// with EISDIR. The temp file must be cleaned up after the error.
func TestWriteCommit_RenameFails(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".shipkit-recovery-*")
	if err != nil {
		t.Fatal(err)
	}

	// Pre-create a directory at the destination so Rename fails.
	dest := filepath.Join(dir, "dest-dir")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	got := writeCommit(f, []byte(`{"version":1}`), dest)
	if got == nil {
		t.Fatal("want error from writeCommit with dir-as-dest; got nil")
	}
	if !strings.Contains(got.Error(), "rename") {
		t.Errorf("want error mentioning 'rename', got %q", got.Error())
	}
	// Temp file must be cleaned up after the error.
	if _, statErr := os.Stat(f.Name()); !os.IsNotExist(statErr) {
		t.Errorf("temp file %q still exists after error cleanup", f.Name())
	}
}

// TestMarshalManifest_Fails covers the marshal-error branch of Write.
// Strategy: replace the marshalManifest seam with an error-returning func.
func TestMarshalManifest_Fails(t *testing.T) {
	injected := errors.New("injected marshal error")
	orig := marshalManifest
	marshalManifest = func(_ Manifest) ([]byte, error) { return nil, injected }
	t.Cleanup(func() { marshalManifest = orig })

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.json")
	got := Write(dest, Manifest{Version: 1, AppName: "x"})
	if got == nil {
		t.Fatal("want error from Write with injected marshal failure; got nil")
	}
	if !strings.Contains(got.Error(), "marshal") {
		t.Errorf("want error mentioning 'marshal', got %q", got.Error())
	}
}

// TestWriteCommit_WriteSeam verifies that swapping writeCommit routes the Write
// function through the seam, confirming the seam is actually called in production.
func TestWriteCommit_WriteSeam(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "manifest.json")

	injectedErr := os.ErrInvalid
	orig := writeCommit
	writeCommit = func(_ *os.File, _ []byte, _ string) error {
		return injectedErr
	}
	t.Cleanup(func() { writeCommit = orig })

	err := Write(dest, Manifest{Version: 1, AppName: "seam-test"})
	if err == nil {
		t.Fatal("want error via seam; got nil")
	}
	if !strings.Contains(err.Error(), injectedErr.Error()) {
		t.Errorf("want injected error surfaced; got %v", err)
	}
}
