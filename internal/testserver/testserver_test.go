package testserver_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/internal/testserver"
)

// makeFixture writes a minimal valid tar.gz at dir/<tag>/<name> containing a
// single file "dummy" with content version. It returns the asset path.
func makeFixture(t *testing.T, releasesDir, tag, name, version string) string {
	t.Helper()
	assetDir := filepath.Join(releasesDir, tag)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", assetDir, err)
	}
	assetPath := filepath.Join(assetDir, name)
	f, err := os.Create(assetPath)
	if err != nil {
		t.Fatalf("create fixture %s: %v", assetPath, err)
	}
	defer f.Close() //nolint:errcheck

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	content := []byte(version)
	hdr := &tar.Header{
		Name:    "dummy",
		Mode:    0o755,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return assetPath
}

// TestNew_MissingDir verifies that New calls t.Fatalf when releasesDir does not exist.
func TestNew_MissingDir(t *testing.T) {
	fakeT := &fatalCatcher{t: t}
	testserver.New(fakeT, "/nonexistent/path/that/does/not/exist")
	if !fakeT.fatal {
		t.Error("expected t.Fatalf to be called for missing releasesDir")
	}
}

// TestReleases_Empty verifies that an empty releases dir returns an empty JSON array.
func TestReleases_Empty(t *testing.T) {
	dir := t.TempDir()
	srv := testserver.New(t, dir)

	resp := mustGet(t, srv.Addr()+"/repos/owner/repo/releases")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}

	var releases []any
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("got %d releases; want 0", len(releases))
	}
}

// TestReleases_MultipleTags verifies that releases are returned newest-first
// and contain correct asset metadata.
func TestReleases_MultipleTags(t *testing.T) {
	dir := t.TempDir()
	// Create two releases. v0.0.1 sorts before v0.0.2 alphabetically so
	// v0.0.2 should be "newer" (higher index).
	makeFixture(t, dir, "v0.0.1", "app_0.0.1_linux_amd64.tar.gz", "v0.0.1")
	makeFixture(t, dir, "v0.0.2", "app_0.0.2_linux_amd64.tar.gz", "v0.0.2")

	srv := testserver.New(t, dir)

	resp := mustGet(t, srv.Addr()+"/repos/owner/repo/releases")
	defer resp.Body.Close() //nolint:errcheck

	type ghRelease struct {
		TagName     string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(releases) != 2 {
		t.Fatalf("got %d releases; want 2", len(releases))
	}

	// Newest first.
	if releases[0].TagName != "v0.0.2" {
		t.Errorf("releases[0].tag_name = %q; want v0.0.2", releases[0].TagName)
	}
	if releases[1].TagName != "v0.0.1" {
		t.Errorf("releases[1].tag_name = %q; want v0.0.1", releases[1].TagName)
	}

	// Asset URL is a full URL pointing at our server.
	asset := releases[0].Assets[0]
	if !strings.HasPrefix(asset.BrowserDownloadURL, srv.Addr()) {
		t.Errorf("BrowserDownloadURL %q does not start with server addr %q", asset.BrowserDownloadURL, srv.Addr())
	}
	if asset.Size == 0 {
		t.Error("asset.size = 0; want > 0")
	}
}

// TestReleases_IgnoresFiles verifies that non-directory entries in releasesDir
// are not treated as releases.
func TestReleases_IgnoresFiles(t *testing.T) {
	dir := t.TempDir()
	// Write a plain file at top level - should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	makeFixture(t, dir, "v0.0.1", "app_0.0.1_linux_amd64.tar.gz", "v0.0.1")

	srv := testserver.New(t, dir)

	resp := mustGet(t, srv.Addr()+"/repos/owner/repo/releases")
	defer resp.Body.Close() //nolint:errcheck

	var releases []any
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(releases) != 1 {
		t.Errorf("got %d releases; want 1 (README should be skipped)", len(releases))
	}
}

// TestAsset_Download verifies that an asset can be downloaded and contains
// the expected bytes.
func TestAsset_Download(t *testing.T) {
	dir := t.TempDir()
	makeFixture(t, dir, "v0.0.1", "app_0.0.1_linux_amd64.tar.gz", "v0.0.1-content")

	srv := testserver.New(t, dir)

	// Discover the download URL from the releases response.
	resp := mustGet(t, srv.Addr()+"/repos/owner/repo/releases")
	defer resp.Body.Close() //nolint:errcheck

	type ghRelease struct {
		Assets []struct {
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		t.Fatalf("decode releases: %v", err)
	}
	if len(releases) == 0 || len(releases[0].Assets) == 0 {
		t.Fatal("no releases or assets returned")
	}

	dlURL := releases[0].Assets[0].BrowserDownloadURL
	dlResp := mustGet(t, dlURL)
	defer dlResp.Body.Close() //nolint:errcheck

	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d; want 200", dlResp.StatusCode)
	}

	body, err := io.ReadAll(dlResp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) == 0 {
		t.Error("downloaded asset is empty")
	}

	// Verify we can decompress it (validates the tar.gz fixture).
	gr, err := gzip.NewReader(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close() //nolint:errcheck

	tr := tar.NewReader(gr)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	if hdr.Name != "dummy" {
		t.Errorf("tar entry name = %q; want dummy", hdr.Name)
	}
}

// TestAsset_NotFound verifies that requesting a non-existent asset returns 404.
func TestAsset_NotFound(t *testing.T) {
	dir := t.TempDir()
	srv := testserver.New(t, dir)

	resp := mustGet(t, srv.Addr()+"/assets/v0.0.1/nonexistent.tar.gz")
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d; want 404", resp.StatusCode)
	}
}

// TestAsset_PathTraversal verifies that path traversal attempts are rejected.
func TestAsset_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	srv := testserver.New(t, dir)

	for _, path := range []string{
		"/assets/../etc/passwd",
		"/assets/v0.0.1/../../../etc/passwd",
	} {
		resp := mustGet(t, srv.Addr()+path)
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode == http.StatusOK {
			t.Errorf("path %q returned 200; want non-200 (path traversal not blocked)", path)
		}
	}
}

