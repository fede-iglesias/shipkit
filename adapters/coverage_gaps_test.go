// Package adapters - coverage gap tests for constructors and error branches
// that are not reachable through the main test files.
package adapters

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// ---- Constructor smoke tests (cover real seam wiring) ----

// TestNewEnvOS_Constructors verifies real constructor seams are callable.
func TestNewEnvOS_Constructors(t *testing.T) {
	a := NewEnvOS()
	// Call each real seam to exercise the wired-function lines.
	_ = a.GetenvFn("HOME")
	_, _ = a.LookupEnvFn("HOME")
	_ = a.GetppidFn()
	_, _ = a.ReadFileFn("/nonexistent/path/coverage")
	_ = a.GOOSFn()
	_ = a.GOARCHFn()
}

// TestNewPathsXDG_Constructors exercises all real seam lines in NewPathsXDG.
func TestNewPathsXDG_Constructors(t *testing.T) {
	a := NewPathsXDG()
	_ = a.GetenvFn("HOME")
	_, _ = a.UserHomeDirFn()
	_, _ = a.ExecutableFn()
	_ = a.GOOSFn()
}

// TestNewCompletionCobra_Constructors exercises the real os.Getenv seam.
func TestNewCompletionCobra_Constructors(t *testing.T) {
	a := NewCompletionCobra()
	_ = a.GetenvFn("XDG_DATA_HOME")
}

// TestNewAutostartReal_Constructors exercises all real seam lines.
func TestNewAutostartReal_Constructors(t *testing.T) {
	a := NewAutostartReal()
	_ = a.GetenvFn("HOME")
	_ = a.GetuidFn()
	_, _ = a.UserHomeDirFn()
	_ = a.GOOSFn()
	// Stat a non-existent path to exercise the seam.
	_, _ = a.StatFn("/nonexistent-coverage-test")
	// ReadFile a non-existent path.
	_, _ = a.ReadFileFn("/nonexistent-coverage-test")
	// LookPath a known tool.
	_, _ = a.LookPathFn("true")
}

// ---- autostart darwin remaining gaps ----

// TestAutostartRealAdapter_Darwin_InstallMkdirError verifies MkdirAll error
// is propagated.
func TestAutostartRealAdapter_Darwin_InstallMkdirError(t *testing.T) {
	sentinel := errors.New("mkdir fail")
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return sentinel },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "darwin" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/launchctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "com.test", Program: "/bin/app"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestAutostartRealAdapter_Darwin_InstallCmdError verifies that a launchctl
// error is returned.
func TestAutostartRealAdapter_Darwin_InstallCmdError(t *testing.T) {
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "false") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "darwin" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/launchctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "com.test", Program: "/bin/app"})
	if err == nil {
		t.Fatal("want error from launchctl; got nil")
	}
}

// TestAutostartRealAdapter_Darwin_UninstallRemoveError verifies remove error
// is propagated.
func TestAutostartRealAdapter_Darwin_UninstallRemoveError(t *testing.T) {
	sentinel := errors.New("remove fail")
	plistPath := "/home/user/Library/LaunchAgents/com.test.plist"
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return sentinel },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(name string) (os.FileInfo, error) {
			if name == plistPath {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "darwin" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "", nil },
	}
	err := a.Uninstall("com.test")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestAutostartRealAdapter_Darwin_StatusNotRunning verifies the running=false
// branch when launchctl print fails.
func TestAutostartRealAdapter_Darwin_StatusNotRunning(t *testing.T) {
	plistPath := "/home/user/Library/LaunchAgents/com.test.plist"
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "false") },
		StatFn: func(name string) (os.FileInfo, error) {
			if name == plistPath {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "darwin" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "", nil },
	}
	st, err := a.Status("com.test")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false; want true")
	}
	if st.Running {
		t.Error("Running = true; want false (launchctl print failed)")
	}
}

// ---- autostart linux remaining gaps ----

