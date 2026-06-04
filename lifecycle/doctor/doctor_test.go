package doctor_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/doctor"
	"github.com/fede-iglesias/shipkit/lifecycle/recovery"
	"github.com/fede-iglesias/shipkit/ports"
)

// fixedTime is a stable timestamp used in all tests for determinism.
var fixedTime = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

// newDeps returns a Deps with all ports mocked and reasonable defaults.
// Tests override individual fields as needed.
func newDeps(appName, binPath, version string) doctor.Deps {
	mockPaths := ports.NewMockPathsPort()
	mockPaths.ExecutableResult = binPath
	mockPaths.InPATHResult = true

	mockSpawn := ports.NewMockSpawnPort()
	mockSpawn.HealthCheckFunc = func(ctx context.Context, p string, t time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{Ok: true, Version: version}, nil
	}

	mockAutostart := ports.NewMockAutostartPort()
	mockAutostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: false, Running: false}, nil
	}

	return doctor.Deps{
		AppName:    appName,
		BinPath:    binPath,
		Version:    version,
		DataRoot:   "/tmp/testdata/" + appName,
		ConfigRoot: "/tmp/testconfig/" + appName,
		CacheRoot:  "/tmp/testcache/" + appName,
		HTTP:       ports.NewMockHTTPPort(),
		FS:         ports.NewMockFsPort(),
		Spawn:      mockSpawn,
		Paths:      mockPaths,
		Env:        ports.NewMockEnvPort(),
		Autostart:  mockAutostart,
		Completion: ports.NewMockCompletionPort(),
		Clock:      ports.NewMockClockPort(fixedTime),
	}
}

// --- binary.in-path ---

func TestRun_BinaryInPath_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Paths.(*ports.MockPathsPort).InPATHResult = true

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "binary.in-path")
	if check.Status != doctor.StatusPass {
		t.Errorf("binary.in-path: got %s, want pass; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "/usr/local/bin/myapp") {
		t.Errorf("binary.in-path details should mention binary path, got: %s", check.Details)
	}
}

func TestRun_BinaryInPath_Warn(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Paths.(*ports.MockPathsPort).InPATHResult = false

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "binary.in-path")
	if check.Status != doctor.StatusWarn {
		t.Errorf("binary.in-path: got %s, want warn", check.Status)
	}
	if check.Hint == "" {
		t.Error("binary.in-path warn should have a hint")
	}
}

// --- binary.executable ---

func TestRun_BinaryExecutable_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// StatExecutable returns executable (mode 0755) by default.
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "binary.executable")
	if check.Status != doctor.StatusPass {
		t.Errorf("binary.executable: got %s, want pass", check.Status)
	}
}

func TestRun_BinaryExecutable_Fail(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatExecutableFunc = func(path string) (bool, error) { return false, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "binary.executable")
	if check.Status != doctor.StatusFail {
		t.Errorf("binary.executable: got %s, want fail", check.Status)
	}
}

// --- binary.version ---

func TestRun_BinaryVersion_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// HealthCheck returns 0.1.0 which matches Version field.

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "binary.version")
	if check.Status != doctor.StatusPass {
		t.Errorf("binary.version: got %s, want pass; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "0.1.0") {
		t.Errorf("binary.version details should mention version, got: %s", check.Details)
	}
}

func TestRun_BinaryVersion_Fail(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// Health check returns a different version.
	deps.Spawn.(*ports.MockSpawnPort).HealthCheckFunc = func(ctx context.Context, p string, d time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{Ok: true, Version: "0.2.0"}, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "binary.version")
	if check.Status != doctor.StatusFail {
		t.Errorf("binary.version: got %s, want fail", check.Status)
	}
}

// --- xdg.data-dir ---

func TestRun_XDGDataDir_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// StatDirFunc says the data dir exists.
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "xdg.data-dir")
	if check.Status != doctor.StatusPass {
		t.Errorf("xdg.data-dir: got %s, want pass; details: %s", check.Status, check.Details)
	}
}

