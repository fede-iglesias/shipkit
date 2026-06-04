package adapters

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
	updateports "github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// TestNewHTTPBridge verifies the constructor returns a non-nil adapter.
func TestNewHTTPBridge(t *testing.T) {
	a := NewHTTPBridge()
	if a == nil {
		t.Fatal("NewHTTPBridge returned nil")
	}
	if a.inner == nil {
		t.Fatal("inner adapter is nil")
	}
}

// TestNewHTTPBridgeWithBaseURL verifies that the constructor sets the BaseURL
// on the inner adapter. Used by the cancha end-to-end workflow to redirect
// API calls to a local testserver.
func TestNewHTTPBridgeWithBaseURL(t *testing.T) {
	const customURL = "http://127.0.0.1:18080"
	a := NewHTTPBridgeWithBaseURL(customURL)
	if a == nil {
		t.Fatal("NewHTTPBridgeWithBaseURL returned nil")
	}
	if a.inner == nil {
		t.Fatal("inner adapter is nil")
	}
	if a.inner.BaseURL != customURL {
		t.Errorf("inner.BaseURL = %q; want %q", a.inner.BaseURL, customURL)
	}
}

// TestHTTPBridgeAdapter_LatestRelease_Success verifies that a successful
// upstream response is converted to a shipkit/ports.Release.
func TestHTTPBridgeAdapter_LatestRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GitHubHTTPAdapter calls /repos/{owner}/{repo}/releases?per_page=30
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"tag_name":"myapp-v0.1.0","published_at":"2024-01-02T15:04:05Z","assets":[{"name":"myapp_linux_amd64.tar.gz","browser_download_url":"https://example.com/a.tar.gz","size":1024}]}]`))
	}))
	defer srv.Close()

	inner := &updateadapters.GitHubHTTPAdapter{
		Client:    srv.Client(),
		BaseURL:   srv.URL,
		UserAgent: "test/1.0",
	}
	a := &HTTPBridgeAdapter{inner: inner}

	rel, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-")
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if rel.Tag != "myapp-v0.1.0" {
		t.Errorf("Tag = %q; want %q", rel.Tag, "myapp-v0.1.0")
	}
	if len(rel.Assets) != 1 {
		t.Fatalf("len(Assets) = %d; want 1", len(rel.Assets))
	}
	if rel.Assets[0].Name != "myapp_linux_amd64.tar.gz" {
		t.Errorf("Asset[0].Name = %q", rel.Assets[0].Name)
	}
	if rel.Assets[0].Size != 1024 {
		t.Errorf("Asset[0].Size = %d; want 1024", rel.Assets[0].Size)
	}
	want := time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)
	if !rel.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v; want %v", rel.PublishedAt, want)
	}
}

// TestHTTPBridgeAdapter_LatestRelease_Error verifies error propagation.
func TestHTTPBridgeAdapter_LatestRelease_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	inner := &updateadapters.GitHubHTTPAdapter{
		Client:    srv.Client(),
		BaseURL:   srv.URL,
		UserAgent: "test/1.0",
	}
	a := &HTTPBridgeAdapter{inner: inner}
	_, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-")
	if err == nil {
		t.Fatal("want error from 404 response; got nil")
	}
}

// TestHTTPBridgeAdapter_DownloadAsset_Success verifies that DownloadAsset
// delegates to the inner adapter and streams content.
func TestHTTPBridgeAdapter_DownloadAsset_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("binary-data"))
	}))
	defer srv.Close()

	inner := &updateadapters.GitHubHTTPAdapter{
		Client:    srv.Client(),
		BaseURL:   srv.URL,
		UserAgent: "test/1.0",
	}
	a := &HTTPBridgeAdapter{inner: inner}

	var buf bytes.Buffer
	if err := a.DownloadAsset(context.Background(), srv.URL+"/asset", &buf); err != nil {
		t.Fatalf("DownloadAsset: %v", err)
	}
	if buf.String() != "binary-data" {
		t.Errorf("body = %q; want %q", buf.String(), "binary-data")
	}
}

// TestConvertRelease exercises the convertRelease helper with a multi-asset
// release to verify all fields are transferred correctly.
func TestConvertRelease(t *testing.T) {
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	ur := updateports.Release{
		Tag:         "app-v1.2.3",
		PublishedAt: ts,
		Assets: []updateports.Asset{
			{Name: "a.tar.gz", DownloadURL: "https://dl/a.tar.gz", Size: 999},
			{Name: "b.tar.gz", DownloadURL: "https://dl/b.tar.gz", Size: 888},
		},
	}
	got := convertRelease(ur)
	if got.Tag != ur.Tag {
		t.Errorf("Tag = %q; want %q", got.Tag, ur.Tag)
	}
	if !got.PublishedAt.Equal(ts) {
		t.Errorf("PublishedAt = %v; want %v", got.PublishedAt, ts)
	}
	if len(got.Assets) != 2 {
		t.Fatalf("len(Assets) = %d; want 2", len(got.Assets))
	}
	for i, a := range got.Assets {
		if a.Name != ur.Assets[i].Name {
			t.Errorf("Assets[%d].Name = %q; want %q", i, a.Name, ur.Assets[i].Name)
		}
		if a.DownloadURL != ur.Assets[i].DownloadURL {
			t.Errorf("Assets[%d].DownloadURL = %q", i, a.DownloadURL)
		}
		if a.Size != ur.Assets[i].Size {
			t.Errorf("Assets[%d].Size = %d; want %d", i, a.Size, ur.Assets[i].Size)
		}
	}
}
