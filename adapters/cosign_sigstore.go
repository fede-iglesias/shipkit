package adapters

import updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"

// SigstoreCosignAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.CosignPort]. It uses sigstore-go
// with TUF-backed root of trust and Rekor transparency log verification.
//
// The real sigstore-go verification is injected at startup via SetVerifyCore
// from the consumer cmd layer (e.g. cmd/myapp/update_sigstore.go). Without
// that wiring VerifyBundle returns ErrCosignNotConfigured. This design keeps
// network-calling code out of package-level init and enables 100% unit test
// coverage without a live Sigstore infrastructure.
//
// This type re-exports [lifecycle/update/adapters.SigstoreCosignAdapter].
type SigstoreCosignAdapter = updateadapters.SigstoreCosignAdapter

// ErrCosignNotConfigured is returned by SigstoreCosignAdapter.VerifyBundle
// when SetVerifyCore has not been called. Consumer startup code must call
// SetVerifyCore with a real sigstore verify function before the update verb
// is invoked with cosign verification enabled.
//
// Re-exported from lifecycle/update/adapters so consumers import a single
// package.
var ErrCosignNotConfigured = updateadapters.ErrCosignNotConfigured

// NewSigstoreCosign returns a SigstoreCosignAdapter whose verifyCore returns
// ErrCosignNotConfigured until SetVerifyCore is called with a real
// implementation. Set CertIdentityRegex and OIDCIssuer to the consumer
// application's GitHub Actions identity before use.
func NewSigstoreCosign() *SigstoreCosignAdapter {
	return updateadapters.NewSigstoreCosign()
}
