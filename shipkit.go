package shipkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fede-iglesias/shipkit/adapters"
	"github.com/fede-iglesias/shipkit/lifecycle/clean"
	"github.com/fede-iglesias/shipkit/lifecycle/doctor"
	"github.com/fede-iglesias/shipkit/lifecycle/install"
	"github.com/fede-iglesias/shipkit/lifecycle/uninstall"
	"github.com/fede-iglesias/shipkit/lifecycle/update"
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// Config holds the static configuration for all lifecycle verbs. It is the
// single object the consumer must supply to [RegisterLifecycle] or any of the
// per-verb getters. Required fields are checked by [Config.Validate]; optional
// fields are filled by [Config.WithDefaults].
type Config struct {
	// AppName is the canonical application name used to locate XDG directories,
	// name shell RC blocks, and label service units. Required.
	AppName string

	// BinaryName is the name of the installed binary file. Defaults to AppName.
	BinaryName string

	// Version is the current binary version string injected at build time via
	// -ldflags (e.g. "v0.1.3"). Required.
	Version string

	// Repo is the "owner/repo" GitHub slug used by the update verb and the
	// doctor network check (e.g. "fede-iglesias/tools"). Required.
	Repo string

	// TagPrefix is the release tag prefix for the tools repo (e.g. "myapp-").
	// Required.
	TagPrefix string

	// BinaryPath is the absolute path to the installed binary. Required for
	// install, uninstall, doctor, and update.
	BinaryPath string

	// DataRoot is the XDG data directory for the app (used for snapshots and
	// the recovery manifest). Defaults to XDG_DATA_HOME/<AppName> when empty.
	DataRoot string

	// ConfigRoot is the XDG config directory for the app. Defaults to
	// XDG_CONFIG_HOME/<AppName> when empty.
	ConfigRoot string

	// CacheRoot is the XDG cache directory for the app. Defaults to
	// XDG_CACHE_HOME/<AppName> when empty.
	CacheRoot string

	// SnapshotDir is the directory where update snapshots are stored. Defaults
	// to DataRoot/snapshots when empty.
	SnapshotDir string

	// EnableAutostart, when true, allows the install --autostart flag to
	// register a service unit. Defaults to false.
	EnableAutostart bool

	// AutostartLabel is the platform service label (e.g.
	// "com.fede-iglesias.myapp"). Defaults to "com.<AppName>" when empty.
	AutostartLabel string

	// AutostartArgs is the argument list passed to the binary when the
	// autostart unit runs. Defaults to ["daemon", "run"] when nil.
	AutostartArgs []string

	// HealthCheckTimeout is the maximum duration for the post-update binary
	// health check. Defaults to 10 seconds when zero.
	HealthCheckTimeout time.Duration
}

// ErrInvalidConfig is returned by [Config.Validate] when a required field is
// missing or empty.
var ErrInvalidConfig = errors.New("invalid shipkit config")

// Validate returns an error wrapping [ErrInvalidConfig] when any required
// field is empty.
func (cfg Config) Validate() error {
	if cfg.AppName == "" {
		return fmt.Errorf("%w: AppName must not be empty", ErrInvalidConfig)
	}
	if cfg.Version == "" {
		return fmt.Errorf("%w: Version must not be empty", ErrInvalidConfig)
	}
	if cfg.Repo == "" {
		return fmt.Errorf("%w: Repo must not be empty", ErrInvalidConfig)
	}
	if cfg.TagPrefix == "" {
		return fmt.Errorf("%w: TagPrefix must not be empty", ErrInvalidConfig)
	}
	if cfg.BinaryPath == "" {
		return fmt.Errorf("%w: BinaryPath must not be empty", ErrInvalidConfig)
	}
	return nil
}

// WithDefaults returns a copy of cfg with optional fields filled in. It does
// not modify the receiver. Callers should chain with [Config.Validate]:
//
//	cfg = cfg.WithDefaults()
//	if err := cfg.Validate(); err != nil { ... }
func (cfg Config) WithDefaults() Config {
	if cfg.BinaryName == "" {
		cfg.BinaryName = cfg.AppName
	}
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdgDataHome = filepath.Join(home, ".local", "share")
		}
	}
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdgConfigHome = filepath.Join(home, ".config")
		}
	}
	xdgCacheHome := os.Getenv("XDG_CACHE_HOME")
	if xdgCacheHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdgCacheHome = filepath.Join(home, ".cache")
		}
	}
	if cfg.DataRoot == "" && xdgDataHome != "" && cfg.AppName != "" {
		cfg.DataRoot = filepath.Join(xdgDataHome, cfg.AppName)
	}
	if cfg.ConfigRoot == "" && xdgConfigHome != "" && cfg.AppName != "" {
		cfg.ConfigRoot = filepath.Join(xdgConfigHome, cfg.AppName)
	}
	if cfg.CacheRoot == "" && xdgCacheHome != "" && cfg.AppName != "" {
		cfg.CacheRoot = filepath.Join(xdgCacheHome, cfg.AppName)
	}
	if cfg.SnapshotDir == "" && cfg.DataRoot != "" {
		cfg.SnapshotDir = filepath.Join(cfg.DataRoot, "snapshots")
	}
	if cfg.AutostartLabel == "" && cfg.AppName != "" {
		cfg.AutostartLabel = "com." + cfg.AppName
	}
	if cfg.AutostartArgs == nil {
		cfg.AutostartArgs = []string{"daemon", "run"}
	}
	if cfg.HealthCheckTimeout == 0 {
		cfg.HealthCheckTimeout = 10 * time.Second
	}
	return cfg
}

