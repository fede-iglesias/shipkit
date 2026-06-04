package adapters_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// fakeCmd returns an *exec.Cmd that runs the current test binary with a
// special env var to make it print a controlled stdout and exit with a
// specific code. This pattern avoids any real binary dependency in tests.
func fakeCmd(stdout string, exitCode int) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"HELPER_STDOUT="+stdout,
			"HELPER_EXIT="+exitCodeStr(exitCode),
		)
		return cmd
	}
}

// fakeCmdWithStderr returns an *exec.Cmd that writes to stderr on exit != 0.
func fakeCmdWithStderr(stderr string, exitCode int) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcessStderr")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS_STDERR=1",
			"HELPER_STDERR="+stderr,
			"HELPER_EXIT="+exitCodeStr(exitCode),
		)
		return cmd
	}
}

func exitCodeStr(code int) string {
	if code == 0 {
		return "0"
	}
	return "1"
}

// TestHelperProcess is the subprocess entrypoint used by fakeCmd.
// It is NOT a real test - it is a helper binary that fakeCmd invokes.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	stdout := os.Getenv("HELPER_STDOUT")
	if stdout != "" {
		os.Stdout.WriteString(stdout) //nolint:errcheck
	}
	if os.Getenv("HELPER_EXIT") != "0" {
		os.Exit(1)
	}
	os.Exit(0)
}

// TestHelperProcessStderr is the subprocess entrypoint for stderr tests.
func TestHelperProcessStderr(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_STDERR") != "1" {
		return
	}
	stderr := os.Getenv("HELPER_STDERR")
	if stderr != "" {
		os.Stderr.WriteString(stderr) //nolint:errcheck
	}
	if os.Getenv("HELPER_EXIT") != "0" {
		os.Exit(1)
	}
	os.Exit(0)
}

// --- Unit tests ---

func TestNewRealSpawn_DefaultsWired(t *testing.T) {
	a := adapters.NewRealSpawn()
	if a == nil {
		t.Fatal("NewRealSpawn() returned nil")
	}
	if a.CommandFn == nil {
		t.Fatal("NewRealSpawn() did not wire CommandFn")
	}
}

func TestHealthCheck_HappyPath(t *testing.T) {
	a := &adapters.RealSpawnAdapter{
		CommandFn: fakeCmd("myapp version 0.0.12", 0),
	}
	res, err := a.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected Ok=true, got false; Reason=%q", res.Reason)
	}
	if res.Version != "0.0.12" {
		t.Fatalf("expected Version=0.0.12, got %q", res.Version)
	}
	if res.Reason != "" {
		t.Fatalf("expected empty Reason on success, got %q", res.Reason)
	}
}

func TestHealthCheck_ExitNonZeroReturnsOkFalse(t *testing.T) {
	a := &adapters.RealSpawnAdapter{
		CommandFn: fakeCmd("error output", 1),
	}
	res, err := a.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected Ok=false for non-zero exit")
	}
	if res.Reason == "" {
		t.Fatal("expected non-empty Reason on failure")
	}
}

func TestHealthCheck_ExitNonZeroNoOutputReturnsReasonWithExitStatus(t *testing.T) {
	// Exit non-zero with no stdout and no stderr - exercises the final branch
	// of buildExitReason that returns just the exit code.
	a := &adapters.RealSpawnAdapter{
		CommandFn: fakeCmd("", 1),
	}
	res, err := a.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected Ok=false for non-zero exit with no output")
	}
	if !strings.Contains(res.Reason, "exit status") {
		t.Fatalf("expected Reason to contain 'exit status', got: %q", res.Reason)
	}
}

func TestHealthCheck_ExitNonZeroWithStderrReturnsReasonFromStderr(t *testing.T) {
	a := &adapters.RealSpawnAdapter{
		CommandFn: fakeCmdWithStderr("permission denied", 1),
	}
	res, err := a.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected Ok=false for non-zero exit with stderr")
	}
	if !strings.Contains(res.Reason, "permission denied") {
		t.Fatalf("expected Reason to contain stderr content, got: %q", res.Reason)
	}
}

