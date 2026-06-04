package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Filename is the canonical on-disk name for the recovery manifest, written by
// lifecycle/update on rollback and read by lifecycle/clean (to protect
// snapshots) and lifecycle/doctor (to surface pending recoveries).
const Filename = ".shipkit.recovery-manifest.json"

// Manifest captures enough context for downstream tooling to understand which
// app rollback affected, which snapshot is still required, and which forward
// states were completed before failure.
type Manifest struct {
	Version      int       `json:"version"`
	AppName      string    `json:"app_name"`
	SnapshotPath string    `json:"snapshot_path"`
	Steps        []string  `json:"steps"`
	Cause        string    `json:"cause,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Path returns the canonical full path of the manifest under a given data root.
func Path(dataRoot string) string {
	return filepath.Join(dataRoot, Filename)
}

// writeCommit is a seam for tests: it receives the open temp file and performs
// the write, chmod, close, and rename steps. Production code MUST NOT replace
// this var; it exists solely to make the error paths unit-testable.
//
// Individual OS-call steps are factored into writeChmod and writeClose seam
// vars so their error branches can be tested in isolation.
var writeCommit = func(tmp *os.File, data []byte, finalPath string) error {
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("recovery: write: %w", err)
	}
	if err := writeChmod(tmp, 0o644); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("recovery: chmod: %w", err)
	}
	if err := writeClose(tmp); err != nil {
		cleanup()
		return fmt.Errorf("recovery: close: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		cleanup()
		return fmt.Errorf("recovery: rename: %w", err)
	}
	return nil
}

// writeChmod is a seam for tests: it calls Chmod on the temp file.
// Tests replace this var to force chmod-error paths without OS-level tricks.
var writeChmod = func(f *os.File, perm os.FileMode) error { return f.Chmod(perm) }

// writeClose is a seam for tests: it calls Close on the temp file.
// Tests replace this var to force close-error paths without OS-level tricks.
var writeClose = func(f *os.File) error { return f.Close() }

// marshalManifest is a seam for tests: it serializes a Manifest to JSON.
// Tests replace this var to force marshal-error paths.
var marshalManifest = func(m Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// Write serializes m to path using a temp+rename pattern so partial writes
// are never observable. The parent directory must already exist.
func Write(path string, m Manifest) error {
	data, err := marshalManifest(m)
	if err != nil {
		return fmt.Errorf("recovery: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".shipkit-recovery-*")
	if err != nil {
		return fmt.Errorf("recovery: create temp: %w", err)
	}
	return writeCommit(tmp, data, path)
}

// Read deserializes a manifest from path. When the path does not exist the
// returned error satisfies errors.Is(err, fs.ErrNotExist).
func Read(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Manifest{}, err
		}
		return Manifest{}, fmt.Errorf("recovery: read: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("recovery: unmarshal: %w", err)
	}
	return m, nil
}
