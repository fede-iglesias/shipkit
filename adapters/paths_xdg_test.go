package adapters

import (
	"errors"
	"path/filepath"
	"testing"
)

// testPathsAdapter returns a PathsXDGAdapter with injectable seams set to
// values convenient for testing. All env vars are empty by default; caller
// sets specific keys via envMap.
func testPathsAdapter(envMap map[string]string, goos string) *PathsXDGAdapter {
	return &PathsXDGAdapter{
		GetenvFn: func(key string) string { return envMap[key] },
		UserHomeDirFn: func() (string, error) {
			if h, ok := envMap["HOME"]; ok {
				return h, nil
			}
			return "/home/user", nil
		},
		ExecutableFn: func() (string, error) { return "/usr/local/bin/app", nil },
		GOOSFn:       func() string { return goos },
	}
}

// TestNewPathsXDG verifies the constructor returns a non-nil adapter.
func TestNewPathsXDG(t *testing.T) {
	a := NewPathsXDG()
	if a == nil {
		t.Fatal("NewPathsXDG returned nil")
	}
}

// TestPathsXDGAdapter_Executable returns the injected path.
func TestPathsXDGAdapter_Executable(t *testing.T) {
	a := testPathsAdapter(nil, "linux")
	got, err := a.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	if got != "/usr/local/bin/app" {
		t.Errorf("Executable = %q; want /usr/local/bin/app", got)
	}
}

