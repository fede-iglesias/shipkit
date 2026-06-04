package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/fede-iglesias/shipkit/ports"
)

// ErrAutostartUnsupported is returned by AutostartRealAdapter.Install when
// the current platform does not support user-scoped service management (e.g.
// linux without systemd-user, or an unsupported OS).
var ErrAutostartUnsupported = fmt.Errorf("autostart: platform does not support user-scoped service management")

// LookPathFn resolves the path of a named binary. Defaults to exec.LookPath.
// Injectable so tests can simulate the presence/absence of systemctl without
// a real Linux init system.
//
// AutostartRealAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.AutostartPort].
//
// # darwin
//
// Creates/removes LaunchAgent plist files under ~/Library/LaunchAgents/ and
// calls `launchctl bootstrap gui/<uid>` / `launchctl bootout gui/<uid>`.
//
// # linux
//
// Creates/removes systemd user unit files under ~/.config/systemd/user/ and
// calls `systemctl --user enable/disable/start/stop`. Returns
// ErrAutostartUnsupported when systemctl is not available.
//
// No elevated privileges (no sudo) are required in either case.
//
// # Injectable seams
//
// WriteFileFn, RemoveFn, MkdirAllFn, CommandFn, StatFn, ReadFileFn, GetenvFn,
// GOOSFn, GetuidFn, UserHomeDirFn are all injectable so every execution path
// (including error branches and status parsing) is unit-testable.
type AutostartRealAdapter struct {
	// WriteFileFn writes data to a file. Defaults to os.WriteFile.
	WriteFileFn func(name string, data []byte, perm os.FileMode) error

	// RemoveFn removes a file. Defaults to os.Remove.
	RemoveFn func(name string) error

	// MkdirAllFn creates directories. Defaults to os.MkdirAll.
	MkdirAllFn func(path string, perm os.FileMode) error

	// CommandFn constructs an exec.Cmd. Defaults to exec.CommandContext.
	// Injectable so tests can verify launchctl/systemctl invocations without
	// a real daemon process.
	CommandFn func(ctx context.Context, name string, args ...string) *exec.Cmd

	// StatFn stats a file path. Defaults to os.Stat.
	StatFn func(name string) (os.FileInfo, error)

	// ReadFileFn reads a file. Defaults to os.ReadFile.
	ReadFileFn func(name string) ([]byte, error)

	// GetenvFn reads environment variables. Defaults to os.Getenv.
	GetenvFn func(key string) string

	// GOOSFn returns the OS string. Defaults to returning runtime.GOOS.
	GOOSFn func() string

	// GetuidFn returns the current user's numeric UID. Defaults to os.Getuid.
	GetuidFn func() int

	// UserHomeDirFn returns the user home directory. Defaults to os.UserHomeDir.
	UserHomeDirFn func() (string, error)

	// LookPathFn checks whether a binary is available. Defaults to exec.LookPath.
	// Injectable so tests can simulate systemctl presence/absence.
	LookPathFn func(file string) (string, error)

	// RenderDarwinPlistFn renders the LaunchAgent plist for unit. Defaults to
	// the built-in template renderer. Injectable to observe/replace rendering.
	RenderDarwinPlistFn func(unit ports.AutostartUnit) string

	// RenderLinuxUnitFn renders the systemd unit file for unit. Defaults to
	// the built-in template renderer. Injectable to observe/replace rendering.
	RenderLinuxUnitFn func(unit ports.AutostartUnit) string
}

// NewAutostartReal returns an AutostartRealAdapter with all seams wired to
// real os functions. Use this in production wiring.
func NewAutostartReal() *AutostartRealAdapter {
	return &AutostartRealAdapter{
		WriteFileFn:   os.WriteFile,
		RemoveFn:      os.Remove,
		MkdirAllFn:    os.MkdirAll,
		CommandFn:     exec.CommandContext,
		StatFn:        os.Stat,
		ReadFileFn:    os.ReadFile,
		GetenvFn:      os.Getenv,
		GOOSFn:        func() string { return runtime.GOOS },
		GetuidFn:      os.Getuid,
		UserHomeDirFn: os.UserHomeDir,
		LookPathFn:          exec.LookPath,
		RenderDarwinPlistFn: defaultRenderDarwinPlist,
		RenderLinuxUnitFn:   defaultRenderLinuxUnit,
	}
}

