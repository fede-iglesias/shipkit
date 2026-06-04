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

func TestMockFsPort_MkdirAll_DispatchesToFunc(t *testing.T) {
	var gotPath string
	var gotPerm fs.FileMode
	sentinel := errors.New("mkdirall sentinel")
	m := ports.NewMockFsPort()
	m.MkdirAllFunc = func(_ context.Context, path string, perm fs.FileMode) error {
		gotPath = path
		gotPerm = perm
		return sentinel
	}
	err := m.MkdirAll(context.Background(), "/x", 0o755)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel, got %v", err)
	}
	if gotPath != "/x" {
		t.Errorf("want path /x, got %q", gotPath)
	}
	if gotPerm != 0o755 {
		t.Errorf("want perm 0755, got %o", gotPerm)
	}
}

func TestMockFsPort_ReadFile_DispatchesToFunc(t *testing.T) {
	want := []byte("hello")
	m := ports.NewMockFsPort()
	m.ReadFileFunc = func(_ context.Context, _ string) ([]byte, error) {
		return want, nil
	}
	got, err := m.ReadFile(context.Background(), "/x/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestMockFsPort_AtomicWrite_DispatchesToFunc(t *testing.T) {
	sentinel := errors.New("disk-full")
	var gotPath string
	var gotData []byte
	var gotPerm fs.FileMode
	m := ports.NewMockFsPort()
	m.AtomicWriteFunc = func(_ context.Context, path string, data []byte, perm fs.FileMode) error {
		gotPath = path
		gotData = data
		gotPerm = perm
		return sentinel
	}
	err := m.AtomicWrite(context.Background(), "/y/out.txt", []byte("payload"), 0o600)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel, got %v", err)
	}
	if gotPath != "/y/out.txt" {
		t.Errorf("want path /y/out.txt, got %q", gotPath)
	}
	if string(gotData) != "payload" {
		t.Errorf("want data payload, got %q", gotData)
	}
	if gotPerm != 0o600 {
		t.Errorf("want perm 0600, got %o", gotPerm)
	}
}

func TestNewMockFsPort_DefaultFuncs_SafeNoops(t *testing.T) {
	m := ports.NewMockFsPort()
	ctx := context.Background()

	if err := m.MkdirAll(ctx, "/x", 0o755); err != nil {
		t.Errorf("MkdirAll default: unexpected error %v", err)
	}

	data, err := m.ReadFile(ctx, "/x/file.txt")
	if err != nil {
		t.Errorf("ReadFile default: unexpected error %v", err)
	}
	if data != nil {
		t.Errorf("ReadFile default: want nil data, got %v", data)
	}

	if err := m.AtomicWrite(ctx, "/x/out.txt", []byte("x"), 0o644); err != nil {
		t.Errorf("AtomicWrite default: unexpected error %v", err)
	}
}
