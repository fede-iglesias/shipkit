package update_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/recovery"
	"github.com/fede-iglesias/shipkit/lifecycle/update"
)

// TestSnapshotError verifies Error() format and Unwrap().
func TestSnapshotError(t *testing.T) {
	cause := errors.New("permission denied")
	e := &update.SnapshotError{Path: "/tmp/snap", Err: cause}

	got := e.Error()
	if got == "" {
		t.Fatal("SnapshotError.Error() returned empty string")
	}
	if !contains(got, "/tmp/snap") {
		t.Errorf("SnapshotError.Error() = %q, want it to contain path %q", got, "/tmp/snap")
	}
	if unwrapped := errors.Unwrap(e); unwrapped != cause {
		t.Errorf("SnapshotError.Unwrap() = %v, want %v", unwrapped, cause)
	}
	// errors.Is should traverse via Unwrap.
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(SnapshotError, cause) = false, want true")
	}
}

// TestVerifyError verifies Error() format and Unwrap().
func TestVerifyError(t *testing.T) {
	cause := errors.New("bad signature")
	e := &update.VerifyError{Asset: "myapp-darwin-arm64.tar.gz", Err: cause}

	got := e.Error()
	if !contains(got, "myapp-darwin-arm64.tar.gz") {
		t.Errorf("VerifyError.Error() = %q, want it to contain asset name", got)
	}
	if unwrapped := errors.Unwrap(e); unwrapped != cause {
		t.Errorf("VerifyError.Unwrap() = %v, want %v", unwrapped, cause)
	}
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(VerifyError, cause) = false, want true")
	}
}

// TestReplaceError verifies Error() format and Unwrap().
func TestReplaceError(t *testing.T) {
	cause := errors.New("disk full")
	e := &update.ReplaceError{Target: "/usr/local/bin/myapp", Err: cause}

	got := e.Error()
	if !contains(got, "/usr/local/bin/myapp") {
		t.Errorf("ReplaceError.Error() = %q, want it to contain target", got)
	}
	if unwrapped := errors.Unwrap(e); unwrapped != cause {
		t.Errorf("ReplaceError.Unwrap() = %v, want %v", unwrapped, cause)
	}
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(ReplaceError, cause) = false, want true")
	}
}

// TestMigrationError verifies Error() format and Unwrap().
func TestMigrationError(t *testing.T) {
	cause := errors.New("schema mismatch")
	e := &update.MigrationError{Version: "0.0.11", Err: cause}

	got := e.Error()
	if !contains(got, "0.0.11") {
		t.Errorf("MigrationError.Error() = %q, want it to contain version", got)
	}
	if unwrapped := errors.Unwrap(e); unwrapped != cause {
		t.Errorf("MigrationError.Unwrap() = %v, want %v", unwrapped, cause)
	}
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(MigrationError, cause) = false, want true")
	}
}

// TestRollbackError verifies Error() format and Unwrap().
func TestRollbackError(t *testing.T) {
	cause := errors.New("restore failed")
	e := &update.RollbackError{At: update.StateRestoreOldBinary, Err: cause}

	got := e.Error()
	if !contains(got, "restore-old-binary") {
		t.Errorf("RollbackError.Error() = %q, want it to contain state name", got)
	}
	if unwrapped := errors.Unwrap(e); unwrapped != cause {
		t.Errorf("RollbackError.Unwrap() = %v, want %v", unwrapped, cause)
	}
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(RollbackError, cause) = false, want true")
	}
}

// TestRollbackUnrecoverableError verifies Error() format, Unwrap(), and Manifest presence.
func TestRollbackUnrecoverableError(t *testing.T) {
	cause := errors.New("catastrophic failure")
	manifest := &recovery.Manifest{
		Version: 1,
		AppName: "myapp",
		Steps: []string{
			"manual-binary-restore: copy /tmp/snap to /usr/local/bin/myapp",
		},
		Cause: cause.Error(),
	}
	e := &update.RollbackUnrecoverableError{Manifest: manifest, Err: cause}

	got := e.Error()
	if got == "" {
		t.Fatal("RollbackUnrecoverableError.Error() returned empty string")
	}
	if unwrapped := errors.Unwrap(e); unwrapped != cause {
		t.Errorf("RollbackUnrecoverableError.Unwrap() = %v, want %v", unwrapped, cause)
	}
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(RollbackUnrecoverableError, cause) = false, want true")
	}
	if e.Manifest == nil {
		t.Error("RollbackUnrecoverableError.Manifest is nil, want non-nil")
	}
	if len(e.Manifest.Steps) != 1 || e.Manifest.Steps[0] == "" {
		t.Errorf("RollbackUnrecoverableError.Manifest.Steps = %v, want one non-empty entry", e.Manifest.Steps)
	}
}

// TestErrorWrapping_AsType verifies errors.As works through the chain.
func TestErrorWrapping_AsType(t *testing.T) {
	cause := fmt.Errorf("root cause")

	// Wrap in SnapshotError, then wrap again.
	inner := &update.SnapshotError{Path: "/snap", Err: cause}
	outer := fmt.Errorf("outer: %w", inner)

	var target *update.SnapshotError
	if !errors.As(outer, &target) {
		t.Error("errors.As through fmt.Errorf wrapping did not find SnapshotError")
	}
	if target.Path != "/snap" {
		t.Errorf("target.Path = %q, want %q", target.Path, "/snap")
	}
}

// contains is a helper to check substring presence (avoids importing strings in test).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
