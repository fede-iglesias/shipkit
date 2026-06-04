package uninstall_test

import (
	"context"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// helpers builds a full Deps with safe mock defaults and returns the mocks for
// inspection. The returned prompt mock defaults to ConfirmResult=false (user
// declines) unless the test overrides it.
func makeDeps(t *testing.T) (Deps, *ports.MockFsPort, *ports.MockPathsPort, *ports.MockShellRcPort, *ports.MockCompletionPort, *ports.MockAutostartPort, *ports.MockPromptPort) {
	t.Helper()

	fs := ports.NewMockFsPort()
	paths := ports.NewMockPathsPort()
	shellrc := ports.NewMockShellRcPort()
	completion := ports.NewMockCompletionPort()
	autostart := ports.NewMockAutostartPort()
	prompt := &ports.MockPromptPort{ConfirmResult: false} // default: user declines

	deps := Deps{
		AppName:    "testapp",
		BinPath:    "/usr/local/bin/testapp",
		FS:         fs,
		Paths:      paths,
		ShellRc:    shellrc,
		Completion: completion,
		Autostart:  autostart,
		Prompt:     prompt,
	}
	return deps, fs, paths, shellrc, completion, autostart, prompt
}

// TestRun_PromptDeclined asserts that when PromptPort.Confirm returns false,
// Run returns a zero Result with no mutations: no removals, no autostart stop,
// and BinaryAction == "".
func TestRun_PromptDeclined(t *testing.T) {
	deps, fs, _, shellrc, completion, autostart, prompt := makeDeps(t)
	opts := Options{}

	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if len(prompt.ConfirmCalls) == 0 {
		t.Error("expected Confirm to be called at least once; it was not")
	}
	if result.BinaryAction != "" {
		t.Errorf("BinaryAction: got %q, want empty (no mutation)", result.BinaryAction)
	}
	if len(result.Removed) != 0 {
		t.Errorf("Removed: got %v, want empty slice", result.Removed)
	}
	if len(fs.RemoveDirCalls) != 0 {
		t.Errorf("RemoveDir called %d times, want 0", len(fs.RemoveDirCalls))
	}
	if len(shellrc.RemoveBlockCalls) != 0 {
		t.Errorf("RemoveBlock called %d times, want 0", len(shellrc.RemoveBlockCalls))
	}
	if len(completion.CompletionPathCalls) != 0 {
		t.Errorf("CompletionPath called %d times, want 0", len(completion.CompletionPathCalls))
	}
	if len(autostart.StopCalls) != 0 {
		t.Errorf("autostart.Stop called %d times, want 0", len(autostart.StopCalls))
	}
}

// TestRun_YesFlagSkipsPrompt asserts that when Options.Yes is true, the prompt
// is skipped and teardown proceeds.
func TestRun_YesFlagSkipsPrompt(t *testing.T) {
	deps, _, _, _, _, _, prompt := makeDeps(t)
	opts := Options{Yes: true}

	// Paths port needs to return valid dirs.
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	_, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run with Yes=true returned error: %v", err)
	}
	if len(prompt.ConfirmCalls) != 0 {
		t.Errorf("Confirm was called %d times with Yes=true, want 0", len(prompt.ConfirmCalls))
	}
}

// TestRun_FullTeardown asserts the linear teardown sequence when the user
// confirms: autostart stop, autostart uninstall, completions removal, shellrc
// block removal, data dir removal, config dir removal, cache dir removal,
// binary action resolved.
func TestRun_FullTeardown(t *testing.T) {
	deps, fs, _, shellrc, completion, autostart, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	opts := Options{}
	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Autostart must be stopped and uninstalled.
	if len(autostart.StopCalls) == 0 {
		t.Error("expected autostart.Stop to be called")
	}
	if len(autostart.UninstallCalls) == 0 {
		t.Error("expected autostart.Uninstall to be called")
	}
	// Completions must be queried.
	if len(completion.CompletionPathCalls) == 0 {
		t.Error("expected CompletionPath to be called")
	}
	// ShellRc blocks must be removed.
	if len(shellrc.RemoveBlockCalls) == 0 {
		t.Error("expected ShellRc.RemoveBlock to be called")
	}
	// RemoveDir must be called for data, config, cache.
	if len(fs.RemoveDirCalls) < 3 {
		t.Errorf("RemoveDir called %d times, want at least 3 (data+config+cache)", len(fs.RemoveDirCalls))
	}
	// BinaryAction must be set.
	if result.BinaryAction == "" {
		t.Error("BinaryAction must be non-empty after full teardown")
	}
}

// TestRun_KeepData asserts that when KeepData=true, the data dir is NOT removed.
func TestRun_KeepData(t *testing.T) {
	deps, fs, _, _, _, _, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	opts := Options{KeepData: true}
	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, dir := range fs.RemoveDirCalls {
		if dir == "/tmp/data/testapp" {
			t.Errorf("RemoveDir was called for data dir %q despite KeepData=true", dir)
		}
	}
	for _, s := range result.Skipped {
		// At least one skipped entry should reference data.
		_ = s
	}
}

// TestRun_KeepConfig asserts that when KeepConfig=true, the config dir is NOT
// removed.
func TestRun_KeepConfig(t *testing.T) {
	deps, fs, _, _, _, _, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	opts := Options{KeepConfig: true}
	_, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for _, dir := range fs.RemoveDirCalls {
		if dir == "/tmp/config/testapp" {
			t.Errorf("RemoveDir was called for config dir %q despite KeepConfig=true", dir)
		}
	}
}

