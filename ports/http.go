package ports

import (
	"context"
	"io"
	"time"
)

// Release represents a published release fetched from the GitHub Releases API.
type Release struct {
	// Tag is the git tag name (e.g. "myapp-v0.0.12").
	Tag string

	// PublishedAt is the UTC timestamp when the release was published.
	PublishedAt time.Time

	// Assets is the list of downloadable artifacts attached to the release.
	Assets []Asset
}

// Asset represents a single downloadable file attached to a Release.
type Asset struct {
	// Name is the filename of the asset (e.g. "myapp_linux_amd64.tar.gz").
	Name string

	// DownloadURL is the HTTPS URL from which the asset can be fetched.
	DownloadURL string

	// Size is the uncompressed byte size reported by the API.
	Size int64
}

// HTTPPort abstracts all outbound HTTP traffic used by lifecycle verbs.
//
// Callers that need to check for new releases or download assets use this port
// rather than reaching directly into net/http. Implementations must be safe for
// concurrent use.
//
// The update verb uses HTTPPort for release discovery and asset download.
// The doctor verb uses HTTPPort for optional network health checks (--network).
type HTTPPort interface {
	// LatestRelease queries the GitHub Releases API anonymously for the given
	// repo (e.g. "owner/repo"), filters releases whose tag starts with
	// tagPrefix (e.g. "myapp-"), and returns the most recently published one.
	// Returns an error if no matching release is found or the request fails.
	LatestRelease(ctx context.Context, repo, tagPrefix string) (Release, error)

	// DownloadAsset streams the asset at url to w. The implementation must
	// propagate ctx cancellation and must not buffer the full body in memory.
	DownloadAsset(ctx context.Context, url string, w io.Writer) error
}

// MockHTTPPort is a test double for HTTPPort. It records calls and returns
// the values set on its exported fields. Use NewMockHTTPPort for safe defaults.
type MockHTTPPort struct {
	// LatestReleaseFunc overrides LatestRelease when non-nil.
	LatestReleaseFunc func(ctx context.Context, repo, tagPrefix string) (Release, error)
	// DownloadAssetFunc overrides DownloadAsset when non-nil.
	DownloadAssetFunc func(ctx context.Context, url string, w io.Writer) error

	// LatestReleaseCalls records each (repo, tagPrefix) pair passed to LatestRelease.
	LatestReleaseCalls [][2]string
	// DownloadAssetCalls records each url passed to DownloadAsset.
	DownloadAssetCalls []string
}

// NewMockHTTPPort returns a MockHTTPPort whose methods return safe zero-value
// defaults (empty Release, nil error) unless overridden via the Func fields.
func NewMockHTTPPort() *MockHTTPPort { return &MockHTTPPort{} }

// LatestRelease implements HTTPPort.
func (m *MockHTTPPort) LatestRelease(ctx context.Context, repo, tagPrefix string) (Release, error) {
	m.LatestReleaseCalls = append(m.LatestReleaseCalls, [2]string{repo, tagPrefix})
	if m.LatestReleaseFunc != nil {
		return m.LatestReleaseFunc(ctx, repo, tagPrefix)
	}
	return Release{}, nil
}

// DownloadAsset implements HTTPPort.
func (m *MockHTTPPort) DownloadAsset(ctx context.Context, url string, w io.Writer) error {
	m.DownloadAssetCalls = append(m.DownloadAssetCalls, url)
	if m.DownloadAssetFunc != nil {
		return m.DownloadAssetFunc(ctx, url, w)
	}
	return nil
}
