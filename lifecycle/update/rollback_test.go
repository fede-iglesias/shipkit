package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// resetFnVars restores package-level injectable function variables to their
// os defaults. Call via t.Cleanup to avoid cross-test pollution.
func resetFnVars(t *testing.T) {
	t.Helper()
	orig := struct {
		writeFile   func(string, []byte, os.FileMode) error
		rename      func(string, string) error
		mkdirAll    func(string, os.FileMode) error
		readFile    func(string) ([]byte, error)
		remove      func(string) error
		jsonMarshal func(any) ([]byte, error)
	}{
		writeFile:   os.WriteFile,
		rename:      os.Rename,
		mkdirAll:    os.MkdirAll,
		readFile:    os.ReadFile,
		remove:      os.Remove,
		jsonMarshal: jsonMarshalFn,
	}
	t.Cleanup(func() {
		writeFileFn = orig.writeFile
		renameFn = orig.rename
		mkdirAllFn = orig.mkdirAll
		readFileFn = orig.readFile
		removeFn = orig.remove
		jsonMarshalFn = orig.jsonMarshal
	})
}

// manifestsEqual compares two RecoveryManifest values by JSON round-trip.
func manifestsEqual(a, b *RecoveryManifest) bool {
	aB, _ := json.Marshal(a)
	bB, _ := json.Marshal(b)
	return string(aB) == string(bB)
}

// ---- PersistRecoveryManifest ----

func TestPersistRecoveryManifest_HappyPath(t *testing.T) {
	dir := t.TempDir()
	m := &RecoveryManifest{
		Cause: "disk full",
		Steps: []RecoveryStep{
			{Action: "restore-binary", Detail: "/usr/local/bin/myapp"},
		},
	}

	if err := PersistRecoveryManifest(dir, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File must exist at the canonical path.
	dest := filepath.Join(dir, RecoveryManifestFilename)
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("file not found after persist: %v", err)
	}

	// Content must unmarshal back to the original manifest.
	var got RecoveryManifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !manifestsEqual(m, &got) {
		t.Errorf("manifest mismatch:\n  want %+v\n  got  %+v", m, got)
	}
}