func TestHealthCheck_UnparseableVersionReturnsOkFalse(t *testing.T) {
	a := &adapters.RealSpawnAdapter{
		CommandFn: fakeCmd("no version here at all", 0),
	}
	res, err := a.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok {
		t.Fatal("expected Ok=false when version not parseable")
	}
	if res.Reason == "" {
		t.Fatal("expected non-empty Reason for unparseable version")
	}
}

func TestHealthCheck_ContextTimeoutReturnsErr(t *testing.T) {
	// Use a very short timeout so the subprocess is killed.
	a := &adapters.RealSpawnAdapter{
		// This helper sleeps by doing nothing and blocking; since GO_WANT_HELPER_PROCESS
		// is set, the process runs but we apply a 1ms HealthCheck timeout which
		// should kill it before it prints anything useful.
		CommandFn: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess_Sleep")
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS_SLEEP=1")
			return cmd
		},
	}
	ctx := context.Background()
	_, err := a.HealthCheck(ctx, "/usr/local/bin/myapp", 1*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from context timeout, got nil")
	}
}

// TestHelperProcess_Sleep is the subprocess entrypoint for the timeout test.
func TestHelperProcess_Sleep(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_SLEEP") != "1" {
		return
	}
	// Block "forever" - the parent will kill us via context timeout.
	select {}
}

func TestHealthCheck_BinaryNotFoundReturnsErr(t *testing.T) {
	// Use real exec.Command with a path that does not exist.
	a := adapters.NewRealSpawn()
	ctx := context.Background()
	_, err := a.HealthCheck(ctx, "/nonexistent/path/to/myapp-binary-zzz", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

func TestHealthCheck_VersionParseFormats(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		wantVer string
		wantOk  bool
	}{
		{"plain semver", "0.0.12", "0.0.12", true},
		{"v-prefix", "v0.0.12", "0.0.12", true},
		{"rc prerelease", "0.0.12-rc1", "0.0.12-rc1", true},
		{"alpha prerelease", "1.2.3-alpha.1", "1.2.3-alpha.1", true},
		{"embedded in sentence", "myapp CLI v0.0.12 (build 42)", "0.0.12", true},
		{"myapp version prefix", "myapp version 1.0.0", "1.0.0", true},
		{"no version", "no semver here", "", false},
		{"empty output", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &adapters.RealSpawnAdapter{
				CommandFn: fakeCmd(tc.output, 0),
			}
			res, err := a.HealthCheck(context.Background(), "/usr/local/bin/myapp", 5*time.Second)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Ok != tc.wantOk {
				t.Errorf("Ok: got %v, want %v (Reason=%q)", res.Ok, tc.wantOk, res.Reason)
			}
			if res.Version != tc.wantVer {
				t.Errorf("Version: got %q, want %q", res.Version, tc.wantVer)
			}
		})
	}
}

// TestSpawn_OnlyKtBinaryReferenced is an anti-regression source grep.
// It reads spawn_real.go bytes and asserts no forbidden binary references exist.
func TestSpawn_OnlyKtBinaryReferenced(t *testing.T) {
	src, err := os.ReadFile("spawn_real.go")
	if err != nil {
		t.Fatalf("could not read spawn_real.go: %v", err)
	}
	content := string(src)

	forbidden := []struct {
		pattern string
		reason  string
	}{
		{`/usr/bin/claude`, "D-7: must not hard-code claude binary path"},
		{`exec.Command("claude"`, "D-7: must not exec claude binary directly"},
		{`"claude"`, "D-7: must not reference 'claude' as a string literal (argv[0] guard)"},
		{`/usr/bin/cosign`, "D-7: must not hard-code cosign binary path"},
		{`exec.Command("cosign"`, "D-7: must not exec cosign binary directly"},
	}
	for _, f := range forbidden {
		if strings.Contains(content, f.pattern) {
			t.Errorf("D-7 violation: found %q in spawn_real.go - %s", f.pattern, f.reason)
		}
	}
}

// compile-time interface compliance check.
var _ ports.SpawnPort = (*adapters.RealSpawnAdapter)(nil)
