package uninstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// TestRun_PromptError asserts that when Confirm returns an error, Run
// propagates it wrapped.
func TestRun_PromptError(t *testing.T) {
	promptErr := errors.New("stdin closed")
	prompt := &ports.MockPromptPort{
		ConfirmFunc: func(question string, defaultYes bool) (bool, error) {
			return false, promptErr
		},
	}

	deps := Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths:   ports.NewMockPathsPort(),
		ShellRc: ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart: ports.NewMockAutostartPort(),
		Prompt: prompt,
	}

	_, err := Run(context.Background(), deps, Options{}, nil)
	if err == nil {
		t.Fatal("expected error from prompt, got nil")
	}
	if !errors.Is(err, promptErr) {
		t.Errorf("expected error chain to contain promptErr, got: %v", err)
	}
}

// TestBuildDryRunResult_PathErrors covers branches where DataDir/ConfigDir/
// CacheDir return errors - those paths set the dir to "" and skip them.
func TestBuildDryRunResult_PathErrors(t *testing.T) {
	pathErr := errors.New("no xdg")
	deps := Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "", pathErr },
			ConfigDirFunc: func(app string) (string, error) { return "", pathErr },
			CacheDirFunc:  func(app string) (string, error) { return "", pathErr },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	result, err := buildDryRunResult(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("buildDryRunResult returned unexpected error: %v", err)
	}
	// Dirs errored so nothing added; binary still shows as to-remove.
	for _, r := range result.Removed {
		if r == "" {
			t.Error("Removed contains empty string (empty dir from error path)")
		}
	}
}

// TestBuildDryRunResult_KeepData covers the KeepData+KeepConfig+KeepBinary
// branches in the dry-run path (added to skipped).
func TestBuildDryRunResult_KeepAll(t *testing.T) {
	deps := Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	result, err := buildDryRunResult(context.Background(), deps, Options{
		KeepData:   true,
		KeepConfig: true,
		KeepBinary: true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All kept: skipped contains data, config, binary; cache always removed in dry-run.
	foundData, foundConfig, foundBinary := false, false, false
	for _, s := range result.Skipped {
		switch s {
		case "/data/testapp":
			foundData = true
		case "/config/testapp":
			foundConfig = true
		case "/usr/local/bin/testapp":
			foundBinary = true
		}
	}
	if !foundData {
		t.Error("expected /data/testapp in Skipped (KeepData=true)")
	}
	if !foundConfig {
		t.Error("expected /config/testapp in Skipped (KeepConfig=true)")
	}
	if !foundBinary {
		t.Error("expected binary in Skipped (KeepBinary=true)")
	}
}

// TestRunTeardown_AutostartWithLabel covers the AutostartLabel != "" branch
// including the running=true path (result.Stopped = true).
func TestRunTeardown_AutostartWithLabel(t *testing.T) {
	autostart := ports.NewMockAutostartPort()
	autostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: true, Running: true}, nil
	}
	stopCalled := false
	autostart.StopFunc = func(label string) error {
		stopCalled = true
		return nil
	}

	deps := Deps{
		AppName:        "testapp",
		BinPath:        "/usr/local/bin/testapp",
		AutostartLabel: "com.example.testapp",
		FS:             ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  autostart,
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	result, err := runTeardown(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}
	if !stopCalled {
		t.Error("expected autostart.Stop to be called when Running=true")
	}
	if !result.Stopped {
		t.Error("expected result.Stopped=true when autostart was running")
	}
}

// TestRunTeardown_AutostartStopFails covers the autostart stop-fails branch
// (stopErr != nil - Stopped stays false).
func TestRunTeardown_AutostartStopFails(t *testing.T) {
	autostart := ports.NewMockAutostartPort()
	autostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: true, Running: true}, nil
	}
	autostart.StopFunc = func(label string) error {
		return errors.New("launchctl bootout failed")
	}

	deps := Deps{
		AppName:        "testapp",
		AutostartLabel: "com.example.testapp",
		FS:             ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  autostart,
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	result, err := runTeardown(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}
	if result.Stopped {
		t.Error("result.Stopped must be false when Stop returns error")
	}
}

