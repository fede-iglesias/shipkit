package frontmatter

import (
	"errors"
	"testing"
)

// TestMarshalWith_MarshalerError covers the marshaler error path via injection.
func TestMarshalWith_MarshalerError(t *testing.T) {
	marshalErr := errors.New("marshal failed")
	_, err := marshalWith("anything", []byte("body"), func(any) ([]byte, error) {
		return nil, marshalErr
	})
	if err == nil {
		t.Fatal("expected error from injected marshaler failure")
	}
	if !errors.Is(err, marshalErr) {
		t.Errorf("want marshalErr in chain, got %v", err)
	}
}

// TestMarshalWith_NoTrailingNewline covers the buf.WriteByte('\n') branch
// via a marshaler that intentionally omits the trailing newline.
func TestMarshalWith_NoTrailingNewline(t *testing.T) {
	noNLMarshaler := func(any) ([]byte, error) {
		return []byte("key: val"), nil // no trailing newline
	}
	out, err := marshalWith("ignored", []byte("body\n"), noNLMarshaler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Output should have the closing --- on its own line.
	s := string(out)
	if s != "---\nkey: val\n---\nbody\n" {
		t.Errorf("unexpected output: %q", s)
	}
}
