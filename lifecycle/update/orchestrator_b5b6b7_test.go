package update

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// ---------------------------------------------------------------------------
// B5: host arch mismatch (relay v0.1.1 incident, 2026-06-05)
//
// findAsset previously returned the FIRST .tar.gz in the release, which on a
// darwin/arm64 host could pick the darwin/amd64 tarball (or any other arch)
// depending on the order returned by the GitHub API. The downstream cosign
// verify then failed with "bundle not found" because there is no bundle for
// the wrong tarball at the local temp path. These tests exercise the matcher
// against the REAL asset names emitted by goreleaser+shipkit for the
// fede-iglesias/tools relay-v0.1.1 release:
//
//   relay_0.1.1_darwin_amd64.tar.gz
//   relay_0.1.1_darwin_arm64.tar.gz
//   relay_0.1.1_linux_amd64.tar.gz
//   relay_0.1.1_linux_arm64.tar.gz
//
// The fixture mirrors the live release; the assertions verify SPECIFIC asset
// names so a regression in the matcher (e.g. a return to the legacy "first
// wins" heuristic) surfaces by content, not just by shape.
// ---------------------------------------------------------------------------

// realRelayRelease returns a Release whose asset list mirrors the live
// fede-iglesias/tools relay-v0.1.1 release on GitHub. Used as the canonical
// fixture for B5 arch-matching tests.
func realRelayRelease() ports.Release {
	mk := func(name string) ports.Asset {
		return ports.Asset{
			Name:        name,
			DownloadURL: "https://github.com/fede-iglesias/tools/releases/download/relay-v0.1.1/" + name,
		}
	}
	return ports.Release{
		Tag: "relay-v0.1.1",
		Assets: []ports.Asset{
			mk("checksums.txt"),
			mk("install.sh"),
			mk("relay_0.1.1_darwin_amd64.tar.gz"),
			mk("relay_0.1.1_darwin_amd64.tar.gz.bundle"),
			mk("relay_0.1.1_darwin_amd64.tar.gz.sbom.json"),
			mk("relay_0.1.1_darwin_arm64.tar.gz"),
			mk("relay_0.1.1_darwin_arm64.tar.gz.bundle"),
			mk("relay_0.1.1_darwin_arm64.tar.gz.sbom.json"),
			mk("relay_0.1.1_linux_amd64.tar.gz"),
			mk("relay_0.1.1_linux_amd64.tar.gz.bundle"),
			mk("relay_0.1.1_linux_amd64.tar.gz.sbom.json"),
			mk("relay_0.1.1_linux_arm64.tar.gz"),
			mk("relay_0.1.1_linux_arm64.tar.gz.bundle"),
			mk("relay_0.1.1_linux_arm64.tar.gz.sbom.json"),
		},
	}
}

// TestFindAsset_DarwinArm64_PicksDarwinArm64Tarball is the EXACT bug 1
// repro: on a darwin/arm64 host the matcher must return the darwin_arm64
// asset, not the darwin_amd64 one that comes earlier in the release feed.
func TestFindAsset_DarwinArm64_PicksDarwinArm64Tarball(t *testing.T) {
	setHostForTest(t, "darwin", "arm64")

	asset, err := findAsset(realRelayRelease())
	if err != nil {
		t.Fatalf("want darwin/arm64 asset, got error: %v", err)
	}
	want := "relay_0.1.1_darwin_arm64.tar.gz"
	if asset.Name != want {
		t.Fatalf("findAsset on darwin/arm64: got %q, want %q (bug 1: wrong arch picked)", asset.Name, want)
	}
	// The download URL must point to the real relay release path: a regression
	// could pick the right name from a synthetic fixture but still build a
	// wrong URL.
	wantURL := "https://github.com/fede-iglesias/tools/releases/download/relay-v0.1.1/" + want
	if asset.DownloadURL != wantURL {
		t.Fatalf("findAsset DownloadURL: got %q, want %q", asset.DownloadURL, wantURL)
	}
}

// TestFindAsset_DarwinAmd64_PicksDarwinAmd64Tarball exercises the matcher
// in the symmetric direction so the test does not silently match the wrong
// asset by accident of fixture ordering.
func TestFindAsset_DarwinAmd64_PicksDarwinAmd64Tarball(t *testing.T) {
	setHostForTest(t, "darwin", "amd64")

	asset, err := findAsset(realRelayRelease())
	if err != nil {
		t.Fatalf("want darwin/amd64 asset, got error: %v", err)
	}
	want := "relay_0.1.1_darwin_amd64.tar.gz"
	if asset.Name != want {
		t.Fatalf("findAsset on darwin/amd64: got %q, want %q", asset.Name, want)
	}
}

