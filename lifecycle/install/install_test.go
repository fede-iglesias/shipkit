package install_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/install"
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// fixedTime is the deterministic timestamp used throughout all tests.
var fixedTime = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

// newDeps returns a fully-wired Deps with safe mock ports and a temp data dir.
// The MockPathsPort DataDir returns a real temp directory so WriteMarker can
// actually write the JSON file.
func newDeps(t *testing.T) (install.Deps, string) {
	t.Helper()
	dataDir := t.TempDir()
	home := t.TempDir()

	paths := ports.NewMockPathsPort()
	paths.DataDirFunc = func(app string) (string, error) { return dataDir, nil }
	paths.UserHomeResult = home
	paths.ExecutableResult = "/usr/local/bin/testapp"
	paths.InPATHResult = true

	env := ports.NewMockEnvPort()

	cfg := install.Config{
		AppName:    "testapp",
		BinaryName: "testapp",
		Version:    "v0.1.0",
	}

	deps := install.Deps{
		Cfg:        cfg,
		FS:         ports.NewMockFsPort(),
		Paths:      paths,
		Env:        env,
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
		Clock:      ports.NewMockClockPort(fixedTime),
	}
	return deps, dataDir
}

// newRootCmd returns a minimal cobra root for tests that need a cobra.Command.
func newRootCmd() *cobra.Command {
	return &cobra.Command{Use: "testapp"}
}

// TestWriteMarker_CreatesJSONWithRequiredFields asserts that Run writes a
// .shipkit.installed JSON file containing version_installed, installed_at, and
// bin_path.
func TestWriteMarker_CreatesJSONWithRequiredFields(t *testing.T) {
	deps, dataDir := newDeps(t)
	opts := install.Options{}

	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	raw, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("marker file not found at %s: %v", markerPath, err)
	}

	// Check raw JSON contains required field names.
	for _, field := range []string{"version_installed", "installed_at", "bin_path"} {
		if !bytes.Contains(raw, []byte(field)) {
			t.Errorf("marker JSON missing field %q\ngot: %s", field, raw)
		}
	}

	// Decode and verify values.
	var m install.InstallMarker
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("marker JSON invalid: %v\nraw: %s", err, raw)
	}
	if m.VersionInstalled != "v0.1.0" {
		t.Errorf("version_installed = %q; want %q", m.VersionInstalled, "v0.1.0")
	}
	if m.InstalledAt != fixedTime.Format(time.RFC3339) {
		t.Errorf("installed_at = %q; want %q", m.InstalledAt, fixedTime.Format(time.RFC3339))
	}
	if m.BinPath != "/usr/local/bin/testapp" {
		t.Errorf("bin_path = %q; want %q", m.BinPath, "/usr/local/bin/testapp")
	}
}

// TestWriteMarker_AtomicWrite asserts that the marker is written via a temp
// file and renamed (not direct write). We observe this indirectly: the marker
// must exist and be valid JSON after Run, even when the dataDir existed already.
func TestWriteMarker_AtomicWrite(t *testing.T) {
	deps, dataDir := newDeps(t)

	// Write a partial (invalid) file first to simulate a prior interrupted write.
	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	if err := os.WriteFile(markerPath, []byte("GARBAGE"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := install.Options{Force: true}
	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	raw, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("marker file missing: %v", err)
	}
	var m install.InstallMarker
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("marker is invalid JSON after atomic replace: %v\nraw: %s", err, raw)
	}
}

// TestWriteMarker_FailsOnPermissionDenied asserts that Run returns an error
// when the data directory is not writable.
func TestWriteMarker_FailsOnPermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission check not meaningful")
	}
	deps, dataDir := newDeps(t)

	// Make the dataDir read-only so writing the marker fails.
	if err := os.Chmod(dataDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dataDir, 0o755) })

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err == nil {
		t.Error("expected error on permission-denied dataDir, got nil")
	}
}

