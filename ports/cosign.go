package ports

import "context"

// CosignPort abstracts sigstore cosign bundle verification.
//
// Implementations must use the sigstore-go embedded API and must NOT invoke an
// external cosign binary via os/exec (D-7 constraint). The embedded verifier
// performs TUF root resolution, Rekor transparency log inclusion, and
// certificate-identity matching entirely in-process.
//
// The production adapter lives in shipkit/adapters. The consumer cmd layer
// wires the real verify function; the library default returns
// ErrCosignNotConfigured so that unit tests can reach 100% coverage without
// network access (sigstoreRealVerify pattern from kt v0.1.3).
type CosignPort interface {
	// VerifyBundle verifies that blobPath (e.g. a .tar.gz asset) matches the
	// sigstore bundle at bundlePath using the configured identity policy
	// (certificate-identity-regexp + oidc-issuer). Returns nil on success and
	// a descriptive error if the signature is absent, invalid, or the bundle
	// does not match the blob.
	VerifyBundle(ctx context.Context, blobPath, bundlePath string) error
}

// MockCosignPort is a test double for CosignPort. It records calls and returns
// the values set on its Func field. Use NewMockCosignPort for safe defaults.
type MockCosignPort struct {
	// VerifyBundleFunc overrides VerifyBundle when non-nil.
	VerifyBundleFunc func(ctx context.Context, blobPath, bundlePath string) error

	// VerifyBundleCalls records each (blobPath, bundlePath) pair passed to VerifyBundle.
	VerifyBundleCalls [][2]string
}

// NewMockCosignPort returns a MockCosignPort whose VerifyBundle returns nil
// (pass) unless VerifyBundleFunc is set.
func NewMockCosignPort() *MockCosignPort { return &MockCosignPort{} }

// VerifyBundle implements CosignPort.
func (m *MockCosignPort) VerifyBundle(ctx context.Context, blobPath, bundlePath string) error {
	m.VerifyBundleCalls = append(m.VerifyBundleCalls, [2]string{blobPath, bundlePath})
	if m.VerifyBundleFunc != nil {
		return m.VerifyBundleFunc(ctx, blobPath, bundlePath)
	}
	return nil
}
