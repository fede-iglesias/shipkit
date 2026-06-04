package ports_test

import (
	"context"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// mockFs is a compile-time proof that FsPort is implementable.
type mockFs struct {
	snapshotFunc      func(ctx context.Context, src, snapshotDir string) (string, error)
	restoreFunc       func(ctx context.Context, snapshotID, dst string) error
	atomicReplaceFunc func(ctx context.Context, target, newFile string) error
	extractTarGzFunc  func(ctx context.Context, archive, destDir string) error
}

func (m *mockFs) Snapshot(ctx context.Context, src, snapshotDir string) (string, error) {
	return m.snapshotFunc(ctx, src, snapshotDir)
}

func (m *mockFs) Restore(ctx context.Context, snapshotID, dst string) error {
	return m.restoreFunc(ctx, snapshotID, dst)
}

func (m *mockFs) AtomicReplace(ctx context.Context, target, newFile string) error {
	return m.atomicReplaceFunc(ctx, target, newFile)
}

func (m *mockFs) ExtractTarGz(ctx context.Context, archive, destDir string) error {
	return m.extractTarGzFunc(ctx, archive, destDir)
}

// TestFsPort_InterfaceCompliance asserts at compile time that *mockFs
// satisfies FsPort.
var _ ports.FsPort = (*mockFs)(nil)

// TestSnapshot_SignatureType verifies that a mock FsPort implementation can
// return a string snapshot ID, exercising the method signature.
func TestSnapshot_SignatureType(t *testing.T) {
	t.Parallel()

	const wantID = "snap-2026-01-02T03:04:05Z"
	m := &mockFs{
		snapshotFunc: func(_ context.Context, src, dir string) (string, error) {
			if src == "" {
				t.Error("Snapshot: src must not be empty")
			}
			if dir == "" {
				t.Error("Snapshot: snapshotDir must not be empty")
			}
			return wantID, nil
		},
	}

	id, err := m.Snapshot(context.Background(), "/usr/local/bin/myapp", "/var/lib/myapp/snapshots")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != wantID {
		t.Errorf("Snapshot ID: got %q, want %q", id, wantID)
	}
}

// TestRestore_SignatureType verifies that a mock FsPort Restore call passes
// snapshotID and dst through without error.
func TestRestore_SignatureType(t *testing.T) {
	t.Parallel()

	var gotID, gotDst string
	m := &mockFs{
		restoreFunc: func(_ context.Context, snapshotID, dst string) error {
			gotID = snapshotID
			gotDst = dst
			return nil
		},
	}

	err := m.Restore(context.Background(), "snap-abc123", "/usr/local/bin/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != "snap-abc123" {
		t.Errorf("snapshotID: got %q, want %q", gotID, "snap-abc123")
	}
	if gotDst != "/usr/local/bin/myapp" {
		t.Errorf("dst: got %q, want %q", gotDst, "/usr/local/bin/myapp")
	}
}

// TestAtomicReplace_SignatureType verifies that AtomicReplace passes target
// and newFile through the interface without error.
func TestAtomicReplace_SignatureType(t *testing.T) {
	t.Parallel()

	var gotTarget, gotNew string
	m := &mockFs{
		atomicReplaceFunc: func(_ context.Context, target, newFile string) error {
			gotTarget = target
			gotNew = newFile
			return nil
		},
	}

	err := m.AtomicReplace(context.Background(), "/usr/local/bin/myapp", "/tmp/myapp.new.abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTarget != "/usr/local/bin/myapp" {
		t.Errorf("target: got %q", gotTarget)
	}
	if gotNew != "/tmp/myapp.new.abc" {
		t.Errorf("newFile: got %q", gotNew)
	}
}

// TestExtractTarGz_SignatureType verifies that ExtractTarGz passes archive
// and destDir through the interface without error.
func TestExtractTarGz_SignatureType(t *testing.T) {
	t.Parallel()

	var gotArchive, gotDest string
	m := &mockFs{
		extractTarGzFunc: func(_ context.Context, archive, destDir string) error {
			gotArchive = archive
			gotDest = destDir
			return nil
		},
	}

	err := m.ExtractTarGz(context.Background(), "/tmp/myapp.tar.gz", "/tmp/myapp-extract/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotArchive != "/tmp/myapp.tar.gz" {
		t.Errorf("archive: got %q", gotArchive)
	}
	if gotDest != "/tmp/myapp-extract/" {
		t.Errorf("destDir: got %q", gotDest)
	}
}
