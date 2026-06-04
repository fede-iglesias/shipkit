package store

import "testing"

// TestAdrPathInvalidPrefix exercises the bad-prefix error in adrPath.
func TestAdrPathInvalidPrefix(t *testing.T) {
	t.Parallel()

	_, err := adrPath("/root", "slug", "INVALID-0042")
	if err == nil {
		t.Error("expected error for adr ID without ADR- prefix")
	}
}

// TestTaskPathInvalidFormat exercises the bad-format error in taskPath.
func TestTaskPathInvalidFormat(t *testing.T) {
	t.Parallel()

	_, err := taskPath("/root", "slug", "BAD-FORMAT")
	if err == nil {
		t.Error("expected error for malformed task ID")
	}
}

// TestKindFromPathUnknownWithinKnowledge covers the default case in
// KindFromPath when the path starts with "knowledge/" but has no match.
func TestKindFromPathUnknownWithinKnowledge(t *testing.T) {
	t.Parallel()

	_, err := KindFromPath("knowledge/unknown-section/foo.md")
	if err == nil {
		t.Error("expected error for unknown section within knowledge/")
	}
}
