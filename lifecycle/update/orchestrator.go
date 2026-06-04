package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fede-iglesias/shipkit/lifecycle/migrations"
	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// Hooks contains optional lifecycle callbacks for the orchestrator.
// Any nil hook is silently skipped.
type Hooks struct {
	// PreUpdate is called immediately before the orchestrator enters state.
	PreUpdate func(state State)

	// PostUpdate is called immediately after the orchestrator leaves state
	// (i.e. on a successful forward transition). Not called when entering rollback.
	PostUpdate func(state State)

	// OnRollback is called once when a rollback is triggered, before any
	// rollback steps are executed. from is the forward state where the failure
	// occurred; cause is the error that triggered the rollback.
	OnRollback func(from State, cause error)
}

// Orchestrator is the state machine driver for the self-update flow.
// Fields are exported to enable test injection without a constructor.
//
// D-7 lock: this type never imports os/exec. All process spawning is via
// SpawnPort. All HTTP is via HTTPPort. Cosign is via CosignPort.
type Orchestrator struct {
	Cfg      Config
	HTTP     ports.HTTPPort
	FS       ports.FsPort
	Cosign   ports.CosignPort
	Spawn    ports.SpawnPort
	Clock    ports.ClockPort
	Migrator *migrations.Registry
	Hooks    Hooks

	// mkdirAll and openFile are injectable for testing. When nil, os.MkdirAll
	// and os.OpenFile are used respectively.
	mkdirAll func(path string, perm os.FileMode) error
	openFile func(name string, flag int, perm os.FileMode) (*os.File, error)
}

// NewOrchestrator returns an Orchestrator with Cfg set and all port fields nil.
// Adapters are injected via field assignment (production wiring done by callers;
// tests inject mocks directly).
//
// NewOrchestrator also registers itself as the package-level OrchestratorFactory
// so that update.Run delegates to this implementation.
func NewOrchestrator(cfg Config) *Orchestrator {
	o := &Orchestrator{
		Cfg:      cfg,
		Migrator: migrations.New(),
		mkdirAll: os.MkdirAll,
		openFile: os.OpenFile,
	}
	return o
}

func init() {
	// Wire the factory so update.Run delegates to *Orchestrator.
	SetOrchestratorFactory(func(cfg Config) OrchestratorRunner {
		return NewOrchestrator(cfg)
	})
}

// callPreUpdate fires o.Hooks.PreUpdate(s) if the hook is set.
func (o *Orchestrator) callPreUpdate(s State) {
	if o.Hooks.PreUpdate != nil {
		o.Hooks.PreUpdate(s)
	}
}

// callPostUpdate fires o.Hooks.PostUpdate(s) if the hook is set.
func (o *Orchestrator) callPostUpdate(s State) {
	if o.Hooks.PostUpdate != nil {
		o.Hooks.PostUpdate(s)
	}
}

// callOnRollback fires o.Hooks.OnRollback(from, cause) if the hook is set.
func (o *Orchestrator) callOnRollback(from State, cause error) {
	if o.Hooks.OnRollback != nil {
		o.Hooks.OnRollback(from, cause)
	}
}

// resolveTargetVersion returns the version string (without TagPrefix) to update to.
// If opts.Version is set, it is returned directly. Otherwise the latest release
// is queried and the tag is stripped of the prefix.
func (o *Orchestrator) resolveTargetVersion(ctx context.Context, opts RunOpts) (string, ports.Release, error) {
	rel, err := o.HTTP.LatestRelease(ctx, o.Cfg.Repo, o.Cfg.TagPrefix)
	if err != nil {
		return "", ports.Release{}, fmt.Errorf("query latest release: %w", err)
	}
	ver := strings.TrimPrefix(rel.Tag, o.Cfg.TagPrefix)
	if opts.Version != "" {
		ver = opts.Version
	}
	return ver, rel, nil
}

// isSameVersion returns true when target == current (semver equality, tolerating
// optional leading "v").
func isSameVersion(current, target string) bool {
	norm := func(v string) string {
		return strings.TrimPrefix(v, "v")
	}
	return norm(current) == norm(target)
}

// isDowngrade returns true when target is strictly lower than current (semver).
func isDowngrade(current, target string) bool {
	// Parse both versions and compare component-by-component.
	aOrd := parseSemverInts(current)
	bOrd := parseSemverInts(target)
	for i := 0; i < 3; i++ {
		if bOrd[i] < aOrd[i] {
			return true
		}
		if bOrd[i] > aOrd[i] {
			return false
		}
	}
	return false
}

// parseSemverInts splits a semver string (optional "v" prefix) into [3]int.
func parseSemverInts(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var r [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		for _, c := range parts[i] {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		r[i] = n
	}
	return r
}

// findAsset returns the first asset in rel whose name ends with ".tar.gz".
// Returns an error when no suitable asset is found.
func findAsset(rel ports.Release) (ports.Asset, error) {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz") {
			return a, nil
		}
	}
	return ports.Asset{}, fmt.Errorf("no .tar.gz asset in release %s", rel.Tag)
}

