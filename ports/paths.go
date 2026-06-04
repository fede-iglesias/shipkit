package ports

// PathsPort abstracts filesystem path resolution for XDG directories, the
// user's home directory, the current binary path, and PATH inspection.
//
// The install, uninstall, and doctor verbs use this port to locate data, config,
// cache, and completion directories without hard-coding OS-specific paths or
// calling os.UserHomeDir / os.Executable directly.
//
// The production adapter (shipkit/adapters) backs this with adrg/xdg for
// cross-platform XDG directory resolution. darwin and linux differ in their
// XDG_DATA_HOME defaults (/Library/Application Support vs ~/.local/share) but
// both honour explicit XDG_* env overrides.
type PathsPort interface {
	// Executable returns the absolute path of the currently running binary,
	// equivalent to os.Executable() but injectable for tests. Returns an error
	// if the path cannot be determined.
	Executable() (string, error)

	// DataDir returns the XDG data directory for the given app name,
	// i.e. XDG_DATA_HOME/<app>/. The directory is NOT created; the caller
	// (install verb) is responsible for mkdir. Returns an error if the base
	// XDG_DATA_HOME cannot be resolved.
	DataDir(app string) (string, error)

	// ConfigDir returns the XDG config directory for the given app name,
	// i.e. XDG_CONFIG_HOME/<app>/. Same mkdir contract as DataDir.
	ConfigDir(app string) (string, error)

	// CacheDir returns the XDG cache directory for the given app name,
	// i.e. XDG_CACHE_HOME/<app>/. Same mkdir contract as DataDir.
	CacheDir(app string) (string, error)

	// UserHome returns the current user's home directory, equivalent to
	// os.UserHomeDir(). Returns an error if the home directory cannot be
	// determined (rare; typically missing HOME env var on linux).
	UserHome() (string, error)

	// DefaultInstallDir returns the conventional system binary directory where
	// the app expects to live (e.g. "/usr/local/bin"). The install verb uses
	// this as a fallback when the user has not explicitly configured BinPath.
	// The return value is a constant string (never an error).
	DefaultInstallDir() string

	// InPATH reports whether the given absolute path p is a directory listed in
	// the current $PATH. Used by install and doctor to warn when the binary
	// directory is not reachable from the shell.
	InPATH(p string) bool

	// PATHList returns the ordered list of directories from $PATH, split on
	// os.PathListSeparator. Useful for doctor's path check diagnostics.
	PATHList() []string
}

// MockPathsPort is a test double for PathsPort. It returns configured values
// for each path query. Use NewMockPathsPort for safe defaults.
type MockPathsPort struct {
	// ExecutableResult is returned by Executable. Defaults to "/usr/local/bin/app".
	ExecutableResult string
	// ExecutableErr is returned as the error from Executable when non-nil.
	ExecutableErr error

	// DataDirFunc overrides DataDir when non-nil.
	DataDirFunc func(app string) (string, error)
	// ConfigDirFunc overrides ConfigDir when non-nil.
	ConfigDirFunc func(app string) (string, error)
	// CacheDirFunc overrides CacheDir when non-nil.
	CacheDirFunc func(app string) (string, error)

	// UserHomeResult is returned by UserHome. Defaults to "/home/user".
	UserHomeResult string
	// UserHomeErr is returned as the error from UserHome when non-nil.
	UserHomeErr error

	// DefaultInstallDirResult is returned by DefaultInstallDir.
	DefaultInstallDirResult string

	// InPATHResult is returned by InPATH when InPATHFunc is nil.
	InPATHResult bool
	// InPATHFunc overrides InPATH when non-nil.
	InPATHFunc func(p string) bool

	// PATHListResult is returned by PATHList.
	PATHListResult []string

	// DataDirCalls records each app name passed to DataDir.
	DataDirCalls []string
	// ConfigDirCalls records each app name passed to ConfigDir.
	ConfigDirCalls []string
	// CacheDirCalls records each app name passed to CacheDir.
	CacheDirCalls []string
}

// NewMockPathsPort returns a MockPathsPort with sensible test defaults:
// paths under /tmp/testapp, InPATH true, PATHList with /usr/local/bin.
func NewMockPathsPort() *MockPathsPort {
	return &MockPathsPort{
		ExecutableResult:        "/usr/local/bin/app",
		UserHomeResult:          "/home/user",
		DefaultInstallDirResult: "/usr/local/bin",
		InPATHResult:            true,
		PATHListResult:          []string{"/usr/local/bin", "/usr/bin"},
	}
}

// Executable implements PathsPort.
func (m *MockPathsPort) Executable() (string, error) {
	return m.ExecutableResult, m.ExecutableErr
}

// DataDir implements PathsPort.
func (m *MockPathsPort) DataDir(app string) (string, error) {
	m.DataDirCalls = append(m.DataDirCalls, app)
	if m.DataDirFunc != nil {
		return m.DataDirFunc(app)
	}
	return "/tmp/testapp/data/" + app, nil
}

// ConfigDir implements PathsPort.
func (m *MockPathsPort) ConfigDir(app string) (string, error) {
	m.ConfigDirCalls = append(m.ConfigDirCalls, app)
	if m.ConfigDirFunc != nil {
		return m.ConfigDirFunc(app)
	}
	return "/tmp/testapp/config/" + app, nil
}

// CacheDir implements PathsPort.
func (m *MockPathsPort) CacheDir(app string) (string, error) {
	m.CacheDirCalls = append(m.CacheDirCalls, app)
	if m.CacheDirFunc != nil {
		return m.CacheDirFunc(app)
	}
	return "/tmp/testapp/cache/" + app, nil
}

// UserHome implements PathsPort.
func (m *MockPathsPort) UserHome() (string, error) {
	return m.UserHomeResult, m.UserHomeErr
}

// DefaultInstallDir implements PathsPort.
func (m *MockPathsPort) DefaultInstallDir() string {
	if m.DefaultInstallDirResult != "" {
		return m.DefaultInstallDirResult
	}
	return "/usr/local/bin"
}

// InPATH implements PathsPort.
func (m *MockPathsPort) InPATH(p string) bool {
	if m.InPATHFunc != nil {
		return m.InPATHFunc(p)
	}
	return m.InPATHResult
}

// PATHList implements PathsPort.
func (m *MockPathsPort) PATHList() []string { return m.PATHListResult }
