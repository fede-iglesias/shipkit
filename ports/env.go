package ports

// ShellKind identifies the user's interactive shell.
type ShellKind string

const (
	// ShellBash identifies the Bash shell.
	ShellBash ShellKind = "bash"

	// ShellZsh identifies the Z shell.
	ShellZsh ShellKind = "zsh"

	// ShellFish identifies the Fish shell.
	ShellFish ShellKind = "fish"

	// ShellUnknown is returned when the shell cannot be detected.
	ShellUnknown ShellKind = "unknown"
)

// EnvPort abstracts environment variable access and OS/architecture detection.
//
// The install and doctor verbs use EnvPort to detect the user's shell (for
// completion path selection and shellrc editing), the operating system (for
// platform-specific behaviour), and environment variable values without reading
// os.Getenv directly.
//
// DetectShell resolution order:
//  1. Basename of $SHELL env var.
//  2. /proc/<ppid>/comm on linux (parent process name).
//  3. ShellUnknown as fallback.
type EnvPort interface {
	// Get returns the value of the environment variable named by key, or an
	// empty string if the variable is not set. Equivalent to os.Getenv.
	Get(key string) string

	// Lookup returns the value of the named variable and whether it was set.
	// Equivalent to os.LookupEnv.
	Lookup(key string) (string, bool)

	// DetectShell attempts to identify the user's interactive shell.
	// Returns one of ShellBash, ShellZsh, ShellFish, or ShellUnknown.
	// This is a best-effort heuristic; callers must handle ShellUnknown.
	DetectShell() ShellKind

	// DetectOS returns the operating system identifier in lowercase:
	// "darwin" or "linux". Equivalent to runtime.GOOS.
	DetectOS() string

	// DetectArch returns the CPU architecture identifier in lowercase:
	// "amd64" or "arm64". Equivalent to runtime.GOARCH.
	DetectArch() string

	// Username returns the current operating system username. Returns an
	// empty string if it cannot be determined.
	Username() string
}

// MockEnvPort is a test double for EnvPort. It returns configured values for
// each query. Use NewMockEnvPort for safe defaults.
type MockEnvPort struct {
	// Env holds the environment variables returned by Get and Lookup.
	Env map[string]string

	// ShellResult is returned by DetectShell. Defaults to ShellZsh.
	ShellResult ShellKind
	// OSResult is returned by DetectOS. Defaults to "darwin".
	OSResult string
	// ArchResult is returned by DetectArch. Defaults to "arm64".
	ArchResult string
	// UsernameResult is returned by Username. Defaults to "testuser".
	UsernameResult string
}

// NewMockEnvPort returns a MockEnvPort with sensible test defaults for macOS
// arm64 running zsh.
func NewMockEnvPort() *MockEnvPort {
	return &MockEnvPort{
		Env:            map[string]string{},
		ShellResult:    ShellZsh,
		OSResult:       "darwin",
		ArchResult:     "arm64",
		UsernameResult: "testuser",
	}
}

// Get implements EnvPort.
func (m *MockEnvPort) Get(key string) string {
	return m.Env[key]
}

// Lookup implements EnvPort.
func (m *MockEnvPort) Lookup(key string) (string, bool) {
	v, ok := m.Env[key]
	return v, ok
}

// DetectShell implements EnvPort.
func (m *MockEnvPort) DetectShell() ShellKind { return m.ShellResult }

// DetectOS implements EnvPort.
func (m *MockEnvPort) DetectOS() string { return m.OSResult }

// DetectArch implements EnvPort.
func (m *MockEnvPort) DetectArch() string { return m.ArchResult }

// Username implements EnvPort.
func (m *MockEnvPort) Username() string { return m.UsernameResult }
