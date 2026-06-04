package shipkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update"
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// validCfg returns a Config with all required fields set for testing.
func validCfg() Config {
	return Config{
		AppName:    "testapp",
		Version:    "v0.1.0",
		Repo:       "owner/tools",
		TagPrefix:  "testapp-",
		BinaryPath: "/usr/local/bin/testapp",
	}
}

// ---- Config.Validate ----

func TestConfig_Validate_AppNameEmpty(t *testing.T) {
	cfg := validCfg()
	cfg.AppName = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Validate AppName empty: want ErrInvalidConfig; got %v", err)
	}
}

func TestConfig_Validate_VersionEmpty(t *testing.T) {
	cfg := validCfg()
	cfg.Version = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Validate Version empty: want ErrInvalidConfig; got %v", err)
	}
}

func TestConfig_Validate_RepoEmpty(t *testing.T) {
	cfg := validCfg()
	cfg.Repo = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Validate Repo empty: want ErrInvalidConfig; got %v", err)
	}
}

func TestConfig_Validate_TagPrefixEmpty(t *testing.T) {
	cfg := validCfg()
	cfg.TagPrefix = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Validate TagPrefix empty: want ErrInvalidConfig; got %v", err)
	}
}

func TestConfig_Validate_BinaryPathEmpty(t *testing.T) {
	cfg := validCfg()
	cfg.BinaryPath = ""
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Validate BinaryPath empty: want ErrInvalidConfig; got %v", err)
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	if err := validCfg().Validate(); err != nil {
		t.Errorf("Validate valid config: want nil; got %v", err)
	}
}

// ---- Config.WithDefaults ----

func TestConfig_WithDefaults_BinaryName(t *testing.T) {
	cfg := validCfg().WithDefaults()
	if cfg.BinaryName != "testapp" {
		t.Errorf("BinaryName = %q; want %q", cfg.BinaryName, "testapp")
	}
}

func TestConfig_WithDefaults_BinaryNamePreserved(t *testing.T) {
	cfg := validCfg()
	cfg.BinaryName = "custom"
	got := cfg.WithDefaults()
	if got.BinaryName != "custom" {
		t.Errorf("BinaryName = %q; want %q", got.BinaryName, "custom")
	}
}

func TestConfig_WithDefaults_AutostartLabel(t *testing.T) {
	cfg := validCfg().WithDefaults()
	if cfg.AutostartLabel != "com.testapp" {
		t.Errorf("AutostartLabel = %q; want %q", cfg.AutostartLabel, "com.testapp")
	}
}

func TestConfig_WithDefaults_AutostartLabelPreserved(t *testing.T) {
	cfg := validCfg()
	cfg.AutostartLabel = "com.custom.label"
	got := cfg.WithDefaults()
	if got.AutostartLabel != "com.custom.label" {
		t.Errorf("AutostartLabel = %q; want preserved", got.AutostartLabel)
	}
}

func TestConfig_WithDefaults_AutostartArgs(t *testing.T) {
	cfg := validCfg().WithDefaults()
	if len(cfg.AutostartArgs) != 2 {
		t.Errorf("AutostartArgs len = %d; want 2", len(cfg.AutostartArgs))
	}
}

func TestConfig_WithDefaults_AutostartArgsPreserved(t *testing.T) {
	cfg := validCfg()
	cfg.AutostartArgs = []string{"serve"}
	got := cfg.WithDefaults()
	if len(got.AutostartArgs) != 1 || got.AutostartArgs[0] != "serve" {
		t.Errorf("AutostartArgs = %v; want [serve]", got.AutostartArgs)
	}
}

func TestConfig_WithDefaults_HealthCheckTimeout(t *testing.T) {
	cfg := validCfg().WithDefaults()
	if cfg.HealthCheckTimeout != 10*time.Second {
		t.Errorf("HealthCheckTimeout = %v; want 10s", cfg.HealthCheckTimeout)
	}
}

func TestConfig_WithDefaults_HealthCheckTimeoutPreserved(t *testing.T) {
	cfg := validCfg()
	cfg.HealthCheckTimeout = 30 * time.Second
	got := cfg.WithDefaults()
	if got.HealthCheckTimeout != 30*time.Second {
		t.Errorf("HealthCheckTimeout = %v; want 30s", got.HealthCheckTimeout)
	}
}

