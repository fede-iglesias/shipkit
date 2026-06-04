package recovery_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/recovery"
)

// TestManifest_RoundTrip writes a fully populated manifest to disk and reads
// it back, asserting that every field survives serialisation unchanged.
func TestManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, recovery.Filename)

	want := recovery.Manifest{
		Version:      1,
		AppName:      "myapp",
		SnapshotPath: "/tmp/x",
		Steps:        []string{"pre-update", "snapshot"},
		Cause:        "verify failed",
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := recovery.Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := recovery.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Version != want.Version {
		t.Errorf("Version: got %d, want %d", got.Version, want.Version)
	}
	if got.AppName != want.AppName {
		t.Errorf("AppName: got %q, want %q", got.AppName, want.AppName)
	}
	if got.SnapshotPath != want.SnapshotPath {
		t.Errorf("SnapshotPath: got %q, want %q", got.SnapshotPath, want.SnapshotPath)
	}
	if len(got.Steps) != len(want.Steps) {
		t.Fatalf("Steps length: got %d, want %d", len(got.Steps), len(want.Steps))
	}
	for i := range got.Steps {
		if got.Steps[i] != want.Steps[i] {
			t.Errorf("Steps[%d]: got %q, want %q", i, got.Steps[i], want.Steps[i])
		}
	}
	if got.Cause != want.Cause {
		t.Errorf("Cause: got %q, want %q", got.Cause, want.Cause)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, want.CreatedAt)
	}
}

// TestManifest_ReadMissing_ReturnsIsNotExist asserts that reading a missing
// manifest returns an error that satisfies errors.Is(err, fs.ErrNotExist).
// Callers rely on this to distinguish "no recovery pending" from real IO errors.
func TestManifest_ReadMissing_ReturnsIsNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	_, err := recovery.Read(path)
	if err == nil {
		t.Fatal("Read: expected error for missing path, got nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Read: error does not satisfy errors.Is(_, fs.ErrNotExist): %v", err)
	}
}

// TestManifest_WriteIsAtomic_NoTempLeft asserts that Write does not leave any
// .shipkit-recovery-* temp files behind on the happy path. This locks in the
// temp+rename pattern that prevents partial writes from being observed.
func TestManifest_WriteIsAtomic_NoTempLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, recovery.Filename)

	m := recovery.Manifest{
		Version:   1,
		AppName:   "atomic",
		CreatedAt: time.Now().UTC(),
	}
	if err := recovery.Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".shipkit-recovery-") {
			t.Errorf("leftover temp file after Write: %q", e.Name())
		}
	}
}

// TestPath_JoinsDataRootAndFilename asserts that Path returns the canonical
// joined path so callers do not hard-code the filename.
func TestPath_JoinsDataRootAndFilename(t *testing.T) {
	dataRoot := filepath.Join("/var", "lib", "myapp")
	got := recovery.Path(dataRoot)
	want := filepath.Join(dataRoot, recovery.Filename)
	if got != want {
		t.Errorf("Path: got %q, want %q", got, want)
	}
}

// TestManifest_WriteParentMissing_ReturnsError asserts that Write surfaces an
// error when the manifest's parent directory does not exist. This locks in the
// contract documented on Write that the parent must already exist.
func TestManifest_WriteParentMissing_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing-subdir", recovery.Filename)

	err := recovery.Write(path, recovery.Manifest{Version: 1, AppName: "x", CreatedAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("Write: expected error for missing parent dir, got nil")
	}
	if !strings.Contains(err.Error(), "create temp") {
		t.Errorf("Write: expected wrapped create-temp error, got %v", err)
	}
}

// TestManifest_ReadReturnsNonNotExistError asserts that Read returns a non
// fs.ErrNotExist error when the path exists but is not a regular file
// (a directory in this case). Callers rely on the NotExist distinction to
// branch between "no recovery pending" and "IO error".
func TestManifest_ReadReturnsNonNotExistError(t *testing.T) {
	dir := t.TempDir()
	// Make the manifest path a directory so os.ReadFile returns is-a-directory.
	path := filepath.Join(dir, recovery.Filename)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	_, err := recovery.Read(path)
	if err == nil {
		t.Fatal("Read: expected error for directory path, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read: error must NOT satisfy errors.Is(_, fs.ErrNotExist) when path is a directory, got %v", err)
	}
}

// TestManifest_ReadInvalidJSON_ReturnsError asserts that malformed JSON at the
// manifest path surfaces as a non-NotExist error so callers report Warn.
func TestManifest_ReadInvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, recovery.Filename)
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := recovery.Read(path)
	if err == nil {
		t.Fatal("Read: expected error for invalid JSON, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read: error must NOT satisfy errors.Is(_, fs.ErrNotExist) for invalid JSON, got %v", err)
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Read: expected wrapped unmarshal error, got %v", err)
	}
}
