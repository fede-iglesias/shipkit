package uninstall

import (
	"context"
	"errors"
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