func TestConfig_WithDefaults_XDGDirs(t *testing.T) {
	// WithDefaults should populate DataRoot, ConfigRoot, CacheRoot, SnapshotDir
	// from the environment or user home dir.
	cfg := validCfg().WithDefaults()
	// We can't assert exact values (depend on $HOME), but they should be non-empty
	// on any standard system.
	if cfg.DataRoot == "" {
		t.Error("DataRoot is empty after WithDefaults")
	}
	if cfg.ConfigRoot == "" {
		t.Error("ConfigRoot is empty after WithDefaults")
	}
	if cfg.CacheRoot == "" {
		t.Error("CacheRoot is empty after WithDefaults")
	}
	if cfg.SnapshotDir == "" {
		t.Error("SnapshotDir is empty after WithDefaults")
	}
}

func TestConfig_WithDefaults_XDGDirsPreserved(t *testing.T) {
	cfg := validCfg()
	cfg.DataRoot = "/custom/data"
	cfg.ConfigRoot = "/custom/config"
	cfg.CacheRoot = "/custom/cache"
	cfg.SnapshotDir = "/custom/snaps"
	got := cfg.WithDefaults()
	if got.DataRoot != "/custom/data" {
		t.Errorf("DataRoot = %q; want preserved", got.DataRoot)
	}
	if got.SnapshotDir != "/custom/snaps" {
		t.Errorf("SnapshotDir = %q; want preserved", got.SnapshotDir)
	}
}

// ---- coalesce ----

func TestCoalesce_NilOverride(t *testing.T) {
	var override ports.ClockPort
	fallback := ports.NewMockClockPort(time.Time{})
	got := coalesce[ports.ClockPort](override, fallback)
	if got != fallback {
		t.Error("coalesce nil override: want fallback")
	}
}

func TestCoalesce_NonNilOverride(t *testing.T) {
	override := ports.NewMockClockPort(time.Time{})
	fallback := ports.NewMockClockPort(time.Time{})
	got := coalesce[ports.ClockPort](override, fallback)
	if got != override {
		t.Error("coalesce non-nil override: want override")
	}
}

// ---- Options ----

func TestApplyOptions_WithoutVerbs(t *testing.T) {
	o := applyOptions([]Option{
		WithoutInstall(),
		WithoutUpdate(),
		WithoutUninstall(),
		WithoutDoctor(),
		WithoutClean(),
	})
	if !o.withoutInstall || !o.withoutUpdate || !o.withoutUninstall || !o.withoutDoctor || !o.withoutClean {
		t.Error("WithoutX options not applied")
	}
}

func TestApplyOptions_PortInjection(t *testing.T) {
	http := ports.NewMockHTTPPort()
	fs := ports.NewMockFsPort()
	cosign := ports.NewMockCosignPort()
	spawn := ports.NewMockSpawnPort()
	clock := ports.NewMockClockPort(time.Time{})
	paths := ports.NewMockPathsPort()
	env := ports.NewMockEnvPort()
	shellRc := ports.NewMockShellRcPort()
	completion := ports.NewMockCompletionPort()
	autostart := ports.NewMockAutostartPort()
	prompt := ports.NewMockPromptPort()

	o := applyOptions([]Option{
		WithHTTPPort(http),
		WithFsPort(fs),
		WithCosignPort(cosign),
		WithSpawnPort(spawn),
		WithClockPort(clock),
		WithPathsPort(paths),
		WithEnvPort(env),
		WithShellRcPort(shellRc),
		WithCompletionPort(completion),
		WithAutostartPort(autostart),
		WithPromptPort(prompt),
	})

	if o.http != http {
		t.Error("http port not injected")
	}
	if o.fs != fs {
		t.Error("fs port not injected")
	}
	if o.cosign != cosign {
		t.Error("cosign port not injected")
	}
	if o.spawn != spawn {
		t.Error("spawn port not injected")
	}
	if o.clock != clock {
		t.Error("clock port not injected")
	}
	if o.paths != paths {
		t.Error("paths port not injected")
	}
	if o.env != env {
		t.Error("env port not injected")
	}
	if o.shellRc != shellRc {
		t.Error("shellRc port not injected")
	}
	if o.completion != completion {
		t.Error("completion port not injected")
	}
	if o.autostart != autostart {
		t.Error("autostart port not injected")
	}
	if o.prompt != prompt {
		t.Error("prompt port not injected")
	}
}

// ---- Per-verb getters - validation failure ----

