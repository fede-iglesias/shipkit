package adapters

import (
	"os/user"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// testEnvAdapter returns an EnvOSAdapter with seams suitable for deterministic
// tests. envMap provides the fake environment; goos controls platform branches.
func testEnvAdapter(envMap map[string]string, goos string) *EnvOSAdapter {
	return &EnvOSAdapter{
		LookupEnvFn: func(key string) (string, bool) {
			v, ok := envMap[key]
			return v, ok
		},
		GetenvFn: func(key string) string { return envMap[key] },
		GetppidFn: func() int { return 1 },
		ReadFileFn: func(name string) ([]byte, error) {
			// Simulate /proc/1/comm for linux shell detection tests.
			if name == "/proc/1/comm" {
				if s, ok := envMap["__proc_comm"]; ok {
					return []byte(s), nil
				}
			}
			return nil, &fakeFileError{path: name}
		},
		UserCurrentFn: func() (*user.User, error) {
			if u, ok := envMap["__username"]; ok {
				return &user.User{Username: u}, nil
			}
			return &user.User{Username: "testuser"}, nil
		},
		GOOSFn:   func() string { return goos },
		GOARCHFn: func() string { return "arm64" },
	}
}

// fakeFileError simulates os.PathError for ReadFile error-path tests.
type fakeFileError struct{ path string }

func (e *fakeFileError) Error() string { return "open " + e.path + ": no such file or directory" }

// TestNewEnvOS verifies the constructor returns a non-nil adapter.
func TestNewEnvOS(t *testing.T) {
	a := NewEnvOS()
	if a == nil {
		t.Fatal("NewEnvOS returned nil")
	}
}

// TestEnvOSAdapter_Get returns the value from the env map.
func TestEnvOSAdapter_Get(t *testing.T) {
	a := testEnvAdapter(map[string]string{"FOO": "bar"}, "linux")
	if got := a.Get("FOO"); got != "bar" {
		t.Errorf("Get(FOO) = %q; want bar", got)
	}
	if got := a.Get("MISSING"); got != "" {
		t.Errorf("Get(MISSING) = %q; want empty", got)
	}
}

// TestEnvOSAdapter_Lookup verifies the ok flag.
func TestEnvOSAdapter_Lookup(t *testing.T) {
	a := testEnvAdapter(map[string]string{"FOO": "bar"}, "linux")
	v, ok := a.Lookup("FOO")
	if !ok || v != "bar" {
		t.Errorf("Lookup(FOO) = %q, %v; want bar, true", v, ok)
	}
	_, ok = a.Lookup("MISSING")
	if ok {
		t.Error("Lookup(MISSING) ok = true; want false")
	}
}

// TestEnvOSAdapter_DetectShell_FromSHELL verifies $SHELL basename detection.
func TestEnvOSAdapter_DetectShell_FromSHELL(t *testing.T) {
	cases := []struct {
		shellEnv string
		want     ports.ShellKind
	}{
		{"/bin/bash", ports.ShellBash},
		{"/usr/bin/zsh", ports.ShellZsh},
		{"/usr/local/bin/fish", ports.ShellFish},
		{"/bin/sh", ports.ShellUnknown},
	}
	for _, c := range cases {
		a := testEnvAdapter(map[string]string{"SHELL": c.shellEnv}, "linux")
		got := a.DetectShell()
		if got != c.want {
			t.Errorf("DetectShell(SHELL=%s) = %q; want %q", c.shellEnv, got, c.want)
		}
	}
}

// TestEnvOSAdapter_DetectShell_FromProc verifies /proc/<ppid>/comm fallback on linux.
func TestEnvOSAdapter_DetectShell_FromProc(t *testing.T) {
	// No SHELL env, but /proc/1/comm returns "zsh".
	a := testEnvAdapter(map[string]string{"__proc_comm": "zsh"}, "linux")
	got := a.DetectShell()
	if got != ports.ShellZsh {
		t.Errorf("DetectShell via /proc/comm = %q; want zsh", got)
	}
}

// TestEnvOSAdapter_DetectShell_UnknownFallback verifies that when both $SHELL
// and /proc/comm are unhelpful, ShellUnknown is returned.
func TestEnvOSAdapter_DetectShell_UnknownFallback(t *testing.T) {
	a := testEnvAdapter(map[string]string{}, "linux")
	got := a.DetectShell()
	if got != ports.ShellUnknown {
		t.Errorf("DetectShell fallback = %q; want unknown", got)
	}
}

// TestEnvOSAdapter_DetectShell_DarwinNoProcFallback verifies that the /proc
// branch is NOT taken on darwin (even when SHELL is absent).
func TestEnvOSAdapter_DetectShell_DarwinNoProcFallback(t *testing.T) {
	a := testEnvAdapter(map[string]string{"__proc_comm": "bash"}, "darwin")
	got := a.DetectShell()
	if got != ports.ShellUnknown {
		t.Errorf("DetectShell on darwin without SHELL = %q; want unknown", got)
	}
}

// TestEnvOSAdapter_DetectOS delegates to GOOSFn.
func TestEnvOSAdapter_DetectOS(t *testing.T) {
	a := testEnvAdapter(nil, "darwin")
	if got := a.DetectOS(); got != "darwin" {
		t.Errorf("DetectOS = %q; want darwin", got)
	}
}

// TestEnvOSAdapter_DetectArch delegates to GOARCHFn.
func TestEnvOSAdapter_DetectArch(t *testing.T) {
	a := testEnvAdapter(nil, "linux")
	if got := a.DetectArch(); got != "arm64" {
		t.Errorf("DetectArch = %q; want arm64", got)
	}
}

// TestEnvOSAdapter_Username_Happy verifies the happy path.
func TestEnvOSAdapter_Username_Happy(t *testing.T) {
	a := testEnvAdapter(map[string]string{"__username": "alice"}, "linux")
	got := a.Username()
	if got != "alice" {
		t.Errorf("Username = %q; want alice", got)
	}
}

// TestEnvOSAdapter_Username_Error verifies that a user.Current error returns
// an empty string.
func TestEnvOSAdapter_Username_Error(t *testing.T) {
	a := &EnvOSAdapter{
		LookupEnvFn:   func(string) (string, bool) { return "", false },
		GetenvFn:      func(string) string { return "" },
		GetppidFn:     func() int { return 0 },
		ReadFileFn:    func(string) ([]byte, error) { return nil, nil },
		UserCurrentFn: func() (*user.User, error) { return nil, &fakeFileError{"user"} },
		GOOSFn:        func() string { return "linux" },
		GOARCHFn:      func() string { return "amd64" },
	}
	if got := a.Username(); got != "" {
		t.Errorf("Username on error = %q; want empty", got)
	}
}

// TestItoa verifies the helper covers 0, positive, and negative values.
func TestItoa(t *testing.T) {
	cases := []struct{ n int; want string }{
		{0, "0"},
		{42, "42"},
		{-7, "-7"},
		{100, "100"},
	}
	for _, c := range cases {
		got := itoa(c.n)
		if got != c.want {
			t.Errorf("itoa(%d) = %q; want %q", c.n, got, c.want)
		}
	}
}

// TestShellFromName verifies all recognized shell names and the unknown fallback.
func TestShellFromName(t *testing.T) {
	cases := []struct {
		name string
		want ports.ShellKind
	}{
		{"bash", ports.ShellBash},
		{"BASH", ports.ShellBash},
		{"zsh", ports.ShellZsh},
		{"ZSH", ports.ShellZsh},
		{"fish", ports.ShellFish},
		{"FISH", ports.ShellFish},
		{"sh", ports.ShellUnknown},
		{"", ports.ShellUnknown},
	}
	for _, c := range cases {
		got := shellFromName(c.name)
		if got != c.want {
			t.Errorf("shellFromName(%q) = %q; want %q", c.name, got, c.want)
		}
	}
}
