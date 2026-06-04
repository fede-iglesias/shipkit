// Package shipkitexample documents the default adapter wiring used by
// shipkit-example and provides a helper for constructing test doubles.
//
// In production all adapters are wired via shipkit defaults (see
// cmd/shipkit-example/main.go). No port overrides are needed beyond
// [github.com/fede-iglesias/shipkit/adapters.NewSigstoreCosign] +
// SetVerifyCore for cosign verification.
//
// The [TestDeps] function below is the recommended pattern for integration
// tests: it builds a [TestConfig] with isolated XDG dirs so that tests do
// not touch the developer's real data root.
package shipkitexample

import (
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit"
)

// TestConfig returns a [shipkit.Config] wired for integration tests. All XDG
// directories are rooted under t.TempDir() so the test is hermetic. The
// BinaryPath points to binPath (the binary under test).
//
// TestConfig calls t.Helper so test failure lines point to the caller.
func TestConfig(t *testing.T, binPath string) shipkit.Config {
	t.Helper()
	tmp := t.TempDir()
	cfg := shipkit.Config{
		AppName:    "shipkit-example",
		BinaryName: "shipkit-example",
		Repo:       "fede-iglesias/shipkit",
		TagPrefix:  "example-",
		Version:    "v0.0.1",
		BinaryPath: binPath,
		DataRoot:   filepath.Join(tmp, "data"),
		ConfigRoot: filepath.Join(tmp, "config"),
		CacheRoot:  filepath.Join(tmp, "cache"),
	}
	return cfg.WithDefaults()
}