// RegisterLifecycle adds the five lifecycle verbs (install, update, uninstall,
// doctor, clean) as subcommands of root. It applies [Config.WithDefaults],
// validates the result, and wires production adapters unless overridden by
// opts. Returns an error if cfg is invalid or any verb command cannot be
// constructed.
func RegisterLifecycle(root *cobra.Command, cfg Config, opts ...Option) error {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	o := applyOptions(opts)

	installFn := o.installCmdFn
	if installFn == nil {
		installFn = InstallCmd
	}
	updateFn := o.updateCmdFn
	if updateFn == nil {
		updateFn = UpdateCmd
	}
	uninstallFn := o.uninstallCmdFn
	if uninstallFn == nil {
		uninstallFn = UninstallCmd
	}
	doctorFn := o.doctorCmdFn
	if doctorFn == nil {
		doctorFn = DoctorCmd
	}
	cleanFn := o.cleanCmdFn
	if cleanFn == nil {
		cleanFn = CleanCmd
	}

	if !o.withoutInstall {
		cmd, err := installFn(cfg, opts...)
		if err != nil {
			return fmt.Errorf("shipkit: install cmd: %w", err)
		}
		root.AddCommand(cmd)
	}

	if !o.withoutUpdate {
		cmd, err := updateFn(cfg, opts...)
		if err != nil {
			return fmt.Errorf("shipkit: update cmd: %w", err)
		}
		root.AddCommand(cmd)
	}

	if !o.withoutUninstall {
		cmd, err := uninstallFn(cfg, opts...)
		if err != nil {
			return fmt.Errorf("shipkit: uninstall cmd: %w", err)
		}
		root.AddCommand(cmd)
	}

	if !o.withoutDoctor {
		cmd, err := doctorFn(cfg, opts...)
		if err != nil {
			return fmt.Errorf("shipkit: doctor cmd: %w", err)
		}
		root.AddCommand(cmd)
	}

	if !o.withoutClean {
		cmd, err := cleanFn(cfg, opts...)
		if err != nil {
			return fmt.Errorf("shipkit: clean cmd: %w", err)
		}
		root.AddCommand(cmd)
	}

	return nil
}

// InstallCmd returns the install subcommand wired with production adapters,
// unless overridden by opts. The command registers config dirs, shell
// completions, and optionally a service unit.
func InstallCmd(cfg Config, opts ...Option) (*cobra.Command, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	o := applyOptions(opts)

	deps := install.Deps{
		Cfg: install.Config{
			AppName:         cfg.AppName,
			BinaryName:      cfg.BinaryName,
			Version:         cfg.Version,
			EnableAutostart: cfg.EnableAutostart,
			AutostartLabel:  cfg.AutostartLabel,
			AutostartArgs:   cfg.AutostartArgs,
		},
		FS:         coalesce[ports.FsPort](o.fs, adapters.NewRealFs()),
		Paths:      coalesce[ports.PathsPort](o.paths, adapters.NewPathsXDG()),
		Env:        coalesce[ports.EnvPort](o.env, adapters.NewEnvOS()),
		ShellRc:    coalesce[ports.ShellRcPort](o.shellRc, adapters.NewShellRcReal()),
		Completion: coalesce[ports.CompletionPort](o.completion, adapters.NewCompletionCobra()),
		Autostart:  coalesce[ports.AutostartPort](o.autostart, adapters.NewAutostartReal()),
		Prompt:     coalesce[ports.PromptPort](o.prompt, adapters.NewPromptTerm()),
		Clock:      coalesce[ports.ClockPort](o.clock, adapters.NewRealClock()),
	}
	return install.NewCommand(deps)
}

