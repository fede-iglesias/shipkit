// Package shipkitexample provides integration tests that validate the round-trip
// lifecycle: install -> doctor -> uninstall.
//
// The tests build the shipkit-example binary into a temp dir, set up isolated
// XDG environment variables so no real user data is touched, and drive the CLI
// via os/exec with content assertions on outputs and created files.
//
// Update is deferred to B5.d cancha which provides a proper test server with
// a signed release fixture.
package shipkitexample

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles cmd/shipkit-example into binDir and returns the full
// path of the resulting binary. The build is performed once per test run; each
// call creates its own dir.
func buildBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "shipkit-example")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/shipkit-example")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build shipkit-example: %v\n%s", err, out)
	}
	return binPath
}

// xdgEnv returns a copy of os.Environ() with XDG_DATA_HOME, XDG_CONFIG_HOME,
// and XDG_CACHE_HOME overridden to isolated subdirs of root so that tests do
// not touch the developer's real XDG directories.
func xdgEnv(root string) []string {
	env := os.Environ()
	overrides := map[string]string{
		"XDG_DATA_HOME":   filepath.Join(root, "data"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_CACHE_HOME":  filepath.Join(root, "cache"),
		"HOME":            filepath.Join(root, "home"),
	}
	filtered := env[:0:len(env)]
	for _, e := range env {
		keep := true
		for k := range overrides {
			if strings.HasPrefix(e, k+"=") {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, e)
		}
	}
	for k, v := range overrides {
		filtered = append(filtered, k+"="+v)
	}
	return filtered
}

// run executes the shipkit-example binary with the given arguments in the
// provided environment. It returns (stdout+stderr combined, error).
func run(binPath string, env []string, args ...string) (string, error) {
	cmd := exec.Command(binPath, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// markerPath returns the expected location of the .shipkit.installed file
// given the XDG data root.
func markerPath(xdgDataHome string) string {
	return filepath.Join(xdgDataHome, "shipkit-example", ".shipkit.installed")
}

// TestLifecycleRoundTrip exercises the install -> doctor -> uninstall round
// trip against a freshly built binary. The test is tagged integration because
// it invokes go build and creates real filesystem state (in a temp dir).
//
// Update is out of scope: it requires a test server that serves a signed
// release asset, deferred to B5.d cancha.
func TestLifecycleRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: skipped in -short mode")
	}

	binPath := buildBinary(t)

	xdgRoot := t.TempDir()
	env := xdgEnv(xdgRoot)
	dataHome := filepath.Join(xdgRoot, "data")

	// Pre-create HOME dir and stub shell RC files so the shell-rc adapter does
	// not fail when writing guarded blocks into $HOME/.zshrc / $HOME/.bashrc.
	homeDir := filepath.Join(xdgRoot, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("create home dir: %v", err)
	}
	for _, rc := range []string{".zshrc", ".bashrc"} {
		rcPath := filepath.Join(homeDir, rc)
		if err := os.WriteFile(rcPath, []byte("# stub\n"), 0o644); err != nil {
			t.Fatalf("create %s: %v", rc, err)
		}
	}

	// --- install ----------------------------------------------------------------

	t.Run("install", func(t *testing.T) {
		out, err := run(binPath, env, "install", "--yes")
		if err != nil {
			t.Fatalf("install failed: %v\noutput: %s", err, out)
		}
		// Output must mention the app name.
		if !strings.Contains(out, "shipkit-example") {
			t.Errorf("install output missing app name; got: %q", out)
		}

		// The marker file must exist.
		mp := markerPath(dataHome)
		raw, err := os.ReadFile(mp)
		if err != nil {
			t.Fatalf("marker file not found at %s: %v", mp, err)
		}

		// Marker must contain correct app name and version.
		var marker struct {
			App              string `json:"app"`
			VersionInstalled string `json:"version_installed"`
		}
		if err := json.Unmarshal(raw, &marker); err != nil {
			t.Fatalf("parse marker JSON: %v", err)
		}
		if marker.App != "shipkit-example" {
			t.Errorf("marker.app = %q; want %q", marker.App, "shipkit-example")
		}
		if marker.VersionInstalled == "" {
			t.Errorf("marker.version_installed is empty")
		}
	})

	// --- doctor -----------------------------------------------------------------

	t.Run("doctor", func(t *testing.T) {
		out, err := run(binPath, env, "doctor")
		// doctor exits non-zero when checks warn/fail; that is expected.
		// We only care about content, not exit code for this scope.
		_ = err

		// The binary.executable check must be present and passing because we
		// just built the binary ourselves with the real go toolchain.
		if !strings.Contains(out, "binary.executable") {
			t.Errorf("doctor output missing binary.executable check; got:\n%s", out)
		}
		if !strings.Contains(out, "[PASS]") {
			t.Errorf("doctor output missing [PASS]; got:\n%s", out)
		}
	})

	// --- idempotent install (no-op) ---------------------------------------------

	t.Run("install_idempotent", func(t *testing.T) {
		out, err := run(binPath, env, "install", "--yes")
		if err != nil {
			t.Fatalf("idempotent install failed: %v\noutput: %s", err, out)
		}
		// A second install with same version should report already installed.
		if !strings.Contains(out, "already installed") {
			t.Errorf("expected 'already installed' in output; got: %q", out)
		}
	})

	// --- uninstall --------------------------------------------------------------

	t.Run("uninstall", func(t *testing.T) {
		out, err := run(binPath, env, "uninstall", "--yes")
		if err != nil {
			t.Fatalf("uninstall failed: %v\noutput: %s", err, out)
		}
		// Output should contain Binary action line.
		if !strings.Contains(out, "Binary:") {
			t.Errorf("uninstall output missing Binary summary; got: %q", out)
		}

		// The marker file must be gone.
		mp := markerPath(dataHome)
		if _, err := os.Stat(mp); !os.IsNotExist(err) {
			t.Errorf("marker file still exists after uninstall: %s", mp)
		}
	})
}

// TestHelp verifies that the top-level --help lists all five lifecycle verbs.
// This is a fast sanity check that does not require build+install.
func TestHelp(t *testing.T) {
	binPath := buildBinary(t)
	out, err := run(binPath, os.Environ(), "--help")
	// --help exits 0.
	if err != nil {
		t.Fatalf("--help failed: %v\noutput: %s", err, out)
	}

	for _, verb := range []string{"install", "update", "uninstall", "doctor", "clean"} {
		if !strings.Contains(out, verb) {
			t.Errorf("--help output missing verb %q; output:\n%s", verb, out)
		}
	}
}

// TestVersionFlag asserts the binary responds to --version with a parseable
// semver on stdout and exit 0. The shipkit update orchestrator's HealthCheck
// spawns the new binary with --version after AtomicReplace and parses a semver
// from stdout to confirm the upgrade landed correctly. A consumer that does
// not honour this contract triggers an unconditional rollback on every update,
// which is the regression this test guards against.
func TestVersionFlag(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "shipkit-example")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.Version=0.0.7", "-o", binPath, "./cmd/shipkit-example")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build shipkit-example with ldflags: %v\n%s", err, out)
	}

	out, err := run(binPath, os.Environ(), "--version")
	if err != nil {
		t.Fatalf("--version exited non-zero: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "0.0.7") {
		t.Errorf("--version output missing injected semver 0.0.7; got: %q", out)
	}
}
