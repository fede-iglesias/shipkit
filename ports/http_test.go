package ports_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockHTTPPort satisfies HTTPPort.
var _ ports.HTTPPort = (*ports.MockHTTPPort)(nil)

func TestMockHTTPPort_LatestRelease_default(t *testing.T) {
	m := ports.NewMockHTTPPort()
	r, err := m.LatestRelease(context.Background(), "owner/repo", "app-")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if r.Tag != "" {
		t.Fatalf("expected empty tag by default, got %q", r.Tag)
	}
	if len(m.LatestReleaseCalls) != 1 {
		t.Fatalf("expected 1 call recorded, got %d", len(m.LatestReleaseCalls))
	}
	if m.LatestReleaseCalls[0][0] != "owner/repo" {
		t.Errorf("expected repo recorded, got %q", m.LatestReleaseCalls[0][0])
	}
}

func TestMockHTTPPort_LatestRelease_func(t *testing.T) {
	want := ports.Release{Tag: "app-v1.0.0"}
	m := ports.NewMockHTTPPort()
	m.LatestReleaseFunc = func(_ context.Context, _, _ string) (ports.Release, error) {
		return want, nil
	}
	got, err := m.LatestRelease(context.Background(), "x", "y")
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != want.Tag {
		t.Errorf("want tag %q, got %q", want.Tag, got.Tag)
	}
}

func TestMockHTTPPort_LatestRelease_error(t *testing.T) {
	m := ports.NewMockHTTPPort()
	sentinel := errors.New("rate limited")
	m.LatestReleaseFunc = func(_ context.Context, _, _ string) (ports.Release, error) {
		return ports.Release{}, sentinel
	}
	_, err := m.LatestRelease(context.Background(), "x", "y")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestMockHTTPPort_DownloadAsset_default(t *testing.T) {
	m := ports.NewMockHTTPPort()
	var buf bytes.Buffer
	err := m.DownloadAsset(context.Background(), "https://example.com/asset.tar.gz", &buf)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(m.DownloadAssetCalls) != 1 {
		t.Fatalf("expected 1 call recorded, got %d", len(m.DownloadAssetCalls))
	}
}

func TestMockHTTPPort_DownloadAsset_func(t *testing.T) {
	m := ports.NewMockHTTPPort()
	sentinel := errors.New("network error")
	m.DownloadAssetFunc = func(_ context.Context, _ string, _ io.Writer) error {
		return sentinel
	}
	err := m.DownloadAsset(context.Background(), "url", &bytes.Buffer{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}