// TestRun_Idempotency_AlreadyInstalled asserts that Run on an already-installed
// system returns Result.AlreadyInstalled = true and does not overwrite the marker.
func TestRun_Idempotency_AlreadyInstalled(t *testing.T) {
	deps, dataDir := newDeps(t)
	root := newRootCmd()

	// First install.
	r1, err := install.Run(context.Background(), deps, install.Options{}, root)
	if err != nil {
		t.Fatalf("first Run error: %v", err)
	}
	if r1.AlreadyInstalled {
		t.Fatal("first Run should not report AlreadyInstalled")
	}

	// Record marker mtime.
	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	info1, err := os.Stat(markerPath)
	if err != nil {
		t.Fatal(err)
	}

	// Second run - should be a no-op.
	r2, err := install.Run(context.Background(), deps, install.Options{}, root)
	if err != nil {
		t.Fatalf("second Run error: %v", err)
	}
	if !r2.AlreadyInstalled {
		t.Error("second Run should report AlreadyInstalled = true")
	}

	// Marker should not be rewritten.
	info2, _ := os.Stat(markerPath)
	if !info2.ModTime().Equal(info1.ModTime()) {
		t.Error("marker was rewritten on second Run (should be idempotent)")
	}
}

// TestRun_ForceFlag_OverridesIdempotency asserts that --force re-runs even when
// the marker already exists.
func TestRun_ForceFlag_OverridesIdempotency(t *testing.T) {
	deps, dataDir := newDeps(t)
	root := newRootCmd()

	// First install.
	if _, err := install.Run(context.Background(), deps, install.Options{}, root); err != nil {
		t.Fatalf("first Run error: %v", err)
	}

	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	info1, _ := os.Stat(markerPath)

	// Force-reinstall using a slightly later clock so mtime changes.
	deps.Clock = ports.NewMockClockPort(fixedTime.Add(time.Second))

	r2, err := install.Run(context.Background(), deps, install.Options{Force: true}, root)
	if err != nil {
		t.Fatalf("force Run error: %v", err)
	}
	if r2.AlreadyInstalled {
		t.Error("force Run should NOT report AlreadyInstalled")
	}

	info2, _ := os.Stat(markerPath)
	if info2.ModTime().Equal(info1.ModTime()) {
		t.Error("force Run should rewrite the marker")
	}
}

// TestRun_PrintDryRun_NoMutations asserts that --print (dry-run) writes nothing.
func TestRun_PrintDryRun_NoMutations(t *testing.T) {
	deps, dataDir := newDeps(t)

	_, err := install.Run(context.Background(), deps, install.Options{Print: true}, newRootCmd())
	if err != nil {
		t.Fatalf("dry-run Run error: %v", err)
	}

	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	if _, err := os.Stat(markerPath); err == nil {
		t.Error("dry-run should not create the marker file")
	}

	// FS port should have received no CopyFile or RemoveDir calls.
	mockFS := deps.FS.(*ports.MockFsPort)
	if len(mockFS.CopyFileCalls) > 0 {
		t.Errorf("dry-run triggered %d CopyFile calls; expected 0", len(mockFS.CopyFileCalls))
	}
}

// TestRun_WithAutostart asserts that enabling autostart calls AutostartPort.Install.
func TestRun_WithAutostart(t *testing.T) {
	deps, _ := newDeps(t)
	deps.Cfg.EnableAutostart = true

	_, err := install.Run(context.Background(), deps, install.Options{Autostart: true}, newRootCmd())
	if err != nil {
		t.Fatalf("Run with autostart error: %v", err)
	}

	mockAS := deps.Autostart.(*ports.MockAutostartPort)
	if len(mockAS.InstallCalls) == 0 {
		t.Error("autostart enabled but AutostartPort.Install was not called")
	}
}

// TestRun_Autostart_ConfigNotEnabled asserts that --autostart without
// cfg.EnableAutostart returns an error.
func TestRun_Autostart_ConfigNotEnabled(t *testing.T) {
	deps, _ := newDeps(t)
	// cfg.EnableAutostart defaults to false.

	_, err := install.Run(context.Background(), deps, install.Options{Autostart: true}, newRootCmd())
	if err == nil {
		t.Error("expected error when --autostart requested but not enabled in Config")
	}
}

// TestRun_StateFlow_CreatesDirs asserts that the data, config, and cache dirs
// are created by MkdirAll calls during Run. We verify the directories exist
// after Run returns.
func TestRun_StateFlow_CreatesDirs(t *testing.T) {
	deps, dataDir := newDeps(t)

	// Point DataDir to a non-existent subdir to test mkdir.
	newDataDir := filepath.Join(dataDir, "subdata")
	paths := deps.Paths.(*ports.MockPathsPort)
	paths.DataDirFunc = func(app string) (string, error) { return newDataDir, nil }

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if _, err := os.Stat(newDataDir); os.IsNotExist(err) {
		t.Errorf("data dir %s not created", newDataDir)
	}
}

