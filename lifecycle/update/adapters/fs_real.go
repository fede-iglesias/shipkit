package adapters

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RealFsAdapter implements ports.FsPort using os primitives + tar/gzip stdlib.
// All os primitives are injectable for failure-path testing.
type RealFsAdapter struct {
	// NowFn returns the current time. Defaults to time.Now().UTC().
	NowFn func() time.Time
	// RandFn generates a random hex suffix for snapshot IDs. Defaults to defaultRandHex.
	RandFn func() (string, error)
	// MkdirAllFn creates directories. Defaults to os.MkdirAll.
	MkdirAllFn func(string, os.FileMode) error
	// CreateFn creates a new file. Defaults to os.Create.
	CreateFn func(string) (*os.File, error)
	// OpenFn opens a file for reading. Defaults to os.Open.
	OpenFn func(string) (*os.File, error)
	// RenameFn renames a file. Defaults to os.Rename.
	RenameFn func(string, string) error
	// RemoveFn removes a file. Defaults to os.Remove.
	RemoveFn func(string) error
	// StatFn stats a file. Defaults to os.Stat.
	StatFn func(string) (os.FileInfo, error)
	// ReadFileFn reads a file. Defaults to os.ReadFile.
	ReadFileFn func(string) ([]byte, error)
	// WriteFileFn writes a file. Defaults to os.WriteFile.
	WriteFileFn func(string, []byte, os.FileMode) error
	// CopyFn copies between writer and reader. Defaults to io.Copy.
	CopyFn func(io.Writer, io.Reader) (int64, error)
}

// randReader is the source of randomness for defaultRandHex. Tests may swap it.
var randReader = rand.Reader