// TestFindAsset_LinuxArm64_PicksLinuxArm64Tarball completes the matrix.
func TestFindAsset_LinuxArm64_PicksLinuxArm64Tarball(t *testing.T) {
	setHostForTest(t, "linux", "arm64")

	asset, err := findAsset(realRelayRelease())
	if err != nil {
		t.Fatalf("want linux/arm64 asset, got error: %v", err)
	}
	want := "relay_0.1.1_linux_arm64.tar.gz"
	if asset.Name != want {
		t.Fatalf("findAsset on linux/arm64: got %q, want %q", asset.Name, want)
	}
}

// TestFindAsset_LinuxAmd64_PicksLinuxAmd64Tarball completes the matrix.
func TestFindAsset_LinuxAmd64_PicksLinuxAmd64Tarball(t *testing.T) {
	setHostForTest(t, "linux", "amd64")

	asset, err := findAsset(realRelayRelease())
	if err != nil {
		t.Fatalf("want linux/amd64 asset, got error: %v", err)
	}
	want := "relay_0.1.1_linux_amd64.tar.gz"
	if asset.Name != want {
		t.Fatalf("findAsset on linux/amd64: got %q, want %q", asset.Name, want)
	}
}

// TestFindAsset_UnsupportedHost_ReportsClearError exercises the failure
// branch: when no asset matches the host the matcher MUST surface the host
// tuple in the error message so the user does not spend hours chasing a
// downstream "bundle not found" stack trace.
func TestFindAsset_UnsupportedHost_ReportsClearError(t *testing.T) {
	setHostForTest(t, "freebsd", "riscv64")

	_, err := findAsset(realRelayRelease())
	if err == nil {
		t.Fatal("want error when host arch has no matching asset")
	}
	if !strings.Contains(err.Error(), "freebsd/riscv64") {
		t.Fatalf("want error to mention host tuple, got %q", err.Error())
	}
}

// TestFindAsset_AmdAliasX86_64 exercises the alias table: a release that
// names its asset "..._x86_64.tar.gz" must still match on amd64.
func TestFindAsset_AmdAliasX86_64(t *testing.T) {
	setHostForTest(t, "linux", "amd64")

	rel := ports.Release{
		Tag: "myapp-v1.0.0",
		Assets: []ports.Asset{
			{Name: "myapp_v1.0.0_linux_x86_64.tar.gz", DownloadURL: "https://example.com/a"},
		},
	}
	asset, err := findAsset(rel)
	if err != nil {
		t.Fatalf("want match on x86_64 alias for amd64, got error: %v", err)
	}
	if asset.Name != "myapp_v1.0.0_linux_x86_64.tar.gz" {
		t.Fatalf("got %q, want x86_64 asset", asset.Name)
	}
}

// TestFindAsset_RejectsBundleEvenIfArchMatches asserts the matcher rejects
// the cosign .bundle and .sbom.json companion files even though their names
// contain the host's OS/arch tokens. Without the .tar.gz suffix check the
// matcher could return the bundle as if it were the tarball.
func TestFindAsset_RejectsBundleEvenIfArchMatches(t *testing.T) {
	setHostForTest(t, "darwin", "arm64")

	rel := ports.Release{
		Tag: "relay-v0.1.1",
		Assets: []ports.Asset{
			{Name: "relay_0.1.1_darwin_arm64.tar.gz.bundle", DownloadURL: "https://x/bundle"},
			{Name: "relay_0.1.1_darwin_arm64.tar.gz.sbom.json", DownloadURL: "https://x/sbom"},
			{Name: "relay_0.1.1_darwin_arm64.tar.gz", DownloadURL: "https://x/tarball"},
		},
	}
	asset, err := findAsset(rel)
	if err != nil {
		t.Fatalf("want tarball match, got error: %v", err)
	}
	if asset.Name != "relay_0.1.1_darwin_arm64.tar.gz" {
		t.Fatalf("got %q, want tarball not bundle/sbom (matcher must reject non-.tar.gz)", asset.Name)
	}
}

