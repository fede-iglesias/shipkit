package migrations

import (
	"context"
	"fmt"
)

// Migration describes a versioned, idempotent transformation applied to a
// knowledge tree when a shipkit consumer upgrades from one version to another.
//
// Implementations MUST be idempotent: running Apply (or Revert) twice on the
// same root must produce the same result as running it once.
type Migration interface {
	// Version returns the target version this migration applies to (semver).
	// Must be a non-empty string in "MAJOR.MINOR.PATCH" form, optionally
	// prefixed with "v". Example: "v0.2.0" or "1.0.0".
	Version() string

	// Description returns a short human-readable description of what this
	// migration does. Used in log and error messages.
	Description() string

	// Apply applies the migration to the tree at root. Must be idempotent:
	// calling Apply on an already-migrated tree must succeed without
	// double-applying changes.
	Apply(ctx context.Context, root string) error

	// Revert undoes the migration for the tree at root. Must be idempotent:
	// calling Revert on a not-yet-migrated (or already-reverted) tree must
	// succeed without error.
	Revert(ctx context.Context, root string) error
}

// Registry holds a sorted set of migrations and exposes upgrade and rollback
// operations. Use [New] to construct one.
//
// Registry is NOT safe for concurrent use. Create one per upgrade operation.
type Registry struct {
	migrations []Migration // sorted ascending by Version semver
}

// New returns an empty Registry ready to accept [Migration] registrations.
func New() *Registry {
	return &Registry{}
}

// Register adds m to the registry and keeps the internal slice sorted by
// semver ascending. It is safe to call Register in any order: the registry
// always maintains its sorted invariant after each call.
func (r *Registry) Register(m Migration) {
	r.migrations = append(r.migrations, m)
	sortMigrations(r.migrations)
}

// Pending returns the migrations whose Version is strictly greater than
// current and less than or equal to target, in ascending semver order.
//
// An empty current string is treated as a version below everything, so all
// migrations up to and including target are returned. If current >= target,
// the returned slice is empty.
//
// The returned slice must not be modified by the caller.
//
// Pending never returns a non-nil error. The error return exists to keep the
// signature open for future validation without breaking callers.
func (r *Registry) Pending(current, target string) ([]Migration, error) {
	var result []Migration
	for _, m := range r.migrations {
		// Skip migrations at or below current.
		if current != "" && compareSemver(m.Version(), current) <= 0 {
			continue
		}
		// Stop after target.
		if compareSemver(m.Version(), target) > 0 {
			break
		}
		result = append(result, m)
	}
	return result, nil
}

// ApplyPending runs all pending migrations from current to target in ascending
// semver order. It stops on the first error and returns the number of
// migrations that completed successfully together with the error.
//
// The root argument is forwarded to each [Migration.Apply] call.
//
// Returns:
//   - (n, nil) on full success, where n is the count of applied migrations.
//   - (n, err) on partial failure: n migrations succeeded before the error;
//     the failed migration's error is wrapped with version and description.
func (r *Registry) ApplyPending(ctx context.Context, root, current, target string) (count int, err error) {
	pending, _ := r.Pending(current, target) //nolint:errcheck // Pending never errors.
	for _, m := range pending {
		if applyErr := m.Apply(ctx, root); applyErr != nil {
			return count, fmt.Errorf("migrations: applying %s (%s): %w", m.Version(), m.Description(), applyErr)
		}
		count++
	}
	return count, nil
}

// Revert undoes migrations from fromTarget down to (but not including)
// toCurrent, in reverse semver order (highest version reverted first).
//
// The root argument is forwarded to each [Migration.Revert] call.
//
// Returns nil on full success. On the first failure, Revert stops and returns
// an error wrapping the migration's version and description.
func (r *Registry) Revert(ctx context.Context, root, fromTarget, toCurrent string) error {
	// Collect the migrations that were applied (toCurrent < ver <= fromTarget).
	toRevert, _ := r.Pending(toCurrent, fromTarget) //nolint:errcheck // Pending never errors.
	// Reverse the slice so we undo from highest to lowest.
	for i, j := 0, len(toRevert)-1; i < j; i, j = i+1, j-1 {
		toRevert[i], toRevert[j] = toRevert[j], toRevert[i]
	}
	for _, m := range toRevert {
		if revertErr := m.Revert(ctx, root); revertErr != nil {
			return fmt.Errorf("migrations: reverting %s (%s): %w", m.Version(), m.Description(), revertErr)
		}
	}
	return nil
}