// defaultRandHex generates 4 random bytes and returns them as a hex string.
// Uses randReader so the error branch is reachable in tests.
func defaultRandHex() (string, error) {
	b := make([]byte, 4)
	if _, err := io.ReadFull(randReader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewRealFs returns a RealFsAdapter with all fields wired to real os/stdlib functions.
func NewRealFs() *RealFsAdapter {
	return &RealFsAdapter{
		NowFn:       func() time.Time { return time.Now().UTC() },
		RandFn:      defaultRandHex,
		MkdirAllFn:  os.MkdirAll,
		CreateFn:    os.Create,
		OpenFn:      os.Open,
		RenameFn:    os.Rename,
		RemoveFn:    os.Remove,
		StatFn:      os.Stat,
		ReadFileFn:  os.ReadFile,
		WriteFileFn: os.WriteFile,
		CopyFn:      io.Copy,
	}
}

// Snapshot copies src into snapshotDir/<timestamp>-<rand>/<basename>.
// Returns the snapshot ID, which is the full path of the snapshot subdirectory.
// Passing this ID to Restore is sufficient to reconstruct the binary location.
func (a *RealFsAdapter) Snapshot(ctx context.Context, src, snapshotDir string) (string, error) {
	randSuffix, err := a.RandFn()
	if err != nil {
		return "", fmt.Errorf("snapshot: generate rand suffix: %w", err)
	}
	ts := a.NowFn().Format("20060102-150405")
	subDirName := fmt.Sprintf("%s-%s", ts, randSuffix)

	snapSubdir := filepath.Join(snapshotDir, subDirName)
	if err := a.MkdirAllFn(snapSubdir, 0o755); err != nil {
		return "", fmt.Errorf("snapshot: mkdir %s: %w", snapSubdir, err)
	}

	srcFile, err := a.OpenFn(src)
	if err != nil {
		return "", fmt.Errorf("snapshot: open src %s: %w", src, err)
	}
	defer srcFile.Close()

	dstPath := filepath.Join(snapSubdir, filepath.Base(src))
	dstFile, err := a.CreateFn(dstPath)
	if err != nil {
		return "", fmt.Errorf("snapshot: create dst %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := a.CopyFn(dstFile, srcFile); err != nil {
		return "", fmt.Errorf("snapshot: copy %s -> %s: %w", src, dstPath, err)
	}

	// Return the full path as the snapshot ID so Restore can locate files without
	// needing snapshotDir passed separately.
	return snapSubdir, nil
}

// Restore copies snapshotDir/<snapshotID>/<basename of dst> back to dst atomically
// (write to temp file in same directory, then rename).
func (a *RealFsAdapter) Restore(ctx context.Context, snapshotID, dst string) error {
	// snapshotID is the full path of the snapshot subdirectory returned by Snapshot.
	// The convention is that snapshotID encodes snapshotDir so Restore can locate
	// the file without needing snapshotDir passed separately.
	snapSubdir := snapshotID // treat snapshotID as full path to snapshot subdir

	baseName := filepath.Base(dst)
	snapFile := filepath.Join(snapSubdir, baseName)

	srcFile, err := a.OpenFn(snapFile)
	if err != nil {
		return fmt.Errorf("restore: open snapshot %s: %w", snapFile, err)
	}
	defer srcFile.Close()

	// Write to temp file in the same directory as dst, then rename atomically.
	dstDir := filepath.Dir(dst)
	if err := a.MkdirAllFn(dstDir, 0o755); err != nil {
		return fmt.Errorf("restore: mkdir %s: %w", dstDir, err)
	}

	randSuffix, err := a.RandFn()
	if err != nil {
		return fmt.Errorf("restore: generate rand: %w", err)
	}
	tmpPath := filepath.Join(dstDir, ".update.restore."+randSuffix)

	tmpFile, err := a.CreateFn(tmpPath)
	if err != nil {
		return fmt.Errorf("restore: create tmp %s: %w", tmpPath, err)
	}

	if _, err := a.CopyFn(tmpFile, srcFile); err != nil {
		tmpFile.Close()
		a.RemoveFn(tmpPath) //nolint:errcheck
		return fmt.Errorf("restore: copy: %w", err)
	}
	tmpFile.Close()

	if err := a.RenameFn(tmpPath, dst); err != nil {
		a.RemoveFn(tmpPath) //nolint:errcheck
		return fmt.Errorf("restore: rename %s -> %s: %w", tmpPath, dst, err)
	}

	return nil
}

// AtomicReplace renames newFile to target. Both must be on the same filesystem.
// On Unix, os.Rename atomically replaces target; running processes retain the old inode.
func (a *RealFsAdapter) AtomicReplace(ctx context.Context, target, newFile string) error {
	if err := a.RenameFn(newFile, target); err != nil {
		return fmt.Errorf("atomic replace %s -> %s: %w", newFile, target, err)
	}
	return nil
}

// ExtractTarGz extracts the tar.gz archive at archive into destDir.
// Context cancellation is checked between entries.
func (a *RealFsAdapter) ExtractTarGz(ctx context.Context, archive, destDir string) error {
	f, err := a.OpenFn(archive)
	if err != nil {
		return fmt.Errorf("extract: open archive %s: %w", archive, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("extract: gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		// Check context between entries.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("extract: tar next: %w", err)
		}

		target := filepath.Join(destDir, filepath.Clean(hdr.Name))

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := a.MkdirAllFn(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("extract: mkdir %s: %w", target, err)
			}

		case tar.TypeReg, 0: // 0 = TypeReg fallback for some archives
			// Ensure parent directory exists.
			parentDir := filepath.Dir(target)
			if err := a.MkdirAllFn(parentDir, 0o755); err != nil {
				return fmt.Errorf("extract: mkdir parent %s: %w", parentDir, err)
			}

			// Read content then write via WriteFileFn (injectable for error testing).
			content, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("extract: read entry %s: %w", hdr.Name, err)
			}
			if err := a.WriteFileFn(target, content, os.FileMode(hdr.Mode)|0o644); err != nil {
				return fmt.Errorf("extract: write %s: %w", target, err)
			}
		}
	}

	return nil
}
