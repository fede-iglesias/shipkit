package doctor_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/doctor"
	"github.com/fede-iglesias/shipkit/ports"
)

// ExampleRun demonstrates how to call Run with mock ports for all checks.
// In a real consumer the Func fields are wired to os.Stat/os.ReadFile
// calls in the cmd layer.
func ExampleRun() {
	// Set up mock ports with healthy defaults.
	mockPaths := ports.NewMockPathsPort()
	mockPaths.ExecutableResult = "/usr/local/bin/myapp"
	mockPaths.InPATHResult = true

	mockSpawn := ports.NewMockSpawnPort()
	mockSpawn.HealthCheckFunc = func(ctx context.Context, p string, d time.Duration) (ports.HealthResult, error) {
		return ports.HealthResult{Ok: true, Version: "0.1.0"}, nil
	}

	mockAutostart := ports.NewMockAutostartPort()
	mockAutostart.StatusFunc = func(label string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: false, Running: false}, nil
	}

	deps := doctor.Deps{
		AppName:    "myapp",
		BinPath:    "/usr/local/bin/myapp",
		Version:    "0.1.0",
		DataRoot:   "/home/user/.local/share/myapp",
		ConfigRoot: "/home/user/.config/myapp",
		CacheRoot:  "/home/user/.cache/myapp",
		HTTP:       ports.NewMockHTTPPort(),
		FS:         ports.NewMockFsPort(),
		Spawn:      mockSpawn,
		Paths:      mockPaths,
		Env:        ports.NewMockEnvPort(),
		Autostart:  mockAutostart,
		Completion: ports.NewMockCompletionPort(),
		Clock:      ports.NewMockClockPort(time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)),
		// Wire stat functions (production: use os.Stat).
		StatExecutableFunc: func(path string) (bool, error) { return true, nil },
		StatDirFunc:        func(path string) (bool, error) { return true, nil },
		// StatFileFunc: completion file exists, recovery manifest absent (healthy).
		StatFileFunc: func(path string) (bool, error) {
			// Healthy state: no pending recovery manifest.
			if strings.Contains(path, "recovery-manifest") {
				return false, nil
			}
			return true, nil
		},
		ReadMarkerFunc: func(path string) (string, error) {
			return `{"version_installed":"0.1.0","installed_at":"2026-06-04T00:00:00Z"}`, nil
		},
	}

	report, err := doctor.Run(context.Background(), deps, doctor.Options{})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	s := report.Summary
	fmt.Printf("pass=%d warn=%d fail=%d skipped=%d ok=%v\n",
		s.Pass, s.Warn, s.Fail, s.Skipped, s.OK)
	// Output:
	// pass=10 warn=0 fail=0 skipped=3 ok=true
}

// ExampleExitCode shows how to convert a Report into a process exit code.
func ExampleExitCode() {
	reportOK := doctor.Report{
		Summary: doctor.Summary{Pass: 5, Warn: 1, OK: true},
	}
	reportFail := doctor.Report{
		Summary: doctor.Summary{Pass: 4, Fail: 1, OK: false},
	}
	fmt.Println(doctor.ExitCode(reportOK))
	fmt.Println(doctor.ExitCode(reportFail))
	// Output:
	// 0
	// 1
}

// ExampleComputeSummary shows how check counts are aggregated.
func ExampleComputeSummary() {
	checks := []doctor.Check{
		{ID: "binary.in-path", Status: doctor.StatusPass},
		{ID: "binary.executable", Status: doctor.StatusPass},
		{ID: "xdg.data-dir", Status: doctor.StatusWarn},
		{ID: "recovery.manifest", Status: doctor.StatusFail},
		{ID: "network.github", Status: doctor.StatusSkipped},
	}
	s := doctor.ComputeSummary(checks)
	fmt.Printf("pass=%d warn=%d fail=%d skipped=%d ok=%v\n",
		s.Pass, s.Warn, s.Fail, s.Skipped, s.OK)
	// Output:
	// pass=2 warn=1 fail=1 skipped=1 ok=false
}
