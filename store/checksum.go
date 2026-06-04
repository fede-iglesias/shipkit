package store

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

// BodyChecksum returns the SHA-256 hex digest of body after normalizing trailing
// whitespace. Normalization trims all trailing spaces, tabs, and newlines, then
// appends a single "\n". Two bodies that differ only in trailing whitespace
// produce the same checksum.
//
// Typical use: compute the checksum before writing a card to detect whether the
// content has changed and avoid spurious writes.
//
// Returns:
//   - A 64-character lowercase hex string encoding the SHA-256 digest.
func BodyChecksum(body []byte) string {
	normalized := append(bytes.TrimRight(body, " \t\n\r"), '\n')
	sum := sha256.Sum256(normalized)
	return fmt.Sprintf("%x", sum)
}
