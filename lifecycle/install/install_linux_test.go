//go:build linux

package install_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/install"
	"github.com/fede-iglesias/shipkit/ports"
)

// TestBash32Linux_NoSkip asserts that bash 3.2 on linux does NOT trigger the
// darwin-specific brew warning and does NOT skip bash completions. The bash 3.2
// skip rule is darwin-only (system-default bash limitation specific to macOS).
func TestBash32Linux_NoSkip(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.OSResult = "linux"
	env.ShellResult = ports.ShellBash
	env.Env["BASH_VERSION"] = "3.2.57(1)-release"

	var stderrBuf bytes.Buffer
	opts := install.Options{
		Completions: []ports.ShellKind{ports.ShellBash},
		Stderr:      &stderrBuf,
	}

	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Bash completion must NOT be skipped on linux.
	mockCP := deps.Completion.(*ports.MockCompletionPort)
	found := false
	for _, shell := range mockCP.EmitCompletionCalls {
		if shell == ports.ShellBash {
			found = true
		}
	}
	if !found {
		t.Error("bash completion should be emitted on linux regardless of bash version")
	}

	// No brew warning on linux.
	if bytes.Contains(stderrBuf.Bytes(), []byte("brew install bash")) {
		t.Errorf("unexpected brew warning on linux: %q", stderrBuf.String())
	}
}