// rollback executes the rollback cascade for a failure that occurred at failedAt.
// snapshotID must be non-empty when failedAt >= StateSnapshotTree, otherwise
// Restore is not called.
//
// Returns (Result{KindRolledBack}, nil) on success.
// Returns (Result{KindFailedUnrecoverable, Manifest: ...}, nil) when rollback itself fails.
// Never returns a non-nil error (rollback failures are encoded in Result).
func (o *Orchestrator) rollback(
	ctx context.Context,
	failedAt State,
	snapshotID string,
	targetVer string,
	currentVer string,
	cause error,
) (Result, error) {
	o.callOnRollback(failedAt, cause)

	manifest := &RecoveryManifest{
		Cause: cause.Error(),
	}

	// Determine whether migrations may have been (partially) applied.
	// They run at StateMigrateTree; only attempt revert if we got there.
	if StateOrder(failedAt) >= StateOrder(StateMigrateTree) {
		// Best-effort; do not surface the individual migration error here.
		dataRoot := o.Cfg.DataRoot
		if revertErr := o.Migrator.Revert(ctx, dataRoot, targetVer, currentVer); revertErr != nil {
			manifest.Steps = append(manifest.Steps, RecoveryStep{
				Action: "manual-migration-revert",
				Detail: revertErr.Error(),
			})
			// Continue to attempt binary restore even if migration revert failed.
		}
	}

	// Restore the binary if we have a snapshot (snapshot was taken before download).
	if StateOrder(failedAt) >= StateOrder(StateDownloadBinary) && snapshotID != "" {
		if restoreErr := o.FS.Restore(ctx, snapshotID, o.Cfg.BinaryPath); restoreErr != nil {
			manifest.Steps = append(manifest.Steps, RecoveryStep{
				Action: "manual-binary-restore",
				Detail: fmt.Sprintf("snapshot=%s target=%s err=%v", snapshotID, o.Cfg.BinaryPath, restoreErr),
			})
			return Result{
				Kind:     KindFailedUnrecoverable,
				AtState:  StateFailedUnrecoverable,
				Reason:   cause.Error(),
				Manifest: manifest,
			}, nil
		}
	}

	return Result{
		Kind:    KindRolledBack,
		AtState: StateRolledBack,
		Reason:  cause.Error(),
	}, nil
}

