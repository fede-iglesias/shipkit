package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// GitHubHTTPAdapter implements ports.HTTPPort using the GitHub REST API
// anonymously (no auth token). It is intended for public repositories.
// All fields are exported so callers can override defaults in tests.
type GitHubHTTPAdapter struct {
	// Client is the HTTP client used for all requests. Defaults to
	// http.DefaultClient when constructed via NewGitHubHTTP.
	Client *http.Client

	// BaseURL is the base URL of the GitHub API.
	// Defaults to "https://api.github.com". Override in tests with the
	// httptest.Server URL.
	BaseURL string

	// UserAgent is the value sent in the User-Agent header.
	// Defaults to "shipkit-update/1.0".
	UserAgent string
}

// NewGitHubHTTP constructs a GitHubHTTPAdapter with production defaults.
func NewGitHubHTTP() *GitHubHTTPAdapter {
	return &GitHubHTTPAdapter{
		Client:    http.DefaultClient,
		BaseURL:   "https://api.github.com",
		UserAgent: "shipkit-update/1.0",
	}
}

// ghRelease mirrors the subset of the GitHub Releases API we need.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

// ghAsset mirrors the asset sub-object in the GitHub Releases API.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// LatestRelease queries GET /repos/{repo}/releases?per_page=30 anonymously,
// filters releases whose tag_name starts with tagPrefix, sorts by
// published_at descending, and returns the most recently published one.
// Returns an error if no matching release is found or the request fails.
func (a *GitHubHTTPAdapter) LatestRelease(ctx context.Context, repo, tagPrefix string) (ports.Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=30", a.BaseURL, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ports.Release{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", a.UserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return ports.Release{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ports.Release{}, fmt.Errorf("GitHub API returned status %d for %s", resp.StatusCode, url)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return ports.Release{}, fmt.Errorf("decode response: %w", err)
	}

	// Filter by tag prefix client-side.
	var matching []ghRelease
	for _, r := range releases {
		if strings.HasPrefix(r.TagName, tagPrefix) {
			matching = append(matching, r)
		}
	}
	if len(matching) == 0 {
		return ports.Release{}, fmt.Errorf("no releases found with tag prefix %q in %s", tagPrefix, repo)
	}

	// Sort descending by published_at so the most recent is first.
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].PublishedAt.After(matching[j].PublishedAt)
	})

	latest := matching[0]

	assets := make([]ports.Asset, len(latest.Assets))
	for i, a := range latest.Assets {
		assets[i] = ports.Asset{
			Name:        a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
		}
	}

	return ports.Release{
		Tag:         latest.TagName,
		PublishedAt: latest.PublishedAt,
		Assets:      assets,
	}, nil
}

// DownloadAsset streams the asset at url to w using chunked I/O so the full
// body is never buffered in memory. Context cancellation is propagated through
// the request lifecycle.
func (a *GitHubHTTPAdapter) DownloadAsset(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", a.UserAgent)

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d for %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("stream body: %w", err)
	}
	return nil
}
