package clean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
)

// ErrNoScope is returned by Run when no scope flag (--snapshots, --tmp,
// --cache, --logs, --all) is set. The cobra layer converts this into a help
// print + exit 1, preventing accidental mass deletion.
var ErrNoScope = errors.New("clean: at least one scope flag (--snapshots, --tmp, --cache, --logs, --all) is required")

// Testing seams. The default values point at the real syscalls; tests replace
// them via the *_internal_test.go file to simulate file-disappearance races
// between ReadDir and the subsequent metadata calls.
var (
	osLstat    = os.Lstat
	osStat     = os.Stat
	osReadlink = os.Readlink
)

// Options controls what clean targets and how it behaves.
type Options struct {
	// Snapshots enables cleaning of snapshot directories under DataDir/snapshots/.
	// Filtered by OlderThan and --keep N.
	Snapshots bool

	// Tmp enables cleaning of work directories under DataDir/tmp/.
	Tmp bool

	// Cache enables cleaning of the cache directory (DataDir/cache/).
	Cache bool

	// Logs enables cleaning of log files under DataDir/logs/.
	Logs bool

	// All is equivalent to setting Snapshots + Tmp + Cache + Logs simultaneously.
	All bool

	// OlderThan filters snapshot entries to those whose modification time
	// predates now by at least this duration. Defaults to 720h (30 days).
	// Ignored for Tmp, Cache, Logs (those are always removed when the flag is set).
	OlderThan time.Duration

	// Keep is the minimum number of snapshots to retain regardless of OlderThan.
	// When Keep > 0, the newest Keep entries are always preserved even if their
	// age exceeds OlderThan. Defaults to 0 (no forced retention).
	Keep int

	// Yes skips the confirmation prompt. Required for non-interactive use.
	Yes bool

	// Print activates dry-run mode: candidates are computed and returned in
	// Result.Items but nothing is deleted.
	Print bool
}

// SnapshotEntry describes a snapshot directory eligible for cleaning.
type SnapshotEntry struct {
	// Path is the absolute path to the snapshot directory.
	Path string
	// ModTime is the modification time of the snapshot directory.
	ModTime time.Time
	// Size is the total byte size of the snapshot (directory tree).
	Size int64
	// SymlinkDest is the resolved destination of a symlink, if the entry's path
	// is a symlink. Empty string means not a symlink.
	SymlinkDest string
}

// TmpEntry describes a work directory under DataDir/tmp/.
type TmpEntry struct {
	// Path is the absolute path to the work directory or file.
	Path string
	// Size is the total byte size.
	Size int64
}

// CacheEntry describes an entry under the cache directory.
type CacheEntry struct {
	// Path is the absolute path to the cache entry.
	Path string
	// Size is the total byte size.
	Size int64
}

// LogEntry describes a log file under DataDir/logs/.
type LogEntry struct {
	// Path is the absolute path to the log file.
	Path string
	// Size is the total byte size.
	Size int64
}

// CleanedItem records a single entry that was cleaned (or would be cleaned in
// dry-run mode).
type CleanedItem struct {
	// Path is the absolute path that was removed.
	Path string

	// Category is one of "snapshot", "tmp", "cache", "log".
	Category string

	// Size is the byte size reclaimed by removing this entry.
	Size int64

	// AgeDays is the age of the entry in whole days at the time of the clean run.
	// Only meaningful for snapshot entries.
	AgeDays int
}

// Result reports what Run did (or what it would do in dry-run mode).
type Result struct {
	// Reclaimed is the total number of bytes freed.
	Reclaimed int64

	// Items lists each entry that was removed (or would be removed in dry-run).
	Items []CleanedItem
}

// RecoveryManifest is the minimal shape of .shipkit.recovery-manifest.json
// that clean needs to read. Only SnapshotPath is used for protection checks.
type RecoveryManifest struct {
	// SnapshotPath is the absolute path of the snapshot that was active when
	// the recovery manifest was written. Clean refuses to delete this path.
	SnapshotPath string `json:"snapshot_path"`
}

