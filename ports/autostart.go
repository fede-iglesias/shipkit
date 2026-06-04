package ports

// AutostartUnit describes a platform service unit to be installed by
// AutostartPort. The fields cover both darwin LaunchAgent plists and linux
// systemd-user unit files; the adapter maps them to the platform-appropriate
// representation.
type AutostartUnit struct {
	// Label is the unique service identifier.
	//   darwin: reverse-DNS label, e.g. "com.fede-iglesias.myapp"
	//   linux:  systemd unit name, e.g. "myapp.service"
	Label string

	// Program is the absolute path to the binary to run.
	Program string

	// Args is the list of arguments to pass to Program.
	Args []string

	// KeepAlive requests that the service be restarted automatically if it
	// exits. Maps to darwin KeepAlive = true; linux Restart=on-failure.
	KeepAlive bool

	// RunAtLoad requests that the service start immediately when the unit is
	// installed (loaded). Maps to darwin RunAtLoad = true; linux [Install]
	// WantedBy=default.target with immediate start.
	RunAtLoad bool

	// StdoutPath is the path to redirect stdout to (optional).
	// Empty means the service inherits or discards stdout.
	StdoutPath string

	// StderrPath is the path to redirect stderr to (optional).
	StderrPath string

	// Environment is a map of additional environment variables to set when
	// running the service. The adapter merges these with the user environment.
	Environment map[string]string
}

// AutostartStatus describes the current state of a service managed by
// AutostartPort.
type AutostartStatus struct {
	// Installed is true when the service unit file exists on disk.
	Installed bool

	// Running is true when the service process is currently active.
	Running bool

	// PID is the process ID of the running service, or 0 if not running.
	PID int
}

// AutostartPort abstracts platform-specific service management for the install,
// uninstall, and doctor verbs.
//
// darwin adapter: creates/removes plist files under ~/Library/LaunchAgents/
// and calls launchctl bootstrap/bootout. No system-level writes; no sudo.
//
// linux adapter: creates/removes unit files under ~/.config/systemd/user/ and
// calls systemctl --user enable/disable/start/stop. If systemd-user is not
// available (Alpine OpenRC, NixOS), Install returns ErrAutostartUnsupported.
//
// Implementations must never require elevated privileges (no sudo, no pkexec).
type AutostartPort interface {
	// Install writes the platform-specific service unit file for unit and
	// registers it with the init system. If the unit already exists and its
	// content matches, Install is a no-op. If the content differs, the unit is
	// replaced and reloaded. Returns ErrAutostartUnsupported when the platform
	// does not support user-scope service management.
	Install(unit AutostartUnit) error

	// Uninstall stops the service identified by label (if running), then
	// removes its unit file. Returns nil if the unit does not exist (idempotent).
	Uninstall(label string) error

	// Status returns the current installation and runtime state of the service
	// identified by label. Returns AutostartStatus with Installed = false if
	// the unit file does not exist.
	Status(label string) (AutostartStatus, error)

	// Stop sends a stop signal to the running service identified by label.
	// Returns nil if the service is not running (idempotent).
	Stop(label string) error
}

// MockAutostartPort is a test double for AutostartPort. It records calls and
// returns the values set on its Func fields. Use NewMockAutostartPort for safe
// defaults.
type MockAutostartPort struct {
	// InstallFunc overrides Install when non-nil.
	InstallFunc func(unit AutostartUnit) error
	// UninstallFunc overrides Uninstall when non-nil.
	UninstallFunc func(label string) error
	// StatusFunc overrides Status when non-nil.
	StatusFunc func(label string) (AutostartStatus, error)
	// StopFunc overrides Stop when non-nil.
	StopFunc func(label string) error

	// InstallCalls records each AutostartUnit passed to Install.
	InstallCalls []AutostartUnit
	// UninstallCalls records each label passed to Uninstall.
	UninstallCalls []string
	// StatusCalls records each label passed to Status.
	StatusCalls []string
	// StopCalls records each label passed to Stop.
	StopCalls []string
}

// NewMockAutostartPort returns a MockAutostartPort whose methods return safe
// defaults: Install/Uninstall/Stop return nil, Status returns
// AutostartStatus{Installed: true, Running: false} unless Func fields are set.
func NewMockAutostartPort() *MockAutostartPort { return &MockAutostartPort{} }

// Install implements AutostartPort.
func (m *MockAutostartPort) Install(unit AutostartUnit) error {
	m.InstallCalls = append(m.InstallCalls, unit)
	if m.InstallFunc != nil {
		return m.InstallFunc(unit)
	}
	return nil
}

// Uninstall implements AutostartPort.
func (m *MockAutostartPort) Uninstall(label string) error {
	m.UninstallCalls = append(m.UninstallCalls, label)
	if m.UninstallFunc != nil {
		return m.UninstallFunc(label)
	}
	return nil
}

// Status implements AutostartPort.
func (m *MockAutostartPort) Status(label string) (AutostartStatus, error) {
	m.StatusCalls = append(m.StatusCalls, label)
	if m.StatusFunc != nil {
		return m.StatusFunc(label)
	}
	return AutostartStatus{Installed: true, Running: false}, nil
}

// Stop implements AutostartPort.
func (m *MockAutostartPort) Stop(label string) error {
	m.StopCalls = append(m.StopCalls, label)
	if m.StopFunc != nil {
		return m.StopFunc(label)
	}
	return nil
}