// defaultRenderDarwinPlist renders a LaunchAgent plist using the built-in
// template. strings.Builder.Write never returns an error and the template is a
// compile-time constant with no function calls, so Execute is guaranteed to
// succeed. The error return is intentionally discarded.
func defaultRenderDarwinPlist(unit ports.AutostartUnit) string {
	var buf strings.Builder
	_ = darwinPlistTemplate.Execute(&buf, unit)
	return buf.String()
}

// defaultRenderLinuxUnit renders a systemd unit file using the built-in
// template. Same invariant as defaultRenderDarwinPlist - Execute cannot fail.
func defaultRenderLinuxUnit(unit ports.AutostartUnit) string {
	var buf strings.Builder
	_ = linuxUnitTemplate.Execute(&buf, unit)
	return buf.String()
}

// Install writes the platform-specific service unit file for unit and
// registers it with the init system. If the unit already exists and its
// content matches, Install is a no-op. Returns ErrAutostartUnsupported on
// unsupported platforms.
func (a *AutostartRealAdapter) Install(unit ports.AutostartUnit) error {
	switch a.GOOSFn() {
	case "darwin":
		return a.installDarwin(unit)
	case "linux":
		return a.installLinux(unit)
	default:
		return ErrAutostartUnsupported
	}
}

// Uninstall stops the service identified by label (if running) and removes
// its unit file. Returns nil if the unit does not exist (idempotent).
func (a *AutostartRealAdapter) Uninstall(label string) error {
	switch a.GOOSFn() {
	case "darwin":
		return a.uninstallDarwin(label)
	case "linux":
		return a.uninstallLinux(label)
	default:
		return ErrAutostartUnsupported
	}
}

// Status returns the current installation and runtime state of the service
// identified by label. Returns AutostartStatus with Installed = false if the
// unit file does not exist.
func (a *AutostartRealAdapter) Status(label string) (ports.AutostartStatus, error) {
	switch a.GOOSFn() {
	case "darwin":
		return a.statusDarwin(label)
	case "linux":
		return a.statusLinux(label)
	default:
		return ports.AutostartStatus{}, ErrAutostartUnsupported
	}
}

// Stop sends a stop signal to the running service identified by label. Returns
// nil if the service is not running (idempotent).
func (a *AutostartRealAdapter) Stop(label string) error {
	switch a.GOOSFn() {
	case "darwin":
		return a.stopDarwin(label)
	case "linux":
		return a.stopLinux(label)
	default:
		return ErrAutostartUnsupported
	}
}

// ---- darwin implementation ----

// darwinLaunchAgentsDir returns ~/Library/LaunchAgents.
func (a *AutostartRealAdapter) darwinLaunchAgentsDir() (string, error) {
	home, err := a.UserHomeDirFn()
	if err != nil {
		return "", fmt.Errorf("autostart darwin: home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

// darwinPlistPath returns the plist path for label.
func (a *AutostartRealAdapter) darwinPlistPath(label string) (string, error) {
	dir, err := a.darwinLaunchAgentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, label+".plist"), nil
}

// darwinPlistTemplate is the macOS LaunchAgent plist template.
var darwinPlistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Program}}</string>
		{{- range .Args}}
		<string>{{.}}</string>
		{{- end}}
	</array>
	<key>KeepAlive</key>
	<{{if .KeepAlive}}true{{else}}false{{end}}/>
	<key>RunAtLoad</key>
	<{{if .RunAtLoad}}true{{else}}false{{end}}/>
	{{- if .StdoutPath}}
	<key>StandardOutPath</key>
	<string>{{.StdoutPath}}</string>
	{{- end}}
	{{- if .StderrPath}}
	<key>StandardErrorPath</key>
	<string>{{.StderrPath}}</string>
	{{- end}}
	{{- if .Environment}}
	<key>EnvironmentVariables</key>
	<dict>
		{{- range $k, $v := .Environment}}
		<key>{{$k}}</key>
		<string>{{$v}}</string>
		{{- end}}
	</dict>
	{{- end}}
