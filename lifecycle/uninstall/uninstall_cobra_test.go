package uninstall_test

import (
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/uninstall"
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// TestNewCommand_FlagsExist asserts that NewCommand registers all required
// flags: --keep-data, --keep-config, --keep-binary, -y/--yes, --print.
func TestNewCommand_FlagsExist(t *testing.T) {
	deps := uninstall.Deps{
		AppName:    "testapp",
		BinPath:    "/usr/local/bin/testapp",
		FS:         ports.NewMockFsPort(),
		Paths:      ports.NewMockPathsPort(),
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	cmd := uninstall.NewCommand(deps, nil)

	required := []string{"keep-data", "keep-config", "keep-binary", "yes", "print"}
	for _, name := range required {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("flag --%s not registered on uninstall command", name)
		}
	}

	// -y must be a shorthand for --yes.
	yFlag := cmd.Flags().ShorthandLookup("y")
	if yFlag == nil {
		t.Error("shorthand flag -y not registered")
	}
}

// TestNewCommand_UseAndShort asserts the command name is "uninstall" and has a
// non-empty Short description.
func TestNewCommand_UseAndShort(t *testing.T) {
	deps := uninstall.Deps{
		AppName:    "testapp",
		BinPath:    "/usr/local/bin/testapp",
		FS:         ports.NewMockFsPort(),
		Paths:      ports.NewMockPathsPort(),
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	cmd := uninstall.NewCommand(deps, nil)

	if cmd.Use != "uninstall" {
		t.Errorf("command Use: got %q, want %q", cmd.Use, "uninstall")
	}
	if cmd.Short == "" {
		t.Error("command Short must be non-empty")
	}
}

// TestNewCommand_ExecYes asserts that running the command with -y flag
// proceeds without error (prompt skipped, mock ports return nil).
func TestNewCommand_ExecYes(t *testing.T) {
	deps := uninstall.Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	root := &cobra.Command{Use: "app"}
	cmd := uninstall.NewCommand(deps, root)
	root.AddCommand(cmd)

	root.SetArgs([]string{"uninstall", "-y"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute with -y returned error: %v", err)
	}
}

// TestNewCommand_ExecPrint asserts that --print dry-run returns without error.
func TestNewCommand_ExecPrint(t *testing.T) {
	deps := uninstall.Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	root := &cobra.Command{Use: "app"}
	cmd := uninstall.NewCommand(deps, root)
	root.AddCommand(cmd)

	root.SetArgs([]string{"uninstall", "--print"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute with --print returned error: %v", err)
	}
}

// TestNewCommand_ExecKeepFlags asserts that --keep-data, --keep-config, and
// --keep-binary can be passed without error.
func TestNewCommand_ExecKeepFlags(t *testing.T) {
	deps := uninstall.Deps{
		AppName: "testapp",
		BinPath: "/usr/local/bin/testapp",
		FS:      ports.NewMockFsPort(),
		Paths: &ports.MockPathsPort{
			DataDirFunc:   func(app string) (string, error) { return "/tmp/data/" + app, nil },
			ConfigDirFunc: func(app string) (string, error) { return "/tmp/config/" + app, nil },
			CacheDirFunc:  func(app string) (string, error) { return "/tmp/cache/" + app, nil },
		},
		ShellRc:    ports.NewMockShellRcPort(),
		Completion: ports.NewMockCompletionPort(),
		Autostart:  ports.NewMockAutostartPort(),
		Prompt:     ports.NewMockPromptPort(),
	}

	root := &cobra.Command{Use: "app"}
	cmd := uninstall.NewCommand(deps, root)
	root.AddCommand(cmd)

	root.SetArgs([]string{"uninstall", "--keep-data", "--keep-config", "--keep-binary", "-y"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute with --keep-* flags returned error: %v", err)
	}
}