// Deps holds the injected ports and functions that Run needs. All fields are
// required; nil values cause nil-pointer panics at runtime. Use the Default*
// functions to wire production implementations.
type Deps struct {
	// AppName is the application name used to resolve XDG directories. Required.
	AppName string

	// FS provides filesystem operations (RemoveDir). Required.
	FS ports.FsPort

	// Paths provides XDG path resolution (DataDir, CacheDir). Required.
	Paths ports.PathsPort

	// Clock provides the current time for age calculations. Required.
	Clock ports.ClockPort

	// Prompt provides interactive confirmation. Required.
	Prompt ports.PromptPort

	// ListSnapshotsFunc enumerates snapshot directories under snapshotDir.
	// Inject DefaultListSnapshots in production; inject a stub in tests.
	ListSnapshotsFunc func(snapshotDir string) ([]SnapshotEntry, error)

	// ListTmpFunc enumerates work entries under tmpDir.
	// Inject DefaultListTmp in production; inject a stub in tests.
	ListTmpFunc func(tmpDir string) ([]TmpEntry, error)

	// ListCacheFunc enumerates cache entries under cacheDir.
	// Inject DefaultListCache in production; inject a stub in tests.
	ListCacheFunc func(cacheDir string) ([]CacheEntry, error)

	// ListLogsFunc enumerates log files under logsDir.
	// Inject DefaultListLogs in production; inject a stub in tests.
	ListLogsFunc func(logsDir string) ([]LogEntry, error)

	// ReadManifestFunc reads the recovery manifest at manifestPath.
	// Returns (nil, nil) when the manifest does not exist.
	// Inject DefaultReadManifest in production; inject a stub in tests.
	ReadManifestFunc func(manifestPath string) (*RecoveryManifest, error)
}

// Run executes the clean state machine:
//
//  1. EnumerateCandidates: list eligible entries for each enabled scope.
//  2. Confirm: show the total to the user and ask for confirmation (skipped
//     with --yes or --print).
//  3. RemoveLoop: delete each candidate and accumulate Reclaimed.
//  4. ReportSummary: return Result.
//
// Returns ErrNoScope when no scope flag is set. Returns nil error on empty
// candidate list (idempotent: nothing to clean is not an error).
func Run(ctx context.Context, deps Deps, opts Options) (Result, error) {
	// Resolve effective scope flags: --all expands to all sub-flags.
	if opts.All {
		opts.Snapshots = true
		opts.Tmp = true
		opts.Cache = true
		opts.Logs = true
	}

	// Safety gate: refuse with no scope selected.
	if !opts.Snapshots && !opts.Tmp && !opts.Cache && !opts.Logs {
		return Result{}, ErrNoScope
	}

	// Apply default retention window.
	if opts.OlderThan == 0 {
		opts.OlderThan = 30 * 24 * time.Hour
	}

	// Resolve data dir.
	dataDir, err := deps.Paths.DataDir(deps.AppName)
	if err != nil {
		return Result{}, fmt.Errorf("clean: resolve data dir: %w", err)
	}

	// Step 1: enumerate candidates.
	candidates, err := enumerateCandidates(ctx, deps, opts, dataDir)
	if err != nil {
		return Result{}, err
	}

	if len(candidates) == 0 {
		return Result{}, nil
	}

	// Step 2: dry-run returns plan without touching anything.
	if opts.Print {
		return Result{Items: candidates}, nil
	}

	// Step 3: confirmation gate.
	if !opts.Yes {
		totalBytes := int64(0)
		for _, c := range candidates {
			totalBytes += c.Size
		}
		question := fmt.Sprintf(
			"Remove %d item(s) and reclaim %s? Proceed?",
			len(candidates), formatBytes(totalBytes),
		)
		confirmed, promptErr := deps.Prompt.Confirm(question, false)
		if promptErr != nil {
			return Result{}, fmt.Errorf("clean: prompt: %w", promptErr)
		}
		if !confirmed {
			return Result{}, nil
		}
	}

	// Step 4: remove loop.
	result := Result{}
	for _, item := range candidates {
		if rmErr := deps.FS.RemoveDir(ctx, item.Path); rmErr == nil {
			result.Reclaimed += item.Size
			result.Items = append(result.Items, item)
		}
		// Removal errors are silently skipped per Opus A Section 5.5 (per-rmdir
		// atomicity; partial failures are not fatal).
	}
	return result, nil
}

