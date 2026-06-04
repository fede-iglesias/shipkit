package adapters

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathsXDGAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.PathsPort]. It resolves XDG Base
// Directory Specification directories and delegates PATH inspection to the
// os.Getenv("PATH") value at call time.
//
// # XDG defaults by platform
//
// On linux, XDG_DATA_HOME defaults to ~/.local/share, XDG_CONFIG_HOME to
// ~/.config, XDG_CACHE_HOME to ~/.cache.
//
// On darwin, the defaults follow the XDG convention with ~/.local/share etc.
// (not ~/Library) unless XDG_* env vars are set. This matches common homebrew
// tooling expectations and avoids writing to macOS Managed locations. The
// adrg/xdg library is intentionally NOT used as a dependency to keep this
// module lightweight; the resolution below is straightforward and tested.
//
// Explicit XDG_* env vars always take precedence over platform defaults.
//
// # Injectable seams
//
// PathsXDGAdapter uses injectable functions for every os call so failure paths
// can be tested without a real filesystem.
type PathsXDGAdapter struct {
	// GetenvFn returns the value of an environment variable. Defaults to os.Getenv.
	GetenvFn func(string) string

	// UserHomeDirFn returns the current user's home directory.
	// Defaults to os.UserHomeDir.
	UserHomeDirFn func() (string, error)

	// ExecutableFn returns the absolute path of the running binary.
	// Defaults to os.Executable.
	ExecutableFn func() (string, error)

	// GOOSFn returns the operating system string. Defaults to returning
	// runtime.GOOS. Injectable so tests can verify darwin vs linux paths.
	GOOSFn func() string
}

// NewPathsXDG returns a PathsXDGAdapter with all seams wired to real os
// functions. This is the constructor consumers must use in production wiring.
func NewPathsXDG() *PathsXDGAdapter {
	return &PathsXDGAdapter{
		GetenvFn:      os.Getenv,
		UserHomeDirFn: os.UserHomeDir,
		ExecutableFn:  os.Executable,
		GOOSFn:        func() string { return runtime.GOOS },
	}
}

// Executable returns the absolute path of the currently running binary.
// Wraps os.Executable. Returns an error only in the rare case that the OS
// cannot determine the binary path.
func (a *PathsXDGAdapter) Executable() (string, error) {
	return a.ExecutableFn()
}

// DataDir returns the XDG data directory for the given app name, i.e.
// $XDG_DATA_HOME/<app>/ (or ~/.local/share/<app>/ when XDG_DATA_HOME is
// unset). The directory is NOT created; the caller is responsible for mkdir.
func (a *PathsXDGAdapter) DataDir(app string) (string, error) {
	base, err := a.xdgDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, app), nil
}

// ConfigDir returns the XDG config directory for the given app name, i.e.
// $XDG_CONFIG_HOME/<app>/ (or ~/.config/<app>/ when unset). Same mkdir
// contract as DataDir.
func (a *PathsXDGAdapter) ConfigDir(app string) (string, error) {
	base, err := a.xdgConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, app), nil
}

// CacheDir returns the XDG cache directory for the given app name, i.e.
// $XDG_CACHE_HOME/<app>/ (or ~/.cache/<app>/ when unset). Same mkdir
// contract as DataDir.
func (a *PathsXDGAdapter) CacheDir(app string) (string, error) {
	base, err := a.xdgCacheHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, app), nil
}

// UserHome returns the current user's home directory. Equivalent to
// os.UserHomeDir(). Returns an error if HOME is unset on linux.
func (a *PathsXDGAdapter) UserHome() (string, error) {
	return a.UserHomeDirFn()
}

// DefaultInstallDir returns "/usr/local/bin", the conventional directory for
// user-installed binaries on darwin and linux.
func (a *PathsXDGAdapter) DefaultInstallDir() string { return "/usr/local/bin" }

// InPATH reports whether p (an absolute directory path) appears in the
// PATH environment variable. The comparison is case-sensitive on linux
// (consistent with the filesystem) and case-insensitive on darwin.
func (a *PathsXDGAdapter) InPATH(p string) bool {
	for _, dir := range a.PATHList() {
		if a.GOOSFn() == "darwin" {
			if strings.EqualFold(dir, p) {
				return true
			}
		} else {
			if dir == p {
				return true
			}
		}
	}
	return false
}

// PATHList returns the ordered list of directories from the PATH environment
// variable, split on os.PathListSeparator.
func (a *PathsXDGAdapter) PATHList() []string {
	raw := a.GetenvFn("PATH")
	if raw == "" {
		return nil
	}
	return filepath.SplitList(raw)
}

// xdgDataHome resolves the XDG_DATA_HOME base (without the app suffix).
func (a *PathsXDGAdapter) xdgDataHome() (string, error) {
	if v := a.GetenvFn("XDG_DATA_HOME"); v != "" {
		return v, nil
	}
	home, err := a.UserHomeDirFn()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// xdgConfigHome resolves the XDG_CONFIG_HOME base.
func (a *PathsXDGAdapter) xdgConfigHome() (string, error) {
	if v := a.GetenvFn("XDG_CONFIG_HOME"); v != "" {
		return v, nil
	}
	home, err := a.UserHomeDirFn()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// xdgCacheHome resolves the XDG_CACHE_HOME base.
func (a *PathsXDGAdapter) xdgCacheHome() (string, error) {
	if v := a.GetenvFn("XDG_CACHE_HOME"); v != "" {
		return v, nil
	}
	home, err := a.UserHomeDirFn()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}
