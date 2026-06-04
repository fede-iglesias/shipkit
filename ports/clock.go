package ports

import "time"

// ClockPort abstracts time operations used by the lifecycle verbs, enabling
// deterministic testing without relying on wall-clock time.
//
// Every verb that needs the current time (for marker timestamps, snapshot IDs,
// or age-based clean decisions) takes a ClockPort in its Deps struct rather
// than calling time.Now() directly. This makes tests time-controllable without
// monkey-patching global state.
type ClockPort interface {
	// NowUTC returns the current time in UTC.
	NowUTC() time.Time

	// Since returns the duration elapsed since t. Equivalent to time.Since(t)
	// but injectable, so tests can control elapsed time precisely.
	Since(t time.Time) time.Duration
}

// MockClockPort is a test double for ClockPort. It returns FixedTime for
// NowUTC and computes Since relative to FixedTime. Use NewMockClockPort for
// safe defaults.
type MockClockPort struct {
	// FixedTime is returned by NowUTC and used as the reference for Since.
	// Zero value is time.Time{} (year 1); set it explicitly in tests.
	FixedTime time.Time
}

// NewMockClockPort returns a MockClockPort whose FixedTime is set to the given
// t. Pass a known time so tests are fully deterministic.
func NewMockClockPort(t time.Time) *MockClockPort { return &MockClockPort{FixedTime: t} }

// NowUTC implements ClockPort. Returns FixedTime.
func (m *MockClockPort) NowUTC() time.Time { return m.FixedTime }

// Since implements ClockPort. Returns FixedTime.Sub(t) (duration from t to
// FixedTime). If FixedTime is before t the result is negative, which mirrors
// the behaviour of time.Since when the clock is rewound in tests.
func (m *MockClockPort) Since(t time.Time) time.Duration { return m.FixedTime.Sub(t) }