// Run implements OrchestratorRunner. It drives the 8-state forward path and,
// on any failure, executes the cascade rollback.
//
// Context cancellation before StateAtomicReplace returns KindCancelled.
// Cancellation after StateAtomicReplace is ignored; the run finishes.
func (o *Orchestrator) Run(ctx context.Context, opts RunOpts) (Result, error) {
	// --- StatePreUpdate ---
	currentState := StatePreUpdate
	o.callPreUpdate(currentState)

	// 1. Resolve target version (queries HTTP.LatestRelease).
	targetVer, rel, err := o.resolveTargetVersion(ctx, opts)
	if err != nil {
		o.callPostUpdate(currentState)
		return Result{}, err
	}

	latestVer := strings.TrimPrefix(rel.Tag, o.Cfg.TagPrefix)

	// 2. Determine current version: use opts.Version when set for pinned target
	// comparisons, otherwise we have no explicit current version (treat as "").
	// The orchestrator does not exec the current binary; version logic relies on
	// the HealthCheck result of the NEW binary.
	//
	// For NoOp / downgrade detection we compare against opts.Version if set.
	currentVer := ""

	// CheckOnly: just return version info, no side effects.
	if opts.CheckOnly {
		o.callPostUpdate(currentState)
		return Result{
			Kind:    KindCheckOnly,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}, nil
	}

	// DryRun: return plan without executing any destructive steps.
	if opts.DryRun {
		o.callPostUpdate(currentState)
		return Result{
			Kind:    KindDryRun,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}, nil
	}

	// NoOp detection: only when opts.Version is explicitly pinned to the same
	// version as the latest release. Without a pinned version we always proceed
	// (we cannot know the installed binary's version without executing it).
	if opts.Version != "" && isSameVersion(opts.Version, latestVer) {
		o.callPostUpdate(currentState)
		return Result{
			Kind:    KindNoOp,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}, nil
	}

	// Downgrade guard: when opts.Version pins a version older than the latest
	// release the user is requesting a downgrade. Deny unless AllowDowngrade.
	allowDowngrade := opts.AllowDowngrade || o.Cfg.AllowDowngrade
	if opts.Version != "" && isDowngrade(latestVer, opts.Version) && !allowDowngrade {
		o.callPostUpdate(currentState)
		return Result{}, fmt.Errorf("target %s is older than latest %s; use AllowDowngrade to permit", opts.Version, latestVer)
	}

	// Find the asset to download.
	asset, err := findAsset(rel)
	if err != nil {
		o.callPostUpdate(currentState)
		return Result{}, err
	}

	o.callPostUpdate(currentState)

	// Context check before destructive work.
	select {
	case <-ctx.Done():
		return Result{Kind: KindCancelled, AtState: StatePreUpdate}, ctx.Err()
	default:
	}

	// --- StateSnapshotTree ---
	currentState = StateSnapshotTree
	o.callPreUpdate(currentState)

	snapshotID, err := o.FS.Snapshot(ctx, o.Cfg.BinaryPath, o.Cfg.SnapshotDir)
	if err != nil {
		o.callPostUpdate(currentState)
		// Pre-snapshot failure: no rollback needed (nothing changed).
		return Result{}, &SnapshotError{Path: o.Cfg.BinaryPath, Err: err}
	}
	o.callPostUpdate(currentState)

	// Context check after snapshot, before download.
	select {
	case <-ctx.Done():
		return Result{Kind: KindCancelled, AtState: StateSnapshotTree}, ctx.Err()
	default:
	}

	// --- StateDownloadBinary ---
	currentState = StateDownloadBinary
	o.callPreUpdate(currentState)

	// Write to a temp file under DataRoot.
	tmpDir := filepath.Join(o.Cfg.DataRoot, "tmp")
	mkdirFn := o.mkdirAll
	if mkdirFn == nil {
		mkdirFn = os.MkdirAll
	}
	if mkErr := mkdirFn(tmpDir, 0o700); mkErr != nil {
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer, mkErr)
	}
	tarPath := filepath.Join(tmpDir, asset.Name)
	openFn := o.openFile
	if openFn == nil {
		openFn = os.OpenFile
	}
	f, openErr := openFn(tarPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
	if openErr != nil {
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer, openErr)
	}
	downloadErr := o.HTTP.DownloadAsset(ctx, asset.DownloadURL, f)
	_ = f.Close()
	if downloadErr != nil {
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer, downloadErr)
	}
	o.callPostUpdate(currentState)

	// Context check after download, before verify.
	select {
	case <-ctx.Done():
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer, ctx.Err())
	default:
	}

	// --- StateVerifyCosign ---
	currentState = StateVerifyCosign
	o.callPreUpdate(currentState)

	if !o.Cfg.SkipVerify {
		// Bundle is expected adjacent to the tar (same name + ".bundle" suffix).
		bundlePath := tarPath + ".bundle"
		if verifyErr := o.Cosign.VerifyBundle(ctx, tarPath, bundlePath); verifyErr != nil {
			o.callPostUpdate(currentState)
			return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer,
				&VerifyError{Asset: asset.Name, Err: verifyErr})
		}
	}
	o.callPostUpdate(currentState)

	// Context check before atomic replace (last safe cancellation point).
	select {
	case <-ctx.Done():
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer, ctx.Err())
	default:
	}

	// --- StateAtomicReplace ---
	// POINT OF NO RETURN: context cancellation is ignored from here onwards.
	currentState = StateAtomicReplace
	o.callPreUpdate(currentState)

	extractDir := filepath.Join(tmpDir, "extracted")
	if extractErr := o.FS.ExtractTarGz(ctx, tarPath, extractDir); extractErr != nil {
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer,
			&ReplaceError{Target: o.Cfg.BinaryPath, Err: extractErr})
	}
	// The binary name inside the archive is derived from the BinaryPath basename
	// so that this package is not hardcoded to a specific binary name.
	newBin := filepath.Join(extractDir, filepath.Base(o.Cfg.BinaryPath))
	if replaceErr := o.FS.AtomicReplace(ctx, o.Cfg.BinaryPath, newBin); replaceErr != nil {
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer,
			&ReplaceError{Target: o.Cfg.BinaryPath, Err: replaceErr})
	}
	o.callPostUpdate(currentState)

	// --- StateMigrateTree ---
	currentState = StateMigrateTree
	o.callPreUpdate(currentState)

	_, migrateErr := o.Migrator.ApplyPending(ctx, o.Cfg.DataRoot, currentVer, targetVer)
	if migrateErr != nil {
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer,
			&MigrationError{Version: targetVer, Err: migrateErr})
	}
	o.callPostUpdate(currentState)

	// --- StateHealthCheck ---
	currentState = StateHealthCheck
	o.callPreUpdate(currentState)

	h, healthErr := o.Spawn.HealthCheck(ctx, o.Cfg.BinaryPath, o.Cfg.HealthCheckTimeout)
	if healthErr != nil || !h.Ok {
		var cause error
		if healthErr != nil {
			cause = healthErr
		} else {
			cause = fmt.Errorf("health check failed: %s", h.Reason)
		}
		o.callPostUpdate(currentState)
		return o.rollback(ctx, currentState, snapshotID, targetVer, currentVer, cause)
	}
	o.callPostUpdate(currentState)

	// --- StateCommitted ---
	currentState = StateCommitted
	o.callPreUpdate(currentState)
	o.callPostUpdate(currentState)

	return Result{
		Kind:    KindOK,
		From:    currentVer,
		To:      h.Version,
		Latest:  latestVer,
		AtState: StateCommitted,
	}, nil
}
