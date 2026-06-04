package ports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockSpawnPort satisfies SpawnPort.
var _ ports.SpawnPort = (*ports.MockSpawnPort)(nil)

func TestMockSpawnPort_HealthCheck_default(t *testing.T) {
	m := ports.NewMockSpawnPort()
	result, err := m.HealthCheck(context.Background(), "/usr/local/bin/app", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ok {
		t.Error("expected Ok=true by default")
	}
	if len(m.HealthCheckCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.HealthCheckCalls))
	}
	if m.HealthCheckCalls[0] != "/usr/local/bin/app" {
		t.Errorf("expected binary path recorded, got %q", m.HealthCheckCalls[0])
	}
}

func TestMockSpawnPort_HealthCheck_fail(t *testing.T) {
	m := ports.NewMockSpawnPort()
	m.HealthCheckFunc = func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{Ok: false, Reason: "timeout"}, nil
	}
	result, err := m.HealthCheck(context.Background(), "/bin/app", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ok {
		t.Error("expected Ok=false")
	}
	if result.Reason != "timeout" {
		t.Errorf("expected reason 'timeout', got %q", result.Reason)
	}
}

func TestMockSpawnPort_HealthCheck_error(t *testing.T) {
	m := ports.NewMockSpawnPort()
	sentinel := errors.New("binary not found")
	m.HealthCheckFunc = func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{}, sentinel
	}
	_, err := m.HealthCheck(context.Background(), "/missing", time.Second)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}
