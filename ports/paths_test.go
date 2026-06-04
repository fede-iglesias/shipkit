package ports_test

import (
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockPathsPort satisfies PathsPort.
var _ ports.PathsPort = (*ports.MockPathsPort)(nil)

func TestMockPathsPort_defaults(t *testing.T) {
	m := ports.NewMockPathsPort()

	exec, err := m.Executable()
	if err != nil || exec != "/usr/local/bin/app" {
		t.Errorf("unexpected Executable: %q, %v", exec, err)
	}

	home, err := m.UserHome()
	if err != nil || home != "/home/user" {
		t.Errorf("unexpected UserHome: %q, %v", home, err)
	}

	if m.DefaultInstallDir() != "/usr/local/bin" {
		t.Errorf("unexpected DefaultInstallDir: %q", m.DefaultInstallDir())
	}

	if !m.InPATH("/usr/local/bin") {
		t.Error("expected InPATH to return true by default")
	}

	if len(m.PATHList()) == 0 {
		t.Error("expected non-empty PATHList")
	}
}

func TestMockPathsPort_DataDir_default(t *testing.T) {
	m := ports.NewMockPathsPort()
	dir, err := m.DataDir("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Error("expected non-empty DataDir")
	}
	if len(m.DataDirCalls) != 1 || m.DataDirCalls[0] != "myapp" {
		t.Errorf("expected call recorded with 'myapp', got %v", m.DataDirCalls)
	}
}

func TestMockPathsPort_DataDir_func(t *testing.T) {
	m := ports.NewMockPathsPort()
	m.DataDirFunc = func(app string) (string, error) { return "/custom/" + app, nil }
	dir, err := m.DataDir("kt")
	if err != nil || dir != "/custom/kt" {
		t.Errorf("unexpected: %q, %v", dir, err)
	}
}

func TestMockPathsPort_DataDir_error(t *testing.T) {
	m := ports.NewMockPathsPort()
	sentinel := errors.New("xdg error")
	m.DataDirFunc = func(_ string) (string, error) { return "", sentinel }
	_, err := m.DataDir("app")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockPathsPort_ConfigDir_default(t *testing.T) {
	m := ports.NewMockPathsPort()
	dir, err := m.ConfigDir("myapp")
	if err != nil || dir == "" {
		t.Errorf("unexpected: %q, %v", dir, err)
	}
}

func TestMockPathsPort_CacheDir_default(t *testing.T) {
	m := ports.NewMockPathsPort()
	dir, err := m.CacheDir("myapp")
	if err != nil || dir == "" {
		t.Errorf("unexpected: %q, %v", dir, err)
	}
}

func TestMockPathsPort_InPATH_func(t *testing.T) {
	m := ports.NewMockPathsPort()
	m.InPATHFunc = func(p string) bool { return p == "/custom/bin" }
	if !m.InPATH("/custom/bin") {
		t.Error("expected true for /custom/bin")
	}
	if m.InPATH("/other") {
		t.Error("expected false for /other")
	}
}

func TestMockPathsPort_Executable_error(t *testing.T) {
	m := ports.NewMockPathsPort()
	sentinel := errors.New("no executable")
	m.ExecutableErr = sentinel
	_, err := m.Executable()
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockPathsPort_ConfigDir_func(t *testing.T) {
	m := ports.NewMockPathsPort()
	m.ConfigDirFunc = func(app string) (string, error) { return "/custom/config/" + app, nil }
	dir, err := m.ConfigDir("kt")
	if err != nil || dir != "/custom/config/kt" {
		t.Errorf("unexpected: %q, %v", dir, err)
	}
}

func TestMockPathsPort_CacheDir_func(t *testing.T) {
	m := ports.NewMockPathsPort()
	m.CacheDirFunc = func(app string) (string, error) { return "/custom/cache/" + app, nil }
	dir, err := m.CacheDir("kt")
	if err != nil || dir != "/custom/cache/kt" {
		t.Errorf("unexpected: %q, %v", dir, err)
	}
}

func TestMockPathsPort_DefaultInstallDir_emptyResult(t *testing.T) {
	// When DefaultInstallDirResult is empty, DefaultInstallDir falls back to /usr/local/bin.
	m := &ports.MockPathsPort{}
	if m.DefaultInstallDir() != "/usr/local/bin" {
		t.Errorf("expected /usr/local/bin fallback, got %q", m.DefaultInstallDir())
	}
}
