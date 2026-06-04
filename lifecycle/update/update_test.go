package update

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- helpers ---

func validConfig() Config {
	return Config{
		Repo:               "owner/repo",
		TagPrefix:          "myapp-",
		BinaryPath:         "/usr/local/bin/myapp",
		DataRoot:           "/tmp/appdata",
		SnapshotDir:        "/tmp/appdata/snapshots",
		HealthCheckTimeout: 5 * time.Second,
	}
}

// mockOrchestrator is a simple OrchestratorRunner test double.
type mockOrchestrator struct {
	result Result
	err    error
	// gotCtx captures the context passed to Run for cancel tests.
	gotCtx context.Context
	// gotOpts captures the RunOpts passed to Run.
	gotOpts RunOpts
}

func (m *mockOrchestrator) Run(ctx context.Context, opts RunOpts) (Result, error) {
	m.gotCtx = ctx
	m.gotOpts = opts
	return m.result, m.err
}

// withFactory sets orchestratorFactory to f for the duration of the test and
// restores the original value (including nil) in a t.Cleanup callback.
func withFactory(t *testing.T, f func(Config) OrchestratorRunner) {
	t.Helper()
	prev := orchestratorFactory
	orchestratorFactory = f
	t.Cleanup(func() { orchestratorFactory = prev })
}

// --- TestConfig_Validate ---

func TestConfig_Validate_HappyPath(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestConfig_Validate_MissingRepoReturnsErr(t *testing.T) {
	cfg := validConfig()
	cfg.Repo = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing Repo, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestConfig_Validate_MissingBinaryPathReturnsErr(t *testing.T) {
	cfg := validConfig()
	cfg.BinaryPath = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing BinaryPath, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestConfig_Validate_MissingTagPrefixReturnsErr(t *testing.T) {
	cfg := validConfig()
	cfg.TagPrefix = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing TagPrefix, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

// --- TestRun ---

func TestRun_NoFactoryReturnsErrNotImplemented(t *testing.T) {
	withFactory(t, nil)
	_, err := Run(context.Background(), validConfig(), RunOpts{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
	}
}

func TestRun_InvalidConfigReturnsErr(t *testing.T) {
	withFactory(t, func(c Config) OrchestratorRunner {
		return &mockOrchestrator{}
	})
	cfg := validConfig()
	cfg.Repo = ""
	_, err := Run(context.Background(), cfg, RunOpts{})
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestRun_FactoryInjectedDelegates(t *testing.T) {
	want := Result{
		Kind: KindOK,
		From: "v0.0.11",
		To:   "v0.0.12",
	}
	mock := &mockOrchestrator{result: want}
	withFactory(t, func(c Config) OrchestratorRunner { return mock })

	opts := RunOpts{CheckOnly: true}
	got, err := Run(context.Background(), validConfig(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	// Opts must be passed through.
	if !mock.gotOpts.CheckOnly {
		t.Fatal("RunOpts not forwarded to orchestrator")
	}
}

func TestRun_FactoryReturnsErrPropagated(t *testing.T) {
	sentinel := errors.New("orchestrator exploded")
	withFactory(t, func(c Config) OrchestratorRunner {
		return &mockOrchestrator{err: sentinel}
	})
	_, err := Run(context.Background(), validConfig(), RunOpts{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestRun_ContextCancelPassedThrough(t *testing.T) {
	mock := &mockOrchestrator{result: Result{Kind: KindNoOp}}
	withFactory(t, func(c Config) OrchestratorRunner { return mock })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before Run

	// Run should still invoke the orchestrator (validation passes, factory set).
	// The mock captures the ctx so we can verify it is the cancelled one.
	_, _ = Run(ctx, validConfig(), RunOpts{})
	if mock.gotCtx == nil {
		t.Fatal("context not forwarded to orchestrator")
	}
	select {
	case <-mock.gotCtx.Done():
		// expected: ctx is done
	default:
		t.Fatal("context passed to orchestrator is not cancelled")
	}
}

func TestSetOrchestratorFactory_OverrideAndReset(t *testing.T) {
	// Start: ensure no factory.
	withFactory(t, nil)

	// Set a factory and confirm Run uses it.
	called := false
	SetOrchestratorFactory(func(c Config) OrchestratorRunner {
		called = true
		return &mockOrchestrator{}
	})
	t.Cleanup(func() { orchestratorFactory = nil })

	_, _ = Run(context.Background(), validConfig(), RunOpts{})
	if !called {
		t.Fatal("SetOrchestratorFactory did not register the factory")
	}

	// Override with nil - Run returns ErrNotImplemented again.
	SetOrchestratorFactory(nil)
	_, err := Run(context.Background(), validConfig(), RunOpts{})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented after nil reset, got %v", err)
	}
}

// --- TestKind_Constants ---

func TestKind_Constants(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindOK, "ok"},
		{KindNoOp, "noop"},
		{KindCheckOnly, "check-only"},
		{KindDryRun, "dry-run"},
		{KindCancelled, "cancelled"},
		{KindRolledBack, "rolled-back"},
		{KindFailedUnrecoverable, "failed-unrecoverable"},
	}
	for _, tc := range cases {
		if string(tc.k) != tc.want {
			t.Errorf("Kind %q: got %q, want %q", tc.k, string(tc.k), tc.want)
		}
	}
}

// --- TestResult_Fields ---

func TestResult_Fields(t *testing.T) {
	m := &RecoveryManifest{Cause: "disk full"}
	r := Result{
		Kind:     KindFailedUnrecoverable,
		From:     "v0.0.10",
		To:       "v0.0.12",
		Latest:   "v0.0.12",
		AtState:  StateAtomicReplace,
		Reason:   "replace failed",
		Manifest: m,
	}
	if r.Kind != KindFailedUnrecoverable {
		t.Errorf("Kind: got %q, want %q", r.Kind, KindFailedUnrecoverable)
	}
	if r.From != "v0.0.10" {
		t.Errorf("From: got %q, want %q", r.From, "v0.0.10")
	}
	if r.To != "v0.0.12" {
		t.Errorf("To: got %q, want %q", r.To, "v0.0.12")
	}
	if r.Latest != "v0.0.12" {
		t.Errorf("Latest: got %q, want %q", r.Latest, "v0.0.12")
	}
	if r.AtState != StateAtomicReplace {
		t.Errorf("AtState: got %q, want %q", r.AtState, StateAtomicReplace)
	}
	if r.Reason != "replace failed" {
		t.Errorf("Reason: got %q, want %q", r.Reason, "replace failed")
	}
	if r.Manifest != m {
		t.Errorf("Manifest: got %v, want %v", r.Manifest, m)
	}
}
