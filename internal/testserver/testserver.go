package testserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TB is the subset of [testing.TB] used by [New]. It is satisfied by
// [*testing.T] and by test doubles used in meta-tests of this package.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// Server is a running HTTP test server that serves a GitHub Releases API
// and asset download endpoints backed by a local directory tree.
//
// Call [New] to start a server and [Server.Close] when finished.
type Server struct {
	srv         *httptest.Server
	releasesDir string

	// readDir is the function used to list directory entries. Defaults to
	// os.ReadDir. Injectable for error-path testing.
	readDir func(string) ([]os.DirEntry, error)
}

// New creates and starts a new test server backed by releasesDir. The
// server is registered for automatic cleanup via t.Cleanup, but callers
// may also call [Server.Close] explicitly.
//
// releasesDir must follow the layout documented in the package doc:
//
//	<releases-dir>/<tag>/<asset-filename>
//
// New calls t.Fatalf if releasesDir does not exist.
func New(t TB, releasesDir string) *Server {
	t.Helper()

	if _, err := os.Stat(releasesDir); err != nil {
		t.Fatalf("testserver.New: releasesDir %q: %v", releasesDir, err)
		return nil
	}

	s := &Server{
		releasesDir: releasesDir,
		readDir:     os.ReadDir,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/", s.handleReleases)
	mux.HandleFunc("/assets/", s.handleAsset)
	s.srv = httptest.NewServer(mux)

	t.Cleanup(s.Close)
	return s
}

// Addr returns the base URL of the server, e.g. "http://127.0.0.1:PORT".
// Pass this as the BaseURL of a GitHubHTTPAdapter to redirect API calls to
// the test server.
func (s *Server) Addr() string {
	return s.srv.URL
}

// Close shuts down the server and blocks until all outstanding requests
// complete. It is idempotent and safe to call multiple times.
func (s *Server) Close() {
	s.srv.Close()
}

// ghRelease mirrors the subset of the GitHub Releases API response shape that
// lifecycle/update/adapters.GitHubHTTPAdapter deserializes.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

// ghAsset mirrors the asset sub-object in the GitHub Releases API response.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// handleReleases serves GET /repos/{owner}/{repo}/releases.
// The route captures any path under /repos/ so owner/repo are ignored - the
// server always returns all releases from releasesDir.
func (s *Server) handleReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, err := s.readDir(s.releasesDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("read releases dir: %v", err), http.StatusInternalServerError)
		return
	}

	var releases []ghRelease
	for i, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tag := entry.Name()
		assetDir := filepath.Join(s.releasesDir, tag)

		assetEntries, err := s.readDir(assetDir)
		if err != nil {
			continue
		}

		var assets []ghAsset
		for _, ae := range assetEntries {
			if ae.IsDir() {
				continue
			}
			info, err := ae.Info()
			if err != nil {
				continue
			}
			assetURL := fmt.Sprintf("%s/assets/%s/%s", s.srv.URL, tag, ae.Name())
			assets = append(assets, ghAsset{
				Name:               ae.Name(),
				BrowserDownloadURL: assetURL,
				Size:               info.Size(),
			})
		}

		// Use a stable deterministic time based on index. Entries from
		// os.ReadDir are sorted by name alphabetically, so higher-indexed
		// directories are assigned later timestamps (= newer).
		releases = append(releases, ghRelease{
			TagName:     tag,
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * 24 * time.Hour),
			Assets:      assets,
		})
	}

	// Sort descending by PublishedAt - newest first (matches real GitHub API
	// behavior after client-side sort in the adapter).
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].PublishedAt.After(releases[j].PublishedAt)
	})

	writeJSON(w, releases)
}

// writeJSON encodes v as JSON to w. Errors after headers are sent are silently
// dropped because there is no way to report them to the client at that point.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Header already sent. Nothing recoverable to do.
		_ = err
	}
}

// handleAsset serves GET /assets/{tag}/{filename}.
// It streams the raw bytes of the requested asset file from disk.
// Go's HTTP server normalizes URL paths before routing, so directory-traversal
// sequences (..) are resolved before this handler runs.
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path: /assets/<tag>/<filename>
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/assets/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "invalid asset path", http.StatusBadRequest)
		return
	}
	tag, filename := parts[0], parts[1]

	assetPath := filepath.Join(s.releasesDir, tag, filename)
	http.ServeFile(w, r, assetPath)
}
