package adapters_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// --- helpers ---

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func serveJSON(t *testing.T, releases []ghRelease) *httptest.Server {
	t.Helper()
	body := mustMarshal(t, releases)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAdapter(t *testing.T, baseURL string) *adapters.GitHubHTTPAdapter {
	t.Helper()
	a := adapters.NewGitHubHTTP()
	a.BaseURL = baseURL
	return a
}

// --- TestNewGitHubHTTP_DefaultsCorrect ---

func TestNewGitHubHTTP_DefaultsCorrect(t *testing.T) {
	a := adapters.NewGitHubHTTP()
	if a.Client == nil {
		t.Fatal("Client must not be nil")
	}
	if a.BaseURL != "https://api.github.com" {
		t.Fatalf("BaseURL = %q, want %q", a.BaseURL, "https://api.github.com")
	}
	if a.UserAgent != "shipkit-update/1.0" {
		t.Fatalf("UserAgent = %q, want %q", a.UserAgent, "shipkit-update/1.0")
	}
}

// --- LatestRelease tests ---

func TestLatestRelease_HappyPath(t *testing.T) {
	releases := []ghRelease{
		{
			TagName:     "myapp-v0.0.9",
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Assets: []ghAsset{
				{Name: "myapp_0.0.9_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/myapp-0.0.9.tar.gz", Size: 1000},
			},
		},
		{
			TagName:     "myapp-v0.0.11",
			PublishedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Assets: []ghAsset{
				{Name: "myapp_0.0.11_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/myapp-0.0.11.tar.gz", Size: 2000},
			},
		},
		{
			TagName:     "other-v1.0.0",
			PublishedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			Assets:      []ghAsset{},
		},
	}

	srv := serveJSON(t, releases)
	a := newAdapter(t, srv.URL)

	rel, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.Tag != "myapp-v0.0.11" {
		t.Fatalf("Tag = %q, want %q", rel.Tag, "myapp-v0.0.11")
	}
	if len(rel.Assets) != 1 {
		t.Fatalf("Assets len = %d, want 1", len(rel.Assets))
	}
	if rel.Assets[0].Name != "myapp_0.0.11_darwin_arm64.tar.gz" {
		t.Fatalf("Asset.Name = %q", rel.Assets[0].Name)
	}
	if rel.Assets[0].Size != 2000 {
		t.Fatalf("Asset.Size = %d, want 2000", rel.Assets[0].Size)
	}
	if rel.Assets[0].DownloadURL != "https://example.com/myapp-0.0.11.tar.gz" {
		t.Fatalf("Asset.DownloadURL = %q", rel.Assets[0].DownloadURL)
	}
}

func TestLatestRelease_NoMatchingPrefixReturnsErr(t *testing.T) {
	releases := []ghRelease{
		{
			TagName:     "other-v1.0.0",
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	srv := serveJSON(t, releases)
	a := newAdapter(t, srv.URL)

	_, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLatestRelease_EmptyListReturnsErr(t *testing.T) {
	srv := serveJSON(t, []ghRelease{})
	a := newAdapter(t, srv.URL)

	_, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err == nil {
		t.Fatal("expected error for empty list, got nil")
	}
}

func TestLatestRelease_HTTPErrorReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	_, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestLatestRelease_MalformedJSONReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `not-json`)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	_, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLatestRelease_ContextCancelled(t *testing.T) {
	// Server that blocks until context is cancelled.
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		// block until client disconnects
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	a := newAdapter(t, srv.URL)

	done := make(chan error, 1)
	go func() {
		_, err := a.LatestRelease(ctx, "owner/repo", "myapp-v")
		done <- err
	}()

	<-started
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestLatestRelease_SortedByPublishedAtDesc(t *testing.T) {
	// API returns releases NOT in date order; adapter must sort and pick most recent.
	releases := []ghRelease{
		{
			TagName:     "myapp-v0.0.8",
			PublishedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Assets:      []ghAsset{{Name: "a.tar.gz", BrowserDownloadURL: "https://example.com/a", Size: 10}},
		},
		{
			TagName:     "myapp-v0.0.12",
			PublishedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
			Assets:      []ghAsset{{Name: "b.tar.gz", BrowserDownloadURL: "https://example.com/b", Size: 20}},
		},
		{
			TagName:     "myapp-v0.0.10",
			PublishedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			Assets:      []ghAsset{{Name: "c.tar.gz", BrowserDownloadURL: "https://example.com/c", Size: 15}},
		},
	}
	srv := serveJSON(t, releases)
	a := newAdapter(t, srv.URL)

	rel, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.Tag != "myapp-v0.0.12" {
		t.Fatalf("Tag = %q, want myapp-v0.0.12 (most recent by date)", rel.Tag)
	}
}

// --- GetReleaseByTag tests ---

// serveTagLookup mimics GitHub's GET /repos/{repo}/releases/tags/{tag}.
// When tag matches release.TagName, returns 200 + JSON of release.
// Otherwise returns 404.
func serveTagLookup(t *testing.T, release ghRelease) *httptest.Server {
	t.Helper()
	body := mustMarshal(t, release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expected path: /repos/{repo}/releases/tags/{tag}
		// We just check that the path ends with the expected tag.
		if !endsWithTag(r.URL.Path, release.TagName) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func endsWithTag(path, tag string) bool {
	n := len(path)
	m := len(tag)
	if n < m {
		return false
	}
	return path[n-m:] == tag
}

func TestGetReleaseByTag_HappyPath(t *testing.T) {
	release := ghRelease{
		TagName:     "myapp-v0.4.0",
		PublishedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		Assets: []ghAsset{
			{
				Name:               "myapp_0.4.0_linux_amd64.tar.gz",
				BrowserDownloadURL: "https://example.com/myapp-0.4.0.tar.gz",
				Size:               4000,
			},
		},
	}

	srv := serveTagLookup(t, release)
	a := newAdapter(t, srv.URL)

	rel, err := a.GetReleaseByTag(context.Background(), "owner/repo", "myapp-v0.4.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.Tag != "myapp-v0.4.0" {
		t.Fatalf("Tag = %q, want %q", rel.Tag, "myapp-v0.4.0")
	}
	if len(rel.Assets) != 1 {
		t.Fatalf("Assets len = %d, want 1", len(rel.Assets))
	}
	if rel.Assets[0].Name != "myapp_0.4.0_linux_amd64.tar.gz" {
		t.Fatalf("Asset.Name = %q", rel.Assets[0].Name)
	}
	if rel.Assets[0].DownloadURL != "https://example.com/myapp-0.4.0.tar.gz" {
		t.Fatalf("Asset.DownloadURL = %q", rel.Assets[0].DownloadURL)
	}
	if rel.Assets[0].Size != 4000 {
		t.Fatalf("Asset.Size = %d, want 4000", rel.Assets[0].Size)
	}
}

func TestGetReleaseByTag_NotFoundReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	_, err := a.GetReleaseByTag(context.Background(), "owner/repo", "myapp-v99.99.99")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	// Reason must include "not found" so the orchestrator can surface a stable
	// substring in Result.Reason without parsing wrapped error chains.
	if !contains(err.Error(), "not found") {
		t.Fatalf("error message %q must contain %q", err.Error(), "not found")
	}
}

func TestGetReleaseByTag_HTTPErrorReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	_, err := a.GetReleaseByTag(context.Background(), "owner/repo", "myapp-v0.4.0")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestGetReleaseByTag_MalformedJSONReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `not-json`)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	_, err := a.GetReleaseByTag(context.Background(), "owner/repo", "myapp-v0.4.0")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestGetReleaseByTag_ContextCancelled(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	a := newAdapter(t, srv.URL)

	done := make(chan error, 1)
	go func() {
		_, err := a.GetReleaseByTag(ctx, "owner/repo", "myapp-v0.4.0")
		done <- err
	}()

	<-started
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestGetReleaseByTag_InvalidBaseURLReturnsErr(t *testing.T) {
	a := adapters.NewGitHubHTTP()
	a.BaseURL = "http://\x00invalid"

	_, err := a.GetReleaseByTag(context.Background(), "owner/repo", "myapp-v0.4.0")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// contains is a tiny helper to avoid importing strings just for this test.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- DownloadAsset tests ---

func TestDownloadAsset_HappyPath(t *testing.T) {
	payload := []byte("binary-content-here")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	var buf bytes.Buffer
	err := a.DownloadAsset(context.Background(), srv.URL+"/asset", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("body = %q, want %q", buf.Bytes(), payload)
	}
}

func TestDownloadAsset_HTTPErrorReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	var buf bytes.Buffer
	err := a.DownloadAsset(context.Background(), srv.URL+"/missing", &buf)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestDownloadAsset_ContextCancelled(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	a := newAdapter(t, srv.URL)

	done := make(chan error, 1)
	go func() {
		var buf bytes.Buffer
		err := a.DownloadAsset(ctx, srv.URL+"/asset", &buf)
		done <- err
	}()

	<-started
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// --- error path coverage: invalid URL ---

// TestLatestRelease_InvalidBaseURLReturnsErr exercises the
// http.NewRequestWithContext error branch in LatestRelease.
func TestLatestRelease_InvalidBaseURLReturnsErr(t *testing.T) {
	a := adapters.NewGitHubHTTP()
	// A URL with a control character is rejected by http.NewRequest.
	a.BaseURL = "http://\x00invalid"

	_, err := a.LatestRelease(context.Background(), "owner/repo", "myapp-v")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// TestDownloadAsset_InvalidURLReturnsErr exercises the
// http.NewRequestWithContext error branch in DownloadAsset.
func TestDownloadAsset_InvalidURLReturnsErr(t *testing.T) {
	a := adapters.NewGitHubHTTP()

	var buf bytes.Buffer
	err := a.DownloadAsset(context.Background(), "http://\x00invalid", &buf)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

// TestDownloadAsset_WriterErrorReturnsErr exercises the io.Copy error branch
// in DownloadAsset by using a writer that always fails.
func TestDownloadAsset_WriterErrorReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "some-content")
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t, srv.URL)
	err := a.DownloadAsset(context.Background(), srv.URL+"/asset", &errWriter{})
	if err == nil {
		t.Fatal("expected error for writer failure, got nil")
	}
}

// errWriter always returns an error on Write.
type errWriter struct{}

func (e *errWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("write failed intentionally")
}

// --- interface compliance ---

func TestGitHubHTTPAdapter_ImplementsHTTPPort(t *testing.T) {
	var _ ports.HTTPPort = adapters.NewGitHubHTTP()
}
