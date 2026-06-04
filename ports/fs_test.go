package ports_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockFsPort satisfies FsPort.
var _ ports.FsPort = (*ports.MockFsPort)(nil)

func TestMockFsPort_Snapshot_default(t *testing.T) {
	m := ports.NewMockFsPort()
	id, err := m.Snapshot(context.Background(), "/bin/app", "/data/snapshots")
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		t.Errorf("expected empty id by default, got %q", id)
	}
	if len(m.SnapshotCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.SnapshotCalls))
	}
}

func TestMockFsPort_Snapshot_func(t *testing.T) {
	m := ports.NewMockFsPort()
	m.SnapshotFunc = func(_ context.Context, _, _ string) (string, error) { return "snap-001", nil }
	id, err := m.Snapshot(context.Background(), "src", "dir")
	if err != nil {
		t.Fatal(err)
	}
	if id != "snap-001" {
		t.Errorf("want snap-001, got %q", id)
	}
}

func TestMockFsPort_Restore_default(t *testing.T) {
	m := ports.NewMockFsPort()
	if err := m.Restore(context.Background(), "snap-001", "/bin/app"); err != nil {
		t.Fatal(err)
	}
	if len(m.RestoreCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.RestoreCalls))
	}
}

func TestMockFsPort_AtomicReplace_default(t *testing.T) {
	m := ports.NewMockFsPort()
	if err := m.AtomicReplace(context.Background(), "/bin/app", "/tmp/app.new"); err != nil {
		t.Fatal(err)
	}
}

func TestMockFsPort_ExtractTarGz_default(t *testing.T) {
	m := ports.NewMockFsPort()
	if err := m.ExtractTarGz(context.Background(), "/tmp/app.tar.gz", "/tmp/extract"); err != nil {
		t.Fatal(err)
	}
}

func TestMockFsPort_CopyFile_default(t *testing.T) {
	m := ports.NewMockFsPort()
	if err := m.CopyFile(context.Background(), "/src/app", "/dst/app", 0o755); err != nil {
		t.Fatal(err)
	}
	if len(m.CopyFileCalls) != 1 {
		t.Fatalf("expected 1 CopyFile call, got %d", len(m.CopyFileCalls))
	}
	if m.CopyFileCalls[0].Src != "/src/app" {
		t.Errorf("expected src /src/app, got %q", m.CopyFileCalls[0].Src)
	}
	if m.CopyFileCalls[0].Mode != fs.FileMode(0o755) {
		t.Errorf("expected mode 0755, got %v", m.CopyFileCalls[0].Mode)
	}
}

func TestMockFsPort_CopyFile_error(t *testing.T) {
	m := ports.NewMockFsPort()
	sentinel := errors.New("permission denied")
	m.CopyFileFunc = func(_ context.Context, _, _ string, _ fs.FileMode) error { return sentinel }
	err := m.CopyFile(context.Background(), "src", "dst", 0o755)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockFsPort_RemoveDir_default(t *testing.T) {
	m := ports.NewMockFsPort()
	if err := m.RemoveDir(context.Background(), "/data/app"); err != nil {
		t.Fatal(err)
	}
	if len(m.RemoveDirCalls) != 1 {
		t.Fatalf("expected 1 RemoveDir call, got %d", len(m.RemoveDirCalls))
	}
	if m.RemoveDirCalls[0] != "/data/app" {
		t.Errorf("expected /data/app, got %q", m.RemoveDirCalls[0])
	}
}

func TestMockFsPort_RemoveDir_error(t *testing.T) {
	m := ports.NewMockFsPort()
	sentinel := errors.New("busy")
	m.RemoveDirFunc = func(_ context.Context, _ string) error { return sentinel }
	err := m.RemoveDir(context.Background(), "/data/app")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockFsPort_Restore_func(t *testing.T) {
	m := ports.NewMockFsPort()
	sentinel := errors.New("snapshot missing")
	m.RestoreFunc = func(_ context.Context, _, _ string) error { return sentinel }
	err := m.Restore(context.Background(), "snap-001", "/bin/app")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockFsPort_AtomicReplace_func(t *testing.T) {
	m := ports.NewMockFsPort()
	sentinel := errors.New("EXDEV")
	m.AtomicReplaceFunc = func(_ context.Context, _, _ string) error { return sentinel }
	err := m.AtomicReplace(context.Background(), "/bin/app", "/tmp/app.new")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockFsPort_ExtractTarGz_func(t *testing.T) {
	m := ports.NewMockFsPort()
	sentinel := errors.New("path traversal detected")
	m.ExtractTarGzFunc = func(_ context.Context, _, _ string) error { return sentinel }
	err := m.ExtractTarGz(context.Background(), "/tmp/evil.tar.gz", "/tmp/out")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}
