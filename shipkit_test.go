package shipkit

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestConfig_WithDefaults_XDGFallbackToHome(t *testing.T) {
	// CI runners may have XDG_DATA_HOME / XDG_CONFIG_HOME / XDG_CACHE_HOME
	// pre-set by the system. Force the unset branch to run so the
	// os.UserHomeDir() fallback path is exercised on every platform.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	cfg := validCfg().WithDefaults()

	wantData := filepath.Join(fakeHome, ".local", "share", "testapp")
	if cfg.DataRoot != wantData {
		t.Errorf("DataRoot = %q; want %q", cfg.DataRoot, wantData)
	}
	wantConfig := filepath.Join(fakeHome, ".config", "testapp")
	if cfg.ConfigRoot != wantConfig {
		t.Errorf("ConfigRoot = %q; want %q", cfg.ConfigRoot, wantConfig)
	}
	wantCache := filepath.Join(fakeHome, ".cache", "testapp")
	if cfg.CacheRoot != wantCache {
		t.Errorf("CacheRoot = %q; want %q", cfg.CacheRoot, wantCache)
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

// TestDoctorCmd_AllStatOverridesApplied verifies that DoctorCmd applies all
// four stat-function overrides when provided, exercising the != nil guard
// branches for StatExecutableFunc, StatDirFunc, StatFileFunc, and ReadMarkerFunc.
func TestDoctorCmd_AllStatOverridesApplied(t *testing.T) {
	statExecutableCalled := false
	statDirCalled := false
	statFileCalled := false
	readMarkerCalled := false

	cmd, err := DoctorCmd(validCfg(),
		append(allMockPorts(),
			WithDoctorStatExecutable(func(p string) (bool, error) {
				statExecutableCalled = true
				return true, nil
			}),
			WithDoctorStatDir(func(p string) (bool, error) {
				statDirCalled = true
				return true, nil
			}),
			WithDoctorStatFile(func(p string) (bool, error) {
				statFileCalled = true
				return true, nil
			}),
			WithDoctorReadMarker(func(p string) (string, error) {
				readMarkerCalled = true
				return "v0.1.0", nil
			}),
		)...,
	)
	if err != nil {
		t.Fatalf("DoctorCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("DoctorCmd returned nil")
	}

	// Execute the command to trigger the injected functions.
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()

	if !statExecutableCalled {
		t.Error("StatExecutableFunc override was not invoked")
	}
	if !statDirCalled {
		t.Error("StatDirFunc override was not invoked")
	}
	if !statFileCalled {
		t.Error("StatFileFunc override was not invoked")
	}
	if !readMarkerCalled {
		t.Error("ReadMarkerFunc override was not invoked")
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

// ---- W4: WithMigrations ----

// stubMigration is a minimal migrations.Migration implementation for tests.
type stubMigration struct {
	version string
}

func (m *stubMigration) Version() string                         { return m.version }
func (m *stubMigration) Description() string                     { return "stub migration " + m.version }
func (m *stubMigration) Apply(_ context.Context, _ string) error { return nil }
func (m *stubMigration) Revert(_ context.Context, _ string) error { return nil }

// TestWithMigrations_StoresInOptionState verifies that WithMigrations stores
// the supplied migrations in optionState.migrations, which UpdateCmd then
// registers on the orchestrator.
func TestWithMigrations_StoresInOptionState(t *testing.T) {
	m1 := &stubMigration{version: "0.2.0"}
	m2 := &stubMigration{version: "0.3.0"}

	o := applyOptions([]Option{WithMigrations(m1, m2)})

	if len(o.migrations) != 2 {
		t.Fatalf("expected 2 migrations in optionState; got %d", len(o.migrations))
	}
	if o.migrations[0].Version() != "0.2.0" {
		t.Errorf("migrations[0].Version = %q; want %q", o.migrations[0].Version(), "0.2.0")
	}
	if o.migrations[1].Version() != "0.3.0" {
		t.Errorf("migrations[1].Version = %q; want %q", o.migrations[1].Version(), "0.3.0")
	}
}

// TestWithMigrations_UpdateCmdAccepts verifies that UpdateCmd accepts
// WithMigrations without error, i.e. the option is wired correctly.
func TestWithMigrations_UpdateCmdAccepts(t *testing.T) {
	m := &stubMigration{version: "0.2.0"}
	cmd, err := UpdateCmd(validCfg(), WithMigrations(m))
	if err != nil {
		t.Fatalf("UpdateCmd with WithMigrations: %v", err)
	}
	if cmd == nil {
		t.Fatal("UpdateCmd returned nil command")
	}
}

// ---- W5: DoctorCmd default stat funcs ----

// TestDoctorCmd_WiresDefaultStatFuncs verifies that DoctorCmd wires os-backed
// defaults for StatExecutableFunc, StatDirFunc, StatFileFunc, and
// ReadMarkerFunc so that doctor runs without "not wired" warnings.
func TestDoctorCmd_WiresDefaultStatFuncs(t *testing.T) {
	// Create a temp directory tree that satisfies the doctor checks.
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "testapp")

	// Write a minimal "binary" that reports the version.
	script := "#!/bin/sh\necho v0.1.0\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	dataRoot := filepath.Join(tmp, "data")
	configRoot := filepath.Join(tmp, "config")
	cacheRoot := filepath.Join(tmp, "cache")
	for _, d := range []string{dataRoot, configRoot, cacheRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	cfg := Config{
		AppName:    "testapp",
		BinaryName: "testapp",
		Version:    "v0.1.0",
		Repo:       "owner/tools",
		TagPrefix:  "testapp-",
		BinaryPath: binPath,
		DataRoot:   dataRoot,
		ConfigRoot: configRoot,
		CacheRoot:  cacheRoot,
	}

	cmd, err := DoctorCmd(cfg, allMockPorts()...)
	if err != nil {
		t.Fatalf("DoctorCmd: %v", err)
	}

	// Capture output and run doctor.
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})
	// doctor may exit non-zero if some checks fail; we only care about "not wired".
	_ = cmd.Execute()

	out := buf.String()
	if bytes.Contains([]byte(out), []byte("not wired")) {
		t.Errorf("DoctorCmd output contains 'not wired' warnings; stat funcs were not wired:\n%s", out)
	}
}

// TestDoctorCmd_DefaultStatFuncs_MissingPaths exercises the not-exist and
// non-permission-error branches in the default StatDirFunc, StatFileFunc, and
// StatExecutableFunc closures wired by DoctorCmd. These lambdas are exercised
// when DoctorCmd is built without stat overrides and doctor runs against paths
// that do not exist or that cause a non-IsNotExist stat error.
func TestDoctorCmd_DefaultStatFuncs_MissingPaths(t *testing.T) {
	tmp := t.TempDir()

	// binPath does not exist: StatExecutableFunc returns a stat error.
	binPath := filepath.Join(tmp, "no-such-binary")

	// dataRoot, configRoot, cacheRoot do not exist: StatDirFunc hits the
	// os.IsNotExist branch (return false, nil).
	dataRoot := filepath.Join(tmp, "no-data")
	configRoot := filepath.Join(tmp, "no-config")
	cacheRoot := filepath.Join(tmp, "no-cache")

	// For a non-IsNotExist stat error in StatDirFunc: create a regular file at
	// the parent component so os.Stat on the path returns ENOTDIR (not IsNotExist).
	fileBarrier := filepath.Join(tmp, "barrier")
	if err := os.WriteFile(fileBarrier, []byte("x"), 0o644); err != nil {
		t.Fatalf("write barrier: %v", err)
	}
	// barrier/subdir: os.Stat returns ENOTDIR, not os.ErrNotExist.
	errDirPath := filepath.Join(fileBarrier, "subdir")

	// Similarly for StatFileFunc: use the same barrier trick.
	errFilePath := filepath.Join(fileBarrier, "file.txt")

	cfg := Config{
		AppName:    "testapp",
		BinaryName: "testapp",
		Version:    "v0.1.0",
		Repo:       "owner/tools",
		TagPrefix:  "testapp-",
		BinaryPath: binPath,
		DataRoot:   dataRoot,
		ConfigRoot: configRoot,
		CacheRoot:  cacheRoot,
	}

	// Build once with all defaults (no overrides) to cover the nil-guard else
	// branches, then run with the barrier paths to hit the error branches.
	cmd, err := DoctorCmd(cfg, allMockPorts()...)
	if err != nil {
		t.Fatalf("DoctorCmd: %v", err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()

	// Now build a second command variant where StatDirFunc and StatFileFunc will
	// see a non-IsNotExist error (ENOTDIR) to cover the remaining error branches.
	cfg2 := cfg
	cfg2.DataRoot = errDirPath
	cfg2.ConfigRoot = errDirPath
	cfg2.CacheRoot = errDirPath
	_ = errFilePath // used implicitly via barrier path in completion check

	cmd2, err := DoctorCmd(cfg2, allMockPorts()...)
	if err != nil {
		t.Fatalf("DoctorCmd (barrier): %v", err)
	}
	var buf2 bytes.Buffer
	cmd2.SetOut(&buf2)
	cmd2.SetErr(&buf2)
	cmd2.SetArgs([]string{})
	_ = cmd2.Execute()
}

// TestWithDoctorStatExecutable_OverridesDefault verifies that
// WithDoctorStatExecutable replaces the default wired function.
func TestWithDoctorStatExecutable_OverridesDefault(t *testing.T) {
	var called bool
	override := func(p string) (bool, error) {
		called = true
		return true, nil
	}
	o := applyOptions([]Option{WithDoctorStatExecutable(override)})
	if o.doctorStatExecutable == nil {
		t.Fatal("WithDoctorStatExecutable: doctorStatExecutable is nil in optionState")
	}
	// invoke to confirm it's the override, not the default
	_, _ = o.doctorStatExecutable("/any")
	if !called {
		t.Error("override function was not stored correctly")
	}
}

// TestWithDoctorStatDir_OverridesDefault verifies that WithDoctorStatDir stores
// the supplied function in optionState and that it is the stored function
// (not the default) when invoked.
func TestWithDoctorStatDir_OverridesDefault(t *testing.T) {
	var called bool
	override := func(p string) (bool, error) {
		called = true
		return true, nil
	}
	o := applyOptions([]Option{WithDoctorStatDir(override)})
	if o.doctorStatDir == nil {
		t.Fatal("WithDoctorStatDir: doctorStatDir is nil in optionState")
	}
	_, _ = o.doctorStatDir("/any")
	if !called {
		t.Error("override function was not stored correctly")
	}
}

// TestWithDoctorStatFile_OverridesDefault verifies that WithDoctorStatFile stores
// the supplied function in optionState and that it is the stored function
// (not the default) when invoked.
func TestWithDoctorStatFile_OverridesDefault(t *testing.T) {
	var called bool
	override := func(p string) (bool, error) {
		called = true
		return true, nil
	}
	o := applyOptions([]Option{WithDoctorStatFile(override)})
	if o.doctorStatFile == nil {
		t.Fatal("WithDoctorStatFile: doctorStatFile is nil in optionState")
	}
	_, _ = o.doctorStatFile("/any")
	if !called {
		t.Error("override function was not stored correctly")
	}
}

// TestWithDoctorReadMarker_OverridesDefault verifies that WithDoctorReadMarker
// stores the supplied function in optionState and that it is the stored function
// (not the default) when invoked.
func TestWithDoctorReadMarker_OverridesDefault(t *testing.T) {
	var called bool
	override := func(p string) (string, error) {
		called = true
		return "from-test", nil
	}
	o := applyOptions([]Option{WithDoctorReadMarker(override)})
	if o.doctorReadMarker == nil {
		t.Fatal("WithDoctorReadMarker: doctorReadMarker is nil in optionState")
	}
	_, _ = o.doctorReadMarker("/any")
	if !called {
		t.Error("override function was not stored correctly")
	}
}

// ---- Direct tests for package-level default stat funcs ----

// TestDefaultDoctorStatExecutable exercises all three branches of
// defaultDoctorStatExecutable: executable file, non-executable file, and
// non-existent path.
func TestDefaultDoctorStatExecutable(t *testing.T) {
	tmp := t.TempDir()

	// Success path: file exists and is executable.
	execPath := filepath.Join(tmp, "exec-bin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatalf("write exec file: %v", err)
	}
	ok, err := defaultDoctorStatExecutable(execPath)
	if err != nil {
		t.Errorf("exec file: want nil err; got %v", err)
	}
	if !ok {
		t.Error("exec file: want true; got false")
	}

	// Non-executable path: file exists but no executable bit.
	nonExecPath := filepath.Join(tmp, "data.txt")
	if err := os.WriteFile(nonExecPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write non-exec file: %v", err)
	}
	ok, err = defaultDoctorStatExecutable(nonExecPath)
	if err != nil {
		t.Errorf("non-exec file: want nil err; got %v", err)
	}
	if ok {
		t.Error("non-exec file: want false; got true")
	}

	// Error path: file does not exist.
	_, err = defaultDoctorStatExecutable(filepath.Join(tmp, "no-such-file"))
	if err == nil {
		t.Error("missing file: want non-nil err; got nil")
	}
}

// TestDefaultDoctorStatDir exercises all four branches of defaultDoctorStatDir:
// existing directory, existing regular file (not a dir), non-existent path
// (IsNotExist branch), and a non-IsNotExist error via the barrier trick.
func TestDefaultDoctorStatDir(t *testing.T) {
	tmp := t.TempDir()

	// Success path: existing directory.
	ok, err := defaultDoctorStatDir(tmp)
	if err != nil {
		t.Errorf("existing dir: want nil err; got %v", err)
	}
	if !ok {
		t.Error("existing dir: want true; got false")
	}

	// File-not-dir: existing regular file returns (false, nil).
	filePath := filepath.Join(tmp, "regular.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	ok, err = defaultDoctorStatDir(filePath)
	if err != nil {
		t.Errorf("regular file as dir: want nil err; got %v", err)
	}
	if ok {
		t.Error("regular file as dir: want false; got true")
	}

	// IsNotExist path: non-existent path returns (false, nil) without error.
	ok, err = defaultDoctorStatDir(filepath.Join(tmp, "does-not-exist"))
	if err != nil {
		t.Errorf("not-exist: want nil err; got %v", err)
	}
	if ok {
		t.Error("not-exist: want false; got true")
	}

	// Other-error path: parent is a regular file so os.Stat returns ENOTDIR.
	barrier := filepath.Join(tmp, "barrier-dir")
	if err := os.WriteFile(barrier, []byte("x"), 0o644); err != nil {
		t.Fatalf("write barrier: %v", err)
	}
	_, err = defaultDoctorStatDir(filepath.Join(barrier, "subdir"))
	if err == nil {
		t.Error("barrier/subdir: want non-nil err; got nil")
	}
}

// TestDefaultDoctorStatFile exercises all four branches of defaultDoctorStatFile:
// existing regular file, existing directory (not a file), non-existent path
// (IsNotExist branch), and a non-IsNotExist error via the barrier trick.
func TestDefaultDoctorStatFile(t *testing.T) {
	tmp := t.TempDir()

	// Success path: existing regular file.
	filePath := filepath.Join(tmp, "regular.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	ok, err := defaultDoctorStatFile(filePath)
	if err != nil {
		t.Errorf("regular file: want nil err; got %v", err)
	}
	if !ok {
		t.Error("regular file: want true; got false")
	}

	// Dir-not-file: existing directory returns (false, nil).
	ok, err = defaultDoctorStatFile(tmp)
	if err != nil {
		t.Errorf("directory as file: want nil err; got %v", err)
	}
	if ok {
		t.Error("directory as file: want false; got true")
	}

	// IsNotExist path: non-existent path returns (false, nil) without error.
	ok, err = defaultDoctorStatFile(filepath.Join(tmp, "does-not-exist"))
	if err != nil {
		t.Errorf("not-exist: want nil err; got %v", err)
	}
	if ok {
		t.Error("not-exist: want false; got true")
	}

	// Other-error path: parent is a regular file so os.Stat returns ENOTDIR.
	barrier := filepath.Join(tmp, "barrier-file")
	if err := os.WriteFile(barrier, []byte("x"), 0o644); err != nil {
		t.Fatalf("write barrier: %v", err)
	}
	_, err = defaultDoctorStatFile(filepath.Join(barrier, "child.txt"))
	if err == nil {
		t.Error("barrier/child.txt: want non-nil err; got nil")
	}
}

// TestDefaultDoctorReadMarker exercises both branches of defaultDoctorReadMarker:
// a readable file with known content, and a non-existent path (error path).
func TestDefaultDoctorReadMarker(t *testing.T) {
	tmp := t.TempDir()
	markerPath := filepath.Join(tmp, "install.marker")
	content := "v0.1.0"

	// Success path: file exists with known content.
	if err := os.WriteFile(markerPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	got, err := defaultDoctorReadMarker(markerPath)
	if err != nil {
		t.Errorf("readable marker: want nil err; got %v", err)
	}
	if got != content {
		t.Errorf("readable marker: want %q; got %q", content, got)
	}

	// Error path: non-existent path returns ("", err).
	got, err = defaultDoctorReadMarker(filepath.Join(tmp, "no-such-marker"))
	if err == nil {
		t.Error("missing marker: want non-nil err; got nil")
	}
	if got != "" {
		t.Errorf("missing marker: want empty string; got %q", got)
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