// TestMethods_NotAllowed verifies that non-GET methods are rejected.
func TestMethods_NotAllowed(t *testing.T) {
	dir := t.TempDir()
	srv := testserver.New(t, dir)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/repos/owner/repo/releases"},
		{http.MethodPost, "/assets/v0.0.1/file.tar.gz"},
	} {
		req, err := http.NewRequest(tc.method, srv.Addr()+tc.path, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status = %d; want 405", tc.method, tc.path, resp.StatusCode)
		}
	}
}

// TestAsset_InvalidPath verifies that malformed /assets/ paths return 400.
func TestAsset_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	srv := testserver.New(t, dir)

	for _, path := range []string{
		"/assets/",
		"/assets/onlytag",
	} {
		resp := mustGet(t, srv.Addr()+path)
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d; want 400", path, resp.StatusCode)
		}
	}
}

// TestClose_Idempotent verifies that calling Close multiple times does not panic.
func TestClose_Idempotent(t *testing.T) {
	dir := t.TempDir()
	// Create the server without t.Cleanup auto-close so we can call Close manually.
	fakeT := &fatalCatcher{t: t}
	srv := testserver.New(fakeT, dir)
	srv.Close()
	srv.Close() // second close - must not panic
}

// mustGet performs a GET request and fails the test on error.
func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// fatalCatcher implements [testserver.TB] and captures Fatalf calls without
// terminating the real test.
type fatalCatcher struct {
	t     *testing.T // may be nil in some meta-tests
	fatal bool
}

func (f *fatalCatcher) Fatalf(format string, args ...any) {
	f.fatal = true
}

func (f *fatalCatcher) Helper() {}

func (f *fatalCatcher) Cleanup(fn func()) {
	if f.t != nil {
		f.t.Cleanup(fn)
	}
}
