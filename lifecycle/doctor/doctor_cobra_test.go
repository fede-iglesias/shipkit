package doctor_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/doctor"
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

func TestNewCommand_FlagsRegistered(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	cmd := doctor.NewCommand(deps)
	if cmd == nil {
		t.Fatal("NewCommand returned nil")
	}

	// Verify expected flags are registered.
	flags := []string{"network", "json", "verbose"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("flag --%s not registered", f)
		}
	}
}

func TestNewCommand_Use(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	cmd := doctor.NewCommand(deps)

	if cmd.Use != "doctor" {
		t.Errorf("Use: got %q, want %q", cmd.Use, "doctor")
	}
}

func TestNewCommand_RunsOK(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }
	deps.StatFileFunc = func(path string) (bool, error) { return false, nil }
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }
	deps.ReadMarkerFunc = func(path string) (string, error) {
		return `{"version_installed":"0.1.0","installed_at":"2026-06-04T00:00:00Z"}`, nil
	}

	cmd := doctor.NewCommand(deps)
	root := &cobra.Command{Use: "myapp"}
	root.AddCommand(cmd)

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"doctor"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()
	// Should print at least one [PASS] or [WARN] line.
	if !strings.Contains(output, "[PASS]") && !strings.Contains(output, "[WARN]") {
		t.Errorf("output should contain [PASS] or [WARN], got: %s", output)
	}
	// Should print summary line.
	if !strings.Contains(output, "Summary:") {
		t.Errorf("output should contain 'Summary:', got: %s", output)
	}
}

func TestNewCommand_JSONOutput(t *testing.T) {
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatDirFunc = func(path string) (bool, error) { return true, nil }
	deps.StatFileFunc = func(path string) (bool, error) { return false, nil }
	deps.StatExecutableFunc = func(path string) (bool, error) { return true, nil }
	deps.ReadMarkerFunc = func(path string) (string, error) {
		return `{"version_installed":"0.1.0","installed_at":"2026-06-04T00:00:00Z"}`, nil
	}

	cmd := doctor.NewCommand(deps)
	root := &cobra.Command{Use: "myapp"}
	root.AddCommand(cmd)

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"doctor", "--json"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute with --json failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"checks"`) {
		t.Errorf("--json output should contain 'checks' key, got: %s", output)
	}
	if !strings.Contains(output, `"summary"`) {
		t.Errorf("--json output should contain 'summary' key, got: %s", output)
	}
}

func TestNewCommand_ExitCode_Warn(t *testing.T) {
	// Warn-only run: exit code should be 0.
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.Paths.(*ports.MockPathsPort).InPATHResult = false // triggers binary.in-path warn

	cmd := doctor.NewCommand(deps)

	var exitCode int
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		report, err := doctor.Run(context.Background(), deps, doctor.Options{})
		if err != nil {
			return err
		}
		exitCode = doctor.ExitCode(report)
		return nil
	}

	root := &cobra.Command{Use: "myapp"}
	root.AddCommand(cmd)
	root.SetArgs([]string{"doctor"})
	_ = root.Execute()

	if exitCode != 0 {
		t.Errorf("ExitCode with warn-only: got %d, want 0", exitCode)
	}
}

func TestNewCommand_ExitCode_Fail(t *testing.T) {
	// Fail run: exit code should be 1.
	deps := newDeps("myapp", "/usr/local/bin/myapp", "0.1.0")
	deps.StatExecutableFunc = func(path string) (bool, error) { return false, nil } // binary.executable fail

	cmd := doctor.NewCommand(deps)

	var exitCode int
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		report, err := doctor.Run(context.Background(), deps, doctor.Options{})
		if err != nil {
			return err
		}
		exitCode = doctor.ExitCode(report)
		return nil
	}

	root := &cobra.Command{Use: "myapp"}
	root.AddCommand(cmd)
	root.SetArgs([]string{"doctor"})
	_ = root.Execute()

	if exitCode != 1 {
		t.Errorf("ExitCode with failure: got %d, want 1", exitCode)
	}
}