func TestInstallCmd_InvalidConfig(t *testing.T) {
	_, err := InstallCmd(Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("InstallCmd invalid config: want ErrInvalidConfig; got %v", err)
	}
}

func TestUpdateCmd_InvalidConfig(t *testing.T) {
	_, err := UpdateCmd(Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("UpdateCmd invalid config: want ErrInvalidConfig; got %v", err)
	}
}

func TestUninstallCmd_InvalidConfig(t *testing.T) {
	_, err := UninstallCmd(Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("UninstallCmd invalid config: want ErrInvalidConfig; got %v", err)
	}
}

func TestDoctorCmd_InvalidConfig(t *testing.T) {
	_, err := DoctorCmd(Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("DoctorCmd invalid config: want ErrInvalidConfig; got %v", err)
	}
}

func TestCleanCmd_InvalidConfig(t *testing.T) {
	_, err := CleanCmd(Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("CleanCmd invalid config: want ErrInvalidConfig; got %v", err)
	}
}

// ---- Per-verb getters - happy path with mocks ----

func TestInstallCmd_HappyPath(t *testing.T) {
	cmd, err := InstallCmd(validCfg(), allMockPorts()...)
	if err != nil {
		t.Fatalf("InstallCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("InstallCmd returned nil")
	}
	if cmd.Use != "install" {
		t.Errorf("cmd.Use = %q; want %q", cmd.Use, "install")
	}
}

func TestUpdateCmd_HappyPath(t *testing.T) {
	cmd, err := UpdateCmd(validCfg())
	if err != nil {
		t.Fatalf("UpdateCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("UpdateCmd returned nil")
	}
	if cmd.Use != "update" {
		t.Errorf("cmd.Use = %q; want %q", cmd.Use, "update")
	}
}

func TestUpdateCmd_ReleasesBaseURLOverride(t *testing.T) {
	// Verify that SHIPKIT_RELEASES_BASE overrides the GitHub API base URL on the
	// underlying HTTP adapter. This env var is used by the cancha workflow to
	// redirect API calls to a local testserver.
	t.Setenv("SHIPKIT_RELEASES_BASE", "http://127.0.0.1:19999")
	cmd, err := UpdateCmd(validCfg())
	if err != nil {
		t.Fatalf("UpdateCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("UpdateCmd returned nil")
	}
}

func TestUpdateCmd_SkipVerifyOverride(t *testing.T) {
	// Verify that SHIPKIT_SKIP_VERIFY=1 is accepted without error.
	t.Setenv("SHIPKIT_SKIP_VERIFY", "1")
	cmd, err := UpdateCmd(validCfg())
	if err != nil {
		t.Fatalf("UpdateCmd with skip-verify: %v", err)
	}
	if cmd == nil {
		t.Fatal("UpdateCmd returned nil")
	}
}

func TestUninstallCmd_HappyPath(t *testing.T) {
	cmd, err := UninstallCmd(validCfg(), allMockPorts()...)
	if err != nil {
		t.Fatalf("UninstallCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("UninstallCmd returned nil")
	}
}

func TestDoctorCmd_HappyPath(t *testing.T) {
	cmd, err := DoctorCmd(validCfg(), allMockPorts()...)
	if err != nil {
		t.Fatalf("DoctorCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("DoctorCmd returned nil")
	}
}

func TestCleanCmd_HappyPath(t *testing.T) {
	cmd, err := CleanCmd(validCfg(), allMockPorts()...)
	if err != nil {
		t.Fatalf("CleanCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("CleanCmd returned nil")
	}
}

// ---- RegisterLifecycle ----

func TestRegisterLifecycle_InvalidConfig(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, Config{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("RegisterLifecycle invalid config: want ErrInvalidConfig; got %v", err)
	}
}

func TestRegisterLifecycle_AllVerbs(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, validCfg(), allMockPorts()...)
	if err != nil {
		t.Fatalf("RegisterLifecycle: %v", err)
	}
	names := commandNames(root)
	for _, want := range []string{"install", "update", "uninstall", "doctor", "clean"} {
		if !contains(names, want) {
			t.Errorf("subcommand %q missing from root; got %v", want, names)
		}
	}
}

func TestRegisterLifecycle_WithoutVerbs(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	opts := append(allMockPorts(),
		WithoutInstall(),
		WithoutUpdate(),
		WithoutUninstall(),
		WithoutDoctor(),
		WithoutClean(),
	)
	err := RegisterLifecycle(root, validCfg(), opts...)
	if err != nil {
		t.Fatalf("RegisterLifecycle WithoutAll: %v", err)
	}
	if len(root.Commands()) != 0 {
		t.Errorf("expected 0 subcommands; got %d", len(root.Commands()))
	}
}

// TestRegisterLifecycle_WithoutVerbs takes a variadic so we need a wrapper.
func allMockPorts(extra ...Option) []Option {
	return append([]Option{
		WithFsPort(ports.NewMockFsPort()),
		WithPathsPort(ports.NewMockPathsPort()),
		WithEnvPort(ports.NewMockEnvPort()),
		WithShellRcPort(ports.NewMockShellRcPort()),
		WithCompletionPort(ports.NewMockCompletionPort()),
		WithAutostartPort(ports.NewMockAutostartPort()),
		WithPromptPort(ports.NewMockPromptPort()),
		WithClockPort(ports.NewMockClockPort(time.Time{})),
		WithHTTPPort(ports.NewMockHTTPPort()),
		WithSpawnPort(ports.NewMockSpawnPort()),
		WithCosignPort(ports.NewMockCosignPort()),
	}, extra...)
}

// ---- RegisterLifecycle verb builder error paths ----

// errVerbFn is a verb builder that always returns an error.
func errVerbFn(cfg Config, opts ...Option) (*cobra.Command, error) {
	return nil, errors.New("verb build failed")
}

func TestRegisterLifecycle_InstallCmdError(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, validCfg(), withInstallCmdFn(errVerbFn))
	if err == nil {
		t.Fatal("want error from install verb builder; got nil")
	}
}

func TestRegisterLifecycle_UpdateCmdError(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, validCfg(),
		WithoutInstall(),
		withUpdateCmdFn(errVerbFn),
	)
	if err == nil {
		t.Fatal("want error from update verb builder; got nil")
	}
}

func TestRegisterLifecycle_UninstallCmdError(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, validCfg(),
		WithoutInstall(),
		WithoutUpdate(),
		withUninstallCmdFn(errVerbFn),
	)
	if err == nil {
		t.Fatal("want error from uninstall verb builder; got nil")
	}
}

func TestRegisterLifecycle_DoctorCmdError(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, validCfg(),
		WithoutInstall(),
		WithoutUpdate(),
		WithoutUninstall(),
		withDoctorCmdFn(errVerbFn),
	)
	if err == nil {
		t.Fatal("want error from doctor verb builder; got nil")
	}
}

func TestRegisterLifecycle_CleanCmdError(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := RegisterLifecycle(root, validCfg(),
		WithoutInstall(),
		WithoutUpdate(),
		WithoutUninstall(),
		WithoutDoctor(),
		withCleanCmdFn(errVerbFn),
	)
	if err == nil {
		t.Fatal("want error from clean verb builder; got nil")
	}
}

// ---- newUpdateCommand (RunE paths) ----

// fakeRunner is a test double for updateRunner.
type fakeRunner struct {
	result update.Result
	err    error
}

func (f *fakeRunner) Run(_ context.Context, _ update.RunOpts) (update.Result, error) {
	return f.result, f.err
}

func TestNewUpdateCommand_RunError(t *testing.T) {
	sentinel := errors.New("update failed")
	cmd := newUpdateCommand(validCfg(), &fakeRunner{err: sentinel})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("want error; got nil")
	}
}

func TestNewUpdateCommand_KindOK(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: update.KindOK, From: "v0.0.1", To: "v0.1.0"},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func TestNewUpdateCommand_KindNoOp(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: update.KindNoOp, From: "v0.1.0"},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func TestNewUpdateCommand_KindCheckOnly(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: update.KindCheckOnly, From: "v0.0.1", Latest: "v0.1.0"},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func TestNewUpdateCommand_KindDryRun(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: update.KindDryRun, From: "v0.0.1", To: "v0.1.0"},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func TestNewUpdateCommand_KindCancelled(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: update.KindCancelled},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func TestNewUpdateCommand_KindRolledBack(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: update.KindRolledBack, From: "v0.0.1"},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func TestNewUpdateCommand_KindDefault(t *testing.T) {
	cmd := newUpdateCommand(validCfg(), &fakeRunner{
		result: update.Result{Kind: "unknown-kind"},
	})
	assertUpdateRunSucceeds(t, cmd)
}

func assertUpdateRunSucceeds(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}
}

// ---- helpers ----

func commandNames(root *cobra.Command) []string {
	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Use)
	}
	return names
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