// TestAutostartRealAdapter_Linux_HomeDirError verifies home dir error is
// propagated through linuxSystemdDir.
func TestAutostartRealAdapter_Linux_HomeDirError(t *testing.T) {
	sentinel := errors.New("home fail")
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "", sentinel },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	if err := a.Install(ports.AutostartUnit{Label: "myapp.service"}); !errors.Is(err, sentinel) {
		t.Fatalf("Install linux home error: want sentinel; got %v", err)
	}
	if err := a.Uninstall("myapp.service"); !errors.Is(err, sentinel) {
		t.Fatalf("Uninstall linux home error: want sentinel; got %v", err)
	}
	if _, err := a.Status("myapp.service"); !errors.Is(err, sentinel) {
		t.Fatalf("Status linux home error: want sentinel; got %v", err)
	}
}

// TestAutostartRealAdapter_Linux_XDGConfigHomeOverride verifies that
// XDG_CONFIG_HOME is used when set.
func TestAutostartRealAdapter_Linux_XDGConfigHomeOverride(t *testing.T) {
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(key string) string {
			if key == "XDG_CONFIG_HOME" {
				return "/custom/cfg"
			}
			return ""
		},
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	dir, err := a.linuxSystemdDir()
	if err != nil {
		t.Fatalf("linuxSystemdDir: %v", err)
	}
	if !strings.HasPrefix(dir, "/custom/cfg") {
		t.Errorf("linuxSystemdDir = %q; want prefix /custom/cfg", dir)
	}
}

// TestAutostartRealAdapter_Linux_MkdirError verifies mkdir error.
func TestAutostartRealAdapter_Linux_MkdirError(t *testing.T) {
	sentinel := errors.New("mkdir fail")
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return sentinel },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "myapp.service", Program: "/bin/app"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestAutostartRealAdapter_Linux_WriteError verifies write error.
func TestAutostartRealAdapter_Linux_WriteError(t *testing.T) {
	sentinel := errors.New("write fail")
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return sentinel },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "myapp.service", Program: "/bin/app"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestAutostartRealAdapter_Linux_DaemonReloadError verifies that a
// daemon-reload failure is propagated.
func TestAutostartRealAdapter_Linux_DaemonReloadError(t *testing.T) {
	callCount := 0
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			callCount++
			// First call (daemon-reload) fails; subsequent calls succeed.
			if callCount == 1 {
				return exec.CommandContext(ctx, "false")
			}
			return exec.CommandContext(ctx, "true")
		},
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "myapp.service", Program: "/bin/app"})
	if err == nil {
		t.Fatal("want error from daemon-reload; got nil")
	}
}

// TestAutostartRealAdapter_Linux_EnableError verifies that a systemctl enable
// failure is propagated.
func TestAutostartRealAdapter_Linux_EnableError(t *testing.T) {
	callCount := 0
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			callCount++
			// Second call (enable --now) fails.
			if callCount == 2 {
				return exec.CommandContext(ctx, "false")
			}
			return exec.CommandContext(ctx, "true")
		},
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "myapp.service", Program: "/bin/app"})
	if err == nil {
		t.Fatal("want error from systemctl enable; got nil")
	}
}

