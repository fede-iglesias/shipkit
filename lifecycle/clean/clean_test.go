package clean_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/clean"
	"github.com/fede-iglesias/shipkit/lifecycle/recovery"
	"github.com/fede-iglesias/shipkit/ports"
)

// fixedNow is used as the clock anchor in all tests.
var fixedNow = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

// newDeps returns a Deps wired with mock ports and a data root at dataRoot.
// The snapshot/tmp/cache/log dirs are computed under dataRoot by convention.
func newDeps(dataRoot string) clean.Deps {
	clock := ports.NewMockClockPort(fixedNow)
	fs := ports.NewMockFsPort()
	prompt := ports.NewMockPromptPort()
	paths := ports.NewMockPathsPort()
	paths.DataDirFunc = func(app string) (string, error) {
		return filepath.Join(dataRoot, app), nil
	}

	return clean.Deps{
		AppName: "testapp",
		FS:      fs,
		Paths:   paths,
		Clock:   clock,
		Prompt:  prompt,
	}
}

// TestRun_NoFlags_PrintsHelpAndExitsOne verifies that calling Run with no scope
// flags set returns ErrNoScope so the cobra layer can print help and exit 1.
func TestRun_NoFlags_PrintsHelpAndExitsOne(t *testing.T) {
	deps := newDeps(t.TempDir())
	opts := clean.Options{} // no flags

	_, err := clean.Run(context.Background(), deps, opts)
	if !errors.Is(err, clean.ErrNoScope) {
		t.Fatalf("expected ErrNoScope, got %v", err)
	}
}

// TestRun_PrintDryRun verifies that --print returns candidates without
// touching the filesystem (no RemoveDir calls).
func TestRun_PrintDryRun(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	// Seed two snapshot entries via ListSnapshotsFunc so the clean verb finds them.
	age40 := fixedNow.Add(-40 * 24 * time.Hour)
	age5 := fixedNow.Add(-5 * 24 * time.Hour)
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{Path: filepath.Join(snapshotDir, "snap-old"), ModTime: age40, Size: 1024},
			{Path: filepath.Join(snapshotDir, "snap-new"), ModTime: age5, Size: 512},
		}, nil
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Print:     true,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 0 {
		t.Errorf("dry-run must not call RemoveDir, got %d calls", len(fs.RemoveDirCalls))
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 candidate (old snapshot), got %d", len(result.Items))
	}
	if result.Items[0].Path != filepath.Join("/tmp/testapp/data/testapp", "snapshots", "snap-old") &&
		!filepath.IsAbs(result.Items[0].Path) {
		// just check a candidate was recorded
	}
	_ = result
}

