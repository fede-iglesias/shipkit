package update

import (
	"errors"
	"fmt"
)

// ErrTarballEntryEscapes is returned when a tarball entry resolves to a path
// outside the destination directory or is a disallowed link type (symlink/hardlink).
var ErrTarballEntryEscapes = errors.New("tarball entry escapes destination directory")

// SnapshotError is returned when snapshotting the current binary or tree fails.
type SnapshotError struct {
	// Path is the path that was being snapshotted.
	Path string
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *SnapshotError) Error() string {
	return fmt.Sprintf("snapshot failed at %q: %v", e.Path, e.Err)
}

// Unwrap returns the underlying cause so errors.Is and errors.As traverse the chain.
func (e *SnapshotError) Unwrap() error { return e.Err }

// VerifyError is returned when cosign bundle verification fails.
type VerifyError struct {
	// Asset is the name of the asset whose bundle was being verified.
	Asset string
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *VerifyError) Error() string {
	return fmt.Sprintf("cosign verify failed for asset %q: %v", e.Asset, e.Err)
}

// Unwrap returns the underlying cause so errors.Is and errors.As traverse the chain.
func (e *VerifyError) Unwrap() error { return e.Err }

// ReplaceError is returned when the atomic binary replace step fails.
type ReplaceError struct {
	// Target is the path of the binary that was being replaced.
	Target string
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *ReplaceError) Error() string {
	return fmt.Sprintf("atomic replace failed for target %q: %v", e.Target, e.Err)
}

// Unwrap returns the underlying cause so errors.Is and errors.As traverse the chain.
func (e *ReplaceError) Unwrap() error { return e.Err }

// MigrationError is returned when a tree migration fails for a given target version.
type MigrationError struct {
	// Version is the target version whose migration failed.
	Version string
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *MigrationError) Error() string {
	return fmt.Sprintf("migration to version %q failed: %v", e.Version, e.Err)
}

// Unwrap returns the underlying cause so errors.Is and errors.As traverse the chain.
func (e *MigrationError) Unwrap() error { return e.Err }

// RollbackError is returned when a rollback step fails at a specific state.
type RollbackError struct {
	// At is the state at which the rollback failed.
	At State
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *RollbackError) Error() string {
	return fmt.Sprintf("rollback failed at state %q: %v", e.At, e.Err)
}

// Unwrap returns the underlying cause so errors.Is and errors.As traverse the chain.
func (e *RollbackError) Unwrap() error { return e.Err }

// RollbackUnrecoverableError is returned when the rollback itself encounters an
// unrecoverable failure. The Manifest field contains the human-readable recovery
// steps that the operator must perform manually.
type RollbackUnrecoverableError struct {
	// Manifest holds the manual recovery steps.
	Manifest *RecoveryManifest
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *RollbackUnrecoverableError) Error() string {
	return fmt.Sprintf("unrecoverable rollback failure: %v", e.Err)
}

// Unwrap returns the underlying cause so errors.Is and errors.As traverse the chain.
func (e *RollbackUnrecoverableError) Unwrap() error { return e.Err }

// RecoveryManifest is persisted to disk when an unrecoverable failure occurs.
// It documents the manual steps the operator must follow to restore the system.
type RecoveryManifest struct {
	Steps []RecoveryStep `json:"steps"`
	Cause string         `json:"cause"`
}

// RecoveryStep is a single manual action the operator must perform.
type RecoveryStep struct {
	Action string `json:"action"`
	Detail string `json:"detail"`
}
