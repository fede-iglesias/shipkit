package adapters

import (
	"errors"
	"io"
	"testing"
)

// errReader always returns an error from Read.
type errReader struct{ err error }

func (e *errReader) Read(p []byte) (int, error) { return 0, e.err }

// TestDefaultRandHex_ReaderError covers the io.ReadFull error branch in defaultRandHex.
func TestDefaultRandHex_ReaderError(t *testing.T) {
	wantErr := errors.New("rand reader failed")
	old := randReader
	randReader = &errReader{err: wantErr}
	t.Cleanup(func() { randReader = old })

	_, err := defaultRandHex()
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

// TestDefaultRandHex_HappyPath covers the success path explicitly.
func TestDefaultRandHex_HappyPath(t *testing.T) {
	s, err := defaultRandHex()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != 8 { // 4 bytes = 8 hex chars
		t.Errorf("expected 8 hex chars, got %q (len=%d)", s, len(s))
	}
}

// Ensure randReader satisfies io.Reader (compile-time check).
var _ io.Reader = randReader
