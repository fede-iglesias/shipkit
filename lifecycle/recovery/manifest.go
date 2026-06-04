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

// Write serializes m to path using a temp+rename pattern so partial writes
// are never observable. The parent directory must already exist.
func Write(path string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("recovery: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".shipkit-recovery-*")
	if err != nil {
		return fmt.Errorf("recovery: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("recovery: write: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("recovery: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("recovery: close: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("recovery: rename: %w", err)
	}
	return nil
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
