package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/migrations"
	"github.com/fede-iglesias/shipkit/lifecycle/recovery"
	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// realClock is the default wall-clock implementation used by NewOrchestrator
// when the caller does not inject a ClockPort. It exists so that production
// callers (e.g. consumer cmd layers) do not need to wire a Clock just to avoid
// a nil-pointer dereference inside rollback's manifest construction.
type realClock struct{}

// NowUTC returns the current time in UTC.
func (realClock) NowUTC() time.Time { return time.Now().UTC() }

// Since returns the duration elapsed since t.
func (realClock) Since(t time.Time) time.Duration { return time.Since(t) }

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
		Clock:    realClock{},
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

// resolveTargetVersion returns the version string (without TagPrefix) to update
// to, the release whose assets should be downloaded, and the latest release
// (always queried for downgrade detection + NoOp short-circuits).
//
// Behavior:
//   - opts.Version == "":         install the latest release. rel == latest.
//   - opts.Version != "":         install the pinned release. rel is fetched
//                                 via GetReleaseByTag(o.Cfg.TagPrefix+v) so the
//                                 returned asset list belongs to the pinned
//                                 release, not the latest's.
//
// Invariant: the returned version string never includes the configured
// TagPrefix. Callers compare against o.Cfg.CurrentVersion (also expected to
// be prefix-free) via isSameVersion which itself tolerates a leading "v".
//
// Fix for B3 ("target-version ignored when skipVerify"): when opts.Version
// is set, GetReleaseByTag is invoked to ensure the asset download uses the
// pinned release's assets (not the latest's). Previously LatestRelease was
// queried even when a version was pinned, and the asset list returned was
// from latest, leading to silent latest-install when skipVerify was set.
//
// Returns a wrapped error whose .Error() contains "not found" when the pinned
// tag does not resolve to a release in the repo. The orchestrator's dispatch
// loop surfaces this through Result.Reason at KindFailedUnrecoverable.
func (o *Orchestrator) resolveTargetVersion(ctx context.Context, opts RunOpts) (string, ports.Release, ports.Release, error) {
	latestRel, err := o.HTTP.LatestRelease(ctx, o.Cfg.Repo, o.Cfg.TagPrefix)
	if err != nil {
		return "", ports.Release{}, ports.Release{}, fmt.Errorf("query latest release: %w", err)
	}

	if opts.Version == "" {
		ver := strings.TrimPrefix(latestRel.Tag, o.Cfg.TagPrefix)
		return ver, latestRel, latestRel, nil
	}

	// Normalize: accept "v0.4.0" as equivalent to "0.4.0". The "v" prefix
	// is stripped only when it precedes a digit so we do not accidentally
	// mangle non-semver targets (e.g. branch names like "vendor-fix").
	v := opts.Version
	if len(v) > 1 && v[0] == 'v' && v[1] >= '0' && v[1] <= '9' {
		v = v[1:]
	}

	pinnedTag := o.Cfg.TagPrefix + v
	pinnedRel, err := o.HTTP.GetReleaseByTag(ctx, o.Cfg.Repo, pinnedTag)
	if err != nil {
		return "", ports.Release{}, ports.Release{}, fmt.Errorf("release v%s not found in %s: %w", v, o.Cfg.Repo, err)
	}

	return v, pinnedRel, latestRel, nil
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

// hostOS returns the runtime OS used to match asset names. Defined as a
// function variable so tests can simulate cross-platform hosts.
var hostOS = func() string { return runtime.GOOS }

// hostArch returns the runtime architecture used to match asset names.
// Defined as a function variable so tests can simulate cross-platform hosts.
var hostArch = func() string { return runtime.GOARCH }

// archAliases returns the case-insensitive arch name variants goreleaser is
// known to emit for a given Go arch. The matcher accepts ANY of the variants.
//
// Why a table and not a single name: goreleaser's default replacements differ
// across templates (e.g. "amd64" vs "x86_64", "arm64" vs "aarch64"). The
// upstream shipkit consumer (e.g. relay) does NOT control the asset names in
// every release feed it might point to, so this matcher accepts the common
// aliases out of the box. Custom layouts can be supported by extending this
// table; no other code in this package needs to change.
func archAliases(goarch string) []string {
	switch goarch {
	case "amd64":
		return []string{"amd64", "x86_64", "x64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	case "386":
		return []string{"386", "i386", "x86"}
	case "arm":
		return []string{"arm", "armv6", "armv7"}
	default:
		return []string{goarch}
	}
}

// osAliases returns the case-insensitive OS name variants goreleaser is known
// to emit for a given Go OS. The matcher accepts ANY of the variants.
func osAliases(goos string) []string {
	switch goos {
	case "darwin":
		return []string{"darwin", "macos", "osx"}
	case "linux":
		return []string{"linux"}
	case "windows":
		return []string{"windows", "win"}
	default:
		return []string{goos}
	}
}

// findAsset returns the asset in rel that targets the running host's
// (GOOS, GOARCH) tuple. Matching is case-insensitive on the asset name and
// accepts the common goreleaser aliases for OS and arch (see osAliases and
// archAliases). The asset name must end in ".tar.gz" and must contain BOTH
// an OS token and an arch token as standalone underscore-delimited segments
// (e.g. "relay_0.1.1_darwin_arm64.tar.gz" matches darwin/arm64).
//
// Returns an error when no asset matches the host. This is intentionally
// stricter than the previous "first .tar.gz wins" behavior: on a host where
// the previous code would have downloaded the wrong arch's tarball and then
// failed cosign verification with a confusing "bundle not found" message,
// the new behavior surfaces the real cause ("no asset for darwin/arm64 in
// release ...") before any download is attempted. See bug 1 from the
// 2026-06-05 relay/v0.1.1 incident.
func findAsset(rel ports.Release) (ports.Asset, error) {
	goos := hostOS()
	goarch := hostArch()
	osNames := osAliases(goos)
	archNames := archAliases(goarch)

	for _, a := range rel.Assets {
		if !strings.HasSuffix(strings.ToLower(a.Name), ".tar.gz") {
			continue
		}
		if assetMatchesHost(a.Name, osNames, archNames) {
			return a, nil
		}
	}
	return ports.Asset{}, fmt.Errorf("no .tar.gz asset matching %s/%s in release %s", goos, goarch, rel.Tag)
}

// assetMatchesHost returns true when name (case-insensitive) contains at
// least one of osNames AND at least one of archNames as a token bounded by
// underscores, dots, or hyphens. The boundary check prevents accidental
// matches like "amd64" inside "myapp-amd64-extras.tar.gz" picking up a
// linux arm64 asset; tokens must stand on their own.
func assetMatchesHost(name string, osNames, archNames []string) bool {
	lower := strings.ToLower(name)
	hasOS := false
	for _, n := range osNames {
		if containsToken(lower, n) {
			hasOS = true
			break
		}
	}
	if !hasOS {
		return false
	}
	for _, a := range archNames {
		if containsToken(lower, a) {
			return true
		}
	}
	return false
}

// containsToken returns true if token appears in name bounded on both sides
// by a separator (underscore, dot, or hyphen) or by the start/end of the
// string. token must be lower-case; name is expected to be already lower-cased
// by the caller.
func containsToken(name, token string) bool {
	if token == "" {
		return false
	}
	for {
		idx := strings.Index(name, token)
		if idx < 0 {
			return false
		}
		start := idx == 0 || isAssetSep(name[idx-1])
		end := idx+len(token) == len(name) || isAssetSep(name[idx+len(token)])
		if start && end {
			return true
		}
		// Advance past this occurrence and keep searching.
		name = name[idx+1:]
	}
}

// isAssetSep returns true when c is a goreleaser-default separator between
// tokens of an asset filename: underscore, hyphen, or dot.
func isAssetSep(c byte) bool {
	return c == '_' || c == '-' || c == '.'
}

// rollback executes the rollback cascade for a failure that occurred at failedAt.
// snapshotID must be non-empty when failedAt >= StateSnapshotTree, otherwise
// Restore is not called.
//
// Returns (Result{KindRolledBack}, nil) on success.
// Returns (Result{KindFailedUnrecoverable, Manifest: ...}, nil) when rollback itself fails.
// Never returns a non-nil error (rollback failures are encoded in Result).
//
// Before returning on either terminal path the canonical recovery manifest
// (see lifecycle/recovery) is persisted under o.Cfg.DataRoot so downstream
// tooling (clean's snapshot protection, doctor's pending-recovery check) can
// observe the pending recovery. Persistence is best-effort: a write failure
// is reported via the OnRollback hook but never masks the original cause.
func (o *Orchestrator) rollback(
	ctx context.Context,
	failedAt State,
	snapshotID string,
	targetVer string,
	currentVer string,
	cause error,
) (Result, error) {
	o.callOnRollback(failedAt, cause)

	// Defensive nil-checks (B2 fix). Production callers (e.g. consumer cmd
	// layers wiring the orchestrator manually) may forget to inject one of the
	// ports; rollback used to panic on the FIRST line below (o.Clock.NowUTC())
	// when Clock was nil. We now degrade gracefully: a missing Clock falls back
	// to wall time, a missing FS skips binary restore (recorded as a manual
	// step in the manifest), and a missing Migrator skips migration revert.
	// The orchestrator never panics from inside rollback, regardless of how
	// the struct was constructed.
	var causeMsg string
	if cause != nil {
		causeMsg = cause.Error()
	} else {
		causeMsg = "rollback invoked without a cause"
	}

	appName := ""
	if o.Cfg.BinaryPath != "" {
		appName = filepath.Base(o.Cfg.BinaryPath)
	}

	var now time.Time
	if o.Clock != nil {
		now = o.Clock.NowUTC()
	} else {
		now = time.Now().UTC()
	}

	manifest := &recovery.Manifest{
		Version:      1,
		AppName:      appName,
		SnapshotPath: snapshotID,
		Cause:        causeMsg,
		CreatedAt:    now,
	}

	// Determine whether migrations may have been (partially) applied.
	// They run at StateMigrateTree; only attempt revert if we got there.
	if StateOrder(failedAt) >= StateOrder(StateMigrateTree) {
		if o.Migrator == nil {
			manifest.Steps = append(manifest.Steps,
				"manual-migration-revert: Migrator not configured")
		} else {
			// Best-effort; do not surface the individual migration error here.
			dataRoot := o.Cfg.DataRoot
			if revertErr := o.Migrator.Revert(ctx, dataRoot, targetVer, currentVer); revertErr != nil {
				manifest.Steps = append(manifest.Steps,
					fmt.Sprintf("manual-migration-revert: %s", revertErr.Error()))
				// Continue to attempt binary restore even if migration revert failed.
			}
		}
	}

	// Restore the binary if we have a snapshot (snapshot was taken before download).
	if StateOrder(failedAt) >= StateOrder(StateDownloadBinary) && snapshotID != "" {
		if o.FS == nil {
			manifest.Steps = append(manifest.Steps,
				fmt.Sprintf("manual-binary-restore: snapshot=%s target=%s err=FS port not configured",
					snapshotID, o.Cfg.BinaryPath))
			o.persistRecoveryManifest(failedAt, manifest)
			return Result{
				Kind:     KindFailedUnrecoverable,
				From:     currentVer,
				To:       targetVer,
				Latest:   o.latestVer,
				AtState:  StateFailedUnrecoverable,
				Reason:   causeMsg,
				Manifest: manifest,
			}, nil
		}
		if restoreErr := o.FS.Restore(ctx, snapshotID, o.Cfg.BinaryPath); restoreErr != nil {
			manifest.Steps = append(manifest.Steps,
				fmt.Sprintf("manual-binary-restore: snapshot=%s target=%s err=%v",
					snapshotID, o.Cfg.BinaryPath, restoreErr))
			o.persistRecoveryManifest(failedAt, manifest)
			return Result{
				Kind:     KindFailedUnrecoverable,
				From:     currentVer,
				To:       targetVer,
				Latest:   o.latestVer,
				AtState:  StateFailedUnrecoverable,
				Reason:   causeMsg,
				Manifest: manifest,
			}, nil
		}
	}

	o.persistRecoveryManifest(failedAt, manifest)
	return Result{
		Kind:     KindRolledBack,
		From:     currentVer,
		To:       targetVer,
		Latest:   o.latestVer,
		AtState:  StateRolledBack,
		Reason:   causeMsg,
		Manifest: manifest,
	}, nil
}

// persistRecoveryManifest writes the canonical recovery manifest under
// o.Cfg.DataRoot. The parent directory is created on demand. Any write
// failure is surfaced through the OnRollback hook so observability is
// preserved; it must never mask the original rollback cause.
func (o *Orchestrator) persistRecoveryManifest(failedAt State, m *recovery.Manifest) {
	if m == nil {
		return
	}
	dataRoot := o.Cfg.DataRoot
	// Ensure the parent directory exists; ignore the error because the
	// subsequent recovery.Write call will surface a more specific failure.
	mkdirFn := o.mkdirAll
	if mkdirFn == nil {
		mkdirFn = os.MkdirAll
	}
	_ = mkdirFn(dataRoot, 0o700)
	if writeErr := recovery.Write(recovery.Path(dataRoot), *m); writeErr != nil {
		o.callOnRollback(failedAt, fmt.Errorf("persist recovery manifest: %w", writeErr))
	}
}

// handlePreUpdate runs the StatePreUpdate phase: version resolution, CheckOnly /
// DryRun / NoOp short-circuits, downgrade guard, asset selection, and the
// pre-snapshot cancellation check.
//
// On any short-circuit (CheckOnly/DryRun/NoOp/pre-snapshot cancellation) it
// populates o.earlyResult; the dispatch loop returns that Result immediately.
// On a forward path it stores targetVer / latestVer / currentVer / asset on
// the orchestrator for later handlers.
//
// Result.From is sourced from Cfg.CurrentVersion when set (B1+B4 fix). Empty
// CurrentVersion preserves legacy behavior: From is left blank and current-vs
// target comparisons fall back to opts.Version pinning only.
func (o *Orchestrator) handlePreUpdate(ctx context.Context) (State, error) {
	currentState := StatePreUpdate
	o.callPreUpdate(currentState)

	// 1. Resolve target version. The resolver queries LatestRelease first
	// (always, for downgrade detection and NoOp short-circuits) and then, when
	// opts.Version is set, fetches the pinned release via GetReleaseByTag so
	// the asset list returned belongs to the pinned tag (not latest's). This
	// fixes B3: previously LatestRelease was queried even when a version was
	// pinned, and the asset list returned was from latest, leading to silent
	// latest-install when skipVerify was set.
	//
	// The resolver runs as the very first action of the very first handler so
	// version resolution always precedes every downstream branch (download,
	// verify, apply). The cosign skipVerify flag, evaluated later in
	// handleVerify, cannot influence the value of targetVer surfaced here.
	//
	// On a resolution failure two paths are possible:
	//  1. Pinned tag not found (opts.Version was set, GetReleaseByTag failed).
	//     The error message contains "not found" and the user's intent is
	//     unambiguous: they asked for a specific release that does not exist.
	//     We surface a KindFailedUnrecoverable Result via earlyResult and
	//     return nil error so the caller sees a terminal Result rather than
	//     a Go error sentinel. Rollback is not invoked: we never took a
	//     snapshot, so rolling back would be pure noise.
	//  2. Any other failure (network error on LatestRelease, malformed
	//     response, transport error on GetReleaseByTag without a 404). We
	//     propagate as a Go error so the existing contract for transient
	//     failures is preserved.
	targetVer, rel, latestRel, err := o.resolveTargetVersion(ctx, o.runOpts)
	if err != nil {
		o.callPostUpdate(currentState)
		if o.runOpts.Version != "" && strings.Contains(err.Error(), "not found") {
			o.earlyResult = &Result{
				Kind:    KindFailedUnrecoverable,
				From:    o.Cfg.CurrentVersion,
				To:      o.runOpts.Version,
				AtState: StatePreUpdate,
				Reason:  err.Error(),
			}
			return StatePreUpdate, nil
		}
		return "", err
	}

	latestVer := strings.TrimPrefix(latestRel.Tag, o.Cfg.TagPrefix)

	// 2. Determine current version: prefer Cfg.CurrentVersion (B1+B4 fix) so
	// Result.From is populated for every terminal Kind. Empty CurrentVersion
	// keeps legacy behavior for clients that have not been updated yet.
	currentVer := o.Cfg.CurrentVersion

	// CheckOnly: just return version info, no side effects.
	if o.runOpts.CheckOnly {
		o.callPostUpdate(currentState)
		o.earlyResult = &Result{
			Kind:    KindCheckOnly,
			From:    currentVer,
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
			From:    currentVer,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}
		return StatePreUpdate, nil
	}

	// NoOp detection. Two paths reach this state:
	//   (a) opts.Version is explicitly pinned to the same version as the latest
	//       release (historical contract, retained for backward compatibility).
	//   (b) Cfg.CurrentVersion is set AND equals targetVer (B1+B4 enabling).
	//       Without this branch the orchestrator could not detect NoOp without
	//       executing the current binary.
	if o.runOpts.Version != "" && isSameVersion(o.runOpts.Version, latestVer) {
		o.callPostUpdate(currentState)
		o.earlyResult = &Result{
			Kind:    KindNoOp,
			From:    currentVer,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}
		return StatePreUpdate, nil
	}
	if currentVer != "" && isSameVersion(currentVer, targetVer) {
		o.callPostUpdate(currentState)
		o.earlyResult = &Result{
			Kind:    KindNoOp,
			From:    currentVer,
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
		o.earlyResult = &Result{
			Kind:    KindCancelled,
			From:    currentVer,
			Latest:  latestVer,
			To:      targetVer,
			AtState: StatePreUpdate,
		}
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
		o.earlyResult = &Result{
			Kind:    KindCancelled,
			From:    o.currentVer,
			To:      o.targetVer,
			Latest:  o.latestVer,
			AtState: StateSnapshotTree,
		}
		return StateSnapshotTree, ctx.Err()
	default:
	}

	return StateDownloadBinary, nil
}

// handleDownload runs the StateDownloadBinary phase: prepare the temp directory,
// open the destination file, stream the asset, and check ctx between download
// and verify. Any failure here triggers rollback (snapshot already exists).
//
// When Cfg.SkipVerify is false, the companion cosign bundle is downloaded
// alongside the tarball at the same temp path with a ".bundle" suffix. This
// is the path StateVerifyCosign reads via Cosign.VerifyBundle (see
// handleVerify), and the path SigstoreCosignAdapter.VerifyBundle stats before
// invoking the cosign core. Bug 2 from the 2026-06-05 relay/v0.1.1 incident:
// previously only the tarball was downloaded, causing cosign verify to fail
// with "bundle not found at .../tmp/<name>.tar.gz.bundle: no such file or
// directory" and a misleading rollback. The bundle URL is derived from the
// tarball asset URL by appending ".bundle" so this works for any release feed
// that follows the goreleaser+cosign convention (which is what shipkit's
// release pipeline emits).
//
// SkipVerify=true preserves the legacy single-download path; the bundle is
// not fetched because handleVerify will not consume it.
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

	// Download the cosign bundle companion (B2 fix) when verification is on.
	if !o.Cfg.SkipVerify {
		bundlePath := tarPath + ".bundle"
		bf, bOpenErr := openFn(bundlePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
		if bOpenErr != nil {
			o.callPostUpdate(currentState)
			return "", fmt.Errorf("open bundle file %s: %w", bundlePath, bOpenErr)
		}
		bDownloadErr := o.HTTP.DownloadAsset(ctx, o.asset.DownloadURL+".bundle", bf)
		_ = bf.Close()
		if bDownloadErr != nil {
			o.callPostUpdate(currentState)
			return "", fmt.Errorf("download bundle %s: %w", o.asset.Name+".bundle", bDownloadErr)
		}
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
	// Seed currentVer from Cfg so handlers and the rollback path see the
	// declared current version even before handlePreUpdate runs (B1+B4).
	o.currentVer = o.Cfg.CurrentVersion
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
			// Short-circuit pre-rollback (B2 fix). Two layered conditions must
			// both hold before we hand off to rollback:
			//   1. StateOrder(state) >= StateOrder(StateDownloadBinary) so we
			//      have crossed the point where rollback could conceivably
			//      have work to do (snapshot was taken, download started).
			//   2. snapshotID != "" so rollback has at least one piece of
			//      state to revert. Without a snapshot, the binary on disk is
			//      the original one; calling rollback would build a manifest
			//      whose only effect is noise on the next doctor/clean pass.
			// If either condition fails, return the original error directly.
			// This mirrors the spec's "if manifest == nil || len(manifest.Steps)
			// == 0 return error" guard, expressed in terms of the live state
			// the orchestrator actually owns (snapshotID + StateOrder).
			if StateOrder(state) < StateOrder(StateDownloadBinary) {
				return Result{}, err
			}
			if o.snapshotID == "" {
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

	// Pick a To value that is honest with the consumer: prefer the version the
	// new binary reported via health check; fall back to the resolved target
	// version when health check did not report (e.g. cosign-skip variants).
	to := o.healthVersion
	if to == "" {
		to = o.targetVer
	}

	return Result{
		Kind:    KindOK,
		From:    o.currentVer,
		To:      to,
		Latest:  o.latestVer,
		AtState: StateCommitted,
	}, nil
}
