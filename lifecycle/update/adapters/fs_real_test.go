package adapters_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/update"
	"github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
)

// buildTarGz creates an in-memory tar.gz with the given files map (name -> content).
func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

// TestNewRealFs_DefaultsWired ensures all function fields are non-nil after NewRealFs.
func TestNewRealFs_DefaultsWired(t *testing.T) {
	a := adapters.NewRealFs()
	if a.NowFn == nil {
		t.Error("NowFn must be non-nil")
	}
	if a.RandFn == nil {
		t.Error("RandFn must be non-nil")
	}
	if a.MkdirAllFn == nil {
		t.Error("MkdirAllFn must be non-nil")
	}
	if a.CreateFn == nil {
		t.Error("CreateFn must be non-nil")
	}
	if a.OpenFn == nil {
		t.Error("OpenFn must be non-nil")
	}
	if a.RenameFn == nil {
		t.Error("RenameFn must be non-nil")
	}
	if a.RemoveFn == nil {
		t.Error("RemoveFn must be non-nil")
	}
	if a.StatFn == nil {
		t.Error("StatFn must be non-nil")
	}
	if a.ReadFileFn == nil {
		t.Error("ReadFileFn must be non-nil")
	}
	if a.WriteFileFn == nil {
		t.Error("WriteFileFn must be non-nil")
	}
	if a.CopyFn == nil {
		t.Error("CopyFn must be non-nil")
	}
}

// --- Snapshot tests ---

func TestSnapshot_HappyPath(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "myapp")
	if err := os.WriteFile(srcFile, []byte("binary-content"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshotDir := filepath.Join(dir, "snapshots")

	a := adapters.NewRealFs()
	id, err := a.Snapshot(context.Background(), srcFile, snapshotDir)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	if id == "" {
		t.Fatal("snapshot ID must not be empty")
	}

	// ID is the full path to the snapshot subdirectory; the binary lives inside it.
	snapFile := filepath.Join(id, "myapp")
	got, err := os.ReadFile(snapFile)
	if err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
	if string(got) != "binary-content" {
		t.Errorf("snapshot content = %q, want %q", string(got), "binary-content")
	}
}

func TestSnapshot_MkdirError(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "myapp")
	if err := os.WriteFile(srcFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	errMkdir := errors.New("mkdir fail")
	a := adapters.NewRealFs()
	a.MkdirAllFn = func(string, os.FileMode) error { return errMkdir }

	_, err := a.Snapshot(context.Background(), srcFile, filepath.Join(dir, "snap"))
	if !errors.Is(err, errMkdir) {
		t.Errorf("expected errMkdir, got %v", err)
	}
}

func TestSnapshot_OpenSrcError(t *testing.T) {
	dir := t.TempDir()
	snapshotDir := filepath.Join(dir, "snap")

	errOpen := errors.New("open fail")
	a := adapters.NewRealFs()
	a.OpenFn = func(string) (*os.File, error) { return nil, errOpen }

	_, err := a.Snapshot(context.Background(), filepath.Join(dir, "myapp"), snapshotDir)
	if !errors.Is(err, errOpen) {
		t.Errorf("expected errOpen, got %v", err)
	}
}

func TestSnapshot_CreateDstError(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "myapp")
	if err := os.WriteFile(srcFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshotDir := filepath.Join(dir, "snap")

	errCreate := errors.New("create fail")
	a := adapters.NewRealFs()
	a.CreateFn = func(string) (*os.File, error) { return nil, errCreate }

	_, err := a.Snapshot(context.Background(), srcFile, snapshotDir)
	if !errors.Is(err, errCreate) {
		t.Errorf("expected errCreate, got %v", err)
	}
}

func TestSnapshot_CopyError(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "myapp")
	if err := os.WriteFile(srcFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshotDir := filepath.Join(dir, "snap")

	errCopy := errors.New("copy fail")
	a := adapters.NewRealFs()
	a.CopyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errCopy }

	_, err := a.Snapshot(context.Background(), srcFile, snapshotDir)
	if !errors.Is(err, errCopy) {
		t.Errorf("expected errCopy, got %v", err)
	}
}

// --- Restore tests ---

