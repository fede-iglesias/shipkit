package ports

import (
	"context"
	"io"
	"time"
)

// Release represents a published release from the GitHub API.
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

// HTTPPort abstracts all outbound HTTP traffic used by the update process.
// Implementations must be safe for concurrent use.
type HTTPPort interface {
	// LatestRelease queries the GitHub Releases API anonymously for the given
	// repo (e.g. "owner/repo"), filters releases whose tag starts with
	// tagPrefix (e.g. "myapp-"), and returns the most recently published one.
	// Returns an error if no matching release is found or the request fails.
	LatestRelease(ctx context.Context, repo, tagPrefix string) (Release, error)

	// GetReleaseByTag retrieves a specific release by its exact tag name
	// (e.g. "myapp-v0.4.0", including the TagPrefix). Used by the orchestrator
	// when opts.Version pins a target version so the asset list returned is
	// the pinned release's, not the latest's. This fixes B3: when SkipVerify is
	// set with a pinned target version, the previous implementation queried
	// LatestRelease and silently installed the latest asset.
	//
	// Returns an error wrapping a "release not found" sentinel when the tag
	// does not exist in the repo. Implementations should produce errors whose
	// .Error() contains the literal substring "not found" so callers can
	// surface a stable Result.Reason without parsing nested error chains.
	GetReleaseByTag(ctx context.Context, repo, tag string) (Release, error)

	// DownloadAsset streams the asset at url to w. The implementation must
	// propagate ctx cancellation and must not buffer the full body in memory.
	DownloadAsset(ctx context.Context, url string, w io.Writer) error
}
