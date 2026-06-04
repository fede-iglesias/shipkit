package install_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/install"
	"github.com/fede-iglesias/shipkit/ports"
)

// TestBash32Darwin_SkipWithBrewWarn asserts that requesting bash completions on
// darwin when BASH_VERSION starts with "3." skips the bash completion step and
// emits a warning to stderr mentioning "brew install bash".
//
// macOS ships Bash 3.2 (GPLv2). Bash >= 4 is required for programmable
// completion. The install verb must detect this via EnvPort and warn rather than
// silently install a broken completion script.
func TestBash32Darwin_SkipWithBrewWarn(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.OSResult = "darwin"
	env.ShellResult = ports.ShellBash
	// Simulate darwin's default Bash 3.2.
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

	// Bash completion must be SKIPPED.
	mockCP := deps.Completion.(*ports.MockCompletionPort)
	for _, shell := range mockCP.EmitCompletionCalls {
		if shell == ports.ShellBash {
			t.Error("bash completion was emitted on darwin bash 3.2; should be skipped")
		}
	}

	// Warning must mention "brew install bash".
	warn := stderrBuf.String()
	if !bytes.Contains([]byte(warn), []byte("brew install bash")) {
		t.Errorf("expected stderr warning containing %q\ngot: %q", "brew install bash", warn)
	}
}

// TestBash4Darwin_NoSkip asserts that bash >= 4 on darwin does NOT skip
// completion install and produces no brew warning.
func TestBash4Darwin_NoSkip(t *testing.T) {
	deps, _ := newDeps(t)
	env := deps.Env.(*ports.MockEnvPort)
	env.OSResult = "darwin"
	env.ShellResult = ports.ShellBash
	env.Env["BASH_VERSION"] = "5.2.21(1)-release"

	var stderrBuf bytes.Buffer
	opts := install.Options{
		Completions: []ports.ShellKind{ports.ShellBash},
		Stderr:      &stderrBuf,
	}

	_, err := install.Run(context.Background(), deps, opts, newRootCmd())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Bash completion must be emitted.
	mockCP := deps.Completion.(*ports.MockCompletionPort)
	found := false
	for _, shell := range mockCP.EmitCompletionCalls {
		if shell == ports.ShellBash {
			found = true
		}
	}
	if !found {
		t.Error("bash completion should be emitted on darwin bash 5.x")
	}

	// No brew warning.
	if bytes.Contains(stderrBuf.Bytes(), []byte("brew install bash")) {
		t.Errorf("unexpected brew warning on bash 5.x: %q", stderrBuf.String())
	}
}

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
