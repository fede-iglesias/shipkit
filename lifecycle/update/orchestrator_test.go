package update

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/migrations"
	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockHTTPPort captures calls and allows injectable failure per method.
type mockHTTPPort struct {
	latestReleaseFn func(ctx context.Context, repo, tagPrefix string) (ports.Release, error)
	downloadAssetFn func(ctx context.Context, url string, w io.Writer) error
}

func (m *mockHTTPPort) LatestRelease(ctx context.Context, repo, tagPrefix string) (ports.Release, error) {
	if m.latestReleaseFn != nil {
		return m.latestReleaseFn(ctx, repo, tagPrefix)
	}
	return ports.Release{
		Tag:    tagPrefix + "v0.1.0",
		Assets: []ports.Asset{{Name: "myapp_linux_amd64.tar.gz", DownloadURL: "http://example.com/myapp.tar.gz"}},
	}, nil
}

func (m *mockHTTPPort) DownloadAsset(ctx context.Context, url string, w io.Writer) error {
	if m.downloadAssetFn != nil {
		return m.downloadAssetFn(ctx, url, w)
	}
	_, err := w.Write([]byte("fake-binary-content"))
	return err
}

// mockFsPort tracks calls and supports injectable failures per method.
type mockFsPort struct {
	snapshotID      string
	snapshotFn      func(ctx context.Context, src, snapshotDir string) (string, error)
	restoreFn       func(ctx context.Context, snapshotID, dst string) error
	atomicReplaceFn func(ctx context.Context, target, newFile string) error
	extractTarGzFn  func(ctx context.Context, archive, destDir string) error
}

func (m *mockFsPort) Snapshot(ctx context.Context, src, snapshotDir string) (string, error) {
	if m.snapshotFn != nil {
		return m.snapshotFn(ctx, src, snapshotDir)
	}
	id := m.snapshotID
	if id == "" {
		id = "snap-001"
	}
	return id, nil
}

func (m *mockFsPort) Restore(ctx context.Context, snapshotID, dst string) error {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, snapshotID, dst)
	}
	return nil
}

func (m *mockFsPort) AtomicReplace(ctx context.Context, target, newFile string) error {
	if m.atomicReplaceFn != nil {
		return m.atomicReplaceFn(ctx, target, newFile)
	}
	return nil
}

func (m *mockFsPort) ExtractTarGz(ctx context.Context, archive, destDir string) error {
	if m.extractTarGzFn != nil {
		return m.extractTarGzFn(ctx, archive, destDir)
	}
	return nil
}

// mockCosignPort returns ok or injected error.
type mockCosignPort struct {
	verifyBundleFn func(ctx context.Context, blobPath, bundlePath string) error
}

func (m *mockCosignPort) VerifyBundle(ctx context.Context, blobPath, bundlePath string) error {
	if m.verifyBundleFn != nil {
		return m.verifyBundleFn(ctx, blobPath, bundlePath)
	}
	return nil
}

// mockSpawnPort returns a configurable HealthResult.
type mockSpawnPort struct {
	healthCheckFn func(ctx context.Context, binaryPath string, timeout time.Duration) (ports.HealthResult, error)
}

func (m *mockSpawnPort) HealthCheck(ctx context.Context, binaryPath string, timeout time.Duration) (ports.HealthResult, error) {
	if m.healthCheckFn != nil {
		return m.healthCheckFn(ctx, binaryPath, timeout)
	}
	return ports.HealthResult{Ok: true, Version: "v0.1.0"}, nil
}

// mockClock returns a fixed time.
type mockClock struct {
	now time.Time
}

func (c *mockClock) NowUTC() time.Time { return c.now }
func (c *mockClock) Since(t time.Time) time.Duration {
	return c.now.Sub(t)
}

// ---------------------------------------------------------------------------
// Mock migration
// ---------------------------------------------------------------------------

type mockMigration struct {
	version     string
	description string
	applyFn     func(ctx context.Context, root string) error
	revertFn    func(ctx context.Context, root string) error
	applyCalls  int
	revertCalls int
}

