package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit/store"
)

func TestAtomicWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")
	data := []byte("hello world\n")

	if err := store.AtomicWrite(path, data, 0o644); err != nil {
		t.Fatalf("AtomicWrite error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("AtomicWrite: got %q, want %q", got, data)
	}
}

func TestAtomicWriteCreatesParent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "output.md")
	data := []byte("nested\n")

	if err := store.AtomicWrite(path, data, 0o644); err != nil {
		t.Fatalf("AtomicWrite to nested path error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("AtomicWrite nested: got %q, want %q", got, data)
	}
}

func TestAtomicWriteOverwrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")

	if err := store.AtomicWrite(path, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("first AtomicWrite: %v", err)
	}
	if err := store.AtomicWrite(path, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("second AtomicWrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "second\n" {
		t.Errorf("AtomicWrite overwrite: got %q, want %q", got, "second\n")
	}
}

func TestAtomicWriteInvalidDir(t *testing.T) {
	t.Parallel()

	// Try to write to a path where a file already exists at parent.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "output.md") // blocker is a file, not dir

	if err := store.AtomicWrite(path, []byte("data"), 0o644); err == nil {
		t.Error("expected error when parent path is a file")
	}
}

func TestWalkDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create some files
	files := []string{
		filepath.Join(dir, "a.md"),
		filepath.Join(dir, "b.md"),
		filepath.Join(dir, "c.txt"),
		filepath.Join(dir, "sub", "d.md"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.WalkDir(dir, ".md")
	if err != nil {
		t.Fatalf("WalkDir error: %v", err)
	}

	// Expect 3 .md files (including sub/d.md)
	if len(got) != 3 {
		t.Errorf("WalkDir: got %d files, want 3: %v", len(got), got)
	}
}

func TestWalkDirEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	got, err := store.WalkDir(dir, ".md")
	if err != nil {
		t.Fatalf("WalkDir empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("WalkDir empty: got %d files, want 0", len(got))
	}
}

func TestWalkDirNonexistent(t *testing.T) {
	t.Parallel()

	got, err := store.WalkDir("/nonexistent-dir-kt-test", ".md")
	// Should return empty (not error) when dir doesn't exist
	if err != nil {
		t.Fatalf("WalkDir nonexistent: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("WalkDir nonexistent: expected empty, got %v", got)
	}
}

func TestEnsureParent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "file.md")

	if err := store.EnsureParent(path); err != nil {
		t.Fatalf("EnsureParent: %v", err)
	}

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat after EnsureParent: %v", err)
	}
	if !info.IsDir() {
		t.Error("EnsureParent: expected directory to be created")
	}
}

func TestMoveToArchive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Set up: knowledge/people/alice/index.md
	src := filepath.Join(dir, "knowledge", "people", "alice", "index.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("alice card\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	archiveRoot := filepath.Join(dir, "knowledge", "archive")
	newPath, err := store.MoveToArchive(src, archiveRoot)
	if err != nil {
		t.Fatalf("MoveToArchive: %v", err)
	}

	// Source should be gone.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("MoveToArchive: source file should not exist after move")
	}

	// Destination should exist with same content.
	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("ReadFile after MoveToArchive: %v", err)
	}
	if string(got) != "alice card\n" {
		t.Errorf("MoveToArchive content: got %q, want %q", got, "alice card\n")
	}
}
