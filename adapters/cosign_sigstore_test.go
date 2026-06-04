package adapters

import (
	"errors"
	"testing"
)

// TestNewSigstoreCosign verifies constructor returns a non-nil adapter.
func TestNewSigstoreCosign(t *testing.T) {
	a := NewSigstoreCosign()
	if a == nil {
		t.Fatal("NewSigstoreCosign returned nil")
	}
}

// TestErrCosignNotConfigured verifies the error sentinel is exported correctly.
func TestErrCosignNotConfigured(t *testing.T) {
	if ErrCosignNotConfigured == nil {
		t.Fatal("ErrCosignNotConfigured is nil")
	}
	if !errors.Is(ErrCosignNotConfigured, ErrCosignNotConfigured) {
		t.Fatal("ErrCosignNotConfigured is not its own sentinel")
	}
}
