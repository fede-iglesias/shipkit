package adapters

import (
	"context"
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	sigroot "github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// sigstoreRealVerify performs a real sigstore-go bundle verification using the
// Sigstore public-good TUF trusted root. Makes network calls (TUF + Rekor) and
// cannot be unit-tested without a live Sigstore infrastructure; coverage of
// this function is exercised at integration time via the consumer's update
// flow against a real release.
//
// This function is wired as the default verifyCore in NewSigstoreCosign so the
// adapter works out-of-the-box for consumers that distribute via GitHub
// Releases with the standard goreleaser+cosign pipeline. Tests inject a stub
// via SetVerifyCore.
//
// Promoted from example/shipkit-example/cmd/shipkit-example/main.go in v0.2.4
// to fix the "verifyCore not configured" runtime error when consumers imported
// the adapter without manually wiring SetVerifyCore.
func sigstoreRealVerify(ctx context.Context, oidcIssuer, certIdentityRegex, blobPath, bundlePath string) error {
	opts := tuf.DefaultOptions()
	client, err := tuf.New(opts)
	if err != nil {
		return fmt.Errorf("cosign: create TUF client: %w", err)
	}

	trustedMaterial, err := sigroot.GetTrustedRoot(client)
	if err != nil {
		return fmt.Errorf("cosign: get trusted root: %w", err)
	}

	verifier, err := verify.NewVerifier(
		trustedMaterial,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("cosign: create verifier: %w", err)
	}

	b, err := bundle.LoadJSONFromPath(bundlePath)
	if err != nil {
		return fmt.Errorf("cosign: load bundle from %q: %w", bundlePath, err)
	}

	certID, err := verify.NewShortCertificateIdentity(
		oidcIssuer,
		"",
		"",
		certIdentityRegex,
	)
	if err != nil {
		return fmt.Errorf("cosign: build certificate identity: %w", err)
	}

	f, err := os.Open(blobPath)
	if err != nil {
		return fmt.Errorf("cosign: open blob %q: %w", blobPath, err)
	}
	defer f.Close() //nolint:errcheck

	policy := verify.NewPolicy(
		verify.WithArtifact(f),
		verify.WithCertificateIdentity(certID),
	)
	if _, err := verifier.Verify(b, policy); err != nil {
		return fmt.Errorf("cosign: bundle verification failed: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}
