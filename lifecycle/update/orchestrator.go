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

// stateHandler runs the work for one forward-path state.
// On success it returns the next State (driven by Transitions()) and nil error.
// On failure it returns the zero State and a non-nil error; the dispatch loop
// then decides between direct error return and rollback based on StateOrder.
// A handler may also stash a Result in o.earlyResult to short-circuit the
// dispatch with a non-OK outcome (CheckOnly, DryRun, NoOp, Cancelled before
// snapshot).
type stateHandler func(ctx context.Context) (State, error)

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

	// handlers maps every non-terminal forward-path state to its handler.
	// Populated in NewOrchestrator. The dispatch loop in Run reads from it.
	handlers map[State]stateHandler

	// Per-run scratch fields populated by handlers and read by later handlers
	// or by the dispatch loop's rollback / terminal-result paths. They are
	// reset at the top of every Run invocation.
	runOpts       RunOpts
	targetVer     string
	currentVer    string
	latestVer     string
	asset         ports.Asset
	snapshotID    string
	tmpDir        string
	tarPath       string
	healthVersion string

	// earlyResult is set by a handler that wants to short-circuit the dispatch
	// loop with a non-OK terminal Result (CheckOnly, DryRun, NoOp, pre-snapshot
	// Cancelled). The dispatch loop returns *earlyResult immediately when set.
	earlyResult *Result
}

// NewOrchestrator returns an Orchestrator with Cfg set and all port fields nil.
// Adapters are injected via field assignment (production wiring done by callers;
// tests inject mocks directly).
//
// NewOrchestrator also registers itself as the package-level OrchestratorFactory
// so that update.Run delegates to this implementation. The handlers map is
// derived from Transitions() (the canonical state machine table) so that
// adding a new forward-path state requires touching exactly one place: the
// Transitions table plus the matching handler method below.
func NewOrchestrator(cfg Config) *Orchestrator {
	o := &Orchestrator{
		Cfg:      cfg,
		Migrator: migrations.New(),
		mkdirAll: os.MkdirAll,
		openFile: os.OpenFile,
	}
	o.handlers = o.buildHandlers()
	return o
}

// buildHandlers walks Transitions() and registers one handler per forward-path
// non-terminal state, picking the method by state via handlerFor. This is the
// single source of truth that wires the state machine table to the
// orchestrator's implementation.
func (o *Orchestrator) buildHandlers() map[State]stateHandler {
	out := make(map[State]stateHandler)
	for _, tr := range Transitions() {
		if !IsForwardPath(tr.From) || IsTerminal(tr.From) {
			continue
		}
		if _, already := out[tr.From]; already {
			continue
		}
		out[tr.From] = o.handlerFor(tr.From)
	}
	return out
}

