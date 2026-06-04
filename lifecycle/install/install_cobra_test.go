package install_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/install"
)

// TestNewCommand_WiresFlags asserts that NewCommand returns a cobra.Command with
// all required flags: --force, --autostart, --completions, --print, and -y.
func TestNewCommand_WiresFlags(t *testing.T) {
	deps, _ := newDeps(t)
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand returned error: %v", err)
	}
	if cmd == nil {
		t.Fatal("NewCommand returned nil command")
	}

	required := []string{"force", "autostart", "completions", "print"}
	for _, name := range required {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("flag --%s not found on install command", name)
		}
	}

	// -y shorthand for --yes.
	if f := cmd.Flags().ShorthandLookup("y"); f == nil {
		t.Error("shorthand -y not found (expected for --yes flag)")
	}
}

// TestNewCommand_UseAndShortDesc asserts the cobra command has Use = "install"
// and a non-empty Short description.
func TestNewCommand_UseAndShortDesc(t *testing.T) {
	deps, _ := newDeps(t)
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand returned error: %v", err)
	}
	if cmd.Use != "install" {
		t.Errorf("Use = %q; want %q", cmd.Use, "install")
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}
}

// TestNewCommand_Execute_HappyPath exercises the cobra RunE closure on a
// clean first-install path, covering the command execution branches.
func TestNewCommand_Execute_HappyPath(t *testing.T) {
	deps, _ := newDeps(t)
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand error: %v", err)
	}
	cmd.SetContext(context.Background())

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Execute without any flags - default first install.
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if out.Len() == 0 {
		t.Log("no output (acceptable - cobra may not print on success)")
	}
}

// TestNewCommand_Execute_AlreadyInstalled exercises the AlreadyInstalled branch.
func TestNewCommand_Execute_AlreadyInstalled(t *testing.T) {
	deps, _ := newDeps(t)
	root := newRootCmd()

	// First install via Run to create the marker.
	if _, err := install.Run(context.Background(), deps, install.Options{}, root); err != nil {
		t.Fatalf("first install error: %v", err)
	}

	// Now build the command with the same deps (marker already present).
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand error: %v", err)
	}
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if outStr := out.String(); len(outStr) > 0 {
		t.Logf("output: %q", outStr) // acceptable: message about already installed
	}
}

// TestNewCommand_Execute_WithCompletionsFlag exercises the --completions flag
// parsing branch in RunE.
func TestNewCommand_Execute_WithCompletionsFlag(t *testing.T) {
	deps, _ := newDeps(t)
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand error: %v", err)
	}
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--completions", "zsh,bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute with --completions error: %v", err)
	}
}

// TestNewCommand_Execute_PrintFlag exercises the --print dry-run branch.
func TestNewCommand_Execute_PrintFlag(t *testing.T) {
	deps, _ := newDeps(t)
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand error: %v", err)
	}
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--print"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute --print error: %v", err)
	}
}

// TestNewCommand_Execute_Error exercises the error-return path in RunE
// (requesting --autostart without Config.EnableAutostart).
func TestNewCommand_Execute_Error(t *testing.T) {
	deps, _ := newDeps(t)
	// cfg.EnableAutostart is false by default.
	cmd, err := install.NewCommand(deps)
	if err != nil {
		t.Fatalf("NewCommand error: %v", err)
	}
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--autostart"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error from RunE when --autostart requested without config")
	}
}
