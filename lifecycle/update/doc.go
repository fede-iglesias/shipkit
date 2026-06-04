// Package update implements cosign-verified self-update for CLI tools distributed
// via GitHub Releases. It provides a state-machine-driven update flow with
// snapshot/rollback, migration support, and full port injection for testing.
//
// # Design
//
// The update flow is modelled as an 8-state forward machine (pre-update ->
// snapshot -> download -> verify -> atomic-replace -> migrate -> health-check ->
// committed) backed by a cascade rollback path. Each external dependency
// (HTTP, filesystem, cosign verification, process spawning, clock) is expressed
// as a port interface. Concrete adapters live in the adapters sub-package.
//
// The cosign verification adapter uses sigstore-go embedded (no os/exec cosign
// binary). Production wiring for the real TUF+Rekor network path must be
// injected at the consumer cmd layer via [adapters.SigstoreCosignAdapter.SetVerifyCore].
// Without that wiring, [adapters.SigstoreCosignAdapter.VerifyBundle] returns
// [adapters.ErrCosignNotConfigured].
//
// # Usage
//
//	cfg := update.Config{
//	    Repo:               "owner/tools",
//	    TagPrefix:          "myapp-",
//	    BinaryPath:         "/usr/local/bin/myapp",
//	    DataRoot:           os.ExpandEnv("$HOME/.myapp"),
//	    SnapshotDir:        os.ExpandEnv("$HOME/.myapp/snapshots"),
//	    HealthCheckTimeout: 5 * time.Second,
//	}
//	result, err := update.Run(context.Background(), cfg, update.RunOpts{})
//
// # See also
//
// [github.com/fede-iglesias/shipkit/lifecycle/update/ports] for port interfaces.
// [github.com/fede-iglesias/shipkit/lifecycle/update/adapters] for concrete adapters.
// [github.com/fede-iglesias/shipkit/lifecycle/migrations] for the migration registry.
package update
