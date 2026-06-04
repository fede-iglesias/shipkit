package frontmatter_test

import (
	"testing"

	"github.com/fede-iglesias/shipkit/frontmatter"
)

func TestEnsureType(t *testing.T) {
	t.Run("injects-type-when-absent", func(t *testing.T) {
		yaml := []byte("slug: foo\ncount: 1\n")
		out, err := frontmatter.EnsureType(yaml, "contact")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Must contain type: contact.
		if !containsLine(t, out, "type: contact") {
			t.Errorf("output should contain 'type: contact', got %q", out)
		}
		// Other fields must still be present.
		if !containsLine(t, out, "slug: foo") {
			t.Errorf("output should still contain 'slug: foo', got %q", out)
		}
	})

	t.Run("preserves-existing-type", func(t *testing.T) {
		yaml := []byte("type: person\nslug: bar\n")
		out, err := frontmatter.EnsureType(yaml, "contact")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Type must remain person, not contact.
		if !containsLine(t, out, "type: person") {
			t.Errorf("existing type should be preserved, got %q", out)
		}
		// Must NOT have type: contact injected.
		if containsLine(t, out, "type: contact") {
			t.Errorf("should not have injected 'type: contact' when type already present, got %q", out)
		}
	})

	t.Run("empty-yaml-gets-type-injected", func(t *testing.T) {
		yaml := []byte("")
		out, err := frontmatter.EnsureType(yaml, "note")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsLine(t, out, "type: note") {
			t.Errorf("empty yaml should get type injected, got %q", out)
		}
	})

	t.Run("type-injected-at-start-or-present", func(t *testing.T) {
		yaml := []byte("slug: baz\nchannel_preference:\n  - email\n")
		out, err := frontmatter.EnsureType(yaml, "team")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsLine(t, out, "type: team") {
			t.Errorf("should inject type: team, got %q", out)
		}
		// Verify other fields preserved.
		if !containsLine(t, out, "slug: baz") {
			t.Errorf("slug should be preserved, got %q", out)
		}
	})

	t.Run("invalid-yaml-returns-error", func(t *testing.T) {
		yaml := []byte(": broken: [[\n")
		_, err := frontmatter.EnsureType(yaml, "contact")
		if err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}
	})
}

// containsLine checks if the output bytes contain the given string as a line substring.
func containsLine(t *testing.T, data []byte, s string) bool {
	t.Helper()
	for _, line := range splitLines(data) {
		if line == s {
			return true
		}
	}
	return false
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	s := string(data)
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