func TestRun_XDGDataDir_Warn(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// StatDirFunc says data dir does NOT exist.
	deps.StatDirFunc = func(path string) (bool, error) { return false, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "xdg.data-dir")
	if check.Status != doctor.StatusWarn {
		t.Errorf("xdg.data-dir: got %s, want warn", check.Status)
	}
	if check.Hint == "" {
		t.Error("xdg.data-dir warn should have a hint")
	}
}

// --- xdg.config-dir ---

func TestRun_XDGConfigDir_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "xdg.config-dir")
	if check.Status != doctor.StatusPass {
		t.Errorf("xdg.config-dir: got %s, want pass", check.Status)
	}
}

func TestRun_XDGConfigDir_Warn(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return false, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "xdg.config-dir")
	if check.Status != doctor.StatusWarn {
		t.Errorf("xdg.config-dir: got %s, want warn", check.Status)
	}
}

// --- xdg.cache-dir ---

func TestRun_XDGCacheDir_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "xdg.cache-dir")
	if check.Status != doctor.StatusPass {
		t.Errorf("xdg.cache-dir: got %s, want pass", check.Status)
	}
}

// --- marker ---

func TestRun_Marker_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.ReadMarkerFunc = func(path string) (string, error) {
		return `{"version_installed":"0.1.0","installed_at":"2026-06-04T00:00:00Z"}`, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "marker")
	if check.Status != doctor.StatusPass {
		t.Errorf("marker: got %s, want pass; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "0.1.0") {
		t.Errorf("marker details should mention version, got: %s", check.Details)
	}
}

func TestRun_Marker_WarnVersionMismatch(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.ReadMarkerFunc = func(path string) (string, error) {
		// Marker shows 0.0.9 but running version is 0.1.0.
		return `{"version_installed":"0.0.9","installed_at":"2026-06-04T00:00:00Z"}`, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "marker")
	if check.Status != doctor.StatusWarn {
		t.Errorf("marker: got %s, want warn; details: %s", check.Status, check.Details)
	}
}

func TestRun_Marker_WarnMissing(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// ReadMarkerFunc returns error (file not found).
	deps.ReadMarkerFunc = func(path string) (string, error) {
		return "", &markerNotFoundError{path: path}
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "marker")
	if check.Status != doctor.StatusWarn {
		t.Errorf("marker missing: got %s, want warn", check.Status)
	}
	if check.Hint == "" {
		t.Error("marker warn should have a hint")
	}
}

// --- completion ---

func TestRun_Completion_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Env.(*ports.MockEnvPort).ShellResult = ports.ShellZsh
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }
	// Completion file exists.
	deps.StatFileFunc = func(path string) (bool, error) { return true, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "completion.zsh")
	if check.Status != doctor.StatusPass {
		t.Errorf("completion.zsh: got %s, want pass; details: %s", check.Status, check.Details)
	}
}

func TestRun_Completion_Warn(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Env.(*ports.MockEnvPort).ShellResult = ports.ShellZsh
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }
	// Completion file does NOT exist.
	deps.StatFileFunc = func(path string) (bool, error) { return false, nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "completion.zsh")
	if check.Status != doctor.StatusWarn {
		t.Errorf("completion.zsh: got %s, want warn", check.Status)
	}
	if check.Hint == "" {
		t.Error("completion.zsh warn should have a hint mentioning install")
	}
}

func TestRun_Completion_UnknownShell_Skip(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Env.(*ports.MockEnvPort).ShellResult = ports.ShellUnknown

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// No completion check should appear (or it's skipped).
	// Verify that NO completion.* check with fail status exists.
	for _, c := range report.Checks {
		if strings.HasPrefix(string(c.ID), "completion.") && c.Status == doctor.StatusFail {
			t.Errorf("completion check should not fail for unknown shell, got: %s %s", c.ID, c.Status)
		}
	}
}

// --- autostart (not enabled) ---

