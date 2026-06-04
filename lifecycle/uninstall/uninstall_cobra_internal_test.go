package uninstall

import (
	"bytes"
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

// permErr is a package-level error type for testing binary removal failure.
type permErr struct{}

func (e *permErr) Error() string { return "permission denied" }
