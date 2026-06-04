package adapters

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fede-iglesias/shipkit/ports"
)

// EnvOSAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.EnvPort]. It reads environment
// variables via os.Getenv/os.LookupEnv and detects shell, OS, arch from the
// process environment and runtime constants.
//
// # Shell detection heuristic
//
// DetectShell follows this precedence:
//  1. Basename of $SHELL environment variable (e.g. "/bin/zsh" -> "zsh").
//  2. Read /proc/<ppid>/comm on linux as a second source.
//  3. Return ShellUnknown as a safe fallback.
//
// The heuristic is best-effort. Callers must handle ShellUnknown gracefully
// (e.g. by skipping shell-specific completion installation with a hint).
//
// # Injectable seams
//
// LookupEnvFn, GetenvFn, GetppidFn, ReadFileFn, and UserCurrentFn are all
// injectable so every branch (including the /proc fallback and error paths) is
// exercisable in unit tests without a real OS.
type EnvOSAdapter struct {
	// LookupEnvFn is the injectable os.LookupEnv. Defaults to os.LookupEnv.
	LookupEnvFn func(key string) (string, bool)

	// GetenvFn is the injectable os.Getenv. Defaults to os.Getenv.
	GetenvFn func(key string) string

	// GetppidFn returns the parent process ID. Defaults to os.Getppid.
	// Used on linux to read /proc/<ppid>/comm for shell detection.
	GetppidFn func() int

	// ReadFileFn reads a file by path. Defaults to os.ReadFile.
	// Used to read /proc/<ppid>/comm on linux.
	ReadFileFn func(name string) ([]byte, error)

	// UserCurrentFn returns the current OS user. Defaults to user.Current.
	UserCurrentFn func() (*user.User, error)

	// GOOSFn returns the OS string. Defaults to returning runtime.GOOS.
	GOOSFn func() string

	// GOARCHFn returns the arch string. Defaults to returning runtime.GOARCH.
	GOARCHFn func() string
}

// NewEnvOS returns an EnvOSAdapter with all seams wired to real os functions.
// This is the constructor consumers must use in production wiring.
func NewEnvOS() *EnvOSAdapter {
	return &EnvOSAdapter{
		LookupEnvFn:   os.LookupEnv,
		GetenvFn:      os.Getenv,
		GetppidFn:     os.Getppid,
		ReadFileFn:    os.ReadFile,
		UserCurrentFn: user.Current,
		GOOSFn:        func() string { return runtime.GOOS },
		GOARCHFn:      func() string { return runtime.GOARCH },
	}
}

// Get returns the value of the environment variable named by key, or an empty
// string if the variable is not set. Equivalent to os.Getenv.
func (a *EnvOSAdapter) Get(key string) string {
	return a.GetenvFn(key)
}

// Lookup returns the value of the named variable and whether it was set.
// Equivalent to os.LookupEnv.
func (a *EnvOSAdapter) Lookup(key string) (string, bool) {
	return a.LookupEnvFn(key)
}

// DetectShell attempts to identify the user's interactive shell using the
// $SHELL environment variable and, on linux, /proc/<ppid>/comm. Returns one
// of ShellBash, ShellZsh, ShellFish, or ShellUnknown.
func (a *EnvOSAdapter) DetectShell() ports.ShellKind {
	// Priority 1: $SHELL basename.
	if s, ok := a.LookupEnvFn("SHELL"); ok && s != "" {
		kind := shellFromName(filepath.Base(s))
		if kind != ports.ShellUnknown {
			return kind
		}
	}

	// Priority 2: /proc/<ppid>/comm on linux only.
	if a.GOOSFn() == "linux" {
		ppid := a.GetppidFn()
		commPath := filepath.Join("/proc", itoa(ppid), "comm")
		if data, err := a.ReadFileFn(commPath); err == nil {
			name := strings.TrimSpace(string(data))
			kind := shellFromName(name)
			if kind != ports.ShellUnknown {
				return kind
			}
		}
	}

	return ports.ShellUnknown
}

// DetectOS returns the operating system identifier in lowercase: "darwin" or
// "linux". Equivalent to runtime.GOOS.
func (a *EnvOSAdapter) DetectOS() string { return a.GOOSFn() }

// DetectArch returns the CPU architecture identifier in lowercase: "amd64" or
// "arm64". Equivalent to runtime.GOARCH.
func (a *EnvOSAdapter) DetectArch() string { return a.GOARCHFn() }

// Username returns the current operating system username. Returns an empty
// string if os/user.Current() fails (e.g. in a minimal container without
// /etc/passwd).
func (a *EnvOSAdapter) Username() string {
	u, err := a.UserCurrentFn()
	if err != nil {
		return ""
	}
	return u.Username
}

// shellFromName maps a process/binary name to a ShellKind. Comparison is
// case-insensitive to handle unusual PATH entries like "ZSH".
func shellFromName(name string) ports.ShellKind {
	switch strings.ToLower(name) {
	case "bash":
		return ports.ShellBash
	case "zsh":
		return ports.ShellZsh
	case "fish":
		return ports.ShellFish
	default:
		return ports.ShellUnknown
	}
}

// itoa converts an int to its decimal string representation without importing
// strconv (avoids a package-level import just for this helper).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
