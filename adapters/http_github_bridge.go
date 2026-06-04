package adapters

import (
	"context"
	"io"

	updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
	updateports "github.com/fede-iglesias/shipkit/lifecycle/update/ports"
	"github.com/fede-iglesias/shipkit/ports"
)

// HTTPBridgeAdapter wraps [GitHubHTTPAdapter] (which implements
// [lifecycle/update/ports.HTTPPort]) and presents it as a
// [shipkit/ports.HTTPPort]. The bridge converts between the structurally
// identical but nominally distinct Release/Asset types used by the two ports
// packages.
//
// Use [NewHTTPBridge] to obtain a production instance backed by the real
// GitHub API.
type HTTPBridgeAdapter struct {
	inner *updateadapters.GitHubHTTPAdapter
}

// NewHTTPBridge returns an HTTPBridgeAdapter backed by a production
// GitHubHTTPAdapter. It satisfies [shipkit/ports.HTTPPort] and is the
// standard adapter for the doctor, install, and uninstall verbs.
func NewHTTPBridge() *HTTPBridgeAdapter {
	return &HTTPBridgeAdapter{inner: updateadapters.NewGitHubHTTP()}
}

// LatestRelease queries the GitHub Releases API and converts the result to
// [ports.Release]. Delegates to the underlying GitHubHTTPAdapter.
func (a *HTTPBridgeAdapter) LatestRelease(ctx context.Context, repo, tagPrefix string) (ports.Release, error) {
	rel, err := a.inner.LatestRelease(ctx, repo, tagPrefix)
	if err != nil {
		return ports.Release{}, err
	}
	return convertRelease(rel), nil
}

// DownloadAsset streams the asset to w. Delegates to the underlying adapter.
func (a *HTTPBridgeAdapter) DownloadAsset(ctx context.Context, url string, w io.Writer) error {
	return a.inner.DownloadAsset(ctx, url, w)
}

// convertRelease converts a [lifecycle/update/ports.Release] to a
// [shipkit/ports.Release]. The structs are structurally identical; this
// conversion is purely nominal.
func convertRelease(r updateports.Release) ports.Release {
	assets := make([]ports.Asset, len(r.Assets))
	for i, a := range r.Assets {
		assets[i] = ports.Asset{
			Name:        a.Name,
			DownloadURL: a.DownloadURL,
			Size:        a.Size,
		}
	}
	return ports.Release{
		Tag:         r.Tag,
		PublishedAt: r.PublishedAt,
		Assets:      assets,
	}
}