// enumerateCandidates builds the list of CleanedItems eligible for removal
// based on the enabled scope flags and protection rules.
func enumerateCandidates(ctx context.Context, deps Deps, opts Options, dataDir string) ([]CleanedItem, error) {
	var candidates []CleanedItem
	now := deps.Clock.NowUTC()

	if opts.Snapshots {
		snapshotDir := filepath.Join(dataDir, "snapshots")
		items, err := collectSnapshots(deps, opts, snapshotDir, dataDir, now)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, items...)
	}

	if opts.Tmp && deps.ListTmpFunc != nil {
		tmpDir := filepath.Join(dataDir, "tmp")
		entries, err := deps.ListTmpFunc(tmpDir)
		if err == nil {
			for _, e := range entries {
				candidates = append(candidates, CleanedItem{
					Path:     e.Path,
					Category: "tmp",
					Size:     e.Size,
				})
			}
		}
	}

	if opts.Cache && deps.ListCacheFunc != nil {
		cacheDir := filepath.Join(dataDir, "cache")
		entries, err := deps.ListCacheFunc(cacheDir)
		if err == nil {
			for _, e := range entries {
				candidates = append(candidates, CleanedItem{
					Path:     e.Path,
					Category: "cache",
					Size:     e.Size,
				})
			}
		}
	}

	if opts.Logs && deps.ListLogsFunc != nil {
		logsDir := filepath.Join(dataDir, "logs")
		entries, err := deps.ListLogsFunc(logsDir)
		if err == nil {
			for _, e := range entries {
				candidates = append(candidates, CleanedItem{
					Path:     e.Path,
					Category: "log",
					Size:     e.Size,
				})
			}
		}
	}

	return candidates, nil
}

// collectSnapshots enumerates snapshot candidates with age filtering, --keep N
// protection, recovery manifest protection, and symlink escape prevention.
func collectSnapshots(deps Deps, opts Options, snapshotDir, dataDir string, now time.Time) ([]CleanedItem, error) {
	if deps.ListSnapshotsFunc == nil {
		return nil, nil
	}
	entries, err := deps.ListSnapshotsFunc(snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("clean: list snapshots: %w", err)
	}

	// Read recovery manifest to identify protected snapshots.
	manifestPath := filepath.Join(dataDir, ".shipkit.recovery-manifest.json")
	var protectedPath string
	if deps.ReadManifestFunc != nil {
		manifest, manifestErr := deps.ReadManifestFunc(manifestPath)
		if manifestErr == nil && manifest != nil {
			protectedPath = manifest.SnapshotPath
		}
	}

	// Sort entries newest-first so that --keep N is easy to apply.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ModTime.After(entries[j].ModTime)
	})

	// Determine which indices are eligible after applying --keep N.
	// The first Keep entries (newest) are always preserved.
	keepCount := opts.Keep

	var candidates []CleanedItem
	for i, entry := range entries {
		// --keep N: preserve the newest Keep entries unconditionally.
		if keepCount > 0 && i < keepCount {
			continue
		}

		// Age filter: only remove entries older than OlderThan.
		age := now.Sub(entry.ModTime)
		if age < opts.OlderThan {
			continue
		}

		// Recovery manifest protection: refuse to delete the active recovery snapshot.
		if protectedPath != "" && entry.Path == protectedPath {
			continue
		}

		// Symlink escape prevention: refuse if the resolved target escapes DataDir.
		if entry.SymlinkDest != "" && !strings.HasPrefix(entry.SymlinkDest, dataDir) {
			continue
		}

		ageDays := int(age.Hours() / 24)
		candidates = append(candidates, CleanedItem{
			Path:     entry.Path,
			Category: "snapshot",
			Size:     entry.Size,
			AgeDays:  ageDays,
		})
	}
	return candidates, nil
}

