// Package doctor_internal_test provides white-box tests for unexported helpers
// and internal error paths that black-box tests cannot reach cleanly.
package doctor

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
)

// --- checkBinaryExecutable: error path ---

func TestCheckBinaryExecutable_Error(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		BinPath: "/usr/local/bin/myapp",
		StatExecutableFunc: func(path string) (bool, error) {
			return false, fmt.Errorf("permission denied")
		},
	}
	c := checkBinaryExecutable(deps)
	if c.Status != StatusFail {
		t.Errorf("checkBinaryExecutable error: got %s, want fail", c.Status)
	}
	if !strings.Contains(c.Details, "permission denied") {
		t.Errorf("details should mention error, got: %s", c.Details)
	}
}

// --- checkBinaryVersion: !result.Ok ---

func TestCheckBinaryVersion_NotOk(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		BinPath: "/usr/local/bin/myapp",
		Version: "0.1.0",
		Spawn:   makeMockSpawnNotOk(),
	}
	c := checkBinaryVersion(context.Background(), deps)
	if c.Status != StatusFail {
		t.Errorf("checkBinaryVersion !ok: got %s, want fail", c.Status)
	}
}

func makeMockSpawnNotOk() *ports.MockSpawnPort {
	m := ports.NewMockSpawnPort()
	m.HealthCheckFunc = func(ctx context.Context, p string, d time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{Ok: false, Reason: "exit status 1"}, nil
	}
	return m
}

// --- checkBinaryVersion: HealthCheck returns error ---

func TestCheckBinaryVersion_HealthCheckError(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		BinPath: "/usr/local/bin/myapp",
		Version: "0.1.0",
		Spawn:   makeMockSpawnError(),
	}
	c := checkBinaryVersion(context.Background(), deps)
	if c.Status != StatusFail {
		t.Errorf("checkBinaryVersion health error: got %s, want fail", c.Status)
	}
	if !strings.Contains(c.Details, "health check failed") {
		t.Errorf("details should mention health check, got: %s", c.Details)
	}
}

func makeMockSpawnError() *ports.MockSpawnPort {
	m := ports.NewMockSpawnPort()
	m.HealthCheckFunc = func(ctx context.Context, p string, d time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{}, fmt.Errorf("binary not found")
	}
	return m
}

// --- checkDir: error path ---

func TestCheckDir_StatError(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		StatDirFunc: func(path string) (bool, error) {
			return false, fmt.Errorf("I/O error")
		},
	}
	c := checkDir("xdg.data-dir", "XDG data directory", "/tmp/data", "myapp", deps)
	if c.Status != StatusFail {
		t.Errorf("checkDir stat error: got %s, want fail", c.Status)
	}
}

// --- checkMarker: bad JSON ---

func TestCheckMarker_BadJSON(t *testing.T) {
	deps := Deps{
		AppName:  "myapp",
		DataRoot: "/tmp/data",
		Version:  "0.1.0",
		ReadMarkerFunc: func(path string) (string, error) {
			return "not-json", nil
		},
	}
	c := checkMarker(deps)
	if c.Status != StatusWarn {
		t.Errorf("checkMarker bad JSON: got %s, want warn", c.Status)
	}
	if !strings.Contains(c.Details, "not valid JSON") {
		t.Errorf("details should mention JSON error, got: %s", c.Details)
	}
}

// --- checkCompletion: CompletionPath error ---

func TestCheckCompletion_PathError(t *testing.T) {
	mockCompletion := ports.NewMockCompletionPort()
	mockCompletion.CompletionPathFunc = func(shell ports.ShellKind, app, home string) (string, error) {
		return "", fmt.Errorf("unsupported shell")
	}

	deps := Deps{
		AppName:    "myapp",
		Completion: mockCompletion,
		Env:        makeEnvWithShell(ports.ShellBash),
		Paths:      ports.NewMockPathsPort(),
	}
	checks := checkCompletion(deps)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Status != StatusWarn {
		t.Errorf("completion path error: got %s, want warn", checks[0].Status)
	}
}

// --- checkCompletion: StatFileFunc error ---

func TestCheckCompletion_StatFileError(t *testing.T) {
	deps := Deps{
		AppName:    "myapp",
		Completion: ports.NewMockCompletionPort(),
		Env:        makeEnvWithShell(ports.ShellZsh),
		Paths:      ports.NewMockPathsPort(),
		StatFileFunc: func(path string) (bool, error) {
			return false, fmt.Errorf("I/O error checking file")
		},
	}
	checks := checkCompletion(deps)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Status != StatusWarn {
		t.Errorf("completion stat error: got %s, want warn", checks[0].Status)
	}
}

// --- checkAutostart: Status error ---

func TestCheckAutostart_StatusError(t *testing.T) {
	mockAutostart := ports.NewMockAutostartPort()
	mockAutostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{}, fmt.Errorf("launchctl error")
	}
	deps := Deps{
		AppName:        "myapp",
		AutostartLabel: "com.fede-iglesias.myapp",
		Autostart:      mockAutostart,
	}
	c := checkAutostart(deps)
	if c.Status != StatusWarn {
		t.Errorf("checkAutostart status error: got %s, want warn", c.Status)
	}
}

// --- checkRecoveryManifest: stat error ---

