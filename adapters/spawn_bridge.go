package adapters

import (
	"context"
	"time"

	updateadapters "github.com/fede-iglesias/shipkit/lifecycle/update/adapters"
	"github.com/fede-iglesias/shipkit/ports"
)

// SpawnBridgeAdapter wraps [RealSpawnAdapter] (which implements
// [lifecycle/update/ports.SpawnPort]) and presents it as a
// [shipkit/ports.SpawnPort]. The bridge converts between the structurally
// identical but nominally distinct HealthResult types.
//
// Use [NewSpawnBridge] to obtain a production instance.
type SpawnBridgeAdapter struct {
	inner *updateadapters.RealSpawnAdapter
}

// NewSpawnBridge returns a SpawnBridgeAdapter backed by a production
// RealSpawnAdapter. It satisfies [shipkit/ports.SpawnPort] and is the
// standard adapter for the doctor verb.
func NewSpawnBridge() *SpawnBridgeAdapter {
	return &SpawnBridgeAdapter{inner: updateadapters.NewRealSpawn()}
}

// HealthCheck runs binaryPath --version, parses the version from stdout, and
// returns a [ports.HealthResult]. Delegates to the underlying RealSpawnAdapter
// and converts the result type.
func (a *SpawnBridgeAdapter) HealthCheck(ctx context.Context, binaryPath string, timeout time.Duration) (ports.HealthResult, error) {
	res, err := a.inner.HealthCheck(ctx, binaryPath, timeout)
	if err != nil {
		return ports.HealthResult{}, err
	}
	return ports.HealthResult{
		Ok:      res.Ok,
		Version: res.Version,
		Reason:  res.Reason,
	}, nil
}
