package clean

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDefaultListSnapshots_Empty verifies that listing a non-existent dir returns nil.
func TestDefaultListSnapshots_Empty(t *testing.T) {
	entries, err := DefaultListSnapshots("/nonexistent-path-abc123")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for missing dir, got %v", entries)
	}
}

// TestDefaultListSnapshots_WithEntries verifies that real directories are enumerated.
func TestDefaultListSnapshots_WithEntries(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a snapshot subdirectory with a file inside.
	snap1 := filepath.Join(snapDir, "snap-001")
	if err := os.Mkdir(snap1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snap1, "binary"), []byte("hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := DefaultListSnapshots(snapDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != snap1 {
		t.Errorf("expected path %s, got %s", snap1, entries[0].Path)
	}
	if entries[0].Size != 5 {
		t.Errorf("expected size 5, got %d", entries[0].Size)
	}
	if entries[0].SymlinkDest != "" {
		t.Errorf("expected empty SymlinkDest for non-symlink, got %q", entries[0].SymlinkDest)
	}
}

// TestDefaultListTmp_Empty verifies nil return for missing dir.
func TestDefaultListTmp_Empty(t *testing.T) {
	entries, err := DefaultListTmp("/nonexistent-tmp-dir-xyz")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

// TestDefaultListTmp_WithEntries verifies enumeration of tmp work entries.
func TestDefaultListTmp_WithEntries(t *testing.T) {
	dir := t.TempDir()
	tmpDir := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(tmpDir, "work-1234")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := DefaultListTmp(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != work {
		t.Errorf("expected path %s, got %s", work, entries[0].Path)
	}
}

// TestDefaultListCache_Empty verifies nil return for missing dir.
func TestDefaultListCache_Empty(t *testing.T) {
	entries, err := DefaultListCache("/nonexistent-cache-dir-abc")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

// TestDefaultListCache_WithEntries verifies cache entry enumeration.
func TestDefaultListCache_WithEntries(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dl := filepath.Join(cacheDir, "downloads")
	if err := os.Mkdir(dl, 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := DefaultListCache(cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != dl {
		t.Errorf("expected %s, got %s", dl, entries[0].Path)
	}
}

// TestDefaultListLogs_Empty verifies nil return for missing dir.
func TestDefaultListLogs_Empty(t *testing.T) {
	entries, err := DefaultListLogs("/nonexistent-logs-dir-xyz")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

// TestDefaultListLogs_WithEntries verifies log file enumeration (dirs skipped).
func TestDefaultListLogs_WithEntries(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(logsDir, "run-001.log")
	if err := os.WriteFile(logFile, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Add a subdir that should be skipped.
	if err := os.Mkdir(filepath.Join(logsDir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := DefaultListLogs(logsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (directory skipped), got %d", len(entries))
	}
	if entries[0].Path != logFile {
		t.Errorf("expected %s, got %s", logFile, entries[0].Path)
	}
}

// TestDefaultReadManifest_Missing verifies (nil, nil) for missing manifest.
func TestDefaultReadManifest_Missing(t *testing.T) {
	m, err := DefaultReadManifest("/nonexistent-manifest-path.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if m != nil {
		t.Errorf("expected nil manifest for missing file, got %+v", m)
	}
}

// TestDefaultReadManifest_Valid verifies parsing of a valid manifest JSON.
func TestDefaultReadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".shipkit.recovery-manifest.json")
	data, _ := json.Marshal(RecoveryManifest{SnapshotPath: "/data/snaps/snap-abc"})
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := DefaultReadManifest(manifestPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.SnapshotPath != "/data/snaps/snap-abc" {
		t.Errorf("expected SnapshotPath /data/snaps/snap-abc, got %q", m.SnapshotPath)
	}
}

// TestDefaultReadManifest_Invalid verifies error on invalid JSON.
func TestDefaultReadManifest_Invalid(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".shipkit.recovery-manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{invalid json}"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := DefaultReadManifest(manifestPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestDirSize_Empty verifies that dirSize returns 0 for an empty directory.
func TestDirSize_Empty(t *testing.T) {
	dir := t.TempDir()
	if s := dirSize(dir); s != 0 {
		t.Errorf("expected 0 for empty dir, got %d", s)
	}
}

// TestDirSize_WithFiles verifies that dirSize sums file sizes recursively.
func TestDirSize_WithFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.bin"), []byte("world!"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 5 + 6 = 11 bytes total.
	if s := dirSize(dir); s != 11 {
		t.Errorf("expected 11, got %d", s)
	}
}

// TestFormatBytes covers the format size branches.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}
	for _, tc := range tests {
		got := formatBytes(tc.bytes)
		if got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// TestDefaultListSnapshots_Symlink verifies that symlink entries are returned
// with SymlinkDest populated via Lstat + Readlink.
func TestDefaultListSnapshots_Symlink(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a real dir and a symlink to it.
	target := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(snapDir, "snap-sym")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	entries, err := DefaultListSnapshots(snapDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Symlink entries are included; SymlinkDest must be populated.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Lstat detects ModeSymlink; SymlinkDest should be the resolved abs target.
	if entries[0].SymlinkDest == "" {
		t.Error("expected SymlinkDest to be populated for a symlink entry")
	}
}

// TestDefaultListSnapshots_DanglingSymlink verifies that a dangling symlink
// (Lstat ok, Stat fails) is skipped gracefully.
func TestDefaultListSnapshots_DanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a symlink pointing to a non-existent target (dangling).
	dangling := filepath.Join(snapDir, "snap-dangling")
	if err := os.Symlink("/nonexistent-target-xyz", dangling); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	entries, err := DefaultListSnapshots(snapDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Dangling symlink has Lstat succeed but Stat fail; it is skipped.
	if len(entries) != 0 {
		t.Errorf("expected dangling symlink to be skipped, got %d entries", len(entries))
	}
}

// TestRun_OlderThanDefault verifies that OlderThan defaults to 30 days
// when left at zero.
func TestRun_OlderThanDefault(t *testing.T) {
	dir := t.TempDir()
	deps := newInternalDeps(dir)

	// Snapshot that is 29 days old - should NOT be cleaned with 30d default.
	age29 := fixedNowInternal.Add(-29 * 24 * time.Hour)
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]SnapshotEntry, error) {
		return []SnapshotEntry{
			{Path: filepath.Join(snapshotDir, "snap-recent"), ModTime: age29, Size: 100},
		}, nil
	}

	opts := Options{
		Snapshots: true,
		// OlderThan left at zero - should default to 30d
		Yes: true,
	}
	result, err := Run(context.Background(), deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reclaimed != 0 {
		t.Errorf("29-day-old snapshot should not be removed with 30d default, got %d reclaimed", result.Reclaimed)
	}
}

// TestRun_DataDirError verifies that a DataDir error propagates from Run.
func TestRun_DataDirError(t *testing.T) {
	dir := t.TempDir()
	deps := newInternalDeps(dir)
	deps.Paths = &errorPathsPort{}

	opts := Options{Snapshots: true, Yes: true}
	_, err := Run(context.Background(), deps, opts)
	if err == nil {
		t.Fatal("expected error when DataDir fails, got nil")
	}
}

// TestRun_ListSnapshotsError verifies that a ListSnapshots error propagates.
func TestRun_ListSnapshotsError(t *testing.T) {
	dir := t.TempDir()
	deps := newInternalDeps(dir)
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]SnapshotEntry, error) {
		return nil, os.ErrPermission
	}

	opts := Options{Snapshots: true, Yes: true}
	_, err := Run(context.Background(), deps, opts)
	if err == nil {
		t.Fatal("expected error when ListSnapshots fails, got nil")
	}
}

// TestCollectSnapshots_NilListFunc verifies nil return when ListSnapshotsFunc is nil.
func TestCollectSnapshots_NilListFunc(t *testing.T) {
	deps := Deps{ListSnapshotsFunc: nil}
	items, err := collectSnapshots(deps, Options{OlderThan: 30 * 24 * time.Hour},
		"/any/snaps", "/any", fixedNowInternal)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if items != nil {
		t.Errorf("expected nil items when no func, got %v", items)
	}
}

// TestDefaultListSnapshots_ReadDirError verifies non-NotExist error propagation.
func TestDefaultListSnapshots_ReadDirError(t *testing.T) {
	// Create a file (not a dir) where a dir is expected - ReadDir returns error.
	dir := t.TempDir()
	notADir := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pass the file path as if it were a directory - os.ReadDir on a file returns an error.
	_, err := DefaultListSnapshots(notADir)
	if err == nil {
		t.Fatal("expected error when ReadDir called on a file, got nil")
	}
}

// TestDefaultListTmp_ReadDirError verifies non-NotExist error propagation.
func TestDefaultListTmp_ReadDirError(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := DefaultListTmp(notADir)
	if err == nil {
		t.Fatal("expected error when ReadDir called on a file, got nil")
	}
}

// TestDefaultListCache_ReadDirError verifies non-NotExist error propagation.
func TestDefaultListCache_ReadDirError(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := DefaultListCache(notADir)
	if err == nil {
		t.Fatal("expected error when ReadDir called on a file, got nil")
	}
}

// TestDefaultListLogs_ReadDirError verifies non-NotExist error propagation.
func TestDefaultListLogs_ReadDirError(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "notadir.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := DefaultListLogs(notADir)
	if err == nil {
		t.Fatal("expected error when ReadDir called on a file, got nil")
	}
}

// TestDefaultReadManifest_ReadError verifies non-NotExist error propagation.
func TestDefaultReadManifest_ReadError(t *testing.T) {
	dir := t.TempDir()
	// Create a directory where a file is expected - os.ReadFile on a dir returns error.
	notAFile := filepath.Join(dir, "manifest-dir")
	if err := os.Mkdir(notAFile, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := DefaultReadManifest(notAFile)
	if err == nil {
		t.Fatal("expected error when reading a directory as a file, got nil")
	}
}

// TestDirSize_NonExistent verifies 0 return for non-existent path.
func TestDirSize_NonExistent(t *testing.T) {
	if s := dirSize("/nonexistent-path-abcxyz"); s != 0 {
		t.Errorf("expected 0 for non-existent path, got %d", s)
	}
}

// TestRun_PromptError verifies that a prompt error propagates from Run.
func TestRun_PromptError(t *testing.T) {
	dir := t.TempDir()
	deps := newInternalDeps(dir)

	age40 := fixedNowInternal.Add(-40 * 24 * time.Hour)
	deps.ListSnapshotsFunc = func(snapshotDir string) ([]SnapshotEntry, error) {
		return []SnapshotEntry{
			{Path: filepath.Join(snapshotDir, "snap-old"), ModTime: age40, Size: 100},
		}, nil
	}
	deps.Prompt = &errorPromptPort{}

	opts := Options{Snapshots: true, OlderThan: 30 * 24 * time.Hour}
	_, err := Run(context.Background(), deps, opts)
	if err == nil {
		t.Fatal("expected error when Prompt fails, got nil")
	}
}

// TestDefaultListSnapshots_LstatRace verifies that when osLstat fails for an
// entry (file disappeared between ReadDir and Lstat), the entry is silently
// skipped and no error is returned.
func TestDefaultListSnapshots_LstatRace(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a real entry so ReadDir returns something.
	if err := os.Mkdir(filepath.Join(snapDir, "snap-race"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig := osLstat
	defer func() { osLstat = orig }()
	osLstat = func(name string) (os.FileInfo, error) {
		return nil, errors.New("simulated race: file disappeared")
	}

	entries, err := DefaultListSnapshots(snapDir)
	if err != nil {
		t.Fatalf("expected nil error on lstat race, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when lstat fails, got %d", len(entries))
	}
}

// TestDefaultListSnapshots_StatRace verifies that when osStat fails for an
// entry (e.g. symlink target removed between Lstat and Stat), the entry is
// skipped and no error is returned.
func TestDefaultListSnapshots_StatRace(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(snapDir, "snap-stat-race"), 0o755); err != nil {
		t.Fatal(err)
	}

	// osLstat must return a valid non-symlink FileInfo so the code reaches osStat.
	origLstat := osLstat
	defer func() { osLstat = origLstat }()
	osLstat = func(name string) (os.FileInfo, error) {
		// Delegate to the real Lstat so we get a real FileInfo (not a symlink).
		return os.Lstat(name)
	}

	origStat := osStat
	defer func() { osStat = origStat }()
	osStat = func(name string) (os.FileInfo, error) {
		return nil, errors.New("simulated race: stat failed")
	}

	entries, err := DefaultListSnapshots(snapDir)
	if err != nil {
		t.Fatalf("expected nil error on stat race, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when stat fails, got %d", len(entries))
	}
}

// TestDefaultListSnapshots_ReadlinkBranch covers both arms of the readlink
// branch: the happy path (readErr == nil, SymlinkDest populated) is covered by
// TestDefaultListSnapshots_Symlink; this test covers the readErr != nil arm
// by injecting a failing osReadlink while osLstat reports a symlink.
func TestDefaultListSnapshots_ReadlinkBranch(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(snapDir, "snap-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	origReadlink := osReadlink
	defer func() { osReadlink = origReadlink }()
	osReadlink = func(name string) (string, error) {
		return "", errors.New("simulated readlink error")
	}

	entries, err := DefaultListSnapshots(snapDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The symlink is still included (Stat follows it fine); SymlinkDest is just empty.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SymlinkDest != "" {
		t.Errorf("expected empty SymlinkDest when readlink fails, got %q", entries[0].SymlinkDest)
	}
}

// TestDefaultListLogs_InfoRace verifies that when osLstat fails for a log file
// entry (file disappeared between ReadDir and Lstat), the entry is silently
// skipped and no error is returned.
func TestDefaultListLogs_InfoRace(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a regular file so ReadDir returns a non-directory entry.
	if err := os.WriteFile(filepath.Join(logsDir, "run.log"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := osLstat
	defer func() { osLstat = orig }()
	osLstat = func(name string) (os.FileInfo, error) {
		return nil, errors.New("simulated race: log file disappeared")
	}

	entries, err := DefaultListLogs(logsDir)
	if err != nil {
		t.Fatalf("expected nil error on lstat race in logs, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries when lstat fails, got %d", len(entries))
	}
}

// fixedNowInternal is the clock anchor for package-internal tests.
var fixedNowInternal = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

// newInternalDeps builds a Deps using package-local mock adapters.
func newInternalDeps(dataRoot string) Deps {
	return Deps{
		AppName: "testapp",
		FS:      &simpleRemoveDirPort{},
		Paths:   &simplePathsPort{dataRoot: dataRoot},
		Clock:   &simpleClockPort{fixed: fixedNowInternal},
		Prompt:  &simplePromptPort{result: true},
	}
}

// --- package-local minimal adapters ---

type simpleRemoveDirPort struct {
	calls []string
}

func (p *simpleRemoveDirPort) Snapshot(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (p *simpleRemoveDirPort) Restore(_ context.Context, _, _ string) error        { return nil }
func (p *simpleRemoveDirPort) AtomicReplace(_ context.Context, _, _ string) error   { return nil }
func (p *simpleRemoveDirPort) ExtractTarGz(_ context.Context, _, _ string) error    { return nil }
func (p *simpleRemoveDirPort) CopyFile(_ context.Context, _, _ string, _ fs.FileMode) error {
	return nil
}
func (p *simpleRemoveDirPort) RemoveDir(_ context.Context, dir string) error {
	p.calls = append(p.calls, dir)
	return nil
}

type simplePathsPort struct{ dataRoot string }

func (p *simplePathsPort) Executable() (string, error) { return "/usr/local/bin/app", nil }
func (p *simplePathsPort) DataDir(app string) (string, error) {
	return filepath.Join(p.dataRoot, app), nil
}
func (p *simplePathsPort) ConfigDir(app string) (string, error) {
	return filepath.Join(p.dataRoot, "config", app), nil
}
func (p *simplePathsPort) CacheDir(app string) (string, error) {
	return filepath.Join(p.dataRoot, "cache", app), nil
}
func (p *simplePathsPort) UserHome() (string, error)  { return "/home/user", nil }
func (p *simplePathsPort) DefaultInstallDir() string  { return "/usr/local/bin" }
func (p *simplePathsPort) InPATH(_ string) bool       { return true }
func (p *simplePathsPort) PATHList() []string          { return []string{"/usr/local/bin"} }

type simpleClockPort struct{ fixed time.Time }

func (c *simpleClockPort) NowUTC() time.Time              { return c.fixed }
func (c *simpleClockPort) Since(t time.Time) time.Duration { return c.fixed.Sub(t) }

type simplePromptPort struct{ result bool }

func (p *simplePromptPort) Confirm(_ string, _ bool) (bool, error) { return p.result, nil }
func (p *simplePromptPort) IsInteractive() bool                    { return false }

// errorPathsPort returns an error from DataDir.
type errorPathsPort struct{}

func (p *errorPathsPort) Executable() (string, error)             { return "", nil }
func (p *errorPathsPort) DataDir(_ string) (string, error)         { return "", os.ErrPermission }
func (p *errorPathsPort) ConfigDir(_ string) (string, error)       { return "", nil }
func (p *errorPathsPort) CacheDir(_ string) (string, error)        { return "", nil }
func (p *errorPathsPort) UserHome() (string, error)                { return "", nil }
func (p *errorPathsPort) DefaultInstallDir() string                { return "/usr/local/bin" }
func (p *errorPathsPort) InPATH(_ string) bool                     { return true }
func (p *errorPathsPort) PATHList() []string                       { return nil }

// errorPromptPort returns an error from Confirm.
type errorPromptPort struct{}

func (p *errorPromptPort) Confirm(_ string, _ bool) (bool, error) { return false, os.ErrClosed }
func (p *errorPromptPort) IsInteractive() bool                     { return true }
