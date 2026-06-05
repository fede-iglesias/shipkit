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

// ErrCosignNotConfigured is retained for backward compatibility. As of v0.2.4
// NewSigstoreCosign wires the real sigstore-go verifier as the default, so
// VerifyBundle no longer returns this error unless the caller explicitly
// disables verifyCore via SetVerifyCore(nil) or replaces it with a stub that
// returns it.
var ErrCosignNotConfigured = errors.New("cosign: verifyCore not configured, set via SetVerifyCore")

// sigstoreVerifyFunc is the type for the injectable verify function. The
// default implementation (sigstoreRealVerify in cosign_sigstore_real.go) makes
// TUF + Rekor network calls and is exercised at integration time. Tests inject
// a stub via SetVerifyCore.
type sigstoreVerifyFunc func(ctx context.Context, oidcIssuer, certIdentityRegex, blobPath, bundlePath string) error

// errNotConfiguredVerifyCore is a no-op verifyCore that always returns
// ErrCosignNotConfigured. Used only when callers explicitly pass nil to
// SetVerifyCore, preserving the historical "not configured" sentinel for
// tests that exercise that branch.
func errNotConfiguredVerifyCore(_ context.Context, _, _, _, _ string) error {
	return ErrCosignNotConfigured
}

// SigstoreCosignAdapter implements ports.CosignPort with a configurable verify
// function. As of v0.2.4 NewSigstoreCosign wires sigstoreRealVerify (real
// sigstore-go TUF + Rekor verification) as the default so the adapter works
// out-of-the-box without consumer wiring. Tests override the default via
// SetVerifyCore.
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
	// sigstoreRealVerify (real TUF + Rekor verification). Tests override via
	// SetVerifyCore; passing nil to SetVerifyCore restores the legacy
	// "not configured" sentinel for back-compat coverage of that branch.
	verifyCore sigstoreVerifyFunc
}

// NewSigstoreCosign returns a SigstoreCosignAdapter with empty policy fields
// and verifyCore set to the real sigstore-go verifier (sigstoreRealVerify).
// The caller must still set CertIdentityRegex and OIDCIssuer for their repo;
// without those the verifier will reject every bundle because the certificate
// identity will not match.
//
// As of v0.2.4 the consumer no longer needs to call SetVerifyCore for the
// adapter to function in production; the default is the real TUF-backed
// implementation. SetVerifyCore is retained for tests and for advanced
// consumers that want to plug in a custom verifier.
func NewSigstoreCosign() *SigstoreCosignAdapter {
	return &SigstoreCosignAdapter{
		verifyCore: sigstoreRealVerify,
	}
}

// SetVerifyCore replaces the low-level verify implementation. Tests inject a
// stub here to avoid real TUF + Rekor network calls. Passing nil restores the
// legacy "not configured" sentinel: VerifyBundle returns ErrCosignNotConfigured
// in that case so callers can observe an unwired core if they explicitly opted
// in to that behavior.
//
// Production code typically does NOT need to call this; NewSigstoreCosign
// already wires sigstoreRealVerify as the default.
func (a *SigstoreCosignAdapter) SetVerifyCore(fn sigstoreVerifyFunc) {
	if fn == nil {
		a.verifyCore = errNotConfiguredVerifyCore
		return
	}
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
//   - verifyCore has been disabled via SetVerifyCore(nil) (ErrCosignNotConfigured)
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
