package testserver

// Internal tests for error branches that cannot be triggered via the public
// HTTP interface (require injecting os.ReadDir failures, sub-directory entries,
// broken DirEntry implementations, or a failing ResponseWriter).

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// errReadDir exercises the path where s.readDir(releasesDir) fails. This
// branch is unreachable via the public HTTP API once New has validated the
// directory.
func TestHandleReleases_ReadDirError(t *testing.T) {
	dir := t.TempDir()
	fakeT := &fatalCatcher{t: t}
	srv := New(fakeT, dir)
	defer srv.Close()

	// Inject a failing readDir.
	srv.readDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("injected readDir error")
	}

	req := httptest.NewRequest(http.MethodGet, "/repos/owner/repo/releases", nil)
	w := httptest.NewRecorder()
	srv.handleReleases(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", w.Code)
	}
}

// TestHandleReleases_AssetDirReadError exercises the inner readDir(assetDir)
// failure branch (the `continue` at line ~117). Triggered by injecting a
// readDir that fails on the second call (after the top-level one succeeds).
func TestHandleReleases_AssetDirReadError(t *testing.T) {
	dir := t.TempDir()

	// Create a real tag directory so the top-level read returns an entry.
	if err := os.MkdirAll(dir+"/v0.0.1", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fakeT := &fatalCatcher{t: t}
	srv := New(fakeT, dir)
	defer srv.Close()

	calls := 0
	realReadDir := os.ReadDir
	srv.readDir = func(path string) ([]os.DirEntry, error) {
		calls++
		if calls == 1 {
			// First call (releasesDir) succeeds with real data.
			return realReadDir(path)
		}
		// Second call (assetDir) fails.
		return nil, errors.New("injected assetDir read error")
	}

	req := httptest.NewRequest(http.MethodGet, "/repos/owner/repo/releases", nil)
	w := httptest.NewRecorder()
	srv.handleReleases(w, req)

	// Must respond 200 (the failed assetDir is skipped via continue, not fatal).
	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}

// TestHandleReleases_SkipsSubdirsInAssetDir exercises the ae.IsDir() continue
// branch by having a sub-directory inside a tag directory.
func TestHandleReleases_SkipsSubdirsInAssetDir(t *testing.T) {
	dir := t.TempDir()

	// Create tag dir with a sub-directory inside it.
	tagDir := dir + "/v0.0.1"
	if err := os.MkdirAll(tagDir+"/subdir", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Also add a real asset file so the release is non-empty.
	if err := os.WriteFile(tagDir+"/app.tar.gz", []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fakeT := &fatalCatcher{t: t}
	srv := New(fakeT, dir)
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/repos/owner/repo/releases", nil)
	w := httptest.NewRecorder()
	srv.handleReleases(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}

// TestHandleReleases_BrokenDirEntry exercises the ae.Info() error branch by
// injecting a DirEntry whose Info() method returns an error.
func TestHandleReleases_BrokenDirEntry(t *testing.T) {
	dir := t.TempDir()

	// Create a real tag directory.
	if err := os.MkdirAll(dir+"/v0.0.1", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fakeT := &fatalCatcher{t: t}
	srv := New(fakeT, dir)
	defer srv.Close()

	calls := 0
	realReadDir := os.ReadDir
	srv.readDir = func(path string) ([]os.DirEntry, error) {
		calls++
		entries, err := realReadDir(path)
		if err != nil {
			return nil, err
		}
		if calls == 2 {
			// Inject a broken DirEntry for the assetDir listing.
			return []os.DirEntry{&brokenEntry{name: "broken.tar.gz"}}, nil
		}
		return entries, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/repos/owner/repo/releases", nil)
	w := httptest.NewRecorder()
	srv.handleReleases(w, req)

	// Must respond 200; the broken entry is skipped via continue.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", w.Code)
	}
}

// TestWriteJSON_EncoderError exercises the json.Encode error branch in writeJSON
// by using a ResponseWriter that fails after the header is written.
func TestWriteJSON_EncoderError(t *testing.T) {
	// writeJSON must not panic when the encoder returns an error.
	// We use an errWriter that returns an error on Write.
	w := &errWriter{header: make(http.Header)}
	writeJSON(w, []string{"x"}) // must not panic
}

// errWriter is an http.ResponseWriter whose Write method always fails.
type errWriter struct {
	header http.Header
	code   int
}

func (e *errWriter) Header() http.Header         { return e.header }
func (e *errWriter) WriteHeader(code int)         { e.code = code }
func (e *errWriter) Write(_ []byte) (int, error)  { return 0, errors.New("write failed") }

// brokenEntry is an fs.DirEntry whose Info() always returns an error.
type brokenEntry struct {
	name string
}

func (b *brokenEntry) Name() string               { return b.name }
func (b *brokenEntry) IsDir() bool                { return false }
func (b *brokenEntry) Type() fs.FileMode          { return 0 }
func (b *brokenEntry) Info() (fs.FileInfo, error) { return nil, errors.New("info failed") }

// fatalCatcher satisfies TB and captures Fatalf calls without terminating the
// outer test. Defined here because the internal test file is in the same
// package; the external test file also defines its own copy.
type fatalCatcher struct {
	t     *testing.T
	fatal bool
}

func (f *fatalCatcher) Fatalf(format string, args ...any) { f.fatal = true }
func (f *fatalCatcher) Helper()                           {}
func (f *fatalCatcher) Cleanup(fn func()) {
	if f.t != nil {
		f.t.Cleanup(fn)
	}
}

// fakeFileInfo implements fs.FileInfo for testing without real files.
type fakeFileInfo struct {
	name string
	size int64
}

func (f *fakeFileInfo) Name() string       { return f.name }
func (f *fakeFileInfo) Size() int64        { return f.size }
func (f *fakeFileInfo) Mode() fs.FileMode  { return 0o644 }
func (f *fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeFileInfo) IsDir() bool        { return false }
func (f *fakeFileInfo) Sys() any           { return nil }
