// Command testserver starts a standalone HTTP server that simulates the
// GitHub Releases API and asset download endpoints. Intended for use in
// cancha end-to-end CI workflows.
//
// Usage:
//
//	go run github.com/fede-iglesias/shipkit/internal/testserver/cmd \
//	  -releases /path/to/releases \
//	  -addr 127.0.0.1:18080
//
// Environment variables (override flags):
//
//	TESTSERVER_RELEASES   path to releases directory (overrides -releases)
//	TESTSERVER_ADDR       listen address host:port (overrides -addr)
//
// The server prints its base URL to stdout on startup:
//
//	TESTSERVER_ADDR=http://127.0.0.1:18080
//
// Routes:
//
//	GET /repos/{owner}/{repo}/releases   GitHub-compatible releases JSON
//	GET /assets/{tag}/{filename}         raw asset bytes
//
// Directory layout under -releases:
//
//	<releases-dir>/<tag>/<asset-filename>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func main() {
	releases := flag.String("releases", "", "path to releases directory (required)")
	addr := flag.String("addr", "127.0.0.1:18080", "listen address host:port")
	flag.Parse()

	if v := os.Getenv("TESTSERVER_RELEASES"); v != "" {
		*releases = v
	}
	if v := os.Getenv("TESTSERVER_ADDR"); v != "" {
		*addr = v
	}

	if *releases == "" {
		fmt.Fprintln(os.Stderr, "error: -releases is required (or set TESTSERVER_RELEASES)")
		os.Exit(1)
	}

	absReleases, err := filepath.Abs(*releases)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve releases path: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absReleases); err != nil {
		fmt.Fprintf(os.Stderr, "error: releases dir %q: %v\n", absReleases, err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen %s: %v\n", *addr, err)
		os.Exit(1)
	}

	baseURL := "http://" + listener.Addr().String()
	h := &handler{releasesDir: absReleases, baseURL: baseURL}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/", h.handleReleases)
	mux.HandleFunc("/assets/", h.handleAsset)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Print the base URL so CI can capture it.
	fmt.Println("TESTSERVER_ADDR=" + baseURL)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		_ = srv.Close()
	}()

	if err := srv.Serve(listener); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		fmt.Fprintf(os.Stderr, "error: serve: %v\n", err)
		os.Exit(1)
	}
}

type handler struct {
	releasesDir string
	baseURL     string
}

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

func (h *handler) handleReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, err := os.ReadDir(h.releasesDir)
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
		assetDir := filepath.Join(h.releasesDir, tag)

		assetEntries, err := os.ReadDir(assetDir)
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
			assetURL := fmt.Sprintf("%s/assets/%s/%s", h.baseURL, tag, ae.Name())
			assets = append(assets, ghAsset{
				Name:               ae.Name(),
				BrowserDownloadURL: assetURL,
				Size:               info.Size(),
			})
		}

		releases = append(releases, ghRelease{
			TagName:     tag,
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * 24 * time.Hour),
			Assets:      assets,
		})
	}

	sort.Slice(releases, func(i, j int) bool {
		return releases[i].PublishedAt.After(releases[j].PublishedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(releases)
}

func (h *handler) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/assets/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "invalid asset path", http.StatusBadRequest)
		return
	}
	tag, filename := parts[0], parts[1]
	http.ServeFile(w, r, filepath.Join(h.releasesDir, tag, filename))
}
