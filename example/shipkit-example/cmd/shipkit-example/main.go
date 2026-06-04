// Package main is the shipkit reference consumer CLI. It demonstrates the
// minimal wiring required to integrate all five shipkit lifecycle verbs
// (install, update, uninstall, doctor, clean) using the production adapters
// from [github.com/fede-iglesias/shipkit/adapters].
//
// See [github.com/fede-iglesias/shipkit] for the full API reference.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fede-iglesias/shipkit"
	"github.com/fede-iglesias/shipkit/adapters"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	sigroot "github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/spf13/cobra"
)

// Version is the current binary version. Injected at build time via
// -ldflags "-X main.Version=v0.1.0". Defaults to "dev" when built without
// ldflags.
var Version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "shipkit-example",
		Short: "shipkit reference consumer CLI",
		Long: `shipkit-example is a reference consumer that demonstrates how to wire
shipkit's five lifecycle verbs (install, update, uninstall, doctor, clean)
using the production adapters from github.com/fede-iglesias/shipkit/adapters.

Use it as a starting template when creating a new personal CLI.`,
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "shipkit-example: resolve binary path: %v\n", err)
		os.Exit(1)
	}

	cfg := shipkit.Config{
		AppName:    "shipkit-example",
		BinaryName: "shipkit-example",
		Repo:       "fede-iglesias/shipkit",
		TagPrefix:  "example-",
		Version:    Version,
		BinaryPath: self,
	}
	cfg = cfg.WithDefaults()

	cos := adapters.NewSigstoreCosign()
	cos.CertIdentityRegex = `https://github\.com/fede-iglesias/shipkit/.*`
	cos.OIDCIssuer = "https://token.actions.githubusercontent.com"
	cos.SetVerifyCore(sigstoreRealVerify)

	if err := shipkit.RegisterLifecycle(rootCmd, cfg,
		shipkit.WithCosignPort(cos),
	); err != nil {
		fmt.Fprintf(os.Stderr, "shipkit-example: register lifecycle: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// sigstoreRealVerify performs a real sigstore-go bundle verification using the
// Sigstore public-good TUF trusted root. Lives in cmd/shipkit-example (not in
// adapters) because it makes network calls (TUF + Rekor) and cannot be unit-
// tested without a live Sigstore infrastructure. The adapter is wired with
// this function at startup via SigstoreCosignAdapter.SetVerifyCore.
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
