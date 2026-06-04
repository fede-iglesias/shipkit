package install_test

import (
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
