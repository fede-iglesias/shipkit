package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RecoveryManifestFilename is the canonical name of the recovery manifest file
// written under the data root when an unrecoverable rollback failure occurs.
const RecoveryManifestFilename = ".update.recovery-manifest.json"

// Package-level injectable function variables. The defaults are the real os
// functions; tests swap them to inject failures without real filesystem I/O.
var writeFileFn = os.WriteFile
var renameFn = os.Rename
var mkdirAllFn = os.MkdirAll
var readFileFn = os.ReadFile
var removeFn = os.Remove
var jsonMarshalFn = func(v any) ([]byte, error) { return json.Marshal(v) }

// PersistRecoveryManifest writes manifest to dataRoot/RecoveryManifestFilename
// atomically using a temp-file + rename pattern.
//
// The directory dataRoot is created (with mode 0700) if it does not exist.
// Returns the first error encountered; on error the final file is not written
// (the rename never happened).
func PersistRecoveryManifest(dataRoot string, manifest *RecoveryManifest) error {
	if err := mkdirAllFn(dataRoot, 0o700); err != nil {
		return fmt.Errorf("create data root %q: %w", dataRoot, err)
	}

	data, err := jsonMarshalFn(manifest)
	if err != nil {
		return fmt.Errorf("marshal recovery manifest: %w", err)
	}

	dest := filepath.Join(dataRoot, RecoveryManifestFilename)
	tmp := dest + ".tmp"

	if err := writeFileFn(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp manifest %q: %w", tmp, err)
	}

	if err := renameFn(tmp, dest); err != nil {
		return fmt.Errorf("rename manifest %q -> %q: %w", tmp, dest, err)
	}

	return nil
}

// LoadRecoveryManifest reads the manifest from dataRoot/RecoveryManifestFilename.
//
// Returns an os.IsNotExist error when the file is absent so callers can
// distinguish "no prior failure" from other I/O errors.
func LoadRecoveryManifest(dataRoot string) (*RecoveryManifest, error) {
	path := filepath.Join(dataRoot, RecoveryManifestFilename)

	data, err := readFileFn(path)
	if err != nil {
		// Preserve the original error so os.IsNotExist works for callers.
		return nil, err
	}

	var m RecoveryManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal recovery manifest %q: %w", path, err)
	}

	return &m, nil
}

// ClearRecoveryManifest removes dataRoot/RecoveryManifestFilename.
//
// It is best-effort: a missing file is silently ignored. Any other removal
// error is returned to the caller.
func ClearRecoveryManifest(dataRoot string) error {
	path := filepath.Join(dataRoot, RecoveryManifestFilename)

	if err := removeFn(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove recovery manifest %q: %w", path, err)
	}

	return nil
}

// AppendRecoveryStep appends a new RecoveryStep to manifest.Steps in-memory.
// It performs no I/O; call PersistRecoveryManifest to write the updated
// manifest to disk.
func AppendRecoveryStep(manifest *RecoveryManifest, action, detail string) {
	manifest.Steps = append(manifest.Steps, RecoveryStep{
		Action: action,
		Detail: detail,
	})
}
