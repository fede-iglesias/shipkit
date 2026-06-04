// Package update is documented in doc.go.
package update

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Config holds the static configuration for the update subsystem.
// All fields except AllowDowngrade, SkipVerify, and HealthCheckTimeout are
// required; Validate will return ErrInvalidConfig when a required field is empty.
type Config struct {
	// Repo is the "owner/repo" slug on GitHub (e.g. "fede-iglesias/tools").
	Repo string

	// TagPrefix is the prefix used to identify release tags
	// (e.g. "myapp-"). Only tags starting with this prefix are considered.
	TagPrefix string

	// BinaryPath is the absolute path to the binary that will be replaced
	// (e.g. "/usr/local/bin/myapp").
	BinaryPath string

	// DataRoot is the root directory for persistent data
	// (e.g. "~/.myapp"). Used to store the recovery manifest.
	DataRoot string

	// SnapshotDir is the directory where binary snapshots are stored before
	// replacement (e.g. "~/.myapp/snapshots"). Used to restore on rollback.
	SnapshotDir string

	// AllowDowngrade permits updating to a version lower than the current one.
	// Can be overridden per-run via RunOpts.AllowDowngrade.
	AllowDowngrade bool

	// SkipVerify disables cosign bundle verification. Emergency override only.
	SkipVerify bool

	// HealthCheckTimeout is the maximum duration allowed for the post-update
	// binary health check. Zero means no timeout (not recommended).
	HealthCheckTimeout time.Duration
}

// ErrInvalidConfig is returned by Config.Validate when a required field is missing.
var ErrInvalidConfig = errors.New("invalid update config")

// ErrNotImplemented is returned by Run when no OrchestratorRunner has been
// registered via SetOrchestratorFactory. It is a sentinel until the production
// orchestrator is wired by the consumer cmd layer.
var ErrNotImplemented = errors.New("update orchestrator not implemented")

// Validate checks that all required Config fields are non-empty.
// Returns an error wrapping ErrInvalidConfig when any required field is absent.
func (cfg Config) Validate() error {
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

// RunOpts holds per-invocation options that can override Config defaults.
type RunOpts struct {
	// DryRun causes Run to plan but not execute any destructive operations.
	DryRun bool

	// CheckOnly causes Run to report available versions without downloading.
	CheckOnly bool

	// Version pins the update to a specific tag (without the TagPrefix),
	// e.g. "v0.0.12". Empty means latest.
	Version string

	// AllowDowngrade overrides Config.AllowDowngrade for this invocation.
	AllowDowngrade bool
}

// Kind is the discriminant for a Result, describing the outcome of a Run call.
type Kind string

const (
	// KindOK means the update completed successfully.
	KindOK Kind = "ok"

	// KindNoOp means the current version is already the target version.
	KindNoOp Kind = "noop"

	// KindCheckOnly means Run was called with RunOpts.CheckOnly and returned
	// version information without applying any changes.
	KindCheckOnly Kind = "check-only"

	// KindDryRun means Run was called with RunOpts.DryRun and returned the
	// update plan without executing it.
	KindDryRun Kind = "dry-run"

	// KindCancelled means the context was cancelled before a point-of-no-return
	// and the system was left in a clean state.
	KindCancelled Kind = "cancelled"

	// KindRolledBack means an error occurred and the system was successfully
	// rolled back to the previous state.
	KindRolledBack Kind = "rolled-back"

	// KindFailedUnrecoverable means an error occurred and the rollback itself
	// also failed. The Manifest field contains manual recovery steps.
	KindFailedUnrecoverable Kind = "failed-unrecoverable"
)

// Result is the discriminated union returned by Run, describing the outcome.
type Result struct {
	// Kind is the outcome discriminant.
	Kind Kind

	// From is the version string of the binary before the update attempt.
	From string

	// To is the version string that was targeted (may equal From for noops).
	To string

	// Latest is the latest version detected from the release feed, which may
	// equal To when Version is not pinned.
	Latest string

	// AtState is the state machine state at which the result was produced.
	// Useful for diagnostics.
	AtState State

	// Reason is a human-readable explanation, populated for non-ok outcomes.
	Reason string

	// Manifest holds manual recovery steps and is populated only when
	// Kind is KindFailedUnrecoverable.
	Manifest *RecoveryManifest
}

// OrchestratorRunner is the state-machine driver. The production implementation
// lives in orchestrator.go. Tests inject a mock via SetOrchestratorFactory.
type OrchestratorRunner interface {
	Run(ctx context.Context, opts RunOpts) (Result, error)
}

// orchestratorFactory is the package-level factory that constructs an
// OrchestratorRunner from a Config. It is nil until wired by the consumer or a test.
var orchestratorFactory func(Config) OrchestratorRunner

// SetOrchestratorFactory registers the factory that Run uses to construct an
// OrchestratorRunner. In production, orchestrator.go calls this at init time.
// In tests, inject a mock here.
//
// Passing nil resets the factory, causing Run to return ErrNotImplemented.
func SetOrchestratorFactory(f func(Config) OrchestratorRunner) {
	orchestratorFactory = f
}

// Run is the public entry point for the update subsystem. It validates cfg,
// constructs an OrchestratorRunner via the registered factory, then delegates
// execution.
//
// Returns ErrInvalidConfig when cfg fails validation.
// Returns ErrNotImplemented when no factory has been registered.
func Run(ctx context.Context, cfg Config, opts RunOpts) (Result, error) {
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	if orchestratorFactory == nil {
		return Result{}, ErrNotImplemented
	}
	o := orchestratorFactory(cfg)
	return o.Run(ctx, opts)
}
