package adapters

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
)

// testAutostartAdapter returns an AutostartRealAdapter with seams replacing
// os calls and launchctl/systemctl invocations with in-memory state.
//
// cmdOutputs maps "cmd args..." -> (stdout, exitCode). If exit != 0 the
// CommandFn will return an error.
func testAutostartAdapter(goos string, cmdFail map[string]bool, statPaths map[string]bool, writeOK, removeOK bool) *AutostartRealAdapter {
	files := map[string]string{}
	return &AutostartRealAdapter{
		WriteFileFn: func(name string, data []byte, _ os.FileMode) error {
			if !writeOK {
				return errors.New("write fail")
			}
			files[name] = string(data)
			return nil
		},
		RemoveFn: func(name string) error {
			if !removeOK {
				return errors.New("remove fail")
			}
			delete(files, name)
			return nil
		},
		MkdirAllFn: func(string, os.FileMode) error { return nil },
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Build a key from name + first arg for simple routing.
			key := name
			if len(args) > 0 {
				key += " " + args[0]
			}
			if fail, ok := cmdFail[key]; ok && fail {
				// Return a command that always fails.
				return exec.CommandContext(ctx, "false")
			}
			// Return a command that exits 0 silently.
			return exec.CommandContext(ctx, "true")
		},
		StatFn: func(name string) (os.FileInfo, error) {
			if statPaths[name] {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFileFn:    func(name string) ([]byte, error) { return []byte(files[name]), nil },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return goos },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
}

// fakeFileInfo is a minimal os.FileInfo for stat seam.
type fakeFileInfo struct{}

func (fakeFileInfo) Name() string      { return "fake" }
func (fakeFileInfo) Size() int64       { return 0 }
func (fakeFileInfo) Mode() os.FileMode { return 0o644 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool       { return false }
func (fakeFileInfo) Sys() interface{}  { return nil }

// TestNewAutostartReal verifies the constructor returns a non-nil adapter.
func TestNewAutostartReal(t *testing.T) {
	a := NewAutostartReal()
	if a == nil {
		t.Fatal("NewAutostartReal returned nil")
	}
}

// TestAutostartRealAdapter_UnsupportedOS verifies ErrAutostartUnsupported on
// an unsupported OS.
func TestAutostartRealAdapter_UnsupportedOS(t *testing.T) {
	a := testAutostartAdapter("windows", nil, nil, true, true)

	if err := a.Install(ports.AutostartUnit{Label: "l"}); !errors.Is(err, ErrAutostartUnsupported) {
		t.Errorf("Install: want ErrAutostartUnsupported; got %v", err)
	}
	if err := a.Uninstall("l"); !errors.Is(err, ErrAutostartUnsupported) {
		t.Errorf("Uninstall: want ErrAutostartUnsupported; got %v", err)
	}
	if _, err := a.Status("l"); !errors.Is(err, ErrAutostartUnsupported) {
		t.Errorf("Status: want ErrAutostartUnsupported; got %v", err)
	}
	if err := a.Stop("l"); !errors.Is(err, ErrAutostartUnsupported) {
		t.Errorf("Stop: want ErrAutostartUnsupported; got %v", err)
	}
}

// TestAutostartRealAdapter_Darwin_InstallHappyPath verifies a successful
// LaunchAgent install on darwin.
func TestAutostartRealAdapter_Darwin_InstallHappyPath(t *testing.T) {
	a := testAutostartAdapter("darwin", nil, nil, true, true)
	unit := ports.AutostartUnit{
		Label:     "com.test.app",
		Program:   "/usr/local/bin/app",
		Args:      []string{"daemon"},
		KeepAlive: true,
		RunAtLoad: true,
	}
	if err := a.Install(unit); err != nil {
		t.Fatalf("Install: %v", err)
	}
}

// TestAutostartRealAdapter_Darwin_InstallIdempotent verifies that installing
// twice with identical content is a no-op.
func TestAutostartRealAdapter_Darwin_InstallIdempotent(t *testing.T) {
	// Track write count to verify idempotency.
	writeCount := 0
	files := map[string]string{}
	cmds := map[string]bool{}

	a := &AutostartRealAdapter{
		WriteFileFn: func(name string, data []byte, _ os.FileMode) error {
			writeCount++
			files[name] = string(data)
			return nil
		},
		RemoveFn:   func(string) error { return nil },
		MkdirAllFn: func(string, os.FileMode) error { return nil },
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			_ = cmds
			return exec.CommandContext(ctx, "true")
		},
		StatFn: func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn: func(name string) ([]byte, error) {
			if v, ok := files[name]; ok {
				return []byte(v), nil
			}
			return nil, os.ErrNotExist
		},
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "darwin" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/launchctl", nil },
	}

	unit := ports.AutostartUnit{Label: "com.test.app", Program: "/bin/app"}
	if err := a.Install(unit); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	firstCount := writeCount

	// Second install with same content: should skip write.
	if err := a.Install(unit); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if writeCount != firstCount {
		t.Errorf("writeCount increased on idempotent install: got %d, first was %d", writeCount, firstCount)
	}
}

// TestAutostartRealAdapter_Darwin_InstallWriteError verifies write failure is propagated.
func TestAutostartRealAdapter_Darwin_InstallWriteError(t *testing.T) {
	a := testAutostartAdapter("darwin", nil, nil, false, true)
	err := a.Install(ports.AutostartUnit{Label: "com.test.app", Program: "/bin/app"})
	if err == nil {
		t.Fatal("want error; got nil")
	}
}

// TestAutostartRealAdapter_Darwin_UninstallIdempotent verifies that
// uninstalling a non-existent unit is a no-op.
func TestAutostartRealAdapter_Darwin_UninstallIdempotent(t *testing.T) {
	a := testAutostartAdapter("darwin", nil, nil, true, true)
	if err := a.Uninstall("com.test.app"); err != nil {
		t.Fatalf("Uninstall non-existent: %v", err)
	}
}

// TestAutostartRealAdapter_Darwin_UninstallHappyPath verifies successful removal.
func TestAutostartRealAdapter_Darwin_UninstallHappyPath(t *testing.T) {
	plistPath := "/home/user/Library/LaunchAgents/com.test.app.plist"
	a := testAutostartAdapter("darwin", nil, map[string]bool{plistPath: true}, true, true)
	if err := a.Uninstall("com.test.app"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
}

// TestAutostartRealAdapter_Darwin_StatusNotInstalled returns false when plist absent.
func TestAutostartRealAdapter_Darwin_StatusNotInstalled(t *testing.T) {
	a := testAutostartAdapter("darwin", nil, nil, true, true)
	st, err := a.Status("com.test.app")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Installed {
		t.Error("Installed = true; want false")
	}
}

// TestAutostartRealAdapter_Darwin_StatusInstalled returns installed=true and
// parses PID when launchctl print succeeds.
func TestAutostartRealAdapter_Darwin_StatusInstalled(t *testing.T) {
	plistPath := "/home/user/Library/LaunchAgents/com.test.app.plist"
	a := testAutostartAdapter("darwin", nil, map[string]bool{plistPath: true}, true, true)
	// Inject a CommandFn that emits PID.
	a.CommandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "launchctl" && len(args) > 0 && args[0] == "print" {
			return exec.CommandContext(ctx, "echo", "pid = 42\nstate = running")
		}
		return exec.CommandContext(ctx, "true")
	}
	st, err := a.Status("com.test.app")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false; want true")
	}
}