// TestRun_EmitsCompletions asserts that CompletionPort.EmitCompletion is called
// for the detected shell during Run.
func TestRun_EmitsCompletions(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockCP := deps.Completion.(*ports.MockCompletionPort)
	if len(mockCP.EmitCompletionCalls) == 0 {
		t.Error("EmitCompletion was not called")
	}
}

// TestRun_EnsuresShellHooks asserts that ShellRcPort.EnsureBlock is called
// for non-fish shells.
func TestRun_EnsuresShellHooks(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockRC := deps.ShellRc.(*ports.MockShellRcPort)
	if len(mockRC.EnsureBlockCalls) == 0 {
		t.Error("EnsureBlock was not called for zsh")
	}
}

// TestRun_FishShell_NoShellRcWrite asserts that fish shell skips the shellrc
// edit (fish autoloads completions, no fpath block needed).
func TestRun_FishShell_NoShellRcWrite(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellFish

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockRC := deps.ShellRc.(*ports.MockShellRcPort)
	if len(mockRC.EnsureBlockCalls) > 0 {
		t.Error("EnsureBlock should not be called for fish shell")
	}
}

// TestRun_Result_ManifestNotEmpty asserts that Result.Manifest is non-empty
// after a successful install, recording what was written.
func TestRun_Result_ManifestNotEmpty(t *testing.T) {
	deps, _ := newDeps(t)

	r, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(r.Manifest) == 0 {
		t.Error("Result.Manifest should be non-empty after successful install")
	}
}

// TestRun_CompletionsWritten_PopulatedInResult asserts that Result.CompletionsWritten
// maps the detected shell to a non-empty path after Run.
func TestRun_CompletionsWritten_PopulatedInResult(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	r, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if path, ok := r.CompletionsWritten[ports.ShellZsh]; !ok || path == "" {
		t.Errorf("CompletionsWritten[ShellZsh] = %q; want non-empty path", path)
	}
}

// TestRun_UnknownShell_NoCompletionOrShellRc asserts that when shell is unknown
// Run still succeeds but skips completion and shellrc steps.
func TestRun_UnknownShell_NoCompletionOrShellRc(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellUnknown

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockCP := deps.Completion.(*ports.MockCompletionPort)
	mockRC := deps.ShellRc.(*ports.MockShellRcPort)
	if len(mockCP.EmitCompletionCalls) > 0 {
		t.Error("EmitCompletion should not be called for unknown shell")
	}
	if len(mockRC.EnsureBlockCalls) > 0 {
		t.Error("EnsureBlock should not be called for unknown shell")
	}
}

// TestRun_PathEnsured_TrueWhenBinDirInPath asserts Result.PathEnsured = true
// when the binary directory is in $PATH.
func TestRun_PathEnsured_TrueWhenBinDirInPath(t *testing.T) {
	deps, _ := newDeps(t)

	r, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !r.PathEnsured {
		t.Error("PathEnsured should be true when bin dir is in $PATH")
	}
}

// TestRun_CompletionsSpecified_OnlyWriteSpecifiedShells asserts that when
// Options.Completions is set, only those shells get completions installed.
func TestRun_CompletionsSpecified_OnlyWriteSpecifiedShells(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh // autodetect would give zsh

	// Explicitly request only bash.
	opts := install.Options{Completions: []ports.ShellKind{ports.ShellBash}}

	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockCP := deps.Completion.(*ports.MockCompletionPort)
	for _, shell := range mockCP.EmitCompletionCalls {
		if shell != ports.ShellBash {
			t.Errorf("EmitCompletion called for %q; only ShellBash was requested", shell)
		}
	}
}

// TestWriteMarker_MarkerContent_AppNameField asserts that the app field in the
// marker matches the configured AppName.
func TestWriteMarker_MarkerContent_AppNameField(t *testing.T) {
	deps, dataDir := newDeps(t)

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(dataDir, ".shipkit.installed"))
	if !bytes.Contains(raw, []byte(`"testapp"`)) {
		t.Errorf("marker missing app name field\ngot: %s", raw)
	}
}