func TestPersistRecoveryManifest_AtomicWrite(t *testing.T) {
	resetFnVars(t)
	dir := t.TempDir()
	m := &RecoveryManifest{Cause: "test"}

	var writtenPath, renamedSrc, renamedDst string

	writeFileFn = func(name string, data []byte, perm os.FileMode) error {
		writtenPath = name
		return os.WriteFile(name, data, perm)
	}
	renameFn = func(src, dst string) error {
		renamedSrc = src
		renamedDst = dst
		return os.Rename(src, dst)
	}

	if err := PersistRecoveryManifest(dir, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dest := filepath.Join(dir, RecoveryManifestFilename)

	// The temp path written must NOT equal the final destination.
	if writtenPath == dest {
		t.Errorf("expected atomic temp write, got direct write to %s", dest)
	}
	// The rename must land at the final destination.
	if renamedDst != dest {
		t.Errorf("rename dst = %s; want %s", renamedDst, dest)
	}
	// The rename source must match the path that was written.
	if renamedSrc != writtenPath {
		t.Errorf("rename src = %s; want %s (the temp path)", renamedSrc, writtenPath)
	}
}

func TestPersistRecoveryManifest_MkdirError(t *testing.T) {
	resetFnVars(t)
	sentinel := errors.New("mkdir failed")
	mkdirAllFn = func(string, os.FileMode) error { return sentinel }

	err := PersistRecoveryManifest("/some/dir", &RecoveryManifest{Cause: "x"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

func TestPersistRecoveryManifest_MarshalError(t *testing.T) {
	resetFnVars(t)
	sentinel := errors.New("marshal failed")
	jsonMarshalFn = func(any) ([]byte, error) { return nil, sentinel }

	err := PersistRecoveryManifest(t.TempDir(), &RecoveryManifest{Cause: "x"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

func TestPersistRecoveryManifest_WriteError(t *testing.T) {
	resetFnVars(t)
	dir := t.TempDir()
	sentinel := errors.New("write failed")
	writeFileFn = func(string, []byte, os.FileMode) error { return sentinel }

	err := PersistRecoveryManifest(dir, &RecoveryManifest{Cause: "x"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

func TestPersistRecoveryManifest_RenameError(t *testing.T) {
	resetFnVars(t)
	dir := t.TempDir()
	sentinel := errors.New("rename failed")
	// Let writeFile succeed (use real), only fail rename.
	writeFileFn = os.WriteFile
	renameFn = func(string, string) error { return sentinel }

	err := PersistRecoveryManifest(dir, &RecoveryManifest{Cause: "x"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

// ---- LoadRecoveryManifest ----

func TestLoadRecoveryManifest_HappyPath(t *testing.T) {
	dir := t.TempDir()
	want := &RecoveryManifest{
		Cause: "network error",
		Steps: []RecoveryStep{
			{Action: "retry-download", Detail: "https://example.com/myapp.tar.gz"},
			{Action: "manual-binary-restore", Detail: "snapshot=/tmp/snap"},
		},
	}

	// Write via the persist helper so both functions are tested together.
	if err := PersistRecoveryManifest(dir, want); err != nil {
		t.Fatalf("persist: %v", err)
	}

	got, err := LoadRecoveryManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !manifestsEqual(want, got) {
		t.Errorf("manifest mismatch:\n  want %+v\n  got  %+v", want, got)
	}
}

func TestLoadRecoveryManifest_NotFoundReturnsErrIsNotExist(t *testing.T) {
	dir := t.TempDir() // empty; no manifest written

	_, err := LoadRecoveryManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.IsNotExist error, got %v", err)
	}
}

func TestLoadRecoveryManifest_MalformedJSON(t *testing.T) {
	resetFnVars(t)
	readFileFn = func(string) ([]byte, error) { return []byte("{not-json}"), nil }

	_, err := LoadRecoveryManifest("/irrelevant")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// ---- ClearRecoveryManifest ----

func TestClearRecoveryManifest_HappyPath(t *testing.T) {
	dir := t.TempDir()

	// Write a manifest so there is something to clear.
	if err := PersistRecoveryManifest(dir, &RecoveryManifest{Cause: "x"}); err != nil {
		t.Fatalf("persist: %v", err)
	}

	if err := ClearRecoveryManifest(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File must be gone.
	dest := filepath.Join(dir, RecoveryManifestFilename)
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("expected file to be removed, stat returned: %v", statErr)
	}
}

func TestClearRecoveryManifest_NotFoundIsNoOp(t *testing.T) {
	dir := t.TempDir() // empty; no manifest

	// Must not return an error when the file does not exist.
	if err := ClearRecoveryManifest(dir); err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
}

func TestClearRecoveryManifest_RemoveError(t *testing.T) {
	resetFnVars(t)
	sentinel := errors.New("remove failed")
	// Inject a remove that fails with something other than ErrNotExist.
	removeFn = func(string) error { return sentinel }

	err := ClearRecoveryManifest("/irrelevant")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel error, got %v", err)
	}
}

// ---- AppendRecoveryStep ----

func TestAppendRecoveryStep_AppendsInOrder(t *testing.T) {
	m := &RecoveryManifest{Cause: "test"}

	AppendRecoveryStep(m, "step-one", "detail-one")
	AppendRecoveryStep(m, "step-two", "detail-two")

	if len(m.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(m.Steps))
	}
	if m.Steps[0].Action != "step-one" || m.Steps[0].Detail != "detail-one" {
		t.Errorf("step[0] = %+v; want {step-one detail-one}", m.Steps[0])
	}
	if m.Steps[1].Action != "step-two" || m.Steps[1].Detail != "detail-two" {
		t.Errorf("step[1] = %+v; want {step-two detail-two}", m.Steps[1])
	}
	// Ensure Cause field is not touched.
	if m.Cause != "test" {
		t.Errorf("Cause modified unexpectedly: %q", m.Cause)
	}
}
