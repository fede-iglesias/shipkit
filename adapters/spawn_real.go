package adapters

import updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"

// RealSpawnAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.SpawnPort]. It spawns the target
// binary using os/exec and parses its --version output for a semver string.
//
// D-7 compliance: RealSpawnAdapter ONLY invokes the consumer's own binary.
// It never spawns cosign, claude, or any other external tool.
//
// This type re-exports [lifecycle/update/adapters.RealSpawnAdapter].
type RealSpawnAdapter = updateadapters.RealSpawnAdapter

// NewRealSpawn returns a RealSpawnAdapter wired with exec.CommandContext. The
// CommandFn field can be replaced in tests to avoid spawning a real process.
func NewRealSpawn() *RealSpawnAdapter {
	return updateadapters.NewRealSpawn()
}
