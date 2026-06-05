package update_test

import (
	"context"
	"fmt"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update"
)

// ExampleRun shows the minimal wiring needed to call update.Run with a
// fake orchestrator that immediately returns a no-op result.
//
// In production, the consumer cmd layer constructs adapters.NewSigstoreCosign
// (which since v0.2.4 ships with the real sigstoreRealVerify wired as the
// default verifyCore), sets its CertIdentityRegex / OIDCIssuer for the repo,
// and passes it to the orchestrator. No SetVerifyCore call is required.
func ExampleRun() {
	// Inject a no-op factory so Run does not return ErrNotImplemented.
	update.SetOrchestratorFactory(func(cfg update.Config) update.OrchestratorRunner {
		return &fakeOrchestrator{}
	})
	defer update.SetOrchestratorFactory(nil)

	cfg := update.Config{
		Repo:               "owner/myapp",
		TagPrefix:          "myapp-",
		BinaryPath:         "/usr/local/bin/myapp",
		DataRoot:           "/tmp/myapp",
		SnapshotDir:        "/tmp/myapp/snapshots",
		HealthCheckTimeout: 5 * time.Second,
	}

	result, err := update.Run(context.Background(), cfg, update.RunOpts{CheckOnly: true})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Println(string(result.Kind))
	// Output: noop
}

// fakeOrchestrator is a minimal OrchestratorRunner for the example.
type fakeOrchestrator struct{}

func (f *fakeOrchestrator) Run(_ context.Context, _ update.RunOpts) (update.Result, error) {
	return update.Result{Kind: update.KindNoOp}, nil
}