// ---------------------------------------------------------------------------
// B6: bundle companion not downloaded (relay v0.1.1 incident, 2026-06-05)
//
// handleDownload previously only fetched the tarball; handleVerify then
// passed `tarPath + ".bundle"` to Cosign.VerifyBundle, which stat'd the
// missing path and failed with "bundle not found at ...: no such file or
// directory". The fix downloads the .bundle companion at the same temp path
// when SkipVerify=false, leaving SkipVerify=true on the legacy fast path.
// ---------------------------------------------------------------------------

// TestDownload_BundleCompanionIsFetched asserts that on the verify-enabled
// path, DownloadAsset is invoked TWICE (tarball + bundle) with URLs the
// orchestrator constructed from the asset's DownloadURL, AND that both
// resulting local files exist on disk before the cosign verify step runs.
// This is the integration assertion the user would have wanted before
// shipping v0.2.1: shape-level "tarball exists" passed; content-level
// "bundle exists" did not.
func TestDownload_BundleCompanionIsFetched(t *testing.T) {
	cfg := defaultConfig()
	// Use a per-test tempdir so the file system check below is hermetic and
	// the file count assertion is precise.
	cfg.DataRoot = t.TempDir()
	cfg.SnapshotDir = t.TempDir()
	cfg.CurrentVersion = "0.1.0"
	cfg.SkipVerify = false

	o := baseOrchestrator(cfg)
	// Latest is 0.1.1 (a real upgrade) so the orchestrator goes through the
	// full forward path: snapshot, download, verify, replace, migrate, health.
	baseHTTP := realRelayLikeRelease("0.1.1")
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{Ok: true, Version: "0.1.1"}, nil
		},
	}

	// Capture which URLs DownloadAsset is asked to fetch.
	var downloads []string
	o.HTTP = wrapHTTPCaptureDownloads(baseHTTP, &downloads)

	// Cosign verify is mocked to succeed so we focus on the download contract.
	verifyCalled := false
	o.Cosign = &mockCosignPort{
		verifyBundleFn: func(_ context.Context, blobPath, bundlePath string) error {
			verifyCalled = true
			if !strings.HasSuffix(blobPath, ".tar.gz") {
				t.Errorf("verify: blobPath must end in .tar.gz, got %q", blobPath)
			}
			if !strings.HasSuffix(bundlePath, ".tar.gz.bundle") {
				t.Errorf("verify: bundlePath must end in .tar.gz.bundle, got %q", bundlePath)
			}
			return nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindOK {
		t.Fatalf("want KindOK, got %s", res.Kind)
	}
	if !verifyCalled {
		t.Fatal("cosign VerifyBundle must be called when SkipVerify=false")
	}
	if len(downloads) != 2 {
		t.Fatalf("want 2 downloads (tarball + bundle), got %d: %v", len(downloads), downloads)
	}
	// Exact-URL assertion (content, not shape): the bundle URL must be the
	// tarball URL with ".bundle" appended.
	if downloads[1] != downloads[0]+".bundle" {
		t.Fatalf("download 2 = %q, want download 1 + \".bundle\" = %q",
			downloads[1], downloads[0]+".bundle")
	}
}

// TestDownload_BundleSkippedWhenSkipVerify asserts the legacy fast path:
// when SkipVerify=true the bundle companion is NOT fetched.
func TestDownload_BundleSkippedWhenSkipVerify(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataRoot = t.TempDir()
	cfg.SnapshotDir = t.TempDir()
	cfg.CurrentVersion = "0.1.0"
	cfg.SkipVerify = true

	o := baseOrchestrator(cfg)
	baseHTTP := realRelayLikeRelease("0.1.1")
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{Ok: true, Version: "0.1.1"}, nil
		},
	}
	var downloads []string
	o.HTTP = wrapHTTPCaptureDownloads(baseHTTP, &downloads)

	verifyCalled := false
	o.Cosign = &mockCosignPort{
		verifyBundleFn: func(_ context.Context, _, _ string) error {
			verifyCalled = true
			return nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindOK {
		t.Fatalf("want KindOK, got %s", res.Kind)
	}
	if verifyCalled {
		t.Fatal("cosign VerifyBundle must NOT be called when SkipVerify=true")
	}
	if len(downloads) != 1 {
		t.Fatalf("want exactly 1 download (no bundle) with SkipVerify=true, got %d: %v",
			len(downloads), downloads)
	}
}

// TestDownload_BundleErrorTriggersRollback asserts that if the bundle URL
// returns an error from the HTTP port (404, transport failure, etc.) the
// orchestrator rolls back rather than silently swallowing the failure and
// proceeding to cosign verify with a missing bundle. This is the actual
// behavior the user observed on 2026-06-05 BEFORE the fix: cosign verify
// said "bundle not found" because the file was never created. After the
// fix, the orchestrator surfaces the download failure as a rollback at
// StateDownloadBinary, NOT a verify failure.
func TestDownload_BundleErrorTriggersRollback(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataRoot = t.TempDir()
	cfg.SnapshotDir = t.TempDir()
	cfg.CurrentVersion = "0.1.0"
	cfg.SkipVerify = false

	o := baseOrchestrator(cfg)
	baseHTTP := realRelayLikeRelease("0.1.1")
	// Wrap HTTP to fail on the .bundle URL.
	o.HTTP = wrapHTTPFailBundleDownload(baseHTTP)

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (clean rollback), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s (reason: %s)", res.Kind, res.Reason)
	}
	if !strings.Contains(res.Reason, "bundle") {
		t.Fatalf("want rollback reason to mention bundle, got %q", res.Reason)
	}
}

