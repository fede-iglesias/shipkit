package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAcquireOpenFileError exercises the os.OpenFile failure in Acquire.
// We use a path that is actually a directory, which causes os.OpenFile
// with O_RDWR to fail on macOS/Linux.
func TestAcquireOpenFileError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockDir := filepath.Join(dir, "lockdir")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// lockDir is a directory; opening it with O_RDWR|O_CREATE fails.
	_, err := Acquire(lockDir, 0)
	if err == nil {
		t.Error("expected error when lock path is a directory")
	}
}

// TestAcquireEnsureParentError exercises the EnsureParent failure in Acquire.
// We create a regular file at a position that would be the parent dir.
func TestAcquireEnsureParentError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notADir := filepath.Join(dir, "notadir")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Try to acquire a lock at notADir/lock - EnsureParent(notADir/lock) ->
	// MkdirAll(notADir) which fails because notADir is a file.
	_, err := Acquire(filepath.Join(notADir, "lock"), time.Millisecond)
	if err == nil {
		t.Error("expected error when parent of lock path is a regular file")
	}
}

// TestReleaseUnlockError exercises the error path in Release when the flock
// LOCK_UN call fails. Uses unlockFn injection.
// NOT parallel - modifies global unlockFn.
func TestReleaseUnlockError(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lk, err := Acquire(lockPath, time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	orig := unlockFn
	unlockFn = func(fd int) error {
		return fmt.Errorf("injected unlock error")
	}
	defer func() { unlockFn = orig }()

	if err := lk.Release(); err == nil {
		t.Error("expected error from injected unlock failure")
	}
}

// TestAcquireFatalFlockError exercises the fatal (non-EWOULDBLOCK) flock
// error path by injecting a custom flock function that returns an unexpected
// error.
// NOT parallel - modifies global flockFn.
func TestAcquireFatalFlockError(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	orig := flockFn
	flockFn = func(fd, how int) error {
		return fmt.Errorf("injected fatal flock error")
	}
	defer func() { flockFn = orig }()

	_, err := Acquire(lockPath, time.Second)
	if err == nil {
		t.Error("expected error from fatal flock failure")
	}
}