// TestRunTeardown_AutostartUninstallFails covers the autostart uninstall-fails
// branch (added to Skipped).
func TestRunTeardown_AutostartUninstallFails(t *testing.T) {
	autostart := ports.NewMockAutostartPort()
	autostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: true, Running: false}, nil
	}
	autostart.UninstallFunc = func(label string) error {
		return errors.New("plist locked")
	}

	deps := Deps{
		AppName:        "testapp",
		AutostartLabel: "com.example.testapp",
		FS:             ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  autostart,
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	result, err := runTeardown(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}
	found := false
	for _, s := range result.Skipped {
		if s == "autostart:com.example.testapp" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected autostart:com.example.testapp in Skipped when Uninstall fails, got %v", result.Skipped)
	}
}

// TestRunTeardown_CompletionPathError covers the CompletionPath error branch
// (continue - no entry added).
func TestRunTeardown_CompletionPathError(t *testing.T) {
	completion := ports.NewMockCompletionPort()
	completion.CompletionPathFunc = func(shell ports.ShellKind, app, home string) (string, error) {
		return "", errors.New("unsupported shell")
	}

	deps := Deps{
		AppName:    "testapp",
		ShellKinds: []ports.ShellKind{ports.ShellBash},
		FS:         ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: completion,
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	_, err := runTeardown(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}
}

// TestRunTeardown_RemoveDirFails covers the RemoveDir failure branch for
// completion file removal (added to Skipped) and directory removal.
func TestRunTeardown_RemoveDirFails(t *testing.T) {
	fsPort := ports.NewMockFsPort()
	rmErr := errors.New("read-only filesystem")
	fsPort.RemoveDirFunc = func(_ context.Context, dir string) error {
		return rmErr
	}

	deps := Deps{
		AppName:    "testapp",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         fsPort,
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/testuser",
			DataDirFunc:    func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc:  func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	result, err := runTeardown(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}
	if len(result.Skipped) == 0 {
		t.Error("expected Skipped to be non-empty when RemoveDir fails")
	}
}

// TestRunTeardown_DataDirError covers the DataDir/ConfigDir/CacheDir error
// branch in runTeardown (dirs skipped entirely).
func TestRunTeardown_DataDirError(t *testing.T) {
	pathErr := errors.New("xdg lookup failed")
	deps := Deps{
		AppName: "testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "", pathErr },
			ConfigDirFunc: func(app string) (string, error) { return "", pathErr },
			CacheDirFunc:  func(app string) (string, error) { return "", pathErr },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     &ports.MockPromptPort{ConfirmResult: true},
	}

	_, err := runTeardown(context.Background(), deps, Options{}, nil)
	if err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}
}

// TestBuildRcPaths_EmptyHome covers the empty-home fallback branch.
func TestBuildRcPaths_EmptyHome(t *testing.T) {
	paths := buildRcPaths("")
	if len(paths) == 0 {
		t.Error("buildRcPaths with empty home must return non-empty slice")
	}
	for _, p := range paths {
		if p == "" {
			t.Error("buildRcPaths must not return empty string paths")
		}
	}
}

// TestUninstall_WalksUpEmptyParents verifies that after removing the completion
// script, the walk-up loop removes empty parent directories (site-functions/,
// zsh/) but stops at the XDG_DATA_HOME root and never touches the app dataDir
// (which is a sibling of zsh/ under XDG_DATA_HOME).
//
// XDG-real layout (matches what adapters.CobraCompletionPort + DataDir produce):
//
//	$T/data/                              <- XDG_DATA_HOME root (walk-up bound)
//	$T/data/kt/                           <- dataDir for app "kt" (sibling)
//	$T/data/zsh/site-functions/_kt        <- completion script (sibling subtree)
func TestUninstall_WalksUpEmptyParents(t *testing.T) {
	T := t.TempDir()
	xdgDataHome := filepath.Join(T, "data")
	dataDir := filepath.Join(xdgDataHome, "kt") // app-specific, sibling of zsh/
	scriptDir := filepath.Join(xdgDataHome, "zsh", "site-functions")
	scriptPath := filepath.Join(scriptDir, "_kt")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scriptDir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("# completion"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wire a real FS that removes the script file on RemoveDir, simulating the
	// port behaviour: RemoveDir on a single file removes it.
	fsPort := ports.NewMockFsPort()
	fsPort.RemoveDirFunc = func(_ context.Context, path string) error {
		return os.Remove(path)
	}

	deps := Deps{
		AppName:    "kt",
		BinPath:    "/usr/local/bin/kt",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         fsPort,
		Paths: &ports.MockPathsPort{
			UserHomeResult: T,
			DataDirFunc:    func(app string) (string, error) { return dataDir, nil },
			ConfigDirFunc:  func(app string) (string, error) { return filepath.Join(T, "config"), nil },
			CacheDirFunc:   func(app string) (string, error) { return filepath.Join(T, "cache"), nil },
		},
		ShellRc: ports.NewMockShellRcPort(),
		Completion: &ports.MockCompletionPort{
			CompletionPathFunc: func(shell ports.ShellKind, app, home string) (string, error) {
				return scriptPath, nil
			},
		},
		Autostart: ports.NewMockAutostartPort(),
		Prompt:    &ports.MockPromptPort{ConfirmResult: true},
	}

	if _, err := runTeardown(context.Background(), deps, Options{}, nil); err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}

	// site-functions/ must be gone (was empty after script removal).
	if _, err := os.Stat(scriptDir); err == nil {
		t.Error("site-functions/ still exists; walk-up did not remove it")
	}
	// zsh/ must be gone (was empty after site-functions/ removal).
	if _, err := os.Stat(filepath.Join(xdgDataHome, "zsh")); err == nil {
		t.Error("zsh/ still exists; walk-up did not remove it")
	}
	// XDG_DATA_HOME root must still exist (walk-up stops at the boundary).
	if _, err := os.Stat(xdgDataHome); err != nil {
		t.Errorf("xdgDataHome was removed by walk-up; bound check failed: %v", err)
	}
	// Verify the completion removal was recorded.
	found := false
	for _, r := range fsPort.RemoveDirCalls {
		if r == scriptPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RemoveDir was not called for scriptPath %q", scriptPath)
	}
}

// TestUninstall_StopsAtNonEmptyParent verifies that when a sibling file exists
// in site-functions/, os.Remove fails on the dir and the walk-up stops there,
// leaving site-functions/ intact. Uses the XDG-real layout where the completion
// is a sibling of dataDir under XDG_DATA_HOME.
func TestUninstall_StopsAtNonEmptyParent(t *testing.T) {
	T := t.TempDir()
	xdgDataHome := filepath.Join(T, "data")
	dataDir := filepath.Join(xdgDataHome, "kt")
	scriptDir := filepath.Join(xdgDataHome, "zsh", "site-functions")
	scriptPath := filepath.Join(scriptDir, "_kt")
	sibling := filepath.Join(scriptDir, "_other")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scriptDir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("# completion"), 0o644); err != nil {
		t.Fatalf("WriteFile script: %v", err)
	}
	if err := os.WriteFile(sibling, []byte("# other"), 0o644); err != nil {
		t.Fatalf("WriteFile sibling: %v", err)
	}

	fsPort := ports.NewMockFsPort()
	fsPort.RemoveDirFunc = func(_ context.Context, path string) error {
		return os.Remove(path)
	}

	deps := Deps{
		AppName:    "kt",
		BinPath:    "/usr/local/bin/kt",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         fsPort,
		Paths: &ports.MockPathsPort{
			UserHomeResult: T,
			DataDirFunc:    func(app string) (string, error) { return dataDir, nil },
			ConfigDirFunc:  func(app string) (string, error) { return filepath.Join(T, "config"), nil },
			CacheDirFunc:   func(app string) (string, error) { return filepath.Join(T, "cache"), nil },
		},
		ShellRc: ports.NewMockShellRcPort(),
		Completion: &ports.MockCompletionPort{
			CompletionPathFunc: func(shell ports.ShellKind, app, home string) (string, error) {
				return scriptPath, nil
			},
		},
		Autostart: ports.NewMockAutostartPort(),
		Prompt:    &ports.MockPromptPort{ConfirmResult: true},
	}

	if _, err := runTeardown(context.Background(), deps, Options{}, nil); err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}

	// site-functions/ must still exist (sibling prevents removal).
	if _, err := os.Stat(scriptDir); os.IsNotExist(err) {
		t.Error("site-functions/ was removed despite having a sibling file; walk-up should have stopped")
	}
	// sibling must still be there.
	if _, err := os.Stat(sibling); os.IsNotExist(err) {
		t.Error("sibling file was removed unexpectedly")
	}
}

