package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// semverRe matches the first semver-like version string in a line of text.
// Captures MAJOR.MINOR.PATCH with an optional pre-release suffix (-rc1, -alpha.1, etc.).
// A leading "v" is tolerated but not captured.
var semverRe = regexp.MustCompile(`v?([0-9]+\.[0-9]+\.[0-9]+(?:-[a-z0-9.-]+)?)`)

// RealSpawnAdapter implements ports.SpawnPort via os/exec.
//
// D-7: ONLY invokes the binary at binaryPath. NO other external binary
// (NO claude, NO cosign, NO sub-process to validators). The anti-regression
// source-grep test in spawn_real_test.go enforces this at test time.
type RealSpawnAdapter struct {
	// CommandFn builds the *exec.Cmd to run. Defaults to exec.CommandContext.
	// Injectable for testing without a real binary on disk.
	CommandFn func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewRealSpawn returns a RealSpawnAdapter wired with the real exec.CommandContext.
func NewRealSpawn() *RealSpawnAdapter {
	return &RealSpawnAdapter{
		CommandFn: exec.CommandContext,
	}
}

// HealthCheck spawns binaryPath --version, parses a semver from stdout, and
// returns a HealthResult. The call honours both ctx and timeout (whichever
// fires first). A non-nil error means the check could not be performed at all
// (binary not found, OS error); a false HealthResult.Ok with a nil error means
// the binary ran but the version string was not parseable or the exit code was
// non-zero.
func (a *RealSpawnAdapter) HealthCheck(
	ctx context.Context,
	binaryPath string,
	timeout time.Duration,
) (ports.HealthResult, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := a.CommandFn(tctx, binaryPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		// Distinguish a context/timeout error from a plain exec error.
		if tctx.Err() != nil {
			return ports.HealthResult{}, fmt.Errorf("health check timed out after %s: %w", timeout, tctx.Err())
		}
		// ExitError means the binary ran but exited non-zero.
		var exitErr *exec.ExitError
		if isExitError(err, &exitErr) {
			reason := buildExitReason(exitErr, string(out))
			return ports.HealthResult{Ok: false, Reason: reason}, nil
		}
		// Anything else (binary not found, permission denied, etc.) is a hard error.
		return ports.HealthResult{}, fmt.Errorf("could not spawn %s: %w", binaryPath, err)
	}

	stdout := strings.TrimSpace(string(out))
	m := semverRe.FindStringSubmatch(stdout)
	if m == nil {
		return ports.HealthResult{
			Ok:     false,
			Reason: fmt.Sprintf("no semver found in output: %q", stdout),
		}, nil
	}

	return ports.HealthResult{
		Version: m[1],
		Ok:      true,
	}, nil
}

// isExitError reports whether err is an *exec.ExitError and, if so, sets out.
func isExitError(err error, out **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*out = ee
		return true
	}
	return false
}

// buildExitReason returns a human-readable reason for a non-zero exit.
func buildExitReason(exitErr *exec.ExitError, stdout string) string {
	stderr := strings.TrimSpace(string(exitErr.Stderr))
	if stderr != "" {
		return fmt.Sprintf("exit status %d: %s", exitErr.ExitCode(), stderr)
	}
	if stdout != "" {
		return fmt.Sprintf("exit status %d: %s", exitErr.ExitCode(), strings.TrimSpace(stdout))
	}
	return fmt.Sprintf("exit status %d", exitErr.ExitCode())
}
