package ports_test

import (
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockEnvPort satisfies EnvPort.
var _ ports.EnvPort = (*ports.MockEnvPort)(nil)

func TestMockEnvPort_defaults(t *testing.T) {
	m := ports.NewMockEnvPort()

	if m.DetectShell() != ports.ShellZsh {
		t.Errorf("expected zsh, got %q", m.DetectShell())
	}
	if m.DetectOS() != "darwin" {
		t.Errorf("expected darwin, got %q", m.DetectOS())
	}
	if m.DetectArch() != "arm64" {
		t.Errorf("expected arm64, got %q", m.DetectArch())
	}
	if m.Username() != "testuser" {
		t.Errorf("expected testuser, got %q", m.Username())
	}
}

func TestMockEnvPort_Get_set(t *testing.T) {
	m := ports.NewMockEnvPort()
	m.Env["HOME"] = "/home/alice"
	if m.Get("HOME") != "/home/alice" {
		t.Error("expected /home/alice")
	}
	if m.Get("MISSING") != "" {
		t.Error("expected empty string for missing key")
	}
}

func TestMockEnvPort_Lookup(t *testing.T) {
	m := ports.NewMockEnvPort()
	m.Env["SHELL"] = "/bin/zsh"

	v, ok := m.Lookup("SHELL")
	if !ok || v != "/bin/zsh" {
		t.Errorf("expected /bin/zsh, got %q %v", v, ok)
	}

	_, ok = m.Lookup("UNSET")
	if ok {
		t.Error("expected not ok for unset key")
	}
}

func TestShellKindConstants(t *testing.T) {
	// Verify the string values of ShellKind constants are stable (other packages
	// may persist these values in marker files).
	cases := []struct {
		kind ports.ShellKind
		want string
	}{
		{ports.ShellBash, "bash"},
		{ports.ShellZsh, "zsh"},
		{ports.ShellFish, "fish"},
		{ports.ShellUnknown, "unknown"},
	}
	for _, c := range cases {
		if string(c.kind) != c.want {
			t.Errorf("ShellKind %v: want %q, got %q", c.kind, c.want, string(c.kind))
		}
	}
}

func TestMockEnvPort_overrideShell(t *testing.T) {
	m := ports.NewMockEnvPort()
	m.ShellResult = ports.ShellBash
	if m.DetectShell() != ports.ShellBash {
		t.Errorf("expected bash, got %q", m.DetectShell())
	}
}