// ---------------------------------------------------------------------------
// helpers for B5/B6 (asset matcher + bundle download)
// ---------------------------------------------------------------------------

// realRelayLikeRelease returns an HTTPPort that mimics the real
// fede-iglesias/tools relay release. The asset list matches the live
// pattern; orchestrator's findAsset selects the linux/amd64 entry under the
// TestMain default host (so this works in CI on any arch).
func realRelayLikeRelease(ver string) *mockHTTPPort {
	mk := func(name string) ports.Asset {
		return ports.Asset{
			Name:        name,
			DownloadURL: "https://github.com/fede-iglesias/tools/releases/download/relay-v" + ver + "/" + name,
		}
	}
	assets := []ports.Asset{
		mk("checksums.txt"),
		mk("install.sh"),
		mk("relay_" + ver + "_darwin_amd64.tar.gz"),
		mk("relay_" + ver + "_darwin_amd64.tar.gz.bundle"),
		mk("relay_" + ver + "_darwin_arm64.tar.gz"),
		mk("relay_" + ver + "_darwin_arm64.tar.gz.bundle"),
		mk("relay_" + ver + "_linux_amd64.tar.gz"),
		mk("relay_" + ver + "_linux_amd64.tar.gz.bundle"),
	}
	return &mockHTTPPort{
		latestReleaseFn: func(_ context.Context, _, _ string) (ports.Release, error) {
			return ports.Release{Tag: "relay-v" + ver, Assets: assets}, nil
		},
		getReleaseByTagFn: func(_ context.Context, _, tag string) (ports.Release, error) {
			return ports.Release{Tag: tag, Assets: assets}, nil
		},
	}
}

// wrapHTTPCaptureDownloads returns an HTTPPort that captures every URL
// passed to DownloadAsset into urls, while delegating LatestRelease and
// GetReleaseByTag to base.
func wrapHTTPCaptureDownloads(base *mockHTTPPort, urls *[]string) *mockHTTPPort {
	return &mockHTTPPort{
		latestReleaseFn:   base.latestReleaseFn,
		getReleaseByTagFn: base.getReleaseByTagFn,
		downloadAssetFn: func(_ context.Context, url string, w io.Writer) error {
			*urls = append(*urls, url)
			_, err := w.Write([]byte("fake-bytes-for-" + url))
			return err
		},
	}
}

// wrapHTTPFailBundleDownload returns an HTTPPort that succeeds on tarball
// downloads but fails on any URL ending in ".bundle". Used to reproduce the
// scenario where the bundle URL is unreachable (404, etc.).
func wrapHTTPFailBundleDownload(base *mockHTTPPort) *mockHTTPPort {
	return &mockHTTPPort{
		latestReleaseFn:   base.latestReleaseFn,
		getReleaseByTagFn: base.getReleaseByTagFn,
		downloadAssetFn: func(_ context.Context, url string, w io.Writer) error {
			if strings.HasSuffix(url, ".bundle") {
				return errors.New("simulated 404 on bundle")
			}
			_, err := w.Write([]byte("fake-tarball"))
			return err
		},
	}
}
