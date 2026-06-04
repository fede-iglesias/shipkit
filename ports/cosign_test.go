package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockCosignPort satisfies CosignPort.
var _ ports.CosignPort = (*ports.MockCosignPort)(nil)

func TestMockCosignPort_VerifyBundle_default(t *testing.T) {
	m := ports.NewMockCosignPort()
	err := m.VerifyBundle(context.Background(), "/tmp/app.tar.gz", "/tmp/app.tar.gz.bundle")
	if err != nil {
		t.Fatalf("expected nil by default, got %v", err)
	}
	if len(m.VerifyBundleCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.VerifyBundleCalls))
	}
	if m.VerifyBundleCalls[0][0] != "/tmp/app.tar.gz" {
		t.Errorf("blobPath not recorded correctly")
	}
}

func TestMockCosignPort_VerifyBundle_fail(t *testing.T) {
	m := ports.NewMockCosignPort()
	sentinel := errors.New("certificate identity mismatch")
	m.VerifyBundleFunc = func(_ context.Context, _, _ string) error { return sentinel }
	err := m.VerifyBundle(context.Background(), "blob", "bundle")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockCosignPort_VerifyBundle_pass(t *testing.T) {
	m := ports.NewMockCosignPort()
	m.VerifyBundleFunc = func(_ context.Context, _, _ string) error { return nil }
	if err := m.VerifyBundle(context.Background(), "blob", "bundle"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