func TestRun_Autostart_NotEnabled(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.AutostartLabel = "" // not enabled

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "autostart")
	if check.Status != doctor.StatusPass {
		t.Errorf("autostart not enabled: got %s, want pass", check.Status)
	}
	if !strings.Contains(check.Details, "not enabled") {
		t.Errorf("autostart details should mention 'not enabled', got: %s", check.Details)
	}
}

// --- autostart (enabled, running) ---

func TestRun_Autostart_EnabledRunning(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.AutostartLabel = "com.fede-iglesias.myapp"
	deps.Autostart.(*ports.MockAutostartPort).StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: true, Running: true, PID: 12345}, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "autostart")
	if check.Status != doctor.StatusPass {
		t.Errorf("autostart enabled+running: got %s, want pass", check.Status)
	}
}

// --- autostart (enabled, not running) ---

func TestRun_Autostart_EnabledNotRunning(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.AutostartLabel = "com.fede-iglesias.myapp"
	deps.Autostart.(*ports.MockAutostartPort).StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: true, Running: false}, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "autostart")
	if check.Status != doctor.StatusWarn {
		t.Errorf("autostart installed but not running: got %s, want warn", check.Status)
	}
}

// --- autostart (enabled, not installed) ---

func TestRun_Autostart_EnabledNotInstalled(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.AutostartLabel = "com.fede-iglesias.myapp"
	deps.Autostart.(*ports.MockAutostartPort).StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: false, Running: false}, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "autostart")
	if check.Status != doctor.StatusWarn {
		t.Errorf("autostart label set but not installed: got %s, want warn", check.Status)
	}
}

// --- recovery.manifest (absent) ---

func TestRun_RecoveryManifest_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// Point DataRoot at a fresh temp dir with no manifest file.
	deps.DataRoot = t.TempDir()

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "recovery.manifest")
	if check.Status != doctor.StatusPass {
		t.Errorf("recovery.manifest absent: got %s, want pass", check.Status)
	}
}

// --- recovery.manifest (present = fail) ---

func TestRun_RecoveryManifest_Fail(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// Write a real recovery manifest at the canonical path under DataRoot.
	dataRoot := t.TempDir()
	deps.DataRoot = dataRoot
	manifest := recovery.Manifest{
		Version:   1,
		AppName:   "myapp",
		CreatedAt: fixedTime,
	}
	if err := recovery.Write(recovery.Path(dataRoot), manifest); err != nil {
		t.Fatalf("recovery.Write: %v", err)
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "recovery.manifest")
	if check.Status != doctor.StatusFail {
		t.Errorf("recovery.manifest present: got %s, want fail", check.Status)
	}
}

// TestDoctor_RecoveryManifestCheck_DetectsPending exercises the full doctor
// pipeline against a real recovery manifest written via recovery.Write at the
// canonical path. After migration, doctor.Run consumes the manifest directly
// via recovery.Read (no StatFileFunc indirection), and the recovery.manifest
// check must report StatusFail with details mentioning the manifest path.
func TestDoctor_RecoveryManifestCheck_DetectsPending(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	dataRoot := t.TempDir()
	deps.DataRoot = dataRoot

	manifest := recovery.Manifest{
		Version:      1,
		AppName:      "myapp",
		SnapshotPath: filepath.Join(dataRoot, "snapshots", "snap-active"),
		Steps:        []string{"pre-update", "snapshot"},
		Cause:        "health-check failed",
		CreatedAt:    fixedTime,
	}
	manifestPath := recovery.Path(dataRoot)
	if err := recovery.Write(manifestPath, manifest); err != nil {
		t.Fatalf("recovery.Write: %v", err)
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "recovery.manifest")
	if check.Status != doctor.StatusFail {
		t.Fatalf("recovery.manifest: got %s, want fail (manifest exists)", check.Status)
	}
	if !strings.Contains(check.Details, manifestPath) {
		t.Errorf("recovery.manifest details should mention the manifest path %q, got: %s", manifestPath, check.Details)
	}
}

