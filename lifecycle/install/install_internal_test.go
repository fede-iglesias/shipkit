package install

import (
	"bytes"
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
	if err := os.WriteFile(path, []byte("NOT_JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readMarker(path)
	if err == nil {
		t.Error("readMarker should return error for invalid JSON")
	}
}

// TestReadMarker_FileNotFound verifies readMarker returns error when absent.
func TestReadMarker_FileNotFound(t *testing.T) {
	_, err := readMarker("/nonexistent/path/.shipkit.installed")
	if err == nil {
		t.Error("readMarker should return error for missing file")
	}
}

// TestAtomicWriteBytes_Success verifies the happy path write+rename.
func TestAtomicWriteBytes_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	data := []byte("hello atomic")

	if err := atomicWriteBytes(path, data, 0o644); err != nil {
		t.Fatalf("atomicWriteBytes error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("data = %q; want %q", got, data)
	}
}

// TestAtomicWriteBytes_BadDir verifies error when the dir does not exist.
func TestAtomicWriteBytes_BadDir(t *testing.T) {
	err := atomicWriteBytes("/nonexistent/dir/file.txt", []byte("x"), 0o644)
	if err == nil {
		t.Error("expected error writing to nonexistent dir")
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

// TestAtomicWriteBytes_Overwrite verifies that atomicWriteBytes replaces existing file.
func TestAtomicWriteBytes_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteBytes(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("content = %q; want %q", got, "new")
	}
}

// TestAtomicWriteBytesHooked_WriteError verifies error path when write fails.
func TestAtomicWriteBytesHooked_WriteError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	hooks := atomicWriteHooks{
		writeFunc: func(_ *os.File, _ []byte) error {
			return os.ErrInvalid
		},
	}
	err := atomicWriteBytesHooked(path, []byte("x"), 0o644, hooks)
	if err == nil {
		t.Error("expected error from write hook")
	}
}

// TestAtomicWriteBytesHooked_ChmodError verifies error path when chmod fails.
func TestAtomicWriteBytesHooked_ChmodError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	hooks := atomicWriteHooks{
		chmodFunc: func(_ *os.File, _ os.FileMode) error {
			return os.ErrPermission
		},
	}
	err := atomicWriteBytesHooked(path, []byte("x"), 0o644, hooks)
	if err == nil {
		t.Error("expected error from chmod hook")
	}
}

// TestAtomicWriteBytesHooked_CloseError verifies error path when close fails.
func TestAtomicWriteBytesHooked_CloseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	hooks := atomicWriteHooks{
		closeFunc: func(_ *os.File) error {
			return os.ErrInvalid
		},
	}
	err := atomicWriteBytesHooked(path, []byte("x"), 0o644, hooks)
	if err == nil {
		t.Error("expected error from close hook")
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
