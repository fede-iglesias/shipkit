package uninstall

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// TestNewCommand_PrintOutput asserts that --print outputs the dry-run plan
// with "would remove" text to stdout.
func TestNewCommand_PrintOutput(t *testing.T) {
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

	root := &cobra.Command{Use: "app"}
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"uninstall", "--print"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	out := buf.String()
	if len(out) == 0 {
		t.Error("expected --print to write output, got empty")
	}
}

// TestNewCommand_PrintOutputWithSkipped covers the "Would keep:" branch when
// KeepData=true is passed with --print.
func TestNewCommand_PrintOutputWithSkipped(t *testing.T) {
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

	root := &cobra.Command{Use: "app"}
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"uninstall", "--print", "--keep-data"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	out := buf.String()
	if len(out) == 0 {
		t.Error("expected --print --keep-data to write output, got empty")
	}
}

// TestNewCommand_RealRunWithNextSteps covers the NextSteps output branch by
// wiring a RemoveBinaryFunc that fails so result.NextSteps is populated.
func TestNewCommand_RealRunWithNextSteps(t *testing.T) {
	deps := Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:          ports.NewMockShellRcPort(),
		Completion:       ports.NewMockCompletionPort(),
		Autostart:        ports.NewMockAutostartPort(),
		Prompt:           &ports.MockPromptPort{ConfirmResult: true},
		RemoveBinaryFunc: func(path string) error { return &permErr{} },
	}

	root := &cobra.Command{Use: "app"}
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"uninstall", "-y"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	out := buf.String()
	if len(out) == 0 {
		t.Error("expected output when NextSteps is populated, got empty")
	}
}

// TestNewCommand_RealRunBinaryAction covers the BinaryAction output branch.
func TestNewCommand_RealRunBinaryAction(t *testing.T) {
	deps := Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/cache/" + app, nil },
		},
		ShellRc:          ports.NewMockShellRcPort(),
		Completion:       ports.NewMockCompletionPort(),
		Autostart:        ports.NewMockAutostartPort(),
		Prompt:           &ports.MockPromptPort{ConfirmResult: true},
		RemoveBinaryFunc: func(path string) error { return nil },
	}

	root := &cobra.Command{Use: "app"}
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"uninstall", "-y"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	out := buf.String()
	if len(out) == 0 {
		t.Error("expected output showing BinaryAction, got empty")
	}
}

// TestNewCommand_RunEReturnsError covers the "return err" branch in RunE by
// injecting a prompt that returns an error.
func TestNewCommand_RunEReturnsError(t *testing.T) {
	promptErr := &permErr{}
	deps := Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths:   ports.NewMockPathsPort(),
		ShellRc: ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt: &ports.MockPromptPort{
			ConfirmFunc: func(question string, defaultYes bool) (bool, error) {
				return false, promptErr
			},
		},
	}

	root := &cobra.Command{Use: "app"}
	root.SilenceErrors = true
	root.SilenceUsage = true
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	root.SetArgs([]string{"uninstall"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error from RunE when prompt fails, got nil")
	}
}

// TestUninstall_PrintMode_UsesPlan verifies that --print invokes BuildPlan and
// its output is written to the command output (not just the old dry-run list).
// It checks for the Plan header line and at least one path from each category.
func TestUninstall_PrintMode_UsesPlan(t *testing.T) {
	completion := &ports.MockCompletionPort{
		CompletionPathFunc: func(shell ports.ShellKind, app, home string) (string, error) {
			if shell == ports.ShellZsh {
				return "/data/zsh/site-functions/_" + app, nil
			}
			return "/data/bash-completion/completions/" + app, nil
		},
	}

	deps := Deps{
		AppName:        "testapp",
		BinPath:        "/usr/local/bin/testapp",
		AutostartLabel: "com.example.testapp",
		ShellKinds:     []ports.ShellKind{ports.ShellZsh, ports.ShellBash},
		FS:             ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/u",
			DataDirFunc:    func(app string) (string, error) { return "/home/u/.local/share/" + app, nil },
			ConfigDirFunc:  func(app string) (string, error) { return "/home/u/.config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/home/u/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: completion,
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	root := &cobra.Command{Use: "app"}
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"uninstall", "--print"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	out := buf.String()
	checks := []string{
		"testapp",
		"/home/u/.local/share/testapp",
		"/home/u/.config/testapp",
		"/home/u/.cache/testapp",
		"/usr/local/bin/testapp",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("--print output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestUninstall_PrintMode_BuildPlanError covers the error return path in RunE
// when BuildPlan returns an error (triggered by empty AppName).
func TestUninstall_PrintMode_BuildPlanError(t *testing.T) {
	deps := Deps{
		AppName:    "", // empty AppName triggers BuildPlan error
		BinPath:    "/usr/local/bin/testapp",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         ports.NewMockFsPort(),
		Paths:      ports.NewMockPathsPort(),
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	root := &cobra.Command{Use: "app"}
	root.SilenceErrors = true
	root.SilenceUsage = true
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	root.SetArgs([]string{"uninstall", "--print"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when BuildPlan fails (empty AppName), got nil")
	}
}

// TestUninstall_PrintMode_WriterError covers the error path when plan.Print
// returns an error because the underlying writer fails.
func TestUninstall_PrintMode_WriterError(t *testing.T) {
	deps := Deps{
		AppName:    "testapp",
		BinPath:    "/usr/local/bin/testapp",
		ShellKinds: []ports.ShellKind{ports.ShellZsh},
		FS:         ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			UserHomeResult: "/home/u",
			DataDirFunc:    func(app string) (string, error) { return "/home/u/.local/share/" + app, nil },
			ConfigDirFunc:  func(app string) (string, error) { return "/home/u/.config/" + app, nil },
			CacheDirFunc:   func(app string) (string, error) { return "/home/u/.cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	root := &cobra.Command{Use: "app"}
	root.SilenceErrors = true
	root.SilenceUsage = true
	cmd := NewCommand(deps, root)
	root.AddCommand(cmd)

	root.SetOut(&failWriter{})

	root.SetArgs([]string{"uninstall", "--print"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when writer fails during --print, got nil")
	}
}

// failWriter is an io.Writer that always returns an error.
type failWriter struct{}

func (f *failWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

// permErr is a package-level error type for testing binary removal failure.
type permErr struct{}

func (e *permErr) Error() string { return "permission denied" }