// --- network checks default skip ---

func TestRun_NetworkChecks_SkippedByDefault(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")

	report, err := doctor.Run(context.Background(), deps, doctor.Options{Network: false})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, c := range report.Checks {
		if strings.HasPrefix(string(c.ID), "network.") && c.Status != doctor.StatusSkipped {
			t.Errorf("network check %s should be skipped by default, got: %s", c.ID, c.Status)
		}
	}

	// Verify all 3 network checks appear as skipped.
	networkIDs := []string{"network.github", "network.cosign-tuf", "network.update-feed"}
	for _, id := range networkIDs {
		check := findCheckNoFail(report, id)
		if check == nil {
			t.Errorf("expected network check %q to appear in report", id)
			continue
		}
		if check.Status != doctor.StatusSkipped {
			t.Errorf("network check %s: got %s, want skipped", id, check.Status)
		}
		if !strings.Contains(check.Details, "--network") {
			t.Errorf("network check %s details should mention --network, got: %s", id, check.Details)
		}
	}
}

// --- network checks enabled ---

func TestRun_NetworkGitHub_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Repo = "fede-iglesias/tools"
	deps.TagPrefix = "myapp-"
	deps.CheckNetworkGitHubFunc = func(ctx context.Context) error { return nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{Network: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "network.github")
	if check.Status != doctor.StatusPass {
		t.Errorf("network.github: got %s, want pass", check.Status)
	}
}

func TestRun_NetworkGitHub_Fail(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Repo = "fede-iglesias/tools"
	deps.TagPrefix = "myapp-"
	deps.CheckNetworkGitHubFunc = func(ctx context.Context) error {
		return errNetwork("github.com unreachable")
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{Network: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "network.github")
	if check.Status != doctor.StatusFail {
		t.Errorf("network.github fail: got %s, want fail", check.Status)
	}
}

func TestRun_NetworkCosignTUF_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.CheckNetworkCosignTUFFunc = func(ctx context.Context) error { return nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{Network: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "network.cosign-tuf")
	if check.Status != doctor.StatusPass {
		t.Errorf("network.cosign-tuf: got %s, want pass", check.Status)
	}
}

func TestRun_NetworkUpdateFeed_Pass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Repo = "fede-iglesias/tools"
	deps.TagPrefix = "myapp-"
	deps.CheckNetworkUpdateFeedFunc = func(ctx context.Context) (string, error) { return "0.1.0", nil }

	report, err := doctor.Run(context.Background(), deps, doctor.Options{Network: true})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	check := findCheck(t, report, "network.update-feed")
	if check.Status != doctor.StatusPass {
		t.Errorf("network.update-feed: got %s, want pass", check.Status)
	}
}

// --- summary ---

func TestRun_Summary_AllPass(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }
	deps.StatFileFunc = func(path string) (bool, error) { return false, nil }
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }
	deps.ReadMarkerFunc = func(path string) (string, error) {
		return `{"version_installed":"0.1.0","installed_at":"2026-06-04T00:00:00Z"}`, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if report.Summary.Fail > 0 {
		t.Errorf("summary.Fail: got %d, want 0", report.Summary.Fail)
	}
	if report.Summary.Pass == 0 {
		t.Error("summary.Pass should be > 0")
	}
	if !report.Summary.OK {
		t.Error("Summary.OK should be true when no failures")
	}
}