func (m *mockMigration) Version() string     { return m.version }
func (m *mockMigration) Description() string { return m.description }
func (m *mockMigration) Apply(ctx context.Context, root string) error {
	m.applyCalls++
	if m.applyFn != nil {
		return m.applyFn(ctx, root)
	}
	return nil
}
func (m *mockMigration) Revert(ctx context.Context, root string) error {
	m.revertCalls++
	if m.revertFn != nil {
		return m.revertFn(ctx, root)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper: build a default orchestrator
// ---------------------------------------------------------------------------

// defaultConfig returns a valid Config for tests.
func defaultConfig() Config {
	return Config{
		Repo:               "owner/repo",
		TagPrefix:          "myapp-",
		BinaryPath:         "/usr/local/bin/myapp",
		DataRoot:           "/tmp/.myapp",
		SnapshotDir:        "/tmp/.myapp/snapshots",
		HealthCheckTimeout: 5 * time.Second,
	}
}

// baseOrchestrator builds a fully-mocked Orchestrator with happy-path defaults.
// Tests override specific ports via field assignment.
func baseOrchestrator(cfg Config) *Orchestrator {
	reg := migrations.New()
	return &Orchestrator{
		Cfg:      cfg,
		HTTP:     &mockHTTPPort{},
		FS:       &mockFsPort{},
		Cosign:   &mockCosignPort{},
		Spawn:    &mockSpawnPort{},
		Clock:    &mockClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		Migrator: reg,
	}
}

// releaseVersion creates an HTTPPort that reports a specific version as the latest release.
func releaseVersion(tagPrefix, ver string) *mockHTTPPort {
	return &mockHTTPPort{
		latestReleaseFn: func(ctx context.Context, repo, tp string) (ports.Release, error) {
			return ports.Release{
				Tag: tp + ver,
				Assets: []ports.Asset{
					{Name: "myapp_linux_amd64.tar.gz", DownloadURL: "http://example.com/myapp.tar.gz"},
				},
			}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestRun_HappyPath: full forward path, all ports healthy, one migration.
func TestRun_HappyPath(t *testing.T) {
	cfg := defaultConfig()
	mig := &mockMigration{version: "v0.1.0", description: "add-index"}
	reg := migrations.New()
	reg.Register(mig)

	var hookOrder []string
	o := &Orchestrator{
		Cfg:      cfg,
		HTTP:     &mockHTTPPort{},
		FS:       &mockFsPort{},
		Cosign:   &mockCosignPort{},
		Spawn:    &mockSpawnPort{},
		Clock:    &mockClock{now: time.Now().UTC()},
		Migrator: reg,
		Hooks: Hooks{
			PreUpdate:  func(s State) { hookOrder = append(hookOrder, "pre:"+string(s)) },
			PostUpdate: func(s State) { hookOrder = append(hookOrder, "post:"+string(s)) },
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindOK {
		t.Fatalf("want KindOK, got %s", res.Kind)
	}
	if res.AtState != StateCommitted {
		t.Fatalf("want StateCommitted, got %s", res.AtState)
	}
	if mig.applyCalls != 1 {
		t.Fatalf("want 1 migration apply call, got %d", mig.applyCalls)
	}
	// Verify hooks fired at least for the first and last state.
	if len(hookOrder) == 0 {
		t.Fatal("want hooks to fire, got none")
	}
}

// TestRun_CheckOnlyReturnsCheckOnly: opts.CheckOnly skips all destructive ops.
func TestRun_CheckOnlyReturnsCheckOnly(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	o.HTTP = releaseVersion("myapp-", "v0.1.0")

	res, err := o.Run(context.Background(), RunOpts{CheckOnly: true})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindCheckOnly {
		t.Fatalf("want KindCheckOnly, got %s", res.Kind)
	}
	if res.Latest == "" {
		t.Fatal("want Latest populated")
	}
}

// TestRun_DryRunReturnsDryRun: opts.DryRun returns plan without side effects.
func TestRun_DryRunReturnsDryRun(t *testing.T) {
	o := baseOrchestrator(defaultConfig())

	var snapshotCalled bool
	o.FS = &mockFsPort{
		snapshotFn: func(_ context.Context, _, _ string) (string, error) {
			snapshotCalled = true
			return "snap", nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{DryRun: true})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindDryRun {
		t.Fatalf("want KindDryRun, got %s", res.Kind)
	}
	if snapshotCalled {
		t.Fatal("DryRun must not call FS.Snapshot")
	}
}

// TestRun_NoOpWhenSameVersion: current == latest => KindNoOp.
func TestRun_NoOpWhenSameVersion(t *testing.T) {
	cfg := defaultConfig()
	o := baseOrchestrator(cfg)
	// Latest matches "current" we embed via opts.Version:
	// Simplest approach: set latest == some version, pass same as opts.Version.
	o.HTTP = releaseVersion("myapp-", "v0.0.1")

	res, err := o.Run(context.Background(), RunOpts{Version: "v0.0.1"})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindNoOp {
		t.Fatalf("want KindNoOp, got %s", res.Kind)
	}
}

// TestRun_DowngradeDeniedWithoutFlag: user pins opts.Version to an older tag.
func TestRun_DowngradeDeniedWithoutFlag(t *testing.T) {
	cfg := defaultConfig()
	o := baseOrchestrator(cfg)
	// latest is v0.1.0; user pins target = v0.0.1 (downgrade).
	o.HTTP = releaseVersion("myapp-", "v0.1.0")

	_, err := o.Run(context.Background(), RunOpts{Version: "v0.0.1", AllowDowngrade: false})
	if err == nil {
		t.Fatal("want error for downgrade without flag, got nil")
	}
}

// TestRun_DowngradeAllowedWithFlag: same scenario with AllowDowngrade=true proceeds.
func TestRun_DowngradeAllowedWithFlag(t *testing.T) {
	cfg := defaultConfig()
	// latest = v0.1.0 but user pins target = v0.0.5 (downgrade allowed).
	o := baseOrchestrator(cfg)
	o.HTTP = releaseVersion("myapp-", "v0.1.0")

	// Health-check returns the pinned version as OK.
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{Ok: true, Version: "v0.0.5"}, nil
		},
	}
	res, err := o.Run(context.Background(), RunOpts{Version: "v0.0.5", AllowDowngrade: true})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindOK {
		t.Fatalf("want KindOK, got %s", res.Kind)
	}
}

// TestRun_LatestReleaseFailsNoRollbackNeeded: HTTP fails before snapshot.
func TestRun_LatestReleaseFailsNoRollbackNeeded(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	httpErr := errors.New("network error")
	o.HTTP = &mockHTTPPort{
		latestReleaseFn: func(_ context.Context, _, _ string) (ports.Release, error) {
			return ports.Release{}, httpErr
		},
	}
	var snapshotCalled bool
	o.FS = &mockFsPort{
		snapshotFn: func(_ context.Context, _, _ string) (string, error) {
			snapshotCalled = true
			return "snap", nil
		},
	}

	_, err := o.Run(context.Background(), RunOpts{})
	if err == nil {
		t.Fatal("want error from LatestRelease, got nil")
	}
	if snapshotCalled {
		t.Fatal("snapshot must not be called if pre-update fails before snapshot step")
	}
}

// TestRun_SnapshotFailsNoRollbackNeeded: Snapshot fails - no binary to restore.
func TestRun_SnapshotFailsNoRollbackNeeded(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	snapErr := errors.New("disk full")
	o.FS = &mockFsPort{
		snapshotFn: func(_ context.Context, _, _ string) (string, error) {
			return "", snapErr
		},
	}
	var restoreCalled bool
	o.FS.(*mockFsPort).restoreFn = func(_ context.Context, _, _ string) error {
		restoreCalled = true
		return nil
	}

	_, err := o.Run(context.Background(), RunOpts{})
	if err == nil {
		t.Fatal("want error from Snapshot, got nil")
	}
	if restoreCalled {
		t.Fatal("restore must not be called if snapshot itself failed")
	}
}

// TestRun_DownloadFailsRollback: download fails after snapshot -> restore called.
func TestRun_DownloadFailsRollback(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	downloadErr := errors.New("connection reset")
	o.HTTP = &mockHTTPPort{
		downloadAssetFn: func(_ context.Context, _ string, _ io.Writer) error {
			return downloadErr
		},
	}
	var restoreCalled bool
	o.FS = &mockFsPort{
		restoreFn: func(_ context.Context, _, _ string) error {
			restoreCalled = true
			return nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("rollback succeeded, want nil error wrapping the rolled-back result; got: %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
	if !restoreCalled {
		t.Fatal("want Restore to be called during rollback after snapshot")
	}
}

// TestRun_VerifyFailsRollback: cosign verify fails -> rollback triggered.
func TestRun_VerifyFailsRollback(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	verifyErr := errors.New("signature mismatch")
	o.Cosign = &mockCosignPort{
		verifyBundleFn: func(_ context.Context, _, _ string) error {
			return verifyErr
		},
	}
	var restoreCalled bool
	o.FS = &mockFsPort{
		restoreFn: func(_ context.Context, _, _ string) error {
			restoreCalled = true
			return nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
	if !restoreCalled {
		t.Fatal("want FS.Restore called during rollback")
	}
}

// TestRun_AtomicReplaceFailsRollback: replace fails -> rollback triggered.
func TestRun_AtomicReplaceFailsRollback(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	replaceErr := errors.New("rename failed")
	o.FS = &mockFsPort{
		atomicReplaceFn: func(_ context.Context, _, _ string) error {
			return replaceErr
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
}

// TestRun_MigrationFailsRollbackMigrationThenBinary: migration error -> revert migs + restore binary.
func TestRun_MigrationFailsRollbackMigrationThenBinary(t *testing.T) {
	cfg := defaultConfig()
	migErr := errors.New("migration boom")
	mig := &mockMigration{
		version:     "v0.1.0",
		description: "fail-mig",
		applyFn: func(_ context.Context, _ string) error {
			return migErr
		},
	}
	reg := migrations.New()
	reg.Register(mig)

	var restoreCalled bool
	o := &Orchestrator{
		Cfg:      cfg,
		HTTP:     &mockHTTPPort{},
		FS:       &mockFsPort{restoreFn: func(_ context.Context, _, _ string) error { restoreCalled = true; return nil }},
		Cosign:   &mockCosignPort{},
		Spawn:    &mockSpawnPort{},
		Clock:    &mockClock{now: time.Now().UTC()},
		Migrator: reg,
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
	if !restoreCalled {
		t.Fatal("want FS.Restore called during rollback")
	}
	if mig.revertCalls < 1 {
		t.Fatal("want migration revert called during rollback")
	}
}

// TestRun_HealthCheckFailsFullRollback: health check fails -> full rollback.
func TestRun_HealthCheckFailsFullRollback(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{Ok: false, Reason: "binary crashed"}, nil
		},
	}
	var restoreCalled bool
	o.FS = &mockFsPort{
		restoreFn: func(_ context.Context, _, _ string) error {
			restoreCalled = true
			return nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
	if !restoreCalled {
		t.Fatal("want FS.Restore called during rollback")
	}
}

// TestRun_RollbackFailsAtBinaryRestoreReturnsManifest: restore fails -> unrecoverable.
func TestRun_RollbackFailsAtBinaryRestoreReturnsManifest(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	// Force health check failure to trigger rollback.
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{Ok: false, Reason: "crash"}, nil
		},
	}
	// Restore itself fails.
	o.FS = &mockFsPort{
		restoreFn: func(_ context.Context, _, _ string) error {
			return errors.New("restore device error")
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (result carries manifest), got %v", err)
	}
	if res.Kind != KindFailedUnrecoverable {
		t.Fatalf("want KindFailedUnrecoverable, got %s", res.Kind)
	}
	if res.Manifest == nil {
		t.Fatal("want non-nil Manifest on unrecoverable failure")
	}
	if len(res.Manifest.Steps) == 0 {
		t.Fatal("want at least one recovery step in Manifest")
	}
}

// TestRun_ContextCancelledPreReplace: cancel before atomic replace -> KindCancelled.
func TestRun_ContextCancelledPreReplace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	o := baseOrchestrator(defaultConfig())
	// Cancel after snapshot succeeds.
	o.FS = &mockFsPort{
		snapshotFn: func(_ context.Context, _, _ string) (string, error) {
			cancel() // trigger cancel
			return "snap-cancel", nil
		},
	}

	res, err := o.Run(ctx, RunOpts{})
	// Cancellation pre-replace should return KindCancelled.
	// err may be ctx.Err() or nil depending on impl; what matters is the Kind.
	_ = err
	if res.Kind != KindCancelled {
		t.Fatalf("want KindCancelled, got %s (err=%v)", res.Kind, err)
	}
}

// TestRun_ContextCancelledPostReplaceContinues: cancel after atomic replace -> completes (KindOK or KindRolledBack).
func TestRun_ContextCancelledPostReplaceContinues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	o := baseOrchestrator(defaultConfig())
	// Cancel after atomic replace.
	o.FS = &mockFsPort{
		atomicReplaceFn: func(_ context.Context, _, _ string) error {
			cancel() // trigger cancel after replace
			return nil
		},
	}

	res, _ := o.Run(ctx, RunOpts{})
	// Post-replace the run MUST finish through health check.
	// It must NOT be KindCancelled.
	if res.Kind == KindCancelled {
		t.Fatal("post-replace cancel must NOT produce KindCancelled; run must complete")
	}
}

// TestRun_SkipVerify: when SkipVerify=true cosign is never called.
func TestRun_SkipVerify(t *testing.T) {
	cfg := defaultConfig()
	cfg.SkipVerify = true
	o := baseOrchestrator(cfg)

	var verifyCalled bool
	o.Cosign = &mockCosignPort{
		verifyBundleFn: func(_ context.Context, _, _ string) error {
			verifyCalled = true
			return nil
		},
	}

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if res.Kind != KindOK {
		t.Fatalf("want KindOK, got %s", res.Kind)
	}
	if verifyCalled {
		t.Fatal("VerifyBundle must not be called when SkipVerify=true")
	}
}

// TestRun_OnRollbackHookFires: OnRollback hook fires when rollback is triggered.
func TestRun_OnRollbackHookFires(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{Ok: false, Reason: "crash"}, nil
		},
	}

	var rollbackFrom State
	o.Hooks = Hooks{
		OnRollback: func(from State, cause error) {
			rollbackFrom = from
		},
	}

	res, _ := o.Run(context.Background(), RunOpts{})
	if res.Kind != KindRolledBack && res.Kind != KindFailedUnrecoverable {
		t.Fatalf("want rollback kind, got %s", res.Kind)
	}
	if rollbackFrom == "" {
		t.Fatal("want OnRollback hook to fire with the failing state")
	}
}

// TestNewOrchestrator: constructor returns non-nil with cfg set, ports nil.
func TestNewOrchestrator(t *testing.T) {
	cfg := defaultConfig()
	o := NewOrchestrator(cfg)
	if o == nil {
		t.Fatal("want non-nil *Orchestrator from NewOrchestrator")
	}
	if o.Cfg.Repo != cfg.Repo {
		t.Fatalf("want Cfg.Repo=%q, got %q", cfg.Repo, o.Cfg.Repo)
	}
}

// TestOrchestratorImplementsRunner: compile-time check that *Orchestrator satisfies OrchestratorRunner.
var _ OrchestratorRunner = (*Orchestrator)(nil)

// TestInit_FactoryRegistered: the init() wires the factory so update.Run delegates.
func TestInit_FactoryRegistered(t *testing.T) {
	// Save and restore factory after test.
	saved := orchestratorFactory
	t.Cleanup(func() { orchestratorFactory = saved })

	// init() was already called; verify the factory is set.
	if orchestratorFactory == nil {
		t.Fatal("orchestratorFactory must be non-nil after init()")
	}
	// Call the factory to exercise the init() closure.
	cfg := defaultConfig()
	runner := orchestratorFactory(cfg)
	if runner == nil {
		t.Fatal("factory must return a non-nil OrchestratorRunner")
	}
}

// TestIsDowngrade_TargetHigher: isDowngrade returns false when target > current.
func TestIsDowngrade_TargetHigher(t *testing.T) {
	if isDowngrade("v0.0.1", "v0.1.0") {
		t.Fatal("v0.1.0 > v0.0.1 is not a downgrade")
	}
}

// TestIsDowngrade_Equal: isDowngrade returns false when versions are equal.
func TestIsDowngrade_Equal(t *testing.T) {
	if isDowngrade("v0.1.0", "v0.1.0") {
		t.Fatal("equal versions should not be a downgrade")
	}
}

// TestParseSemverInts_NonNumericSegment: non-numeric chars are stopped at.
func TestParseSemverInts_NonNumericSegment(t *testing.T) {
	r := parseSemverInts("v1.2.3-beta")
	if r[2] != 3 {
		t.Fatalf("want patch=3 for '3-beta', got %d", r[2])
	}
}

// TestFindAsset_NoTarGz: findAsset returns error when no .tar.gz asset exists.
func TestFindAsset_NoTarGz(t *testing.T) {
	rel := ports.Release{
		Tag:    "myapp-v0.1.0",
		Assets: []ports.Asset{{Name: "checksums.txt", DownloadURL: "http://x"}},
	}
	_, err := findAsset(rel)
	if err == nil {
		t.Fatal("want error when no .tar.gz asset present")
	}
}

// TestRun_FindAssetFails: HTTP returns release with no tar.gz -> error before snapshot.
func TestRun_FindAssetFails(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	o.HTTP = &mockHTTPPort{
		latestReleaseFn: func(_ context.Context, _, _ string) (ports.Release, error) {
			return ports.Release{
				Tag:    "myapp-v0.1.0",
				Assets: []ports.Asset{{Name: "checksums.txt", DownloadURL: "http://x"}},
			}, nil
		},
	}
	_, err := o.Run(context.Background(), RunOpts{})
	if err == nil {
		t.Fatal("want error when release has no tar.gz asset")
	}
}

// TestRun_HealthCheckErrorRollback: HealthCheck returns an error (not just Ok=false) -> rollback.
func TestRun_HealthCheckErrorRollback(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	o.Spawn = &mockSpawnPort{
		healthCheckFn: func(_ context.Context, _ string, _ time.Duration) (ports.HealthResult, error) {
			return ports.HealthResult{}, errors.New("binary not found")
		},
	}
	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
}

// TestRun_MigrationRevertFailsInRollback: migration revert fails -> continues to restore binary.
func TestRun_MigrationRevertFailsInRollback(t *testing.T) {
	cfg := defaultConfig()
	migErr := errors.New("migration-apply-boom")
	revertErr := errors.New("revert-boom")
	mig := &mockMigration{
		version:     "v0.1.0",
		description: "fail-mig",
		applyFn:     func(_ context.Context, _ string) error { return migErr },
		revertFn:    func(_ context.Context, _ string) error { return revertErr },
	}
	reg := migrations.New()
	reg.Register(mig)

	var restoreCalled bool
	o := &Orchestrator{
		Cfg:      cfg,
		HTTP:     &mockHTTPPort{},
		FS:       &mockFsPort{restoreFn: func(_ context.Context, _, _ string) error { restoreCalled = true; return nil }},
		Cosign:   &mockCosignPort{},
		Spawn:    &mockSpawnPort{},
		Clock:    &mockClock{now: time.Now().UTC()},
		Migrator: reg,
	}

	// Migration apply fails, triggering rollback.
	// Rollback tries to revert migration (fails) then restores binary.
	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	// Restoration of binary was attempted despite migration revert failure.
	if !restoreCalled {
		t.Fatal("want FS.Restore called even when migration revert fails")
	}
	// Revert error doesn't block binary restore, so result depends on restore outcome.
	// With restore succeeding we get KindRolledBack.
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
}

// TestRun_ExtractTarGzFails: ExtractTarGz failure at AtomicReplace state -> rollback.
func TestRun_ExtractTarGzFails(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	extractErr := errors.New("corrupt archive")
	o.FS = &mockFsPort{
		extractTarGzFn: func(_ context.Context, _, _ string) error {
			return extractErr
		},
	}
	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack, got %s", res.Kind)
	}
}

// TestRun_ContextCancelledAfterSnapshot: cancel after snapshot, before download -> KindCancelled.
func TestRun_ContextCancelledAfterSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	o := baseOrchestrator(defaultConfig())

	var snapshotDone bool
	o.FS = &mockFsPort{
		snapshotFn: func(_ context.Context, _, _ string) (string, error) {
			snapshotDone = true
			cancel() // cancel after snapshot
			return "snap", nil
		},
	}
	_ = snapshotDone

	res, _ := o.Run(ctx, RunOpts{})
	if res.Kind != KindCancelled {
		t.Fatalf("want KindCancelled after snapshot, got %s", res.Kind)
	}
}

// TestRun_ContextCancelledAfterDownload: cancel after download, before verify -> rollback.
func TestRun_ContextCancelledAfterDownload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	o := baseOrchestrator(defaultConfig())
	o.HTTP = &mockHTTPPort{
		downloadAssetFn: func(_ context.Context, _ string, w io.Writer) error {
			_, err := w.Write([]byte("data"))
			cancel() // cancel after download
			return err
		},
	}

	// After download the context is cancelled; the select before verify should
	// trigger rollback (not KindCancelled since we have a snapshot by now).
	res, _ := o.Run(ctx, RunOpts{})
	// Post-snapshot cancel goes into rollback path (KindRolledBack or continues if not cancelled).
	// The select at verify check should pick up ctx.Done() and call rollback.
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack after cancel post-download, got %s", res.Kind)
	}
}

// TestRun_ContextCancelledAfterVerify: cancel after verify, at last safe point -> rollback.
func TestRun_ContextCancelledAfterVerify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	o := baseOrchestrator(defaultConfig())
	o.Cosign = &mockCosignPort{
		verifyBundleFn: func(_ context.Context, _, _ string) error {
			cancel() // cancel after verify
			return nil
		},
	}

	res, _ := o.Run(ctx, RunOpts{})
	// Post-verify cancel should trigger rollback.
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack after cancel post-verify, got %s", res.Kind)
	}
}

// TestRun_ContextCancelledBeforeSnapshot: ctx cancelled before snapshot step.
func TestRun_ContextCancelledBeforeSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	o := baseOrchestrator(defaultConfig())
	// Cancel during LatestRelease so context is already done by the time we
	// reach the pre-snapshot select.
	o.HTTP = &mockHTTPPort{
		latestReleaseFn: func(_ context.Context, _, _ string) (ports.Release, error) {
			cancel()
			return ports.Release{
				Tag:    "myapp-v0.1.0",
				Assets: []ports.Asset{{Name: "myapp_linux_amd64.tar.gz", DownloadURL: "http://x/myapp.tar.gz"}},
			}, nil
		},
	}

	res, _ := o.Run(ctx, RunOpts{})
	if res.Kind != KindCancelled {
		t.Fatalf("want KindCancelled before snapshot, got %s", res.Kind)
	}
}

// TestRun_MkdirAllFails: os.MkdirAll failure during download -> rollback.
func TestRun_MkdirAllFails(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	mkErr := errors.New("no space left on device")
	o.mkdirAll = func(_ string, _ os.FileMode) error { return mkErr }
	o.openFile = os.OpenFile // explicit default

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack on mkdirAll failure, got %s", res.Kind)
	}
}

// TestRun_OpenFileFails: os.OpenFile failure during download -> rollback.
func TestRun_OpenFileFails(t *testing.T) {
	o := baseOrchestrator(defaultConfig())
	openErr := errors.New("permission denied")
	o.mkdirAll = func(_ string, _ os.FileMode) error { return nil }
	o.openFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) { return nil, openErr }

	res, err := o.Run(context.Background(), RunOpts{})
	if err != nil {
		t.Fatalf("want nil error (rolled back cleanly), got %v", err)
	}
	if res.Kind != KindRolledBack {
		t.Fatalf("want KindRolledBack on openFile failure, got %s", res.Kind)
	}
}
