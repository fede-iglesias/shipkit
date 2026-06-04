package adapters

import (
	"context"
	"os/exec"
	"testing"
	"time"

	updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
)

// TestNewSpawnBridge verifies the constructor returns a non-nil adapter.
func TestNewSpawnBridge(t *testing.T) {
	a := NewSpawnBridge()
	if a == nil {
		t.Fatal("NewSpawnBridge returned nil")
	}
	if a.inner == nil {
		t.Fatal("inner adapter is nil")
	}
}

// TestSpawnBridgeAdapter_HealthCheck_Success verifies that a successful health
// check is forwarded and the HealthResult is converted.
func TestSpawnBridgeAdapter_HealthCheck_Success(t *testing.T) {
	inner := &updateadapters.RealSpawnAdapter{
		// Use "echo" as the "binary" - it outputs "v1.2.3" which the spawn
		// adapter's semverRe will detect.
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "echo", "v1.2.3")
		},
	}
	a := &SpawnBridgeAdapter{inner: inner}

	res, err := a.HealthCheck(context.Background(), "/fake/binary", 5*time.Second)
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if !res.Ok {
		t.Errorf("HealthResult.Ok = false; want true")
	}
	if res.Version != "1.2.3" {
		t.Errorf("Version = %q; want %q", res.Version, "1.2.3")
	}
}

// TestSpawnBridgeAdapter_HealthCheck_BinaryNotFound verifies that a hard error
// (binary not found) is propagated.
func TestSpawnBridgeAdapter_HealthCheck_BinaryNotFound(t *testing.T) {
	inner := &updateadapters.RealSpawnAdapter{
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "/nonexistent-binary-coverage", "--version")
		},
	}
	a := &SpawnBridgeAdapter{inner: inner}
	_, err := a.HealthCheck(context.Background(), "/nonexistent-binary-coverage", 5*time.Second)
	if err == nil {
		t.Fatal("want error for missing binary; got nil")
	}
}

// TestSpawnBridgeAdapter_HealthCheck_NonOkResult verifies that a non-ok result
// (binary ran but no semver) is correctly bridged.
func TestSpawnBridgeAdapter_HealthCheck_NonOkResult(t *testing.T) {
	inner := &updateadapters.RealSpawnAdapter{
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "echo", "no version here")
		},
	}
	a := &SpawnBridgeAdapter{inner: inner}
	res, err := a.HealthCheck(context.Background(), "/fake/binary", 5*time.Second)
	if err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if res.Ok {
		t.Error("HealthResult.Ok = true; want false (no semver in output)")
	}
}
