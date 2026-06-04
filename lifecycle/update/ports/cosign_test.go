package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// mockCosign is a compile-time proof that CosignPort is implementable.
type mockCosign struct {
	verifyBundleFunc func(ctx context.Context, blobPath, bundlePath string) error
}

func (m *mockCosign) VerifyBundle(ctx context.Context, blobPath, bundlePath string) error {
	return m.verifyBundleFunc(ctx, blobPath, bundlePath)
}

// TestCosignPort_InterfaceCompliance asserts at compile time that *mockCosign
// satisfies CosignPort. If CosignPort changes, this line fails compilation.
var _ ports.CosignPort = (*mockCosign)(nil)

// TestVerifyBundle_SignatureType verifies that a mock CosignPort implementation
// receives blobPath and bundlePath correctly through the interface.
func TestVerifyBundle_SignatureType(t *testing.T) {
	t.Parallel()

	var gotBlob, gotBundle string
	m := &mockCosign{
		verifyBundleFunc: func(_ context.Context, blobPath, bundlePath string) error {
			gotBlob = blobPath
			gotBundle = bundlePath
			return nil
		},
	}

	err := m.VerifyBundle(context.Background(), "/tmp/myapp.tar.gz", "/tmp/myapp.tar.gz.bundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBlob != "/tmp/myapp.tar.gz" {
		t.Errorf("blobPath: got %q, want %q", gotBlob, "/tmp/myapp.tar.gz")
	}
	if gotBundle != "/tmp/myapp.tar.gz.bundle" {
		t.Errorf("bundlePath: got %q, want %q", gotBundle, "/tmp/myapp.tar.gz.bundle")
	}
}

// TestVerifyBundle_ContextRespect verifies that a mock CosignPort can propagate
// a cancelled context to the caller via an error return, as a real implementation
// would do when ctx.Done() fires during a sigstore network call.
func TestVerifyBundle_ContextRespect(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	m := &mockCosign{
		verifyBundleFunc: func(ctx context.Context, _, _ string) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	}

	err := m.VerifyBundle(ctx, "/tmp/myapp.tar.gz", "/tmp/myapp.tar.gz.bundle")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