// TestUninstall_StopsAtXDGDataHomeBoundary verifies that the walk-up loop stops
// exactly at XDG_DATA_HOME and never deletes the XDG root itself nor walks
// above it. The XDG-real layout puts the completion script as a sibling of
// dataDir, so the loop traverses zsh/site-functions/ and zsh/ but must bail
// before removing the XDG_DATA_HOME directory.
func TestUninstall_StopsAtXDGDataHomeBoundary(t *testing.T) {
	T := t.TempDir()
	xdgDataHome := filepath.Join(T, "data")
	dataDir := filepath.Join(xdgDataHome, "kt")
	scriptDir := filepath.Join(xdgDataHome, "zsh", "site-functions")
	scriptPath := filepath.Join(scriptDir, "_kt")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll dataDir: %v", err)
	}
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("MkdirAll scriptDir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("# completion"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fsPort := ports.NewMockFsPort()
	fsPort.RemoveDirFunc = func(_ context.Context, path string) error {
		return os.Remove(path)
	}

	deps := Deps{
		AppName:    "kt",
		BinPath:    "/usr/local/bin/kt",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         fsPort,
		Paths: &ports.MockPathsPort{
			UserHomeResult: T,
			DataDirFunc:    func(app string) (string, error) { return dataDir, nil },
			ConfigDirFunc:  func(app string) (string, error) { return filepath.Join(T, "config"), nil },
			CacheDirFunc:   func(app string) (string, error) { return filepath.Join(T, "cache"), nil },
		},
		ShellRc: ports.NewMockShellRcPort(),
		Completion: &ports.MockCompletionPort{
			CompletionPathFunc: func(shell ports.ShellKind, app, home string) (string, error) {
				return scriptPath, nil
			},
		},
		Autostart: ports.NewMockAutostartPort(),
		Prompt:    &ports.MockPromptPort{ConfirmResult: true},
	}

	if _, err := runTeardown(context.Background(), deps, Options{}, nil); err != nil {
		t.Fatalf("runTeardown returned error: %v", err)
	}

	// XDG_DATA_HOME root must still exist - walk-up must NEVER remove it.
	if _, err := os.Stat(xdgDataHome); os.IsNotExist(err) {
		t.Error("xdgDataHome was removed; walk-up did not respect the XDG_DATA_HOME boundary")
	}
	// tmpdir root T must obviously still be present (parent of xdgDataHome).
	if _, err := os.Stat(T); os.IsNotExist(err) {
		t.Error("tmpdir root T was removed; walk-up went above the XDG_DATA_HOME boundary")
	}
}