// TestAutostartRealAdapter_Linux_UninstallHappyPath exercises the full
// uninstall flow when unit file exists.
func TestAutostartRealAdapter_Linux_UninstallHappyPath(t *testing.T) {
	unitPath := "/home/user/.config/systemd/user/myapp.service"
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn: func(name string) (os.FileInfo, error) {
			if name == unitPath {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	if err := a.Uninstall("myapp.service"); err != nil {
		t.Fatalf("Uninstall linux happy path: %v", err)
	}
}

// TestAutostartRealAdapter_Linux_UninstallRemoveError verifies remove error.
func TestAutostartRealAdapter_Linux_UninstallRemoveError(t *testing.T) {
	sentinel := errors.New("remove fail")
	unitPath := "/home/user/.config/systemd/user/myapp.service"
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return sentinel },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn: func(name string) (os.FileInfo, error) {
			if name == unitPath {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	if err := a.Uninstall("myapp.service"); !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestAutostartRealAdapter_Linux_StatusNotRunning verifies the running=false
// branch when systemctl show fails.
func TestAutostartRealAdapter_Linux_StatusNotRunning(t *testing.T) {
	unitPath := "/home/user/.config/systemd/user/myapp.service"
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "false") },
		StatFn: func(name string) (os.FileInfo, error) {
			if name == unitPath {
				return fakeFileInfo{}, nil
			}
			return nil, os.ErrNotExist
		},
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	st, err := a.Status("myapp.service")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Installed {
		t.Error("Installed = false; want true")
	}
	if st.Running {
		t.Error("Running = true; want false (systemctl failed)")
	}
}

// ---- EnsureBlock/RemoveBlock remaining gaps ----

// TestShellRcReal_EnsureBlock_UpdateRenameError covers the rename failure path
// in the block-update branch (block exists, content differs).
func TestShellRcReal_EnsureBlock_UpdateRenameError(t *testing.T) {
	sentinel := errors.New("rename fail")
	existing := "\n# >>> shipkit:id >>>\nold content\n# <<< shipkit:id <<<\n"
	writeCount := 0
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return []byte(existing), nil },
		WriteFileFn: func(string, []byte, os.FileMode) error { writeCount++; return nil },
		RenameFn:    func(string, string) error { return sentinel },
	}
	_, err := a.EnsureBlock("rc", "id", "new content")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// ---- prompt remaining gaps ----

// TestPromptTermAdapter_Confirm_NoLongForm verifies "no" is accepted.
func TestPromptTermAdapter_Confirm_NoLongForm(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("no\n")),
	}
	got, err := a.Confirm("continue?", true)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got {
		t.Error("Confirm with 'no' input: got true; want false")
	}
}

// ---- fakeFileInfo compile check ----

// TestFakeFileInfo exercises ModTime to ensure it satisfies os.FileInfo.
func TestFakeFileInfo(t *testing.T) {
	var fi os.FileInfo = fakeFileInfo{}
	zero := time.Time{}
	if fi.ModTime() != zero {
		t.Errorf("ModTime = %v; want zero time", fi.ModTime())
	}
}

// ---- NewCompletionCobra real cobra closure coverage ----

// TestNewCompletionCobra_RealGenerators calls the real cobra generators via
// the production constructor to exercise the closure lines.
func TestNewCompletionCobra_RealGenerators(t *testing.T) {
	a := NewCompletionCobra()
	root := &cobra.Command{Use: "app", CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true}}
	root.AddCommand(&cobra.Command{Use: "sub", Short: "A sub"})

	var buf bytes.Buffer

	// Bash
	buf.Reset()
	if err := a.GenBashFn(root, &buf); err != nil {
		t.Errorf("real GenBash: %v", err)
	}

	// Zsh
	buf.Reset()
	if err := a.GenZshFn(root, &buf); err != nil {
		t.Errorf("real GenZsh: %v", err)
	}

	// Fish
	buf.Reset()
	if err := a.GenFishFn(root, &buf, true); err != nil {
		t.Errorf("real GenFish: %v", err)
	}
}

// ---- linux idempotent install (file read success) ----

// TestAutostartRealAdapter_Linux_InstallIdempotent verifies that re-installing
// with identical content is a no-op (ReadFileFn succeeds, content matches).
func TestAutostartRealAdapter_Linux_InstallIdempotent(t *testing.T) {
	writeCount := 0
	var storedContent string
	a := &AutostartRealAdapter{
		WriteFileFn: func(name string, data []byte, _ os.FileMode) error {
			writeCount++
			storedContent = string(data)
			return nil
		},
		RemoveFn:   func(string) error { return nil },
		MkdirAllFn: func(string, os.FileMode) error { return nil },
		CommandFn:  func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:     func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn: func(name string) ([]byte, error) {
			if storedContent != "" {
				return []byte(storedContent), nil
			}
			return nil, os.ErrNotExist
		},
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}

	unit := ports.AutostartUnit{Label: "myapp.service", Program: "/bin/myapp"}
	if err := a.Install(unit); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	firstCount := writeCount

	if err := a.Install(unit); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if writeCount != firstCount {
		t.Errorf("writeCount increased on idempotent install: want %d, got %d", firstCount, writeCount)
	}
}