func TestRestore_HappyPath(t *testing.T) {
	dir := t.TempDir()

	// Build a snapshot: snapshotDir/myid/myapp
	// snapshotID = full path to snapshot subdir (adapter convention).
	snapshotDir := filepath.Join(dir, "snapshots")
	snapSubdir := filepath.Join(snapshotDir, "2024-myapp-abc")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapFile := filepath.Join(snapSubdir, "myapp")
	if err := os.WriteFile(snapFile, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// dst is the target binary path (basename must match "myapp")
	dst := filepath.Join(dir, "bin", "myapp")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("new-broken"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	// snapshotID is the full path of the snapshot subdir (adapter-level convention).
	if err := a.Restore(context.Background(), snapSubdir, dst); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Errorf("restored content = %q, want %q", string(got), "old-binary")
	}
}

func TestRestore_AtomicViaInjectedRename(t *testing.T) {
	dir := t.TempDir()

	snapSubdir := filepath.Join(dir, "snapshots", "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "bin"), []byte("snap"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "bin")
	if err := os.WriteFile(dst, []byte("cur"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Track rename calls to verify atomic pattern is used.
	var renameCalls []string
	a := adapters.NewRealFs()
	orig := a.RenameFn
	a.RenameFn = func(src, tgt string) error {
		renameCalls = append(renameCalls, src+"->"+tgt)
		return orig(src, tgt)
	}

	// snapshotID = full path to snapshot subdir.
	if err := a.Restore(context.Background(), snapSubdir, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(renameCalls) == 0 {
		t.Error("expected at least one Rename call for atomic replace")
	}
}

func TestRestore_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "bin", "myapp")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	// snapshotID = full path to a nonexistent snapshot subdir.
	nonexistentSnapshotSubdir := filepath.Join(dir, "snapshots", "nonexistent")
	a := adapters.NewRealFs()
	err := a.Restore(context.Background(), nonexistentSnapshotSubdir, dst)
	if err == nil {
		t.Error("expected error for missing snapshot source")
	}
}

// --- AtomicReplace tests ---

func TestAtomicReplace_HappyPath(t *testing.T) {
	dir := t.TempDir()
	newFile := filepath.Join(dir, "myapp.new")
	target := filepath.Join(dir, "myapp")

	if err := os.WriteFile(newFile, []byte("new-content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old-content"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	if err := a.AtomicReplace(context.Background(), target, newFile); err != nil {
		t.Fatalf("AtomicReplace failed: %v", err)
	}

	// target must have new content.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-content" {
		t.Errorf("target content = %q, want %q", string(got), "new-content")
	}

	// newFile must no longer exist (was renamed).
	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("newFile should not exist after rename")
	}
}

func TestAtomicReplace_RenameError(t *testing.T) {
	dir := t.TempDir()
	newFile := filepath.Join(dir, "myapp.new")
	target := filepath.Join(dir, "myapp")
	if err := os.WriteFile(newFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	errRename := errors.New("rename fail")
	a := adapters.NewRealFs()
	a.RenameFn = func(string, string) error { return errRename }

	err := a.AtomicReplace(context.Background(), target, newFile)
	if !errors.Is(err, errRename) {
		t.Errorf("expected errRename, got %v", err)
	}
}

// --- ExtractTarGz tests ---

func TestExtractTarGz_HappyPath(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "extracted")

	archiveData := buildTarGz(t, map[string]string{
		"hello.txt":       "hello world",
		"sub/goodbye.txt": "goodbye world",
	})
	archivePath := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	if err := a.ExtractTarGz(context.Background(), archivePath, destDir); err != nil {
		t.Fatalf("ExtractTarGz failed: %v", err)
	}

	// Verify files exist with correct content.
	cases := map[string]string{
		filepath.Join(destDir, "hello.txt"):          "hello world",
		filepath.Join(destDir, "sub", "goodbye.txt"): "goodbye world",
	}
	for path, want := range cases {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing file %s: %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("file %s = %q, want %q", path, string(got), want)
		}
	}
}

func TestExtractTarGz_MalformedArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bad.tar.gz")
	if err := os.WriteFile(archivePath, []byte("this is not a tar.gz"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if err == nil {
		t.Error("expected error for malformed archive")
	}
}

func TestExtractTarGz_OpenError(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "nonexistent.tar.gz")
	// Don't create the archive file.
	a := adapters.NewRealFs()
	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if err == nil {
		t.Error("expected error for missing archive file")
	}
}

// buildTruncatedTarGz creates a gzip-compressed file containing a partial/truncated tar
// (valid gzip, valid tar header, but body data is missing) to trigger tar.Reader errors.
func buildTruncatedTarGz(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// Write a header claiming 100 bytes of content.
	hdr := &tar.Header{
		Name: "bigfile.txt",
		Mode: 0o644,
		Size: 100,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar WriteHeader: %v", err)
	}
	// Write only 10 bytes - do NOT flush the tar writer properly, leaving truncated body.
	if _, err := tw.Write([]byte("truncated!")); err != nil {
		t.Fatalf("tar Write: %v", err)
	}
	// Flush gzip without closing tar - this produces a valid gzip with broken tar.
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractTarGz_TarReadError(t *testing.T) {
	dir := t.TempDir()
	archiveData := buildTruncatedTarGz(t)
	archivePath := filepath.Join(dir, "truncated.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if err == nil {
		t.Error("expected error for truncated tar body")
	}
}

// buildGzipWithBadTar creates a valid gzip containing random bytes (not a valid tar).
// This causes tar.Reader.Next() to fail rather than gzip.NewReader.
func buildGzipWithBadTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	// Write random garbage as the tar payload.
	if _, err := gz.Write([]byte("this is not valid tar data at all, just garbage bytes!!")); err != nil {
		t.Fatalf("gzip Write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractTarGz_TarNextError(t *testing.T) {
	dir := t.TempDir()
	archiveData := buildGzipWithBadTar(t)
	archivePath := filepath.Join(dir, "badtar.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	a := adapters.NewRealFs()
	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if err == nil {
		t.Error("expected error for garbled tar data")
	}
}

func TestExtractTarGz_MkdirError(t *testing.T) {
	dir := t.TempDir()

	archiveData := buildTarGz(t, map[string]string{
		"sub/file.txt": "content",
	})
	archivePath := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	errMkdir := errors.New("mkdir denied")
	a := adapters.NewRealFs()
	a.MkdirAllFn = func(string, os.FileMode) error { return errMkdir }

	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if !errors.Is(err, errMkdir) {
		t.Errorf("expected errMkdir, got %v", err)
	}
}

func TestExtractTarGz_WriteError(t *testing.T) {
	dir := t.TempDir()

	archiveData := buildTarGz(t, map[string]string{
		"file.txt": "content",
	})
	archivePath := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	errWrite := errors.New("write denied")
	a := adapters.NewRealFs()
	a.WriteFileFn = func(string, []byte, os.FileMode) error { return errWrite }

	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if !errors.Is(err, errWrite) {
		t.Errorf("expected errWrite, got %v", err)
	}
}

func TestExtractTarGz_ContextCancel(t *testing.T) {
	dir := t.TempDir()

	// Build an archive with multiple files so context cancel can fire.
	archiveData := buildTarGz(t, map[string]string{
		"a.txt": "aaaa",
		"b.txt": "bbbb",
		"c.txt": "cccc",
	})
	archivePath := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	a := adapters.NewRealFs()
	err := a.ExtractTarGz(ctx, archivePath, filepath.Join(dir, "out"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// buildTarGzWithDir creates a tar.gz that includes a directory entry before a file.
func buildTarGzWithDir(t *testing.T, dirName string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// Add directory entry.
	hdr := &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     dirName + "/",
		Mode:     0o755,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar WriteHeader dir: %v", err)
	}
	for name, content := range files {
		fhdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(fhdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractTarGz_DirectoryEntry(t *testing.T) {
	dir := t.TempDir()

	archiveData := buildTarGzWithDir(t, "mydir", map[string]string{
		"mydir/file.txt": "content",
	})
	archivePath := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(dir, "extracted")
	a := adapters.NewRealFs()
	if err := a.ExtractTarGz(context.Background(), archivePath, destDir); err != nil {
		t.Fatalf("ExtractTarGz with directory entry failed: %v", err)
	}

	// The directory must exist.
	info, err := os.Stat(filepath.Join(destDir, "mydir"))
	if err != nil {
		t.Fatalf("directory entry not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected mydir to be a directory")
	}
}

func TestExtractTarGz_DirectoryEntry_MkdirError(t *testing.T) {
	dir := t.TempDir()

	archiveData := buildTarGzWithDir(t, "mydir", map[string]string{})
	archivePath := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	errMkdir := errors.New("mkdir denied for dir entry")
	a := adapters.NewRealFs()
	a.MkdirAllFn = func(string, os.FileMode) error { return errMkdir }

	err := a.ExtractTarGz(context.Background(), archivePath, filepath.Join(dir, "out"))
	if !errors.Is(err, errMkdir) {
		t.Errorf("expected errMkdir, got %v", err)
	}
}

// --- Restore error-path tests ---

func TestRestore_MkdirError(t *testing.T) {
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "myapp"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "newdir", "myapp")

	errMkdir := errors.New("mkdir denied")
	a := adapters.NewRealFs()
	a.MkdirAllFn = func(string, os.FileMode) error { return errMkdir }

	err := a.Restore(context.Background(), snapSubdir, dst)
	if !errors.Is(err, errMkdir) {
		t.Errorf("expected errMkdir, got %v", err)
	}
}

func TestRestore_RandError(t *testing.T) {
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "myapp"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "myapp")

	errRand := errors.New("rand failed")
	a := adapters.NewRealFs()
	a.RandFn = func() (string, error) { return "", errRand }

	err := a.Restore(context.Background(), snapSubdir, dst)
	if !errors.Is(err, errRand) {
		t.Errorf("expected errRand, got %v", err)
	}
}

func TestRestore_CreateError(t *testing.T) {
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "myapp"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "myapp")

	errCreate := errors.New("create failed")
	a := adapters.NewRealFs()
	a.CreateFn = func(string) (*os.File, error) { return nil, errCreate }

	err := a.Restore(context.Background(), snapSubdir, dst)
	if !errors.Is(err, errCreate) {
		t.Errorf("expected errCreate, got %v", err)
	}
}

func TestRestore_CopyError(t *testing.T) {
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "myapp"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "myapp")

	errCopy := errors.New("copy failed")
	a := adapters.NewRealFs()
	a.CopyFn = func(io.Writer, io.Reader) (int64, error) { return 0, errCopy }

	err := a.Restore(context.Background(), snapSubdir, dst)
	if !errors.Is(err, errCopy) {
		t.Errorf("expected errCopy, got %v", err)
	}
}

func TestRestore_RenameError(t *testing.T) {
	dir := t.TempDir()
	snapSubdir := filepath.Join(dir, "snap1")
	if err := os.MkdirAll(snapSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapSubdir, "myapp"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "myapp")

	errRename := errors.New("rename failed")
	a := adapters.NewRealFs()
	a.RenameFn = func(string, string) error { return errRename }

	err := a.Restore(context.Background(), snapSubdir, dst)
	if !errors.Is(err, errRename) {
		t.Errorf("expected errRename, got %v", err)
	}
}

// --- NewRealFs RandFn error path ---

func TestSnapshot_RandError(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "myapp")
	if err := os.WriteFile(srcFile, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	errRand := errors.New("rand fail")
	a := adapters.NewRealFs()
	a.RandFn = func() (string, error) { return "", errRand }

	_, err := a.Snapshot(context.Background(), srcFile, filepath.Join(dir, "snap"))
	if !errors.Is(err, errRand) {
		t.Errorf("expected errRand, got %v", err)
	}
}

// buildTarGzWithEntry creates a tar.gz in memory with a single entry using the
// provided header (no body content).
func buildTarGzWithEntry(t *testing.T, hdr *tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar WriteHeader: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

// TestExtractTarGz_RejectsPathTraversal checks that an entry with a name that
// resolves outside destDir (e.g. "../escaped.bin") is rejected with ErrTarballEntryEscapes.
func TestExtractTarGz_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()

	archiveData := buildTarGzWithEntry(t, &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "../escaped.bin",
		Mode:     0o644,
		Size:     0,
	})
	archivePath := filepath.Join(dir, "traversal.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(dir, "extracted")
	a := adapters.NewRealFs()
	err := a.ExtractTarGz(context.Background(), archivePath, destDir)
	if !errors.Is(err, update.ErrTarballEntryEscapes) {
		t.Errorf("expected ErrTarballEntryEscapes, got %v", err)
	}
}

// TestExtractTarGz_RejectsSymlinkEntry checks that a symlink tar entry is rejected
// with ErrTarballEntryEscapes.
func TestExtractTarGz_RejectsSymlinkEntry(t *testing.T) {
	dir := t.TempDir()

	archiveData := buildTarGzWithEntry(t, &tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "legit-looking.sh",
		Linkname: "/etc/passwd",
		Mode:     0o644,
	})
	archivePath := filepath.Join(dir, "symlink.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(dir, "extracted")
	a := adapters.NewRealFs()
	err := a.ExtractTarGz(context.Background(), archivePath, destDir)
	if !errors.Is(err, update.ErrTarballEntryEscapes) {
		t.Errorf("expected ErrTarballEntryEscapes, got %v", err)
	}
}