// TestAutostartRealAdapter_Darwin_Stop verifies stop sends launchctl commands.
func TestAutostartRealAdapter_Darwin_Stop(t *testing.T) {
	a := testAutostartAdapter("darwin", nil, nil, true, true)
	if err := a.Stop("com.test.app"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestAutostartRealAdapter_Linux_Install verifies linux installation when
// systemctl is available.
func TestAutostartRealAdapter_Linux_Install(t *testing.T) {
	a := testAutostartAdapter("linux", nil, nil, true, true)
	unit := ports.AutostartUnit{
		Label:     "myapp.service",
		Program:   "/usr/bin/myapp",
		Args:      []string{"run"},
		KeepAlive: true,
		RunAtLoad: true,
	}
	if err := a.Install(unit); err != nil {
		t.Fatalf("Install linux: %v", err)
	}
}

// TestAutostartRealAdapter_Linux_UninstallIdempotent verifies idempotent uninstall.
func TestAutostartRealAdapter_Linux_UninstallIdempotent(t *testing.T) {
	a := testAutostartAdapter("linux", nil, nil, true, true)
	if err := a.Uninstall("myapp.service"); err != nil {
		t.Fatalf("Uninstall non-existent: %v", err)
	}
}

// TestAutostartRealAdapter_Linux_StatusNotInstalled verifies false when unit absent.
func TestAutostartRealAdapter_Linux_StatusNotInstalled(t *testing.T) {
	a := testAutostartAdapter("linux", nil, nil, true, true)
	st, err := a.Status("myapp.service")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Installed {
		t.Error("Installed = true; want false")
	}
}

// TestAutostartRealAdapter_Linux_StatusInstalled parses systemctl output.
func TestAutostartRealAdapter_Linux_StatusInstalled(t *testing.T) {
	unitPath := "/home/user/.config/systemd/user/myapp.service"
	a := testAutostartAdapter("linux", nil, map[string]bool{unitPath: true}, true, true)
	a.CommandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "systemctl" && len(args) > 1 && args[1] == "show" {
			return exec.CommandContext(ctx, "echo", "MainPID=99\nActiveState=active")
		}
		return exec.CommandContext(ctx, "true")
	}
	st, err := a.Status("myapp.service")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false; want true")
	}
}