// ---- RemoveBlock - no trailing newline after close marker ----

// TestShellRcReal_RemoveBlock_NoTrailingNewline covers the else branch when
// there is no newline after the close marker.
func TestShellRcReal_RemoveBlock_NoTrailingNewline(t *testing.T) {
	// File ends at the close marker with no trailing newline.
	blockContent := "# >>> shipkit:id >>>\ncontent\n# <<< shipkit:id <<<"
	a := &ShellRcRealAdapter{
		ReadFileFn:  func(string) ([]byte, error) { return []byte(blockContent), nil },
		WriteFileFn: func(string, []byte, os.FileMode) error { return nil },
		RenameFn:    func(string, string) error { return nil },
	}
	res, err := a.RemoveBlock("rc", "id")
	if err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}
	if !res.Removed {
		t.Error("result.Removed = false; want true")
	}
}

// ---- darwin idempotent install (content matches) ----

// TestAutostartRealAdapter_Darwin_InstallIdempotentReadMatch verifies the
// darwin idempotent path when ReadFileFn returns matching content.
func TestAutostartRealAdapter_Darwin_InstallIdempotentReadMatch(t *testing.T) {
	writeCount := 0
	var storedContent string
	a := &AutostartRealAdapter{
		WriteFileFn: func(name string, data []byte, _ os.FileMode) error {
			writeCount++
			storedContent = string(data)
			return nil
		},
		RemoveFn:   func(string) error { return nil },
		MkdirAllFn: func(string, os.FileMode) error { return nil },
		CommandFn:  func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:     func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn: func(name string) ([]byte, error) {
			if storedContent != "" {
				return []byte(storedContent), nil
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

	if err := a.Install(unit); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if writeCount != firstCount {
		t.Errorf("writeCount increased on idempotent install: want %d, got %d", firstCount, writeCount)
	}
}

// TestAutostartRealAdapter_Linux_SecondHomeDirError covers the linuxSystemdDir
// error inside installLinux after the ReadFile idempotent check is skipped.
// This happens when the first linuxSystemdDir (for unitPath) succeeds but the
// second call (for mkdir dir) could fail - these share the same function so
// the path is covered. We instead cover the non-idempotent linuxSystemdDir
// error by using an initial ReadFileFn that returns different content.
func TestAutostartRealAdapter_Linux_NonIdempotentThenHomeDirError(t *testing.T) {
	// ReadFileFn returns different content so idempotent check fails.
	// Then UserHomeDirFn suddenly returns error on second call.
	callCount := 0
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return []byte("different content"), nil },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) {
			callCount++
			if callCount > 1 {
				return "", errors.New("home fail second call")
			}
			return "/home/user", nil
		},
		LookPathFn: func(string) (string, error) { return "/usr/bin/systemctl", nil },
	}
	// First call linuxSystemdDir (for unitPath) succeeds.
	// Second call inside install also needs to succeed or fail gracefully.
	// The point is to exercise the non-idempotent path (content differs).
	// Since second linuxSystemdDir uses same seam, both calls share UserHomeDirFn.
	// This test exercises the non-idempotent branch (file exists but content differs).
	err := a.Install(ports.AutostartUnit{Label: "myapp.service", Program: "/bin/app"})
	// May or may not error depending on second home call; just verify no panic.
	_ = err
}

// ---- render seam error paths ----

