// Package testserver provides a lightweight HTTP test server that mimics the
// GitHub Releases API and asset download endpoints. It is used by the cancha
// end-to-end workflow to exercise the shipkit update lifecycle without hitting
// the real GitHub API or the fede-iglesias/tools release infrastructure.
//
// # Directory layout
//
// The server reads assets from a directory tree rooted at the path passed to
// [New]. The expected layout mirrors the tag namespace used by the real tools
// repo:
//
//	<releases-dir>/
//	  v0.0.1/
//	    shipkit-example_0.0.1_linux_amd64.tar.gz
//	    shipkit-example_0.0.1_darwin_arm64.tar.gz
//	  v0.0.2/
//	    shipkit-example_0.0.2_linux_amd64.tar.gz
//	    shipkit-example_0.0.2_darwin_arm64.tar.gz
//
// Each subdirectory name is the tag. All files inside are served as release
// assets.
//
// # Routes
//
//   - GET /repos/{owner}/{repo}/releases - returns JSON array of releases
//     compatible with the GitHub Releases API shape consumed by
//     [lifecycle/update/adapters.GitHubHTTPAdapter].
//   - GET /assets/{tag}/{filename} - streams the raw asset bytes.
//
// # Usage
//
//	srv := testserver.New(t, "/path/to/releases")
//	defer srv.Close()
//	// Point the HTTP adapter at srv.Addr() during tests.
package testserver