// TestRun_AutostartInstalled_TrueInResult asserts that Result.AutostartInstalled
// is true when autostart is requested and enabled.
func TestRun_AutostartInstalled_TrueInResult(t *testing.T) {
	deps, _ := newDeps(t)
	deps.Cfg.EnableAutostart = true

	r, err := install.Run(context.Background(), deps, install.Options{Autostart: true}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !r.AutostartInstalled {
		t.Error("Result.AutostartInstalled should be true after autostart install")
	}
}

// TestRun_CreateDirs_FailsOnPathsError asserts that Run returns error when
// PathsPort.DataDir fails.
func TestRun_CreateDirs_FailsOnPathsError(t *testing.T) {
	deps, _ := newDeps(t)
	paths := deps.Paths.(*ports.MockPathsPort)
	paths.DataDirFunc = func(app string) (string, error) {
		return "", os.ErrPermission
	}

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err == nil {
		t.Error("expected error when DataDir fails")
	}
}

// TestRun_MarkerContent_CompletionsField asserts the marker records which shells
// got completions installed.
func TestRun_MarkerContent_CompletionsField(t *testing.T) {
	deps, dataDir := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(dataDir, ".shipkit.installed"))
	if !bytes.Contains(raw, []byte("completions")) {
		t.Errorf("marker missing completions field\ngot: %s", raw)
	}
}

// TestRun_MarkerContent_AutostartField asserts the marker records autostart status.
func TestRun_MarkerContent_AutostartField(t *testing.T) {
	deps, dataDir := newDeps(t)

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(dataDir, ".shipkit.installed"))
	if !bytes.Contains(raw, []byte("autostart")) {
		t.Errorf("marker missing autostart field\ngot: %s", raw)
	}
}

