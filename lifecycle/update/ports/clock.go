// Package ports defines the port interfaces for the lifecycle/update subsystem.
// Each interface represents an external dependency that can be replaced
// by a test double, enabling full unit testing without I/O.
package ports

import "time"

// ClockPort abstracts time operations used by the update process, enabling
// deterministic testing without relying on wall-clock time.
type ClockPort interface {
	// NowUTC returns the current time in UTC.
	NowUTC() time.Time

	// Since returns the duration elapsed since t. Equivalent to time.Since(t)
	// but injectable, so tests can control elapsed time precisely.
	Since(t time.Time) time.Duration
}