// formatBytes returns a human-readable byte size string (e.g. "3.5 MiB").
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// DefaultListSnapshots is the production implementation of ListSnapshotsFunc.
// It reads snapshotDir entries and returns one SnapshotEntry per subdirectory.
// Symlinks are resolved via os.Readlink; SymlinkDest is empty for non-symlinks.
// Size is computed as the sum of all regular files within the directory tree.
func DefaultListSnapshots(snapshotDir string) ([]SnapshotEntry, error) {
	infos, err := os.ReadDir(snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []SnapshotEntry
	for _, info := range infos {
		p := filepath.Join(snapshotDir, info.Name())
		// Use Lstat to detect symlinks without following them, then Stat for the
		// full metadata (ModTime, Mode) of the actual target.
		lfi, lstatErr := osLstat(p)
		if lstatErr != nil {
			continue
		}
		symlinkDest := ""
		if lfi.Mode()&os.ModeSymlink != 0 {
			dest, readErr := osReadlink(p)
			if readErr == nil {
				symlinkDest, _ = filepath.Abs(dest)
			}
		}
		// Use Stat (follows symlink) to get ModTime from the target.
		fi, statErr := osStat(p)
		if statErr != nil {
			continue
		}
		size := dirSize(p)
		entries = append(entries, SnapshotEntry{
			Path:        p,
			ModTime:     fi.ModTime(),
			Size:        size,
			SymlinkDest: symlinkDest,
		})
	}
	return entries, nil
}

// DefaultListTmp is the production implementation of ListTmpFunc.
// It reads tmpDir entries and returns one TmpEntry per subdirectory or file.
func DefaultListTmp(tmpDir string) ([]TmpEntry, error) {
	infos, err := os.ReadDir(tmpDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []TmpEntry
	for _, info := range infos {
		p := filepath.Join(tmpDir, info.Name())
		entries = append(entries, TmpEntry{Path: p, Size: dirSize(p)})
	}
	return entries, nil
}

// DefaultListCache is the production implementation of ListCacheFunc.
// It reads cacheDir entries and returns one CacheEntry per item.
func DefaultListCache(cacheDir string) ([]CacheEntry, error) {
	infos, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []CacheEntry
	for _, info := range infos {
		p := filepath.Join(cacheDir, info.Name())
		entries = append(entries, CacheEntry{Path: p, Size: dirSize(p)})
	}
	return entries, nil
}

// DefaultListLogs is the production implementation of ListLogsFunc.
// It reads logsDir and returns one LogEntry per .log file.
func DefaultListLogs(logsDir string) ([]LogEntry, error) {
	infos, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []LogEntry
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		p := filepath.Join(logsDir, info.Name())
		// Use osLstat to obtain the file size. This shares the osLstat seam with
		// DefaultListSnapshots, making the race-condition skip branch testable.
		fi, err := osLstat(p)
		if err != nil {
			// File disappeared between ReadDir and Lstat (rare race condition).
			continue
		}
		entries = append(entries, LogEntry{Path: p, Size: fi.Size()})
	}
	return entries, nil
}

// DefaultReadManifest is the production implementation of ReadManifestFunc.
// It reads and parses .shipkit.recovery-manifest.json at manifestPath.
// Returns (nil, nil) when the file does not exist.
func DefaultReadManifest(manifestPath string) (*RecoveryManifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m RecoveryManifest
	if jsonErr := json.Unmarshal(data, &m); jsonErr != nil {
		return nil, jsonErr
	}
	return &m, nil
}

// dirSize returns the total byte size of all regular files within dir.
// Returns 0 if dir does not exist or cannot be read.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
