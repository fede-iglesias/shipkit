package install

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// TestConfigAutostartLabel_DefaultAndCustom verifies the autostartLabel helper.
func TestConfigAutostartLabel_DefaultAndCustom(t *testing.T) {
	c := Config{AppName: "myapp"}
	if got := c.autostartLabel(); got != "com.fede-iglesias.myapp" {
		t.Errorf("default label = %q; want %q", got, "com.fede-iglesias.myapp")
	}

	c.AutostartLabel = "org.example.myapp"
	if got := c.autostartLabel(); got != "org.example.myapp" {
		t.Errorf("custom label = %q; want %q", got, "org.example.myapp")
	}
}

// TestConfigAutostartArgs_DefaultAndCustom verifies the autostartArgs helper.
func TestConfigAutostartArgs_DefaultAndCustom(t *testing.T) {
	c := Config{AppName: "myapp"}
	args := c.autostartArgs()
	if len(args) != 2 || args[0] != "daemon" || args[1] != "run" {
		t.Errorf("default args = %v; want [daemon run]", args)
	}

	c.AutostartArgs = []string{"serve", "--port", "8080"}
	args = c.autostartArgs()
	if len(args) != 3 || args[0] != "serve" {
		t.Errorf("custom args = %v; want [serve --port 8080]", args)
	}
}

// TestConfigBinaryName_DefaultAndCustom verifies the binaryName helper.
func TestConfigBinaryName_DefaultAndCustom(t *testing.T) {
	c := Config{AppName: "myapp"}
	if got := c.binaryName(); got != "myapp" {
		t.Errorf("default binary name = %q; want %q", got, "myapp")
	}

	c.BinaryName = "myapp-bin"
	if got := c.binaryName(); got != "myapp-bin" {
		t.Errorf("custom binary name = %q; want %q", got, "myapp-bin")
	}
}

// TestReadMarker_InvalidJSON verifies readMarker returns error on malformed JSON.
func TestReadMarker_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, markerFileName)
	raw := []byte("NOT_JSON")

	mockFS := ports.NewMockFsPort()
	mockFS.ReadFileFunc = func(_ context.Context, p string) ([]byte, error) {
		if p == path {
			return raw, nil
		}
		return nil, os.ErrNotExist
	}
	deps := Deps{FS: mockFS}

	_, err := readMarker(context.Background(), deps, path)
	if err == nil {
		t.Error("readMarker should return error for invalid JSON")
	}
}

// TestReadMarker_FileNotFound verifies readMarker returns error when absent.
func TestReadMarker_FileNotFound(t *testing.T) {
	mockFS := ports.NewMockFsPort()
	mockFS.ReadFileFunc = func(_ context.Context, _ string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	deps := Deps{FS: mockFS}

	_, err := readMarker(context.Background(), deps, "/nonexistent/path/.shipkit.installed")
	if err == nil {
		t.Error("readMarker should return error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist; got %v", err)
	}
}


// TestShellRcPath_Bash verifies .bashrc path.
func TestShellRcPath_Bash(t *testing.T) {
	p, err := shellRcPath(ports.ShellBash, "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "/home/user/.bashrc" {
		t.Errorf("bash rc = %q; want /home/user/.bashrc", p)
	}
}

// TestShellRcPath_Zsh verifies .zshrc path.
func TestShellRcPath_Zsh(t *testing.T) {
	p, err := shellRcPath(ports.ShellZsh, "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "/home/user/.zshrc" {
		t.Errorf("zsh rc = %q; want /home/user/.zshrc", p)
	}
}

// TestShellRcPath_Unknown verifies error for unknown shell.
func TestShellRcPath_Unknown(t *testing.T) {
	_, err := shellRcPath(ports.ShellUnknown, "/home/user")
	if err == nil {
		t.Error("expected error for unknown shell")
	}
}

// TestShellRcPath_Fish verifies error for fish (fish uses autoload, no rcPath).
func TestShellRcPath_Fish(t *testing.T) {
	_, err := shellRcPath(ports.ShellFish, "/home/user")
	if err == nil {
		t.Error("expected error for fish shell (no .fishrc)")
	}
}

// TestShouldSkipBash32_NonDarwin verifies no skip on linux.
func TestShouldSkipBash32_NonDarwin(t *testing.T) {
	env := ports.NewMockEnvPort()
	env.OSResult = "linux"
	env.Env["BASH_VERSION"] = "3.2.57(1)-release"

	var buf bytes.Buffer
	if shouldSkipBash32(env, &buf) {
		t.Error("shouldSkipBash32 should return false on linux")
	}
	if buf.Len() > 0 {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

// TestShouldSkipBash32_Darwin4x verifies no skip for bash 4+ on darwin.
func TestShouldSkipBash32_Darwin4x(t *testing.T) {
	env := ports.NewMockEnvPort()
	env.OSResult = "darwin"
	env.Env["BASH_VERSION"] = "5.1.16(1)-release"

	var buf bytes.Buffer
	if shouldSkipBash32(env, &buf) {
		t.Error("shouldSkipBash32 should return false for bash 5.x on darwin")
	}
}

// TestShouldSkipBash32_Darwin3x verifies skip + brew warn for bash 3.x on darwin.
func TestShouldSkipBash32_Darwin3x(t *testing.T) {
	env := ports.NewMockEnvPort()
	env.OSResult = "darwin"
	env.Env["BASH_VERSION"] = "3.2.57(1)-release"

	var buf bytes.Buffer
	if !shouldSkipBash32(env, &buf) {
		t.Error("shouldSkipBash32 should return true for bash 3.x on darwin")
	}
	if !bytes.Contains(buf.Bytes(), []byte("brew install bash")) {
		t.Errorf("expected brew warning, got: %q", buf.String())
	}
}

// TestFpathBlock_WithCompletionPath verifies the fpath block content.
func TestFpathBlock_WithCompletionPath(t *testing.T) {
	block := fpathBlock("myapp", "/home/user/.local/share/zsh/site-functions/_myapp")
	if block == "" {
		t.Fatal("fpathBlock returned empty string")
	}
	if !bytes.Contains([]byte(block), []byte("/home/user/.local/share/zsh/site-functions")) {
		t.Errorf("fpathBlock does not contain completion dir: %q", block)
	}
}

// TestFpathBlock_EmptyPath verifies placeholder when completion path is empty.
func TestFpathBlock_EmptyPath(t *testing.T) {
	block := fpathBlock("myapp", "")
	if !bytes.Contains([]byte(block), []byte("myapp")) {
		t.Errorf("fpathBlock placeholder does not mention app: %q", block)
	}
}

// TestMarkerFileName_IsConstant verifies the constant value.
func TestMarkerFileName_IsConstant(t *testing.T) {
	if markerFileName != ".shipkit.installed" {
		t.Errorf("markerFileName = %q; want .shipkit.installed", markerFileName)
	}
}

// TestMarshalInstallMarker verifies the helper serializes correctly.
func TestMarshalInstallMarker(t *testing.T) {
	m := InstallMarker{
		App:              "myapp",
		VersionInstalled: "v0.1.0",
		InstalledAt:      "2026-06-04T12:00:00Z",
		BinPath:          "/usr/local/bin/myapp",
		Completions:      []ports.ShellKind{ports.ShellZsh},
		Autostart:        false,
	}
	b := marshalInstallMarker(m)
	if !bytes.Contains(b, []byte("version_installed")) {
		t.Errorf("missing version_installed: %s", b)
	}
	if !bytes.Contains(b, []byte("installed_at")) {
		t.Errorf("missing installed_at: %s", b)
	}
}