// handlerFor returns the handler method for a forward-path state.
// Returns nil for unknown states; buildHandlers filters on IsForwardPath first
// so a nil return here is a programmer error (a new forward state was added to
// Transitions() without a matching handler).
func (o *Orchestrator) handlerFor(s State) stateHandler {
	switch s {
	case StatePreUpdate:
		return o.handlePreUpdate
	case StateSnapshotTree:
		return o.handleSnapshot
	case StateDownloadBinary:
		return o.handleDownload
	case StateVerifyCosign:
		return o.handleVerify
	case StateAtomicReplace:
		return o.handleAtomicReplace
	case StateMigrateTree:
		return o.handleMigrate
	case StateHealthCheck:
		return o.handleHealthCheck
	}
	return nil
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

// handlePreUpdate runs the StatePreUpdate phase: version resolution, CheckOnly /
// DryRun / NoOp short-circuits, downgrade guard, asset selection, and the
// pre-snapshot cancellation check.
//
// On any short-circuit (CheckOnly/DryRun/NoOp/pre-snapshot cancellation) it
// populates o.earlyResult; the dispatch loop returns that Result immediately.
// On a forward path it stores targetVer / latestVer / currentVer / asset on
// the orchestrator for later handlers.
func (o *Orchestrator) handlePreUpdate(ctx context.Context) (State, error) {
	currentState := StatePreUpdate
	o.callPreUpdate(currentState)

	// 1. Resolve target version (queries HTTP.LatestRelease).
	targetVer, rel, err := o.resolveTargetVersion(ctx, o.runOpts)
	if err != nil {
		o.callPostUpdate(currentState)
		return "", err
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
	if o.runOpts.CheckOnly {
		o.callPostUpdate(currentState)
		o.earlyResult = &Result{
			Kind:    KindCheckOnly,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}
		return StatePreUpdate, nil
	}

	// DryRun: return plan without executing any destructive steps.
	if o.runOpts.DryRun {
		o.callPostUpdate(currentState)
		o.earlyResult = &Result{
			Kind:    KindDryRun,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}
		return StatePreUpdate, nil
	}

	// NoOp detection: only when opts.Version is explicitly pinned to the same
	// version as the latest release. Without a pinned version we always proceed
	// (we cannot know the installed binary's version without executing it).
	if o.runOpts.Version != "" && isSameVersion(o.runOpts.Version, latestVer) {
		o.callPostUpdate(currentState)
		o.earlyResult = &Result{
			Kind:    KindNoOp,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}
		return StatePreUpdate, nil
	}

	// Downgrade guard: when opts.Version pins a version older than the latest
	// release the user is requesting a downgrade. Deny unless AllowDowngrade.
	allowDowngrade := o.runOpts.AllowDowngrade || o.Cfg.AllowDowngrade
	if o.runOpts.Version != "" && isDowngrade(latestVer, o.runOpts.Version) && !allowDowngrade {
		o.callPostUpdate(currentState)
		return "", fmt.Errorf("target %s is older than latest %s; use AllowDowngrade to permit", o.runOpts.Version, latestVer)
	}

	// Find the asset to download.
	asset, err := findAsset(rel)
	if err != nil {
		o.callPostUpdate(currentState)
		return "", err
	}

	o.targetVer = targetVer
	o.latestVer = latestVer
	o.currentVer = currentVer
	o.asset = asset

	o.callPostUpdate(currentState)

	// Context check before destructive work.
	select {
	case <-ctx.Done():
		o.earlyResult = &Result{Kind: KindCancelled, AtState: StatePreUpdate}
		return StatePreUpdate, ctx.Err()
	default:
	}

	return StateSnapshotTree, nil
}

// handleSnapshot runs the StateSnapshotTree phase: capture the current binary
// snapshot and the post-snapshot cancellation check. A snapshot failure is
// reported back as a direct error (no rollback needed; nothing changed yet).
func (o *Orchestrator) handleSnapshot(ctx context.Context) (State, error) {
	currentState := StateSnapshotTree
	o.callPreUpdate(currentState)

	snapshotID, err := o.FS.Snapshot(ctx, o.Cfg.BinaryPath, o.Cfg.SnapshotDir)
	if err != nil {
		o.callPostUpdate(currentState)
		// Pre-snapshot failure: no rollback needed (nothing changed).
		return "", &SnapshotError{Path: o.Cfg.BinaryPath, Err: err}
	}
	o.snapshotID = snapshotID
	o.callPostUpdate(currentState)

	// Context check after snapshot, before download.
	select {
	case <-ctx.Done():
		o.earlyResult = &Result{Kind: KindCancelled, AtState: StateSnapshotTree}
		return StateSnapshotTree, ctx.Err()
	default:
	}

	return StateDownloadBinary, nil
}

// handleDownload runs the StateDownloadBinary phase: prepare the temp directory,
// open the destination file, stream the asset, and check ctx between download
// and verify. Any failure here triggers rollback (snapshot already exists).
func (o *Orchestrator) handleDownload(ctx context.Context) (State, error) {
	currentState := StateDownloadBinary
	o.callPreUpdate(currentState)

	// Write to a temp file under DataRoot.
	tmpDir := filepath.Join(o.Cfg.DataRoot, "tmp")
	mkdirFn := o.mkdirAll
	if mkdirFn == nil {
		mkdirFn = os.MkdirAll
	}
	if mkErr := mkdirFn(tmpDir, 0o700); mkErr != nil {
		o.callPostUpdate(currentState)
		return "", mkErr
	}
	tarPath := filepath.Join(tmpDir, o.asset.Name)
	openFn := o.openFile
	if openFn == nil {
		openFn = os.OpenFile
	}
	f, openErr := openFn(tarPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
	if openErr != nil {
		o.callPostUpdate(currentState)
		return "", openErr
	}
	downloadErr := o.HTTP.DownloadAsset(ctx, o.asset.DownloadURL, f)
	_ = f.Close()
	if downloadErr != nil {
		o.callPostUpdate(currentState)
		return "", downloadErr
	}
	o.tmpDir = tmpDir
	o.tarPath = tarPath
	o.callPostUpdate(currentState)

	// Context check after download, before verify.
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	return StateVerifyCosign, nil
}

// handleVerify runs the StateVerifyCosign phase: cosign bundle verification
// (skipped when Cfg.SkipVerify is set) and the pre-AtomicReplace cancellation
// check. Verification failure wraps the asset name and triggers rollback.
func (o *Orchestrator) handleVerify(ctx context.Context) (State, error) {
	currentState := StateVerifyCosign
	o.callPreUpdate(currentState)

	if !o.Cfg.SkipVerify {
		// Bundle is expected adjacent to the tar (same name + ".bundle" suffix).
		bundlePath := o.tarPath + ".bundle"
		if verifyErr := o.Cosign.VerifyBundle(ctx, o.tarPath, bundlePath); verifyErr != nil {
			o.callPostUpdate(currentState)
			return "", &VerifyError{Asset: o.asset.Name, Err: verifyErr}
		}
	}
	o.callPostUpdate(currentState)

	// Context check before atomic replace (last safe cancellation point).
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	return StateAtomicReplace, nil
}

// handleAtomicReplace runs the StateAtomicReplace phase: extract the tarball
// then atomically swap in the new binary. POINT OF NO RETURN: context
// cancellation is ignored from here onwards. Failure triggers rollback wrapped
// in ReplaceError.
func (o *Orchestrator) handleAtomicReplace(ctx context.Context) (State, error) {
	currentState := StateAtomicReplace
	o.callPreUpdate(currentState)

	extractDir := filepath.Join(o.tmpDir, "extracted")
	if extractErr := o.FS.ExtractTarGz(ctx, o.tarPath, extractDir); extractErr != nil {
		o.callPostUpdate(currentState)
		return "", &ReplaceError{Target: o.Cfg.BinaryPath, Err: extractErr}
	}
	// The binary name inside the archive is derived from the BinaryPath basename
	// so that this package is not hardcoded to a specific binary name.
	newBin := filepath.Join(extractDir, filepath.Base(o.Cfg.BinaryPath))
	if replaceErr := o.FS.AtomicReplace(ctx, o.Cfg.BinaryPath, newBin); replaceErr != nil {
		o.callPostUpdate(currentState)
		return "", &ReplaceError{Target: o.Cfg.BinaryPath, Err: replaceErr}
	}
	o.callPostUpdate(currentState)

	return StateMigrateTree, nil
}

// handleMigrate runs the StateMigrateTree phase: apply pending migrations
// against DataRoot. Failure triggers a rollback that will also revert
// migrations (per rollback's StateMigrateTree branch).
func (o *Orchestrator) handleMigrate(ctx context.Context) (State, error) {
	currentState := StateMigrateTree
	o.callPreUpdate(currentState)

	_, migrateErr := o.Migrator.ApplyPending(ctx, o.Cfg.DataRoot, o.currentVer, o.targetVer)
	if migrateErr != nil {
		o.callPostUpdate(currentState)
		return "", &MigrationError{Version: o.targetVer, Err: migrateErr}
	}
	o.callPostUpdate(currentState)

	return StateHealthCheck, nil
}

// handleHealthCheck runs the StateHealthCheck phase: spawn the new binary and
// confirm it reports Ok. A failed health check (error OR Ok=false) triggers
// rollback; the success path records the binary's reported version for the
// final Result.
func (o *Orchestrator) handleHealthCheck(ctx context.Context) (State, error) {
	currentState := StateHealthCheck
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
		return "", cause
	}
	o.healthVersion = h.Version
	o.callPostUpdate(currentState)

	return StateCommitted, nil
}

// Run implements OrchestratorRunner. It drives the 8-state forward path via a
// table-driven dispatch loop and, on any failure past StateSnapshotTree,
// executes the cascade rollback.
//
// The dispatch loop reads from o.handlers (populated in NewOrchestrator); each
// handler is the verbatim work for a single state. Per-state data shared with
// later handlers or the rollback path lives on the Orchestrator's per-run
// scratch fields, reset at the top of every Run invocation.
//
// Context cancellation before StateAtomicReplace returns KindCancelled (when
// caught pre-snapshot or post-snapshot) or triggers rollback (post-download or
// post-verify). Cancellation after StateAtomicReplace is ignored; the run
// finishes.
func (o *Orchestrator) Run(ctx context.Context, opts RunOpts) (Result, error) {
	// Reset per-run scratch state.
	o.runOpts = opts
	o.earlyResult = nil
	o.targetVer = ""
	o.currentVer = ""
	o.latestVer = ""
	o.asset = ports.Asset{}
	o.snapshotID = ""
	o.tmpDir = ""
	o.tarPath = ""
	o.healthVersion = ""

	// Defensive: when an Orchestrator is built via struct literal (the historical
	// test path) rather than NewOrchestrator, handlers will be nil. Populate it
	// here so the dispatch loop always finds a handler for forward states.
	if o.handlers == nil {
		o.handlers = o.buildHandlers()
	}

	state := StatePreUpdate
	for !IsTerminal(state) {
		h, ok := o.handlers[state]
		if !ok {
			return Result{}, fmt.Errorf("orchestrator: no handler for state %s", state)
		}
		next, err := h(ctx)
		if o.earlyResult != nil {
			return *o.earlyResult, err
		}
		if err != nil {
			// States before StateDownloadBinary failed before any rollback-worthy
			// change took place (no snapshot OR snapshot itself failed).
			if StateOrder(state) < StateOrder(StateDownloadBinary) {
				return Result{}, err
			}
			return o.rollback(ctx, state, o.snapshotID, o.targetVer, o.currentVer, err)
		}
		state = next
	}

	// Terminal state reached on the success path: emit StateCommitted hooks and
	// build the final OK Result. Other terminal states (rolled-back,
	// failed-unrecoverable) cannot be reached here because rollback returns
	// directly from the dispatch loop above.
	o.callPreUpdate(StateCommitted)
	o.callPostUpdate(StateCommitted)

	return Result{
		Kind:    KindOK,
		From:    o.currentVer,
		To:      o.healthVersion,
		Latest:  o.latestVer,
		AtState: StateCommitted,
	}, nil
}