// TestPathsXDGAdapter_Executable_Error exercises the error path.
func TestPathsXDGAdapter_Executable_Error(t *testing.T) {
	sentinel := errors.New("no executable")
	a := &PathsXDGAdapter{
		ExecutableFn: func() (string, error) { return "", sentinel },
	}
	_, err := a.Executable()
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestPathsXDGAdapter_DataDir_DefaultPath verifies the default
// ~/.local/share/<app> path when XDG_DATA_HOME is not set.
func TestPathsXDGAdapter_DataDir_DefaultPath(t *testing.T) {
	a := testPathsAdapter(map[string]string{"HOME": "/home/user"}, "linux")
	got, err := a.DataDir("myapp")
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	want := "/home/user/.local/share/myapp"
	if got != want {
		t.Errorf("DataDir = %q; want %q", got, want)
	}
}

// TestPathsXDGAdapter_DataDir_XDGOverride verifies that XDG_DATA_HOME takes
// precedence over the default.
func TestPathsXDGAdapter_DataDir_XDGOverride(t *testing.T) {
	a := testPathsAdapter(map[string]string{"XDG_DATA_HOME": "/data"}, "linux")
	got, err := a.DataDir("myapp")
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if got != "/data/myapp" {
		t.Errorf("DataDir = %q; want /data/myapp", got)
	}
}

// TestPathsXDGAdapter_DataDir_HomeError exercises the error path when
// os.UserHomeDir fails.
func TestPathsXDGAdapter_DataDir_HomeError(t *testing.T) {
	sentinel := errors.New("no home")
	a := &PathsXDGAdapter{
		GetenvFn:      func(string) string { return "" },
		UserHomeDirFn: func() (string, error) { return "", sentinel },
		ExecutableFn:  func() (string, error) { return "", nil },
		GOOSFn:        func() string { return "linux" },
	}
	_, err := a.DataDir("app")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestPathsXDGAdapter_ConfigDir_DefaultPath verifies the ~/.config/<app> default.
func TestPathsXDGAdapter_ConfigDir_DefaultPath(t *testing.T) {
	a := testPathsAdapter(map[string]string{"HOME": "/home/user"}, "linux")
	got, err := a.ConfigDir("myapp")
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	want := "/home/user/.config/myapp"
	if got != want {
		t.Errorf("ConfigDir = %q; want %q", got, want)
	}
}

// TestPathsXDGAdapter_ConfigDir_XDGOverride verifies XDG_CONFIG_HOME override.
func TestPathsXDGAdapter_ConfigDir_XDGOverride(t *testing.T) {
	a := testPathsAdapter(map[string]string{"XDG_CONFIG_HOME": "/cfg"}, "linux")
	got, err := a.ConfigDir("myapp")
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if got != "/cfg/myapp" {
		t.Errorf("ConfigDir = %q; want /cfg/myapp", got)
	}
}

// TestPathsXDGAdapter_ConfigDir_HomeError exercises the error path.
func TestPathsXDGAdapter_ConfigDir_HomeError(t *testing.T) {
	sentinel := errors.New("no home")
	a := &PathsXDGAdapter{
		GetenvFn:      func(string) string { return "" },
		UserHomeDirFn: func() (string, error) { return "", sentinel },
		ExecutableFn:  func() (string, error) { return "", nil },
		GOOSFn:        func() string { return "linux" },
	}
	_, err := a.ConfigDir("app")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestPathsXDGAdapter_CacheDir_DefaultPath verifies the ~/.cache/<app> default.
func TestPathsXDGAdapter_CacheDir_DefaultPath(t *testing.T) {
	a := testPathsAdapter(map[string]string{"HOME": "/home/user"}, "linux")
	got, err := a.CacheDir("myapp")
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	want := "/home/user/.cache/myapp"
	if got != want {
		t.Errorf("CacheDir = %q; want %q", got, want)
	}
}

// TestPathsXDGAdapter_CacheDir_XDGOverride verifies XDG_CACHE_HOME override.
func TestPathsXDGAdapter_CacheDir_XDGOverride(t *testing.T) {
	a := testPathsAdapter(map[string]string{"XDG_CACHE_HOME": "/cache"}, "linux")
	got, err := a.CacheDir("myapp")
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if got != "/cache/myapp" {
		t.Errorf("CacheDir = %q; want /cache/myapp", got)
	}
}

// TestPathsXDGAdapter_CacheDir_HomeError exercises the error path.
func TestPathsXDGAdapter_CacheDir_HomeError(t *testing.T) {
	sentinel := errors.New("no home")
	a := &PathsXDGAdapter{
		GetenvFn:      func(string) string { return "" },
		UserHomeDirFn: func() (string, error) { return "", sentinel },
		ExecutableFn:  func() (string, error) { return "", nil },
		GOOSFn:        func() string { return "linux" },
	}
	_, err := a.CacheDir("app")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestPathsXDGAdapter_UserHome verifies delegation to UserHomeDirFn.
func TestPathsXDGAdapter_UserHome(t *testing.T) {
	a := testPathsAdapter(map[string]string{"HOME": "/home/testuser"}, "linux")
	got, err := a.UserHome()
	if err != nil {
		t.Fatalf("UserHome: %v", err)
	}
	if got != "/home/testuser" {
		t.Errorf("UserHome = %q; want /home/testuser", got)
	}
}

// TestPathsXDGAdapter_UserHome_Error exercises the error path.
func TestPathsXDGAdapter_UserHome_Error(t *testing.T) {
	sentinel := errors.New("no home")
	a := &PathsXDGAdapter{
		GetenvFn:      func(string) string { return "" },
		UserHomeDirFn: func() (string, error) { return "", sentinel },
		ExecutableFn:  func() (string, error) { return "", nil },
		GOOSFn:        func() string { return "linux" },
	}
	_, err := a.UserHome()
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel; got %v", err)
	}
}

// TestPathsXDGAdapter_DefaultInstallDir verifies the constant return.
func TestPathsXDGAdapter_DefaultInstallDir(t *testing.T) {
	a := testPathsAdapter(nil, "linux")
	got := a.DefaultInstallDir()
	if got != "/usr/local/bin" {
		t.Errorf("DefaultInstallDir = %q; want /usr/local/bin", got)
	}
}

// TestPathsXDGAdapter_InPATH_LinuxCaseSensitive verifies case-sensitive match
// on linux.
func TestPathsXDGAdapter_InPATH_LinuxCaseSensitive(t *testing.T) {
	a := testPathsAdapter(map[string]string{"PATH": "/usr/local/bin:/usr/bin"}, "linux")
	if !a.InPATH("/usr/local/bin") {
		t.Error("InPATH /usr/local/bin = false; want true")
	}
	if a.InPATH("/USR/LOCAL/BIN") {
		t.Error("InPATH /USR/LOCAL/BIN = true on linux; want false")
	}
}

// TestPathsXDGAdapter_InPATH_DarwinCaseInsensitive verifies case-insensitive
// match on darwin.
func TestPathsXDGAdapter_InPATH_DarwinCaseInsensitive(t *testing.T) {
	a := testPathsAdapter(map[string]string{"PATH": "/usr/local/bin"}, "darwin")
	if !a.InPATH("/USR/LOCAL/BIN") {
		t.Error("InPATH /USR/LOCAL/BIN on darwin = false; want true")
	}
}

// TestPathsXDGAdapter_InPATH_Empty verifies that an empty PATH returns false.
func TestPathsXDGAdapter_InPATH_Empty(t *testing.T) {
	a := testPathsAdapter(map[string]string{"PATH": ""}, "linux")
	if a.InPATH("/usr/local/bin") {
		t.Error("InPATH with empty PATH = true; want false")
	}
}

// TestPathsXDGAdapter_PATHList verifies the list is split correctly.
func TestPathsXDGAdapter_PATHList(t *testing.T) {
	pathVal := "/usr/local/bin" + string(filepath.ListSeparator) + "/usr/bin"
	a := testPathsAdapter(map[string]string{"PATH": pathVal}, "linux")
	list := a.PATHList()
	if len(list) != 2 {
		t.Fatalf("PATHList len = %d; want 2", len(list))
	}
	if list[0] != "/usr/local/bin" {
		t.Errorf("list[0] = %q; want /usr/local/bin", list[0])
	}
}

// TestPathsXDGAdapter_PATHList_EmptyPath verifies that an empty PATH returns nil.
func TestPathsXDGAdapter_PATHList_EmptyPath(t *testing.T) {
	a := testPathsAdapter(map[string]string{}, "linux")
	if a.PATHList() != nil {
		t.Error("PATHList with empty PATH should be nil")
	}
}