</dict>
</plist>
`))

func (a *AutostartRealAdapter) installDarwin(unit ports.AutostartUnit) error {
	plistPath, err := a.darwinPlistPath(unit.Label)
	if err != nil {
		return err
	}

	// Render the plist via the injectable render seam.
	renderFn := a.RenderDarwinPlistFn
	if renderFn == nil {
		renderFn = defaultRenderDarwinPlist
	}
	content := renderFn(unit)

	// Idempotency: skip if file exists with identical content.
	if existing, err := a.ReadFileFn(plistPath); err == nil {
		if string(existing) == content {
			return nil
		}
	}

	dir, err := a.darwinLaunchAgentsDir()
	if err != nil {
		return err
	}
	if err := a.MkdirAllFn(dir, 0o755); err != nil {
		return fmt.Errorf("autostart darwin: mkdir %s: %w", dir, err)
	}

	if err := a.WriteFileFn(plistPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("autostart darwin: write plist %s: %w", plistPath, err)
	}

	// bootstrap gui/<uid> <plistPath>
	uid := strconv.Itoa(a.GetuidFn())
	cmd := a.CommandFn(context.Background(), "launchctl", "bootstrap", "gui/"+uid, plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("autostart darwin: launchctl bootstrap: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (a *AutostartRealAdapter) uninstallDarwin(label string) error {
	plistPath, err := a.darwinPlistPath(label)
	if err != nil {
		return err
	}

	if _, err := a.StatFn(plistPath); os.IsNotExist(err) {
		return nil // idempotent
	}

	uid := strconv.Itoa(a.GetuidFn())
	// Ignore error: service may not be running.
	stopCmd := a.CommandFn(context.Background(), "launchctl", "bootout", "gui/"+uid+"/"+label)
	_ = stopCmd.Run()

	if err := a.RemoveFn(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("autostart darwin: remove plist %s: %w", plistPath, err)
	}
	return nil
}

func (a *AutostartRealAdapter) statusDarwin(label string) (ports.AutostartStatus, error) {
	plistPath, err := a.darwinPlistPath(label)
	if err != nil {
		return ports.AutostartStatus{}, err
	}

	_, statErr := a.StatFn(plistPath)
	installed := statErr == nil

	if !installed {
		return ports.AutostartStatus{Installed: false}, nil
	}

	// Query launchctl for running status.
	uid := strconv.Itoa(a.GetuidFn())
	cmd := a.CommandFn(context.Background(), "launchctl", "print", "gui/"+uid+"/"+label)
	out, err := cmd.Output()
	if err != nil {
		// Service registered but not running.
		return ports.AutostartStatus{Installed: true, Running: false}, nil
	}

	// Parse PID from launchctl print output (line "pid = <N>").
	pid := parseLaunchctlPID(string(out))
	return ports.AutostartStatus{Installed: true, Running: pid > 0, PID: pid}, nil
}

func (a *AutostartRealAdapter) stopDarwin(label string) error {
	uid := strconv.Itoa(a.GetuidFn())
	cmd := a.CommandFn(context.Background(), "launchctl", "stop", label)
	_ = cmd.Run() // ignore: service may not be running

	// Also try kickstart -k for macOS 11+.
	kickCmd := a.CommandFn(context.Background(), "launchctl", "kill", "SIGTERM", "gui/"+uid+"/"+label)
	_ = kickCmd.Run()
	return nil
}

// parseLaunchctlPID scans launchctl print output for "pid = <N>" and returns N.
// Returns 0 if not found.
func parseLaunchctlPID(output string) int {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pid = ") {
			n, err := strconv.Atoi(strings.TrimPrefix(line, "pid = "))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

// ---- linux implementation ----

// linuxSystemdDir returns ~/.config/systemd/user.
func (a *AutostartRealAdapter) linuxSystemdDir() (string, error) {
	configHome := a.GetenvFn("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := a.UserHomeDirFn()
		if err != nil {
			return "", fmt.Errorf("autostart linux: home dir: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "systemd", "user"), nil
}

// linuxUnitPath returns the unit file path for label.
func (a *AutostartRealAdapter) linuxUnitPath(label string) (string, error) {
	dir, err := a.linuxSystemdDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, label), nil
}

// linuxUnitTemplate is the systemd user unit template.
var linuxUnitTemplate = template.Must(template.New("unit").Parse(`[Unit]
Description={{.Label}}
After=network.target