func TestRun_Summary_HasFail(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	// binary.executable fails.
	deps.StatExecutableFunc = func(path string) (bool, error) { return false, nil }
	// recovery.manifest present = fail.
	deps.StatFileFunc = func(path string) (bool, error) {
		if strings.Contains(path, "recovery-manifest") {
			return true, nil
		}
		return false, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if report.Summary.Fail == 0 {
		t.Error("summary.Fail should be > 0")
	}
	if report.Summary.OK {
		t.Error("Summary.OK should be false when there are failures")
	}
}

// --- exit code (warn -> 0, fail -> 1) tested via summary ---

func TestSummary_OK_WarnOnly(t *testing.T) {
	report := doctor.Report{
		Checks: []doctor.Check{
			{ID: "binary.in-path", Status: doctor.StatusWarn},
		},
	}
	report.Summary = doctor.ComputeSummary(report.Checks)

	if !report.Summary.OK {
		t.Error("Summary.OK should be true when only warnings, no failures")
	}
	if report.Summary.Warn != 1 {
		t.Errorf("Summary.Warn: got %d, want 1", report.Summary.Warn)
	}
}

func TestSummary_NotOK_OnFail(t *testing.T) {
	report := doctor.Report{
		Checks: []doctor.Check{
			{ID: "binary.executable", Status: doctor.StatusFail},
		},
	}
	report.Summary = doctor.ComputeSummary(report.Checks)

	if report.Summary.OK {
		t.Error("Summary.OK should be false when there are failures")
	}
}

// --- JSON output mode ---

func TestRun_JSONOutput(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }
	deps.StatFileFunc = func(path string) (bool, error) { return false, nil }
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }
	deps.ReadMarkerFunc = func(path string) (string, error) {
		return `{"version_installed":"0.1.0","installed_at":"2026-06-04T00:00:00Z"}`, nil
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// JSON round-trip: marshal and unmarshal, verify key fields.
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded doctor.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(decoded.Checks) != len(report.Checks) {
		t.Errorf("JSON round-trip: checks count mismatch: got %d want %d", len(decoded.Checks), len(report.Checks))
	}

	// Verify specific JSON keys via direct unmarshal into a map.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal to map: %v", err)
	}
	if _, ok := raw["checks"]; !ok {
		t.Error("JSON should have 'checks' key")
	}
	if _, ok := raw["summary"]; !ok {
		t.Error("JSON should have 'summary' key")
	}

	checksRaw, ok := raw["checks"].([]interface{})
	if !ok {
		t.Fatal("JSON checks should be an array")
	}
	if len(checksRaw) == 0 {
		t.Error("JSON checks array should not be empty")
	}

	firstCheck, ok := checksRaw[0].(map[string]interface{})
	if !ok {
		t.Fatal("first check should be an object")
	}
	requiredKeys := []string{"id", "title", "status", "details"}
	for _, k := range requiredKeys {
		if _, ok := firstCheck[k]; !ok {
			t.Errorf("check JSON missing key %q", k)
		}
	}
}

// --- verbose option (no errors expected) ---

func TestRun_Verbose_DoesNotError(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")

	_, err := doctor.Run(context.Background(), deps, doctor.Options{Verbose: true})
	if err != nil {
		t.Fatalf("Run with Verbose failed: %v", err)
	}
}

// --- helpers ---

// findCheck asserts that a check with the given ID exists and returns it.
func findCheck(t *testing.T, report doctor.Report, id string) doctor.Check {
	t.Helper()
	for _, c := range report.Checks {
		if string(c.ID) == id {
			return c
		}
	}
	t.Fatalf("check %q not found in report; available: %v", id, checkIDs(report))
	return doctor.Check{}
}

// findCheckNoFail returns a pointer to the check if found, nil otherwise.
func findCheckNoFail(report doctor.Report, id string) *doctor.Check {
	for i, c := range report.Checks {
		if string(c.ID) == id {
			return &report.Checks[i]
		}
	}
	return nil
}

func checkIDs(report doctor.Report) []string {
	ids := make([]string, len(report.Checks))
	for i, c := range report.Checks {
		ids[i] = string(c.ID)
	}
	return ids
}

// markerNotFoundError is a sentinel error for "marker file not found".
type markerNotFoundError struct{ path string }

func (e *markerNotFoundError) Error() string { return "marker not found: " + e.path }

// errNetwork is a simple error type for network failures in tests.
type networkError string

func errNetwork(msg string) error { return networkError(msg) }

func (e networkError) Error() string { return string(e) }