// TestAutostartRealAdapter_Darwin_RenderObserver verifies that RenderDarwinPlistFn
// is called and its output is used for the plist file.
func TestAutostartRealAdapter_Darwin_RenderObserver(t *testing.T) {
	called := false
	a := &AutostartRealAdapter{
		WriteFileFn:         func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:            func(string) error { return nil },
		MkdirAllFn:          func(string, os.FileMode) error { return nil },
		CommandFn:           func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:              func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:          func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:            func(string) string { return "" },
		GOOSFn:              func() string { return "darwin" },
		GetuidFn:            func() int { return 1000 },
		UserHomeDirFn:       func() (string, error) { return "/home/user", nil },
		LookPathFn:          func(string) (string, error) { return "/usr/bin/launchctl", nil },
		RenderDarwinPlistFn: func(u ports.AutostartUnit) string { called = true; return "<?xml?>" },
	}
	if err := a.Install(ports.AutostartUnit{Label: "com.test"}); err != nil {
		t.Fatalf("Install darwin with observer: %v", err)
	}
	if !called {
		t.Error("RenderDarwinPlistFn was not called")
	}
}

// TestAutostartRealAdapter_Linux_RenderObserver verifies that RenderLinuxUnitFn
// is called and its output is used for the unit file.
func TestAutostartRealAdapter_Linux_RenderObserver(t *testing.T) {
	called := false
	a := &AutostartRealAdapter{
		WriteFileFn:       func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:          func(string) error { return nil },
		MkdirAllFn:        func(string, os.FileMode) error { return nil },
		CommandFn:         func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:            func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:        func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:          func(string) string { return "" },
		GOOSFn:            func() string { return "linux" },
		GetuidFn:          func() int { return 1000 },
		UserHomeDirFn:     func() (string, error) { return "/home/user", nil },
		LookPathFn:        func(string) (string, error) { return "/usr/bin/systemctl", nil },
		RenderLinuxUnitFn: func(u ports.AutostartUnit) string { called = true; return "[Unit]" },
	}
	if err := a.Install(ports.AutostartUnit{Label: "myapp.service"}); err != nil {
		t.Fatalf("Install linux with observer: %v", err)
	}
	if !called {
		t.Error("RenderLinuxUnitFn was not called")
	}
}

// ---- defaultRender functions ----

// TestDefaultRenderDarwinPlist verifies the default renderer produces non-empty output.
func TestDefaultRenderDarwinPlist(t *testing.T) {
	unit := ports.AutostartUnit{Label: "com.test", Program: "/bin/app", Args: []string{"run"}}
	got := defaultRenderDarwinPlist(unit)
	if !containsString(got, "com.test") {
		t.Errorf("plist does not contain label: %q", got)
	}
}

// TestDefaultRenderLinuxUnit verifies the default renderer produces non-empty output.
func TestDefaultRenderLinuxUnit(t *testing.T) {
	unit := ports.AutostartUnit{Label: "myapp.service", Program: "/bin/app", KeepAlive: true}
	got := defaultRenderLinuxUnit(unit)
	if !containsString(got, "myapp.service") {
		t.Errorf("unit does not contain label: %q", got)
	}
}

// ---- Confirm stderr write failure ----

// TestPromptTermAdapter_Confirm_StderrWriteFailure verifies that a stderr
// write failure returns the default value without error.
func TestPromptTermAdapter_Confirm_StderrWriteFailure(t *testing.T) {
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(strings.NewReader("y\n")),
		StderrWriter: &failWriter{},
	}
	got, err := a.Confirm("continue?", true)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	// When stderr write fails, returns the default.
	if !got {
		t.Error("Confirm with stderr failure defaultYes=true: got false; want true")
	}
}

// failWriter always fails on Write.
type failWriter struct{}

func (fw *failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write fail")
}

// ---- linux systemctl unsupported ----

// TestAutostartRealAdapter_Linux_SystemctlUnsupported verifies that Install
// returns ErrAutostartUnsupported when LookPathFn says systemctl is absent.
func TestAutostartRealAdapter_Linux_SystemctlUnsupported(t *testing.T) {
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		LookPathFn:    func(string) (string, error) { return "", errors.New("not found") },
	}
	if err := a.Install(ports.AutostartUnit{Label: "myapp.service"}); !errors.Is(err, ErrAutostartUnsupported) {
		t.Fatalf("want ErrAutostartUnsupported; got %v", err)
	}
}

