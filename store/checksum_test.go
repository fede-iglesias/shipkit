package store_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/fede-iglesias/shipkit/store"
)

func TestBodyChecksum(t *testing.T) {
	t.Parallel()

	// Precompute expected values for deterministic assertions.
	checksum := func(s string) string {
		h := sha256.Sum256([]byte(s))
		return fmt.Sprintf("%x", h)
	}

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			// Empty body: trim("") + "\n" = "\n"
			name:  "empty body",
			input: []byte{},
			want:  checksum("\n"),
		},
		{
			// Trailing whitespace normalized: trim("hello   ") + "\n" = "hello\n"
			name:  "trailing whitespace normalized",
			input: []byte("hello   "),
			want:  checksum("hello\n"),
		},
		{
			// Multiple trailing newlines normalized.
			name:  "trailing newlines normalized",
			input: []byte("hello\n\n\n"),
			want:  checksum("hello\n"),
		},
		{
			// Identical content produces identical checksum.
			name:  "identical content same hash",
			input: []byte("same content\n"),
			want:  checksum("same content\n"),
		},
		{
			// Normal body with content.
			name:  "normal body",
			input: []byte("# Title\n\nSome content.\n"),
			want:  checksum("# Title\n\nSome content.\n"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := store.BodyChecksum(tc.input)
			if got != tc.want {
				t.Errorf("BodyChecksum(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestBodyChecksumDifferentContent(t *testing.T) {
	t.Parallel()
	a := store.BodyChecksum([]byte("content A"))
	b := store.BodyChecksum([]byte("content B"))
	if a == b {
		t.Errorf("expected different checksums for different content, got same: %q", a)
	}
}

func TestBodyChecksumWhitespaceEquivalence(t *testing.T) {
	t.Parallel()
	// Both should normalize to "hello\n" and produce identical checksums.
	c1 := store.BodyChecksum([]byte("hello"))
	c2 := store.BodyChecksum([]byte("hello   "))
	c3 := store.BodyChecksum([]byte("hello\n"))
	c4 := store.BodyChecksum([]byte("hello\n\n"))

	if c1 != c2 {
		t.Errorf("checksum(%q) = %q != checksum(%q) = %q", "hello", c1, "hello   ", c2)
	}
	if c1 != c3 {
		t.Errorf("checksum(%q) = %q != checksum(%q) = %q", "hello", c1, "hello\n", c3)
	}
	if c1 != c4 {
		t.Errorf("checksum(%q) = %q != checksum(%q) = %q", "hello", c1, "hello\n\n", c4)
	}
}
