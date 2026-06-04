// Package adapters provides concrete implementations of the ports defined in
// lifecycle/update/ports. Each adapter wraps an external dependency (GitHub API,
// filesystem, cosign, etc.) and can be replaced by a test double in tests.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// Compile-time check: SigstoreCosignAdapter must satisfy CosignPort.
var _ ports.CosignPort = (*SigstoreCosignAdapter)(nil)

// ErrCosignNotConfigured is returned by VerifyBundle when no production verify
// implementation has been wired. Startup code in the consumer cmd layer must call
// SetVerifyCore with sigstoreRealVerify before the adapter is used.
var ErrCosignNotConfigured = errors.New("cosign: verifyCore not configured, set via SetVerifyCore")

// sigstoreVerifyFunc is the type for the injectable verify function. The
// production implementation lives in the consumer cmd layer (e.g. cmd/myapp/update_sigstore.go)
// because it performs TUF + Rekor network calls and cannot be unit-tested
// without an integration setup.
type sigstoreVerifyFunc func(ctx context.Context, oidcIssuer, certIdentityRegex, blobPath, bundlePath string) error

// defaultVerifyCore is the placeholder used until SetVerifyCore wires a real
// implementation. Returning ErrCosignNotConfigured keeps the not-wired path
// covered by unit tests while production wiring lives in the cmd layer.
func defaultVerifyCore(_ context.Context, _, _, _, _ string) error {
	return ErrCosignNotConfigured
}

// SigstoreCosignAdapter implements ports.CosignPort with a configurable verify
// function. The real sigstore-go verification (TUF-backed) is injected at
// startup via SetVerifyCore; without that wiring VerifyBundle returns
// ErrCosignNotConfigured.
//
// VerifyFn is a high-level mock for tests that want to bypass the verify-core
// indirection entirely. Production code leaves it nil.
type SigstoreCosignAdapter struct {
	// CertIdentityRegex is the regexp matched against the certificate SAN.
	// Default: GitHub Actions workflow pattern for the consumer's repo.
	CertIdentityRegex string

	// OIDCIssuer is the expected OIDC issuer URL in the Fulcio certificate.
	// Default: GitHub Actions OIDC issuer.
	OIDCIssuer string

	// VerifyFn replaces the entire verify call in unit tests. When non-nil,
	// VerifyBundle calls it after file-existence checks and skips verifyCore.
	// Production code must leave this nil.
	VerifyFn func(ctx context.Context, blobPath, bundlePath string) error

	// verifyCore is the low-level verify implementation. Defaults to
	// defaultVerifyCore (returns ErrCosignNotConfigured). Production startup
	// calls SetVerifyCore with sigstoreRealVerify.
	verifyCore sigstoreVerifyFunc
}

// NewSigstoreCosign returns a SigstoreCosignAdapter with empty policy fields
// and verifyCore set to defaultVerifyCore. The caller must set CertIdentityRegex
// and OIDCIssuer for their repo, then call SetVerifyCore with the real verify
// implementation from the consumer cmd layer.
func NewSigstoreCosign() *SigstoreCosignAdapter {
	return &SigstoreCosignAdapter{
		verifyCore: defaultVerifyCore,
	}
}

// SetVerifyCore wires a real verify implementation into the adapter. Production
// startup must call this with sigstoreRealVerify (from the consumer cmd layer);
// otherwise VerifyBundle returns ErrCosignNotConfigured.
func (a *SigstoreCosignAdapter) SetVerifyCore(fn sigstoreVerifyFunc) {
	a.verifyCore = fn
}

// VerifyBundle verifies that the file at blobPath is covered by the sigstore
// bundle at bundlePath using the configured CertIdentityRegex and OIDCIssuer
// policy.
//
// Returns nil on success. Returns an error if:
//   - ctx is already cancelled
//   - blobPath or bundlePath do not exist or are not readable
//   - the bundle is invalid or the identity policy does not match
//   - the verifyCore has not been wired (ErrCosignNotConfigured)
func (a *SigstoreCosignAdapter) VerifyBundle(ctx context.Context, blobPath, bundlePath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := os.Stat(blobPath); err != nil {
		return fmt.Errorf("cosign: blob not found at %q: %w", blobPath, err)
	}
	if _, err := os.Stat(bundlePath); err != nil {
		return fmt.Errorf("cosign: bundle not found at %q: %w", bundlePath, err)
	}

	if a.VerifyFn != nil {
		return a.VerifyFn(ctx, blobPath, bundlePath)
	}

	return a.verifyCore(ctx, a.OIDCIssuer, a.CertIdentityRegex, blobPath, bundlePath)
}