func TestCheckRecoveryManifest_StatError(t *testing.T) {
	deps := Deps{
		AppName:  "myapp",
		DataRoot: "/tmp/data",
		StatFileFunc: func(path string) (bool, error) {
			return false, fmt.Errorf("I/O error")
		},
	}
	c := checkRecoveryManifest(deps)
	if c.Status != StatusWarn {
		t.Errorf("recovery manifest stat error: got %s, want warn", c.Status)
	}
}

// --- checkNetworkCosignTUF: not wired ---

func TestCheckNetworkCosignTUF_NotWired(t *testing.T) {
	deps := Deps{
		AppName:                   "myapp",
		CheckNetworkCosignTUFFunc: nil,
	}
	c := checkNetworkCosignTUF(context.Background(), deps)
	if c.Status != StatusWarn {
		t.Errorf("cosign-tuf not wired: got %s, want warn", c.Status)
	}
}

// --- checkNetworkCosignTUF: fail ---

func TestCheckNetworkCosignTUF_Fail(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		CheckNetworkCosignTUFFunc: func(ctx context.Context) error {
			return fmt.Errorf("TUF unreachable")
		},
	}
	c := checkNetworkCosignTUF(context.Background(), deps)
	if c.Status != StatusFail {
		t.Errorf("cosign-tuf fail: got %s, want fail", c.Status)
	}
}

// --- checkNetworkUpdateFeed: not wired ---

func TestCheckNetworkUpdateFeed_NotWired(t *testing.T) {
	deps := Deps{
		AppName:                    "myapp",
		CheckNetworkUpdateFeedFunc: nil,
	}
	c := checkNetworkUpdateFeed(context.Background(), deps)
	if c.Status != StatusWarn {
		t.Errorf("update-feed not wired: got %s, want warn", c.Status)
	}
}

// --- checkNetworkUpdateFeed: fail ---

func TestCheckNetworkUpdateFeed_Fail(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		CheckNetworkUpdateFeedFunc: func(ctx context.Context) (string, error) {
			return "", fmt.Errorf("network error")
		},
	}
	c := checkNetworkUpdateFeed(context.Background(), deps)
	if c.Status != StatusFail {
		t.Errorf("update-feed fail: got %s, want fail", c.Status)
	}
}

// --- checkNetworkUpdateFeed: newer version available ---

func TestCheckNetworkUpdateFeed_NewerVersion(t *testing.T) {
	deps := Deps{
		AppName: "myapp",
		Version: "0.1.0",
		CheckNetworkUpdateFeedFunc: func(ctx context.Context) (string, error) {
			return "0.2.0", nil
		},
	}
	c := checkNetworkUpdateFeed(context.Background(), deps)
	if c.Status != StatusPass {
		t.Errorf("update-feed newer: got %s, want pass", c.Status)
	}
	if !strings.Contains(c.Details, "0.2.0") {
		t.Errorf("details should mention newer version, got: %s", c.Details)
	}
}

// --- FormatText: verbose and pass tag ---

func TestFormatText_Verbose(t *testing.T) {
	report := Report{
		Checks: []Check{
			{ID: "binary.in-path", Status: StatusPass, Details: "all good"},
			{ID: "xdg.data-dir", Status: StatusWarn, Details: "missing", Hint: "run install"},
			{ID: "binary.executable", Status: StatusFail, Details: "no exec bits", Hint: "chmod +x"},
			{ID: "network.github", Status: StatusSkipped, Details: "use --network"},
		},
		Summary: ComputeSummary([]Check{
			{Status: StatusPass},
			{Status: StatusWarn},
			{Status: StatusFail},
			{Status: StatusSkipped},
		}),
	}

	out := FormatText(report, true)
	if !strings.Contains(out, "[PASS]") {
		t.Error("FormatText should contain [PASS]")
	}
	if !strings.Contains(out, "[WARN]") {
		t.Error("FormatText should contain [WARN]")
	}
	if !strings.Contains(out, "[FAIL]") {
		t.Error("FormatText should contain [FAIL]")
	}
	if !strings.Contains(out, "[SKIP]") {
		t.Error("FormatText should contain [SKIP]")
	}
	// Verbose shows hints for pass checks too (just no hint on pass).
	if !strings.Contains(out, "run install") {
		t.Error("FormatText verbose should show warn hint")
	}
	if !strings.Contains(out, "FAIL") {
		t.Error("FormatText should have FAIL at end")
	}
}

func TestFormatText_OK(t *testing.T) {
	report := Report{
		Checks:  []Check{{ID: "binary.in-path", Status: StatusPass, Details: "ok"}},
		Summary: Summary{Pass: 1, OK: true},
	}
	out := FormatText(report, false)
	if !strings.Contains(out, "OK") {
		t.Errorf("FormatText OK: should contain OK, got: %s", out)
	}
}

// --- ExitError.Error ---

func TestExitError_Error(t *testing.T) {
	e := &ExitError{
		Code:   1,
		Report: Report{Summary: Summary{Fail: 3}},
	}
	msg := e.Error()
	if !strings.Contains(msg, "3") {
		t.Errorf("ExitError.Error should mention count, got: %s", msg)
	}
}

// --- helpers ---

func makeEnvWithShell(shell ports.ShellKind) *ports.MockEnvPort {
	e := ports.NewMockEnvPort()
	e.ShellResult = shell
	return e
}
