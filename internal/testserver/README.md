Lightweight HTTP test server that mimics the GitHub Releases API for cancha end-to-end tests.

## Usage

```go
srv := testserver.New(t, "/path/to/releases")
// Point GitHubHTTPAdapter.BaseURL at srv.Addr()
```

Releases directory layout: `<releases-dir>/<tag>/<asset-filename>`.
