package adapters_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
)

// ---------------------------------------------------------------------------
// B7: Restore preserves the snapshot's executable bit (perm bits).
//
// Before the fix, Restore wrote to a temp file via os.Create which produces
// 0o666-pre-umask (typically 0o644 on macOS/Linux) and then atomically
// renamed that temp file over the target. The rename inherits the temp
// file's perms, so a binary that started at 0o755 came back as 0o644 after
// a rolled-back failed update. Users observed:
//
//     $ relay update     # update fails, rollback runs
//     $ relay version    # zsh: permission denied: relay
//
// because the rollback "succeeded" but the binary was no longer executable.
//
// The fix captures the snapshot's mode via StatFn before the swap and
// re-applies it via ChmodFn after the rename. When the snapshot has no
// execute bit (which should not happen for a real shipkit-managed binary
// but is a defensible edge case for tests or corrupt snapshots), Restore
// falls back to 0o755 because a shipkit binary must remain executable.
// ---------------------------------------------------------------------------

// TestRestore_PreservesExecutableBit_0755 is the EXACT repro for the
// 2026-06-05 relay v0.1.1 incident: a snapshot at 0o755 must come back at
// 0o755 after Restore, not at 0o644.
func TestRestore_PreservesExecutableBit_0755(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("perm bits semantics differ on Windows")
	}
	dir := t.TempDir()

	// Snapshot at the canonical binary mode 0o755.
	snapSubdir := filepath.Join(dir, "snapshots", "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapFile := filepath.Join(snapSubdir, "relay")
	if err := os.WriteFile(snapFile, []byte("relay-binary-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make sure the snapshot has 0o755 exact (umask could have trimmed it).
	if err := os.Chmod(snapFile, 0o755); err != nil {
		t.Fatal(err)
	}

	// dst starts as a broken upgrade attempt with default 0o644.
	dst := filepath.Join(dir, "bin", "relay")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("broken-upgrade"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	if err := a.Restore(context.Background(), snapSubdir, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Assertions on CONTENT and PERMS, not just shape.
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "relay-binary-bytes" {
		t.Errorf("restored content = %q, want %q", string(got), "relay-binary-bytes")
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o755 {
		t.Fatalf("restored perm = %o, want 0755 (B7: rollback dropped executable bit)", perm)
	}
}

// TestRestore_PreservesGroupExecBit_0750 asserts the implementation
// actually copies the snapshot's exact mode, not a hard-coded 0o755.
func TestRestore_PreservesGroupExecBit_0750(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("perm bits semantics differ on Windows")
	}
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snapshots", "snapX")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapFile := filepath.Join(snapSubdir, "tool")
	if err := os.WriteFile(snapFile, []byte("tool"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(snapFile, 0o750); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	if err := a.Restore(context.Background(), snapSubdir, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o750 {
		t.Fatalf("restored perm = %o, want 0750 (Restore must copy exact snapshot mode)", perm)
	}
}

// TestRestore_FallsBackTo0755_WhenSnapshotHasNoExecBit asserts the
// defensive fallback: if the snapshot somehow lost its exec bit (corrupt
// snapshot, manual fiddling, test fixture), Restore still produces an
// executable binary because a shipkit-managed CLI MUST be executable.
func TestRestore_FallsBackTo0755_WhenSnapshotHasNoExecBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("perm bits semantics differ on Windows")
	}
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snapshots", "snapNoExec")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapFile := filepath.Join(snapSubdir, "tool")
	if err := os.WriteFile(snapFile, []byte("tool"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	if err := a.Restore(context.Background(), snapSubdir, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o755 {
		t.Fatalf("restored perm = %o, want 0o755 fallback for non-executable snapshot", perm)
	}
}

// TestRestore_NilChmodFn_FallsBackToOsChmod asserts the defensive
// fallback for adapters constructed via struct literal that forget to wire
// ChmodFn: Restore falls back to os.Chmod and still produces an executable
// file. This mirrors the historical pattern where NewRealFs is the
// production constructor but tests sometimes build a raw struct literal.
func TestRestore_NilChmodFn_FallsBackToOsChmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("perm bits semantics differ on Windows")
	}
	dir := t.TempDir()

	snapSubdir := filepath.Join(dir, "snapshots", "snap")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapFile := filepath.Join(snapSubdir, "bin")
	if err := os.WriteFile(snapFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(snapFile, 0o755); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "bin")
	if err := os.WriteFile(dst, []byte("cur"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	a.ChmodFn = nil // simulate struct-literal construction missing ChmodFn

	if err := a.Restore(context.Background(), snapSubdir, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o755 {
		t.Fatalf("nil ChmodFn fallback: perm = %o, want 0o755", perm)
	}
}

// TestRestore_ChmodErrorIsSurfaced asserts that when the Chmod step fails
// (e.g. permission denied on the target), Restore returns a clean error
// rather than silently swallowing the perm bug.
func TestRestore_ChmodErrorIsSurfaced(t *testing.T) {
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snapshots", "snap")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "bin"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "bin")
	if err := os.WriteFile(dst, []byte("cur"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	chmodErr := errors.New("chmod boom")
	a.ChmodFn = func(string, os.FileMode) error { return chmodErr }

	err := a.Restore(context.Background(), snapSubdir, dst)
	if err == nil {
		t.Fatal("want error when ChmodFn fails")
	}
	if !errors.Is(err, chmodErr) {
		t.Errorf("want wrapped chmodErr, got %v", err)
	}
}
