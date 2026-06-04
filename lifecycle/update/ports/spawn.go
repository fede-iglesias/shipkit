package ports

import (
	"context"
	"time"
)

// HealthResult holds the outcome of a binary health check.
type HealthResult struct {
	// Version is the version string reported by the binary (e.g. "v0.0.12").
	// Empty when Ok is false.
	Version string

	// Ok is true when the binary executed successfully and reported a parseable
	// version. The zero value is false, which is the safe default: callers
	// must not assume healthy unless explicitly set.
	Ok bool

	// Reason is a human-readable explanation when Ok is false (e.g. "exit
	// status 1", "timeout exceeded", "version mismatch"). Empty when Ok is true.
	Reason string
}

// SpawnPort abstracts spawning the newly installed binary to verify it
// started correctly. It exists to decouple the update orchestrator from
// direct os/exec usage, enabling full unit testing via test doubles.
//
// D-7 note: the only binary this port ever executes is the binary that was
// just installed. It does NOT execute claude, cosign, or any other external
// binary. Implementations must honour that constraint.
type SpawnPort interface {
	// HealthCheck runs binaryPath with --version (via exec.Cmd), parses the
	// version from stdout, and returns a HealthResult. The call is subject to
	// the provided ctx and timeout (whichever fires first). A non-nil error
	// means the check could not be performed at all (e.g. binary not found);
	// a false HealthResult.Ok means the binary ran but the version could not
	// be confirmed.
	HealthCheck(ctx context.Context, binaryPath string, timeout time.Duration) (HealthResult, error)
}
