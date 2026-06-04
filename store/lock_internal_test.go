package store

import (
	"testing"
)

// TestReleaseNilLock verifies that Release on a nil Lock is a no-op.
func TestReleaseNilLock(t *testing.T) {
	t.Parallel()

	var lk *Lock
	if err := lk.Release(); err != nil {
		t.Errorf("Release on nil Lock: got %v, want nil", err)
	}
}

// TestReleaseLockWithNilFile verifies that Release on a Lock with nil file is a no-op.
func TestReleaseLockWithNilFile(t *testing.T) {
	t.Parallel()

	lk := &Lock{f: nil}
	if err := lk.Release(); err != nil {
		t.Errorf("Release on Lock{f:nil}: got %v, want nil", err)
	}
}
