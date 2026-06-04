package ports_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// mockHTTP is a compile-time proof that HTTPPort is implementable.
type mockHTTP struct {
	latestReleaseFunc func(ctx context.Context, repo, tagPrefix string) (ports.Release, error)
	downloadAssetFunc func(ctx context.Context, url string, w io.Writer) error
}

func (m *mockHTTP) LatestRelease(ctx context.Context, repo, tagPrefix string) (ports.Release, error) {
	return m.latestReleaseFunc(ctx, repo, tagPrefix)
}

func (m *mockHTTP) DownloadAsset(ctx context.Context, url string, w io.Writer) error {
	return m.downloadAssetFunc(ctx, url, w)
}

// TestHTTPPort_InterfaceCompliance asserts at compile time that *mockHTTP
// satisfies HTTPPort. If HTTPPort changes, this line fails compilation.
var _ ports.HTTPPort = (*mockHTTP)(nil)

// TestRelease_Fields verifies that Release holds the expected fields and
// that they survive a round-trip through a local value.
func TestRelease_Fields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	r := ports.Release{
		Tag:         "myapp-v0.0.12",
		PublishedAt: now,
		Assets: []ports.Asset{
			{Name: "myapp_linux_amd64.tar.gz", DownloadURL: "https://example.com/myapp.tar.gz", Size: 12345678},
		},
	}

	if r.Tag != "myapp-v0.0.12" {
		t.Errorf("Tag: got %q, want %q", r.Tag, "myapp-v0.0.12")
	}
	if !r.PublishedAt.Equal(now) {
		t.Errorf("PublishedAt: got %v, want %v", r.PublishedAt, now)
	}
	if len(r.Assets) != 1 {
		t.Fatalf("Assets len: got %d, want 1", len(r.Assets))
	}
	if r.Assets[0].Name != "myapp_linux_amd64.tar.gz" {
		t.Errorf("Asset.Name: got %q", r.Assets[0].Name)
	}
	if r.Assets[0].DownloadURL != "https://example.com/myapp.tar.gz" {
		t.Errorf("Asset.DownloadURL: got %q", r.Assets[0].DownloadURL)
	}
	if r.Assets[0].Size != 12345678 {
		t.Errorf("Asset.Size: got %d", r.Assets[0].Size)
	}
}

// TestAsset_Fields verifies Asset struct field round-trip independently.
func TestAsset_Fields(t *testing.T) {
	t.Parallel()

	a := ports.Asset{
		Name:        "myapp_darwin_arm64.tar.gz",
		DownloadURL: "https://cdn.example.com/release/myapp.tar.gz",
		Size:        9876543,
	}

	if a.Name != "myapp_darwin_arm64.tar.gz" {
		t.Errorf("Name: got %q", a.Name)
	}
	if a.DownloadURL != "https://cdn.example.com/release/myapp.tar.gz" {
		t.Errorf("DownloadURL: got %q", a.DownloadURL)
	}
	if a.Size != 9876543 {
		t.Errorf("Size: got %d", a.Size)
	}
}

// TestRelease_EmptyAssetsHandled verifies that a Release with no assets is
// valid and that Assets returns an empty (not nil-panic) slice.
func TestRelease_EmptyAssetsHandled(t *testing.T) {
	t.Parallel()

	r := ports.Release{
		Tag:         "myapp-v0.0.1",
		PublishedAt: time.Now().UTC(),
	}

	if len(r.Assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(r.Assets))
	}
	// Iterating over a nil slice must not panic.
	for _, a := range r.Assets {
		_ = a
	}
}
