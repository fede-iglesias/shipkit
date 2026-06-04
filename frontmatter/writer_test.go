package frontmatter_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fede-iglesias/shipkit/frontmatter"
)

func TestWriteFile(t *testing.T) {
	t.Run("creates-file-with-correct-content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.md")

		meta := roundTripFixture{
			Type:  "contact",
			Slug:  "alice-smith",
			Count: 1,
		}
		body := []byte("The body of the document.\n")

		if err := frontmatter.WriteFile(path, meta, body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read back and verify.
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		s := string(content)
		if !strings.Contains(s, "type: contact") {
			t.Errorf("output should contain 'type: contact', got %q", s)
		}
		if !strings.Contains(s, "slug: alice-smith") {
			t.Errorf("output should contain 'slug: alice-smith', got %q", s)
		}
		if !strings.Contains(s, "The body of the document.") {
			t.Errorf("output should contain body text, got %q", s)
		}
		// Must have YAML delimiters.
		if !strings.HasPrefix(s, "---\n") {
			t.Errorf("output should start with ---\\n, got %q", s[:min(len(s), 10)])
		}
	})

	t.Run("atomic-write-replaces-existing-file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "existing.md")

		// Write initial content.
		initial := []byte("---\ntype: old\n---\nold body\n")
		if err := os.WriteFile(path, initial, 0o644); err != nil {
			t.Fatal(err)
		}

		meta := roundTripFixture{Type: "new", Slug: "new-slug"}
		body := []byte("new body\n")

		if err := frontmatter.WriteFile(path, meta, body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := string(content)
		if !strings.Contains(s, "type: new") {
			t.Errorf("file should have new content, got %q", s)
		}
		if strings.Contains(s, "type: old") {
			t.Errorf("file should not contain old content, got %q", s)
		}
	})

	t.Run("error-when-rename-fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.md")
		meta := roundTripFixture{Type: "contact"}
		body := []byte("body\n")

		// Inject a rename function that always fails.
		renameErr := errors.New("rename failed")
		failingRename := func(src, dst string) error {
			return renameErr
		}

		err := frontmatter.WriteFileWithRename(path, meta, body, failingRename)
		if err == nil {
			t.Fatal("expected error from failing rename, got nil")
		}
		if !errors.Is(err, renameErr) {
			t.Errorf("want renameErr, got %v", err)
		}
	})

	t.Run("error-when-target-dir-not-exists", func(t *testing.T) {
		path := filepath.Join("/nonexistent/dir", "out.md")
		meta := roundTripFixture{Type: "contact"}
		body := []byte("body\n")
		err := frontmatter.WriteFile(path, meta, body)
		if err == nil {
			t.Error("expected error for non-existent target dir")
		}
	})
}
