package adapters

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile writes content to a temp file inside t.TempDir() and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return p
}

// TestNewSigstoreCosign_DefaultsCorrect verifies the adapter is constructed with
// empty policy fields (library convention: caller must configure CertIdentityRegex
// and OIDCIssuer for their repo) and verifyCore is non-nil.
func TestNewSigstoreCosign_DefaultsCorrect(t *testing.T) {
	a := NewSigstoreCosign()
	// shipkit/lifecycle/update is a library: policy fields are intentionally empty.
	// The caller (consumer cmd layer) must configure them for their repo.
	if a.CertIdentityRegex != "" {
		t.Errorf("CertIdentityRegex should be empty for a library adapter, got %q", a.CertIdentityRegex)
	}
	if a.OIDCIssuer != "" {
		t.Errorf("OIDCIssuer should be empty for a library adapter, got %q", a.OIDCIssuer)
	}
	// verifyCore must be set to the real implementation (non-nil) by default.
	if a.verifyCore == nil {
		t.Error("verifyCore must be non-nil after NewSigstoreCosign")
	}
}

func TestVerifyBundle_HappyPath(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "fake-blob-content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	a := NewSigstoreCosign()
	// High-level mock: always succeeds.
	a.VerifyFn = func(_ context.Context, _, _ string) error { return nil }

	if err := a.VerifyBundle(context.Background(), blob, bundlePath); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestVerifyBundle_PolicyMismatchReturnsErr(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "fake-blob-content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	wantErr := errors.New("certificate identity mismatch")
	a := NewSigstoreCosign()
	a.VerifyFn = func(_ context.Context, _, _ string) error { return wantErr }

	err := a.VerifyBundle(context.Background(), blob, bundlePath)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

func TestVerifyBundle_BlobNotFoundReturnsErr(t *testing.T) {
	bundlePath := writeTempFile(t, "release.bundle", "{}")
	a := NewSigstoreCosign()
	// VerifyFn left nil and verifyCore left as real: adapter detects missing
	// blob before reaching either.

	err := a.VerifyBundle(context.Background(), "/nonexistent/blob.tar.gz", bundlePath)
	if err == nil {
		t.Error("expected error for missing blob, got nil")
	}
}

func TestVerifyBundle_BundleNotFoundReturnsErr(t *testing.T) {
	blobPath := writeTempFile(t, "release.tar.gz", "fake-blob-content")
	a := NewSigstoreCosign()

	err := a.VerifyBundle(context.Background(), blobPath, "/nonexistent/bundle.json")
	if err == nil {
		t.Error("expected error for missing bundle, got nil")
	}
}

func TestVerifyBundle_ContextCancel(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "fake-blob-content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before call

	a := NewSigstoreCosign()
	// VerifyFn must not be called when ctx is already cancelled.
	a.VerifyFn = func(_ context.Context, _, _ string) error {
		t.Error("VerifyFn must not be called on cancelled context")
		return nil
	}

	err := a.VerifyBundle(ctx, blob, bundlePath)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestVerifyBundle_MockReceivesCorrectPaths(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	var gotBlob, gotBundle string
	a := NewSigstoreCosign()
	a.VerifyFn = func(_ context.Context, b, bun string) error {
		gotBlob = b
		gotBundle = bun
		return nil
	}

	if err := a.VerifyBundle(context.Background(), blob, bundlePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBlob != blob {
		t.Errorf("blob path: got %q, want %q", gotBlob, blob)
	}
	if gotBundle != bundlePath {
		t.Errorf("bundle path: got %q, want %q", gotBundle, bundlePath)
	}
}

// TestVerifyBundle_VerifyCorePath exercises the path where VerifyFn is nil and
// verifyCore is used. A stub verifyCore avoids real TUF network calls.
func TestVerifyBundle_VerifyCorePath(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	wantErr := errors.New("stub core verify called")
	a := NewSigstoreCosign()
	a.CertIdentityRegex = `https://github\.com/example/myapp/.*`
	a.OIDCIssuer = "https://token.actions.githubusercontent.com"
	// VerifyFn is nil; inject a stub into verifyCore to cover that branch.
	a.verifyCore = func(_ context.Context, issuer, regex, bPath, bndPath string) error {
		if bPath != blob {
			t.Errorf("verifyCore blobPath: got %q, want %q", bPath, blob)
		}
		if bndPath != bundlePath {
			t.Errorf("verifyCore bundlePath: got %q, want %q", bndPath, bundlePath)
		}
		if issuer != a.OIDCIssuer {
			t.Errorf("verifyCore issuer: got %q, want %q", issuer, a.OIDCIssuer)
		}
		if regex != a.CertIdentityRegex {
			t.Errorf("verifyCore regex: got %q, want %q", regex, a.CertIdentityRegex)
		}
		return wantErr
	}

	err := a.VerifyBundle(context.Background(), blob, bundlePath)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

// TestVerifyBundle_DefaultVerifyCoreReturnsErrNotConfigured exercises the path
// where VerifyFn is nil and verifyCore is the package-level defaultVerifyCore,
// which signals that production wiring (SetVerifyCore with sigstoreRealVerify)
// has not been done. Covers defaultVerifyCore so the adapters package can hit
// 100% statement coverage without exercising the real sigstore-go path.
func TestVerifyBundle_DefaultVerifyCoreReturnsErrNotConfigured(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	a := NewSigstoreCosign()
	err := a.VerifyBundle(context.Background(), blob, bundlePath)
	if !errors.Is(err, ErrCosignNotConfigured) {
		t.Fatalf("expected ErrCosignNotConfigured, got %v", err)
	}
}

// TestSetVerifyCore_OverridesDefault asserts that SetVerifyCore replaces the
// package-level defaultVerifyCore with the caller's implementation. Production
// startup wiring relies on this to inject sigstoreRealVerify from cmd layer.
func TestSetVerifyCore_OverridesDefault(t *testing.T) {
	blob := writeTempFile(t, "release.tar.gz", "content")
	bundlePath := writeTempFile(t, "release.bundle", "{}")

	a := NewSigstoreCosign()
	called := false
	a.SetVerifyCore(func(_ context.Context, _, _, _, _ string) error {
		called = true
		return nil
	})

	if err := a.VerifyBundle(context.Background(), blob, bundlePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("custom verifyCore not invoked")
	}
}