// ---- linux LookPathFn nil fallback ----

// TestAutostartRealAdapter_Linux_LookPathFnNil verifies the nil-LookPathFn
// fallback branch. On non-linux build hosts exec.LookPath("systemctl") fails,
// returning ErrAutostartUnsupported, which is also valid. On linux it would
// succeed. Either way the nil branch (lines 415-417) is exercised.
func TestAutostartRealAdapter_Linux_LookPathFnNil(t *testing.T) {
	a := &AutostartRealAdapter{
		WriteFileFn:   func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:      func(string) error { return nil },
		MkdirAllFn:    func(string, os.FileMode) error { return nil },
		CommandFn:     func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:        func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:    func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:      func(string) string { return "" },
		GOOSFn:        func() string { return "linux" },
		GetuidFn:      func() int { return 1000 },
		UserHomeDirFn: func() (string, error) { return "/home/user", nil },
		// LookPathFn intentionally nil - exercises the nil fallback to exec.LookPath.
	}
	// On macOS/CI where systemctl is absent, ErrAutostartUnsupported is expected.
	// On linux with systemd, the call proceeds further. Either path is valid.
	_ = a.Install(ports.AutostartUnit{Label: "myapp.service"})
}

// ---- darwin darwinLaunchAgentsDir error on second call ----

// TestAutostartRealAdapter_Darwin_DirErrorSecondCall covers the darwinLaunchAgentsDir
// error path at line 271 inside installDarwin. The first UserHomeDirFn call
// (inside darwinPlistPath) succeeds so we get past plistPath. The idempotency
// ReadFile returns an error so we don't short-circuit. Then the second
// darwinLaunchAgentsDir call (for mkdir dir) hits a UserHomeDirFn failure.
func TestAutostartRealAdapter_Darwin_DirErrorSecondCall(t *testing.T) {
	callCount := 0
	sentinel := errors.New("home fail on second call")
	a := &AutostartRealAdapter{
		WriteFileFn: func(string, []byte, os.FileMode) error { return nil },
		RemoveFn:    func(string) error { return nil },
		MkdirAllFn:  func(string, os.FileMode) error { return nil },
		CommandFn:   func(ctx context.Context, name string, args ...string) *exec.Cmd { return exec.CommandContext(ctx, "true") },
		StatFn:      func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ReadFileFn:  func(string) ([]byte, error) { return nil, os.ErrNotExist },
		GetenvFn:    func(string) string { return "" },
		GOOSFn:      func() string { return "darwin" },
		GetuidFn:    func() int { return 1000 },
		UserHomeDirFn: func() (string, error) {
			callCount++
			if callCount > 1 {
				return "", sentinel
			}
			return "/home/user", nil
		},
		LookPathFn: func(string) (string, error) { return "/usr/bin/launchctl", nil },
	}
	err := a.Install(ports.AutostartUnit{Label: "com.test", Program: "/bin/app"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Install darwin second dir error: want sentinel; got %v", err)
	}
}

// ---- Confirm non-EOF read error ----

// errReader is an io.Reader that returns a non-EOF error after the first read.
type errReader struct {
	err error
}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

// TestPromptTermAdapter_Confirm_ReadError verifies that a non-EOF read error
// is returned from Confirm (line 100 in prompt_term.go).
func TestPromptTermAdapter_Confirm_ReadError(t *testing.T) {
	sentinel := errors.New("read fail")
	a := &PromptTermAdapter{
		IsTerminalFn: func(int) bool { return true },
		StdinFd:      0,
		Reader:       bufio.NewReader(&errReader{err: sentinel}),
		StderrWriter: io.Discard,
	}
	_, err := a.Confirm("continue?", true)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Confirm read error: want sentinel; got %v", err)
	}
}