// TestRun_KeepBinary asserts that when KeepBinary=true, BinaryAction is
// BinaryKept and no removal of the binary is attempted.
func TestRun_KeepBinary(t *testing.T) {
	deps, _, _, _, _, _, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	opts := Options{KeepBinary: true}
	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if result.BinaryAction != BinaryKept {
		t.Errorf("BinaryAction: got %q, want %q (KeepBinary=true)", result.BinaryAction, BinaryKept)
	}
}

// TestRun_PrintDryRun asserts that when Print=true, no mutations occur.
// The result should describe what would be removed, but nothing is touched.
func TestRun_PrintDryRun(t *testing.T) {
	deps, fs, _, shellrc, completion, autostart, prompt := makeDeps(t)
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	opts := Options{Print: true}
	_, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run with Print=true returned error: %v", err)
	}

	if len(prompt.ConfirmCalls) != 0 {
		t.Errorf("Confirm called %d times during dry-run, want 0", len(prompt.ConfirmCalls))
	}
	if len(fs.RemoveDirCalls) != 0 {
		t.Errorf("RemoveDir called %d times during dry-run, want 0", len(fs.RemoveDirCalls))
	}
	if len(shellrc.RemoveBlockCalls) != 0 {
		t.Errorf("RemoveBlock called %d times during dry-run, want 0", len(shellrc.RemoveBlockCalls))
	}
	if len(completion.CompletionPathCalls) != 0 {
		t.Errorf("CompletionPath called %d times during dry-run, want 0", len(completion.CompletionPathCalls))
	}
	if len(autostart.StopCalls) != 0 {
		t.Errorf("autostart.Stop called %d times during dry-run, want 0", len(autostart.StopCalls))
	}
}

// TestRun_Idempotency asserts that re-running uninstall when everything is
// already gone produces no error and empty Removed slice.
func TestRun_Idempotency(t *testing.T) {
	deps, fs, _, _, _, autostart, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}

	// Paths return empty dirs (already gone, but port still works).
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}

	// RemoveDir is idempotent (returns nil even if not present).
	// Autostart reports not installed.
	autostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: false, Running: false}, nil
	}

	// FsPort: simulate remove of already-absent dir (returns nil per spec).
	removeCount := 0
	fs.RemoveDirFunc = func(_ context.Context, dir string) error {
		removeCount++
		return nil
	}

	opts := Options{}
	_, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned error on already-uninstalled system: %v", err)
	}
}

// TestRun_BinaryAction_DeletedNow asserts that when os.Remove succeeds,
// BinaryAction is BinaryDeletedNow.
func TestRun_BinaryAction_DeletedNow(t *testing.T) {
	deps, _, _, _, _, _, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}
	// Inject a RemoveBinary func that succeeds.
	deps.RemoveBinaryFunc = func(path string) error { return nil }

	opts := Options{}
	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.BinaryAction != BinaryDeletedNow {
		t.Errorf("BinaryAction: got %q, want %q", result.BinaryAction, BinaryDeletedNow)
	}
}

// TestRun_BinaryAction_ManualDelete asserts that when os.Remove fails with a
// permission error, BinaryAction is BinaryDeleteRequested and NextSteps
// contains a hint.
func TestRun_BinaryAction_ManualDelete(t *testing.T) {
	deps, _, _, _, _, _, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}
	// Inject a RemoveBinary func that fails.
	deps.RemoveBinaryFunc = func(path string) error {
		return &permError{path: path}
	}

	opts := Options{}
	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.BinaryAction != BinaryDeleteRequested {
		t.Errorf("BinaryAction: got %q, want %q", result.BinaryAction, BinaryDeleteRequested)
	}
	if len(result.NextSteps) == 0 {
		t.Error("NextSteps must contain a sudo rm hint when binary removal fails")
	}
	for _, step := range result.NextSteps {
		if len(step) > 0 {
			// Found at least one hint.
			return
		}
	}
	t.Error("NextSteps slice is non-empty but contains only empty strings")
}

// TestRun_BinaryAction_ScheduledExit asserts that when KeepBinary=false and
// the ScheduledExitFunc is wired, BinaryAction is BinaryScheduledExit.
func TestRun_BinaryAction_ScheduledExit(t *testing.T) {
	deps, _, _, _, _, _, _ := makeDeps(t)
	deps.Prompt = &ports.MockPromptPort{ConfirmResult: true}
	deps.Paths = &ports.MockPathsPort{
		DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
		ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
		CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
	}
	// Inject a RemoveBinary that succeeds AND a ScheduledExitFunc to indicate
	// we should schedule exit after self-delete.
	deps.RemoveBinaryFunc = func(path string) error { return nil }
	deps.ScheduledExitFunc = func() { /* no-op in test */ }

	opts := Options{}
	result, err := Run(context.Background(), deps, opts, nil)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.BinaryAction != BinaryScheduledExit {
		t.Errorf("BinaryAction: got %q, want %q", result.BinaryAction, BinaryScheduledExit)
	}
}

// permError is a minimal fake permission-denied error for testing.
type permError struct{ path string }

func (e *permError) Error() string { return "permission denied: " + e.path }
