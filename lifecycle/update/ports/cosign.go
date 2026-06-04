package ports

import "context"

// CosignPort abstracts sigstore cosign bundle verification.
// Implementations must use sigstore-go embedded API (no os/exec cosign binary).
type CosignPort interface {
	// VerifyBundle verifies that blobPath (e.g. a .tar.gz asset) matches the
	// sigstore bundle at bundlePath using the configured identity policy
	// (certificate-identity-regexp + oidc-issuer). Returns nil on success and
	// a descriptive error if the signature is absent, invalid, or the bundle
	// does not match the blob.
	VerifyBundle(ctx context.Context, blobPath, bundlePath string) error
}
