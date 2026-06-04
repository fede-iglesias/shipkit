package ports_test

import (
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/lifecycle/update/ports"
)

// mockClock is a compile-time proof that ClockPort is implementable.
type mockClock struct {
	nowUTCFunc func() time.Time
	sinceFunc  func(t time.Time) time.Duration
}

func (m *mockClock) NowUTC() time.Time {
	return m.nowUTCFunc()
}

func (m *mockClock) Since(t time.Time) time.Duration {
	return m.sinceFunc(t)
}

// TestClockPort_InterfaceCompliance asserts at compile time that *mockClock
// satisfies ClockPort. If ClockPort changes, this line fails compilation.
var _ ports.ClockPort = (*mockClock)(nil)

// TestNowUTC_SignatureReturnsTime verifies that a mock ClockPort NowUTC call
// returns the time value set by the caller, enabling deterministic tests.
func TestNowUTC_SignatureReturnsTime(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	m := &mockClock{
		nowUTCFunc: func() time.Time { return fixed },
	}

	got := m.NowUTC()
	if !got.Equal(fixed) {
		t.Errorf("NowUTC: got %v, want %v", got, fixed)
	}
	if got.Location() != time.UTC {
		t.Errorf("NowUTC: location must be UTC, got %v", got.Location())
	}
}

// TestSince_SignatureReturnsDuration verifies that a mock ClockPort Since call
// returns the duration computed by the injected function, allowing callers to
// assert elapsed-time logic without depending on wall-clock time.
func TestSince_SignatureReturnsDuration(t *testing.T) {
	t.Parallel()

	const wantElapsed = 5 * time.Second
	anchor := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)

	m := &mockClock{
		sinceFunc: func(t time.Time) time.Duration {
			if !t.Equal(anchor) {
				return 0
			}
			return wantElapsed
		},
	}

	got := m.Since(anchor)
	if got != wantElapsed {
		t.Errorf("Since: got %v, want %v", got, wantElapsed)
	}
}