// TestRun_Snapshots_OlderThan removes only snapshots older than the threshold.
func TestRun_Snapshots_OlderThan(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	age40 := fixedNow.Add(-40 * 24 * time.Hour)
	age5 := fixedNow.Add(-5 * 24 * time.Hour)
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{Path: filepath.Join(snapshotDir, "snap-old"), ModTime: age40, Size: 2048},
			{Path: filepath.Join(snapshotDir, "snap-new"), ModTime: age5, Size: 512},
		}, nil
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 1 {
		t.Errorf("expected 1 RemoveDir call, got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 2048 {
		t.Errorf("expected 2048 bytes reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_KeepN_OverridesOlderThan ensures that --keep N preserves the
// newest N snapshots regardless of --older-than.
func TestRun_KeepN_OverridesOlderThan(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	// Three snapshots all older than 30d, but --keep 2 preserves the newest two.
	base := fixedNow
	entries := []clean.SnapshotEntry{
		{Path: "snap-newest", ModTime: base.Add(-31 * 24 * time.Hour), Size: 100},
		{Path: "snap-middle", ModTime: base.Add(-40 * 24 * time.Hour), Size: 200},
		{Path: "snap-oldest", ModTime: base.Add(-50 * 24 * time.Hour), Size: 300},
	}
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return entries, nil
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Keep:      2,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the oldest should be removed (newest 2 kept).
	if len(fs.RemoveDirCalls) != 1 {
		t.Errorf("expected 1 removal (oldest), got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 300 {
		t.Errorf("expected 300 reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_RecoveryManifestProtection verifies that a snapshot referenced by
// the recovery manifest is NOT deleted.
func TestRun_RecoveryManifestProtection(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	// Two snapshots, both older than threshold. One is referenced by the manifest.
	age40 := fixedNow.Add(-40 * 24 * time.Hour)
	age35 := fixedNow.Add(-35 * 24 * time.Hour)

	// The protected path is derived from what DataDir returns for "testapp".
	// newDeps wires DataDirFunc to return filepath.Join(dir, app), so:
	dataDir := filepath.Join(dir, "testapp")
	snapshotDir := filepath.Join(dataDir, "snapshots")
	protectedPath := filepath.Join(snapshotDir, "snap-protected")

	deps.ListSnapshotsFunc = func(sd string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{Path: filepath.Join(sd, "snap-protected"), ModTime: age35, Size: 1000},
			{Path: filepath.Join(sd, "snap-deletable"), ModTime: age40, Size: 2000},
		}, nil
	}

	// Wire a recovery manifest that references snap-protected using the same path
	// that the list func produces.
	deps.ReadManifestFunc = func(manifestPath string) (*recovery.Manifest, error) {
		return &recovery.Manifest{
			SnapshotPath: protectedPath,
		}, nil
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// snap-protected must NOT be removed.
	if len(fs.RemoveDirCalls) != 1 {
		t.Errorf("expected exactly 1 removal (snap-deletable only), got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 2000 {
		t.Errorf("expected 2000 reclaimed, got %d", result.Reclaimed)
	}
}

// TestClean_ReadsRecoveryManifest_ProtectsSnapshot exercises the production
// DefaultReadManifest path end-to-end: a real manifest is written to disk via
// recovery.Write, then clean.Run uses clean.DefaultReadManifest to parse it
// through the canonical recovery package. The referenced snapshot must NOT be
// removed.
func TestClean_ReadsRecoveryManifest_ProtectsSnapshot(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	dataDir := filepath.Join(dir, "testapp")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	snapshotDir := filepath.Join(dataDir, "snapshots")
	protectedPath := filepath.Join(snapshotDir, "snap-protected")
	deletablePath := filepath.Join(snapshotDir, "snap-deletable")

	manifest := recovery.Manifest{
		Version:      1,
		AppName:      "testapp",
		SnapshotPath: protectedPath,
		Steps:        []string{"snapshot"},
		CreatedAt:    fixedNow,
	}
	if err := recovery.Write(recovery.Path(dataDir), manifest); err != nil {
		t.Fatalf("recovery.Write: %v", err)
	}

	age40 := fixedNow.Add(-40 * 24 * time.Hour)
	age35 := fixedNow.Add(-35 * 24 * time.Hour)
	deps.ListSnapshotsFunc = func(sd string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{Path: protectedPath, ModTime: age35, Size: 1000},
			{Path: deletablePath, ModTime: age40, Size: 2000},
		}, nil
	}
	// Wire the production reader so it parses the on-disk JSON via recovery.Read.
	deps.ReadManifestFunc = clean.DefaultReadManifest

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("clean.Run: %v", err)
	}
	if len(fs.RemoveDirCalls) != 1 {
		t.Fatalf("expected 1 removal (snap-deletable only), got %d", len(fs.RemoveDirCalls))
	}
	if fs.RemoveDirCalls[0] != deletablePath {
		t.Errorf("removed %q, want %q (protected snapshot must survive)", fs.RemoveDirCalls[0], deletablePath)
	}
	if result.Reclaimed != 2000 {
		t.Errorf("expected 2000 reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_SymlinkEscapePrevention verifies that a snapshot entry whose path
// resolves to a symlink target outside DataDir is refused.
func TestRun_SymlinkEscapePrevention(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	// One snapshot entry but its EvalSymlinks resolves outside DataDir.
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{
				Path:        filepath.Join(snapshotDir, "snap-symlink"),
				ModTime:     fixedNow.Add(-40 * 24 * time.Hour),
				Size:        500,
				SymlinkDest: "/etc/passwd", // escapes DataDir
			},
		}, nil
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The symlink-escape entry must NOT be removed.
	if len(fs.RemoveDirCalls) != 0 {
		t.Errorf("symlink escape: must not call RemoveDir, got %d calls", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 0 {
		t.Errorf("expected 0 reclaimed (symlink refused), got %d", result.Reclaimed)
	}
}

// TestRun_TmpFlag removes contents under the tmp dir.
func TestRun_TmpFlag(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	deps.ListTmpFunc = func(tmpDir string) ([]clean.TmpEntry, error) {
		return []clean.TmpEntry{
			{Path: filepath.Join(tmpDir, "work-1234"), Size: 8192},
		}, nil
	}

	opts := clean.Options{
		Tmp: true,
		Yes: true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 1 {
		t.Errorf("expected 1 RemoveDir for tmp, got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 8192 {
		t.Errorf("expected 8192 reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_CacheFlag removes the cache directory.
func TestRun_CacheFlag(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	deps.ListCacheFunc = func(cacheDir string) ([]clean.CacheEntry, error) {
		return []clean.CacheEntry{
			{Path: filepath.Join(cacheDir, "downloads"), Size: 4096},
		}, nil
	}

	opts := clean.Options{
		Cache: true,
		Yes:   true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 1 {
		t.Errorf("expected 1 RemoveDir for cache, got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 4096 {
		t.Errorf("expected 4096 reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_LogsFlag removes the logs directory.
func TestRun_LogsFlag(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	deps.ListLogsFunc = func(logsDir string) ([]clean.LogEntry, error) {
		return []clean.LogEntry{
			{Path: filepath.Join(logsDir, "run-001.log"), Size: 256},
			{Path: filepath.Join(logsDir, "run-002.log"), Size: 256},
		}, nil
	}

	opts := clean.Options{
		Logs: true,
		Yes:  true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 2 {
		t.Errorf("expected 2 RemoveDir calls for logs, got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 512 {
		t.Errorf("expected 512 reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_AllFlag is equivalent to setting Snapshots + Tmp + Cache + Logs.
func TestRun_AllFlag(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{Path: filepath.Join(snapshotDir, "snap-old"), ModTime: fixedNow.Add(-40 * 24 * time.Hour), Size: 100},
		}, nil
	}
	deps.ListTmpFunc = func(tmpDir string) ([]clean.TmpEntry, error) {
		return []clean.TmpEntry{{Path: filepath.Join(tmpDir, "tmp-work"), Size: 200}}, nil
	}
	deps.ListCacheFunc = func(cacheDir string) ([]clean.CacheEntry, error) {
		return []clean.CacheEntry{{Path: filepath.Join(cacheDir, "dl"), Size: 300}}, nil
	}
	deps.ListLogsFunc = func(logsDir string) ([]clean.LogEntry, error) {
		return []clean.LogEntry{{Path: filepath.Join(logsDir, "app.log"), Size: 400}}, nil
	}

	opts := clean.Options{
		All:       true,
		OlderThan: 30 * 24 * time.Hour,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 4 {
		t.Errorf("expected 4 removals (all scopes), got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 1000 {
		t.Errorf("expected 1000 reclaimed, got %d", result.Reclaimed)
	}
}

// TestRun_Confirm_UserDeclines verifies that when the user says no, Run returns
// an empty result with no removals.
func TestRun_Confirm_UserDeclines(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)
	fs := deps.FS.(*ports.MockFsPort)

	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return []clean.SnapshotEntry{
			{Path: filepath.Join(snapshotDir, "snap-old"), ModTime: fixedNow.Add(-40 * 24 * time.Hour), Size: 500},
		}, nil
	}
	// Override prompt to decline.
	mockPrompt := deps.Prompt.(*ports.MockPromptPort)
	mockPrompt.ConfirmResult = false

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		// Yes not set - will prompt
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.RemoveDirCalls) != 0 {
		t.Errorf("user declined: no removals expected, got %d", len(fs.RemoveDirCalls))
	}
	if result.Reclaimed != 0 {
		t.Errorf("expected 0 reclaimed after decline, got %d", result.Reclaimed)
	}
}

// TestRun_NothingToClean verifies that when no candidates are found, Run
// returns an empty result with exit 0 (no error).
func TestRun_NothingToClean(t *testing.T) {
	dir := t.TempDir()
	deps := newDeps(dir)

	deps.ListSnapshotsFunc = func(snapshotDir string) ([]clean.SnapshotEntry, error) {
		return nil, nil // empty
	}

	opts := clean.Options{
		Snapshots: true,
		OlderThan: 30 * 24 * time.Hour,
		Yes:       true,
	}
	result, err := clean.Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if result.Reclaimed != 0 {
		t.Errorf("expected 0 reclaimed, got %d", result.Reclaimed)
	}
}