[Service]
Type=simple
ExecStart={{.Program}}{{range .Args}} {{.}}{{end}}
{{- if .KeepAlive}}
Restart=on-failure
{{- end}}
{{- if .StdoutPath}}
StandardOutput=file:{{.StdoutPath}}
{{- end}}
{{- if .StderrPath}}
StandardError=file:{{.StderrPath}}
{{- end}}
{{- range $k, $v := .Environment}}
Environment={{$k}}={{$v}}
{{- end}}

[Install]
WantedBy=default.target
`))

func (a *AutostartRealAdapter) installLinux(unit ports.AutostartUnit) error {
	// Check systemctl availability.
	lookPath := a.LookPathFn
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath("systemctl"); err != nil {
		return ErrAutostartUnsupported
	}

	unitPath, err := a.linuxUnitPath(unit.Label)
	if err != nil {
		return err
	}

	// Render the unit file via the injectable render seam.
	renderFn := a.RenderLinuxUnitFn
	if renderFn == nil {
		renderFn = defaultRenderLinuxUnit
	}
	content := renderFn(unit)

	if existing, err := a.ReadFileFn(unitPath); err == nil {
		if string(existing) == content {
			return nil // idempotent
		}
	}

	dir, err := a.linuxSystemdDir()
	if err != nil {
		return err
	}
	if err := a.MkdirAllFn(dir, 0o755); err != nil {
		return fmt.Errorf("autostart linux: mkdir %s: %w", dir, err)
	}

	if err := a.WriteFileFn(unitPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("autostart linux: write unit %s: %w", unitPath, err)
	}

	daemonCmd := a.CommandFn(context.Background(), "systemctl", "--user", "daemon-reload")
	if out, err := daemonCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("autostart linux: daemon-reload: %s: %w", strings.TrimSpace(string(out)), err)
	}
	enableCmd := a.CommandFn(context.Background(), "systemctl", "--user", "enable", "--now", unit.Label)
	if out, err := enableCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("autostart linux: enable %s: %s: %w", unit.Label, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (a *AutostartRealAdapter) uninstallLinux(label string) error {
	unitPath, err := a.linuxUnitPath(label)
	if err != nil {
		return err
	}
	if _, statErr := a.StatFn(unitPath); os.IsNotExist(statErr) {
		return nil // idempotent
	}

	// Ignore errors from stop/disable: service may not be active.
	stopCmd := a.CommandFn(context.Background(), "systemctl", "--user", "stop", label)
	_ = stopCmd.Run()
	disableCmd := a.CommandFn(context.Background(), "systemctl", "--user", "disable", label)
	_ = disableCmd.Run()

	if err := a.RemoveFn(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("autostart linux: remove unit %s: %w", unitPath, err)
	}
	reloadCmd := a.CommandFn(context.Background(), "systemctl", "--user", "daemon-reload")
	_ = reloadCmd.Run()
	return nil
}

func (a *AutostartRealAdapter) statusLinux(label string) (ports.AutostartStatus, error) {
	unitPath, err := a.linuxUnitPath(label)
	if err != nil {
		return ports.AutostartStatus{}, err
	}

	_, statErr := a.StatFn(unitPath)
	installed := statErr == nil

	if !installed {
		return ports.AutostartStatus{Installed: false}, nil
	}

	cmd := a.CommandFn(context.Background(), "systemctl", "--user", "show", label, "--property=MainPID,ActiveState")
	out, err := cmd.Output()
	if err != nil {
		return ports.AutostartStatus{Installed: true, Running: false}, nil
	}

	pid, running := parseSystemctlStatus(string(out))
	return ports.AutostartStatus{Installed: true, Running: running, PID: pid}, nil
}

func (a *AutostartRealAdapter) stopLinux(label string) error {
	cmd := a.CommandFn(context.Background(), "systemctl", "--user", "stop", label)
	_ = cmd.Run()
	return nil
}

// parseSystemctlStatus parses "MainPID=N\nActiveState=active\n" output.
// Returns (pid, running). running is true when ActiveState is "active".
func parseSystemctlStatus(output string) (int, bool) {
	var pid int
	var running bool
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MainPID=") {
			n, _ := strconv.Atoi(strings.TrimPrefix(line, "MainPID="))
			pid = n
		}
		if line == "ActiveState=active" {
			running = true
		}
	}
	return pid, running
}
