package ports_test

import (
	"context"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// TestSpawnPort_InterfaceCompliance verifies that SpawnPort is an interface
// type, not a struct or concrete type. We assert this by declaring a variable
// of the interface type - if it were a concrete type, the assignment would
// behave differently. The nil assignment also confirms it can hold nil.
func TestSpawnPort_InterfaceCompliance(t *testing.T) {
	var _ ports.SpawnPort = (*stubSpawn)(nil)
}

// TestHealthResult_Fields verifies that HealthResult has all three fields
// (Version, Ok, Reason) with the expected types and that field access compiles.
func TestHealthResult_Fields(t *testing.T) {
	hr := ports.HealthResult{
		Version: "v0.0.12",
		Ok:      true,
		Reason:  "healthy",
	}

	if hr.Version != "v0.0.12" {
		t.Errorf("Version = %q, want %q", hr.Version, "v0.0.12")
	}
	if !hr.Ok {
		t.Error("Ok = false, want true")
	}
	if hr.Reason != "healthy" {
		t.Errorf("Reason = %q, want %q", hr.Reason, "healthy")
	}
}

// stubSpawn is a local test double that satisfies SpawnPort.
// It lets TestHealthCheck_SignatureType verify the interface method signature
// without importing os/exec or touching the filesystem.
type stubSpawn struct {
	result ports.HealthResult
	err    error
}

func (s *stubSpawn) HealthCheck(
	_ context.Context,
	_ string,
	_ time.Duration,
) (ports.HealthResult, error) {
	return s.result, s.err
}

// TestHealthCheck_SignatureType confirms the HealthCheck method accepts the
// exact parameter types (context.Context, string, time.Duration) and returns
// (HealthResult, error). The stub above only compiles if the signature matches.
func TestHealthCheck_SignatureType(t *testing.T) {
	var sp ports.SpawnPort = &stubSpawn{
		result: ports.HealthResult{Version: "v0.1.0", Ok: true},
	}

	hr, err := sp.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Version != "v0.1.0" {
		t.Errorf("Version = %q, want %q", hr.Version, "v0.1.0")
	}
	if !hr.Ok {
		t.Error("Ok = false, want true")
	}
}

// TestHealthResult_ZeroValueOkFalse verifies that the zero value of HealthResult
// has Ok=false, which is the safe default (do not assume healthy unless set).
func TestHealthResult_ZeroValueOkFalse(t *testing.T) {
	var hr ports.HealthResult
	if hr.Ok {
		t.Error("zero-value HealthResult.Ok = true, want false (safe default)")
	}
	if hr.Version != "" {
		t.Errorf("zero-value HealthResult.Version = %q, want empty string", hr.Version)
	}
	if hr.Reason != "" {
		t.Errorf("zero-value HealthResult.Reason = %q, want empty string", hr.Reason)
	}
}