// UpdateCmd returns the update subcommand wired with production adapters,
// unless overridden by opts. The command performs a cosign-verified atomic
// self-update with rollback.
//
// Environment variables (for testing / cancha workflow):
//
//	SHIPKIT_RELEASES_BASE   overrides the GitHub API base URL (e.g. http://127.0.0.1:18080)
//	SHIPKIT_SKIP_VERIFY     set to "1" to skip cosign bundle verification
func UpdateCmd(cfg Config, opts ...Option) (*cobra.Command, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	httpAdapter := adapters.NewGitHubHTTP()
	if base := os.Getenv("SHIPKIT_RELEASES_BASE"); base != "" {
		httpAdapter.BaseURL = base
	}

	skipVerify := os.Getenv("SHIPKIT_SKIP_VERIFY") == "1"

	orch := update.NewOrchestrator(update.Config{
		Repo:               cfg.Repo,
		TagPrefix:          cfg.TagPrefix,
		BinaryPath:         cfg.BinaryPath,
		DataRoot:           cfg.DataRoot,
		SnapshotDir:        cfg.SnapshotDir,
		HealthCheckTimeout: cfg.HealthCheckTimeout,
		SkipVerify:         skipVerify,
	})
	// The update orchestrator uses lifecycle/update/ports interfaces which are
	// nominally distinct from shipkit/ports. Wire the native adapters directly.
	orch.HTTP = httpAdapter
	orch.FS = adapters.NewRealFs()
	orch.Cosign = adapters.NewSigstoreCosign()
	orch.Spawn = adapters.NewRealSpawn()
	orch.Clock = adapters.NewRealClock()

	return newUpdateCommand(cfg, orch), nil
}

// UninstallCmd returns the uninstall subcommand wired with production adapters,
// unless overridden by opts. The command removes the binary, config dirs, shell
// completions, and any registered service unit.
func UninstallCmd(cfg Config, opts ...Option) (*cobra.Command, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	o := applyOptions(opts)

	deps := uninstall.Deps{
		AppName:        cfg.AppName,
		BinPath:        cfg.BinaryPath,
		AutostartLabel: cfg.AutostartLabel,
		FS:             coalesce[ports.FsPort](o.fs, adapters.NewRealFs()),
		Paths:          coalesce[ports.PathsPort](o.paths, adapters.NewPathsXDG()),
		ShellRc:        coalesce[ports.ShellRcPort](o.shellRc, adapters.NewShellRcReal()),
		Completion:     coalesce[ports.CompletionPort](o.completion, adapters.NewCompletionCobra()),
		Autostart:      coalesce[ports.AutostartPort](o.autostart, adapters.NewAutostartReal()),
		Prompt:         coalesce[ports.PromptPort](o.prompt, adapters.NewPromptTerm()),
	}
	return uninstall.NewCommand(deps, nil), nil
}

// DoctorCmd returns the doctor subcommand wired with production adapters,
// unless overridden by opts. The command reports health of binary, XDG dirs,
// PATH, shell completions, and optionally network connectivity.
func DoctorCmd(cfg Config, opts ...Option) (*cobra.Command, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	o := applyOptions(opts)

	deps := doctor.Deps{
		AppName:        cfg.AppName,
		BinPath:        cfg.BinaryPath,
		Version:        cfg.Version,
		DataRoot:       cfg.DataRoot,
		ConfigRoot:     cfg.ConfigRoot,
		CacheRoot:      cfg.CacheRoot,
		Repo:           cfg.Repo,
		TagPrefix:      cfg.TagPrefix,
		AutostartLabel: cfg.AutostartLabel,
		HTTP:           coalesce[ports.HTTPPort](o.http, adapters.NewHTTPBridge()),
		FS:             coalesce[ports.FsPort](o.fs, adapters.NewRealFs()),
		Spawn:          coalesce[ports.SpawnPort](o.spawn, adapters.NewSpawnBridge()),
		Paths:          coalesce[ports.PathsPort](o.paths, adapters.NewPathsXDG()),
		Env:            coalesce[ports.EnvPort](o.env, adapters.NewEnvOS()),
		Autostart:      coalesce[ports.AutostartPort](o.autostart, adapters.NewAutostartReal()),
		Clock:          coalesce[ports.ClockPort](o.clock, adapters.NewRealClock()),
		Completion:     coalesce[ports.CompletionPort](o.completion, adapters.NewCompletionCobra()),
	}
	return doctor.NewCommand(deps), nil
}

// CleanCmd returns the clean subcommand wired with production adapters,
// unless overridden by opts. The command prunes old snapshots, caches, and
// work directories with interactive confirmation.
func CleanCmd(cfg Config, opts ...Option) (*cobra.Command, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	o := applyOptions(opts)

	deps := clean.Deps{
		AppName:           cfg.AppName,
		FS:                coalesce[ports.FsPort](o.fs, adapters.NewRealFs()),
		Paths:             coalesce[ports.PathsPort](o.paths, adapters.NewPathsXDG()),
		Clock:             coalesce[ports.ClockPort](o.clock, adapters.NewRealClock()),
		Prompt:            coalesce[ports.PromptPort](o.prompt, adapters.NewPromptTerm()),
		ListSnapshotsFunc: clean.DefaultListSnapshots,
		ListTmpFunc:       clean.DefaultListTmp,
		ListCacheFunc:     clean.DefaultListCache,
		ListLogsFunc:      clean.DefaultListLogs,
		ReadManifestFunc:  clean.DefaultReadManifest,
	}
	return clean.NewCommand(deps), nil
}

// coalesce returns override when it is non-nil (not the zero value of its
// interface type), otherwise returns fallback. This is the standard
// null-coalescing helper for port injection.
func coalesce[T any](override, fallback T) T {
	if any(override) == nil {
		return fallback
	}
	return override
}
