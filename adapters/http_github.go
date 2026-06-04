package adapters

import updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"

// GitHubHTTPAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.HTTPPort] for public GitHub
// repositories. It queries the GitHub Releases API anonymously and streams
// asset downloads without buffering the full body in memory.
//
// This type re-exports [lifecycle/update/adapters.GitHubHTTPAdapter]. Use
// [NewGitHubHTTP] to obtain a pre-configured instance.
type GitHubHTTPAdapter = updateadapters.GitHubHTTPAdapter

// NewGitHubHTTP returns a GitHubHTTPAdapter configured with production
// defaults: http.DefaultClient, api.github.com base URL, and the
// "shipkit-update/1.0" user-agent string.
//
// Override BaseURL in tests with an httptest.Server URL. Override Client to
// inject timeouts or transport-level middleware.
func NewGitHubHTTP() *GitHubHTTPAdapter {
	return updateadapters.NewGitHubHTTP()
}