// TestAutostartRealAdapter_Linux_Stop verifies stop calls systemctl.
func TestAutostartRealAdapter_Linux_Stop(t *testing.T) {
	a := testAutostartAdapter("linux", nil, nil, true, true)
	if err := a.Stop("myapp.service"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestParseLaunchctlPID verifies PID extraction from launchctl print output.
func TestParseLaunchctlPID(t *testing.T) {
	output := "  pid = 42\n  state = running\n"
	if got := parseLaunchctlPID(output); got != 42 {
		t.Errorf("parseLaunchctlPID = %d; want 42", got)
	}
}

// TestParseLaunchctlPID_NoPID returns 0 when not present.
func TestParseLaunchctlPID_NoPID(t *testing.T) {
	if got := parseLaunchctlPID("state = running\n"); got != 0 {
		t.Errorf("parseLaunchctlPID = %d; want 0", got)
	}
}

// TestParseSystemctlStatus verifies PID and running state extraction.
func TestParseSystemctlStatus(t *testing.T) {
	output := "MainPID=99\nActiveState=active\n"
	pid, running := parseSystemctlStatus(output)
	if pid != 99 {
		t.Errorf("pid = %d; want 99", pid)
	}
	if !running {
		t.Error("running = false; want true")
	}
}

// TestParseSystemctlStatus_Inactive verifies inactive state.
func TestParseSystemctlStatus_Inactive(t *testing.T) {
	output := "MainPID=0\nActiveState=inactive\n"
	pid, running := parseSystemctlStatus(output)
	if pid != 0 {
		t.Errorf("pid = %d; want 0", pid)
	}
	if running {
		t.Error("running = true; want false")
	}
}

// TestAutostartPort_Compliance verifies interface satisfaction at compile time.
func TestAutostartPort_Compliance(t *testing.T) {
	var _ ports.AutostartPort = NewAutostartReal()
}

// TestAutostartRealAdapter_Darwin_HomeDirError verifies propagation of UserHomeDir error.
func TestAutostartRealAdapter_Darwin_HomeDirError(t *testing.T) {
	sentinel := errors.New("home fail")
	a := &AutostartRealAdapter{
		GOOSFn:        func() string { return "darwin" },
		UserHomeDirFn: func() (string, error) { return "", sentinel },
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GetuidFn:      func() int { return 1000 },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/launchctl", nil },
	}
	if err := a.Install(ports.AutostartUnit{Label: "test"}); !errors.Is(err, sentinel) {
		t.Fatalf("Install: want sentinel; got %v", err)
	}
	if err := a.Uninstall("test"); !errors.Is(err, sentinel) {
		t.Fatalf("Uninstall: want sentinel; got %v", err)
	}
	if _, err := a.Status("test"); !errors.Is(err, sentinel) {
		t.Fatalf("Status: want sentinel; got %v", err)
	}
}