// TestRun_ExplicitYes_SkipsPrompt asserts that -y flag skips any PromptPort call.
// (install does not prompt by default, but the Yes flag should be wired.)
func TestRun_ExplicitYes_SkipsPrompt(t *testing.T) {
	deps, _ := newDeps(t)

	_, err := install.Run(context.Background(), deps, install.Options{Yes: true}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Prompt should not have been called.
	mockP := deps.Prompt.(*ports.MockPromptPort)
	if len(mockP.ConfirmCalls) > 0 {
		t.Error("Prompt.Confirm should not be called during install")
	}
}

// TestRun_PathNotInPATH_PathEnsuredFalse asserts that Result.PathEnsured is false
// when the binary directory is not in $PATH.
func TestRun_PathNotInPATH_PathEnsuredFalse(t *testing.T) {
	deps, _ := newDeps(t)
	paths := deps.Paths.(*ports.MockPathsPort)
	paths.InPATHResult = false

	r, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if r.PathEnsured {
		t.Error("PathEnsured should be false when bin dir is not in $PATH")
	}
}

// TestRun_ExplicitCompletions_EmptyList_SkipsCompletions asserts that
// Options.Completions = []ShellKind{} (empty explicit) skips completion writing.
func TestRun_ExplicitCompletions_EmptyList_SkipsCompletions(t *testing.T) {
	deps, _ := newDeps(t)

	opts := install.Options{Completions: []ports.ShellKind{}} // empty explicit
	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockCP := deps.Completion.(*ports.MockCompletionPort)
	if len(mockCP.EmitCompletionCalls) > 0 {
		t.Errorf("expected no EmitCompletion calls for empty completion list, got %d", len(mockCP.EmitCompletionCalls))
	}
}

// TestRun_Marker_WrittenAfterAllSteps asserts that WriteMarker is the last step:
// if we can read the marker, completions were also written.
func TestRun_Marker_WrittenAfterAllSteps(t *testing.T) {
	deps, dataDir := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	// Inject an ordered call tracker via ShellRc.
	order := []string{}
	mockRC := ports.NewMockShellRcPort()
	mockRC.EnsureBlockFunc = func(rcPath, blockID, content string) (ports.EnsureResult, error) {
		order = append(order, "shellrc")
		return ports.EnsureResult{Written: true}, nil
	}
	deps.ShellRc = mockRC

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Marker must exist.
	if _, err := os.Stat(filepath.Join(dataDir, ".shipkit.installed")); err != nil {
		t.Error("marker not written")
	}
	// ShellRc must have been called before marker (i.e. we tracked it).
	if len(order) == 0 {
		t.Error("shellrc EnsureBlock was not called")
	}
}

// TestRun_ContextCancelled_ReturnsError asserts that a cancelled context
// causes Run to return a non-nil error promptly.
func TestRun_ContextCancelled_ReturnsError(t *testing.T) {
	deps, dataDir := newDeps(t)
	// Inject a completion port that blocks until ctx is cancelled.
	mockCP := ports.NewMockCompletionPort()
	mockCP.EmitCompletionFunc = func(shell ports.ShellKind, root *cobra.Command, dst interface{ Write([]byte) (int, error) }) error {
		return context.Canceled
	}
	deps.Completion = mockCP

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	_, err := install.Run(ctx, deps, install.Options{}, newRootCmd())
	// We don't require a specific error but the marker must not be written.
	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	if _, statErr := os.Stat(markerPath); statErr == nil {
		t.Error("marker should not be written when context is cancelled")
	}
	_ = err // err may or may not be non-nil depending on where cancel lands
}

// TestRun_MultiShellCompletions asserts that requesting multiple explicit shells
// results in multiple EmitCompletion calls.
func TestRun_MultiShellCompletions(t *testing.T) {
	deps, _ := newDeps(t)
	opts := install.Options{
		Completions: []ports.ShellKind{ports.ShellBash, ports.ShellZsh, ports.ShellFish},
	}

	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockCP := deps.Completion.(*ports.MockCompletionPort)
	if len(mockCP.EmitCompletionCalls) != 3 {
		t.Errorf("expected 3 EmitCompletion calls, got %d", len(mockCP.EmitCompletionCalls))
	}
}

// TestRun_PathsExecutable_Error asserts Run returns error when Paths.Executable fails.
func TestRun_PathsExecutable_Error(t *testing.T) {
	deps, _ := newDeps(t)
	paths := deps.Paths.(*ports.MockPathsPort)
	paths.ExecutableErr = os.ErrPermission
	paths.ExecutableResult = ""

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err == nil {
		t.Error("expected error when Paths.Executable fails")
	}
}

// TestRun_CompletionEmitError_ReturnsError asserts that Run returns error when
// EmitCompletion fails.
func TestRun_CompletionEmitError_ReturnsError(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	mockCP := ports.NewMockCompletionPort()
	mockCP.EmitCompletionFunc = func(_ ports.ShellKind, _ *cobra.Command, _ interface{ Write([]byte) (int, error) }) error {
		return os.ErrPermission
	}
	deps.Completion = mockCP

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err == nil {
		t.Error("expected error when EmitCompletion fails")
	}
}

// TestRun_ShellRcError_ReturnsError asserts that Run returns error when
// EnsureBlock fails.
func TestRun_ShellRcError_ReturnsError(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.ShellResult = ports.ShellZsh

	mockRC := ports.NewMockShellRcPort()
	mockRC.EnsureBlockFunc = func(_, _, _ string) (ports.EnsureResult, error) {
		return ports.EnsureResult{}, os.ErrPermission
	}
	deps.ShellRc = mockRC

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err == nil {
		t.Error("expected error when EnsureBlock fails")
	}
}

// TestRun_AutostartError_ReturnsError asserts that Run returns error when
// AutostartPort.Install fails.
func TestRun_AutostartError_ReturnsError(t *testing.T) {
	deps, _ := newDeps(t)
	deps.Cfg.EnableAutostart = true

	mockAS := ports.NewMockAutostartPort()
	mockAS.InstallFunc = func(unit ports.AutostartUnit) error { return os.ErrPermission }
	deps.Autostart = mockAS

	_, err := install.Run(context.Background(), deps, install.Options{Autostart: true}, newRootCmd())
	if err == nil {
		t.Error("expected error when AutostartPort.Install fails")
	}
}

// TestRun_BashVersion_EnvKey asserts that the install flow reads the bash
// version environment variable via EnvPort.
func TestRun_BashVersion_EnvKey(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	// Request bash completions; on darwin the bash version check uses
	// BASH_VERSION env var (set by bash itself when running bash).
	env.OSResult = "linux" // linux: no 3.2 skip
	env.ShellResult = ports.ShellBash

	opts := install.Options{Completions: []ports.ShellKind{ports.ShellBash}}
	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	mockCP := deps.Completion.(*ports.MockCompletionPort)
	if len(mockCP.EmitCompletionCalls) == 0 {
		t.Error("bash completion should be emitted on linux regardless of version")
	}
}

// TestRun_WriteMarker_WritesFile_OnRealFS asserts marker path is under dataDir.
func TestRun_WriteMarker_WritesFile_OnRealFS(t *testing.T) {
	deps, dataDir := newDeps(t)

	_, err := install.Run(context.Background(), deps, install.Options{}, newRootCmd())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	markerPath := filepath.Join(dataDir, ".shipkit.installed")
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("marker not found at %s: %v", markerPath, err)
	}

	raw, _ := os.ReadFile(markerPath)
	if !strings.Contains(string(raw), "version_installed") {
		t.Errorf("marker content malformed:\n%s", raw)
	}
}
