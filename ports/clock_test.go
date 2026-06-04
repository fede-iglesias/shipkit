package ports_test

import (
	"testing"
	"time"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockClockPort satisfies ClockPort.
var _ ports.ClockPort = (*ports.MockClockPort)(nil)

func TestMockClockPort_NowUTC_returnsFixed(t *testing.T) {
	fixed := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	m := ports.NewMockClockPort(fixed)
	got := m.NowUTC()
	if !got.Equal(fixed) {
		t.Errorf("want %v, got %v", fixed, got)
	}
}

func TestMockClockPort_Since_positive(t *testing.T) {
	fixed := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	past := fixed.Add(-10 * time.Minute)
	m := ports.NewMockClockPort(fixed)
	got := m.Since(past)
	if got != 10*time.Minute {
		t.Errorf("expected 10m, got %v", got)
	}
}

func TestMockClockPort_Since_negative(t *testing.T) {
	// When FixedTime is before the argument, Since returns a negative duration.
	// This mirrors time.Since behaviour when a test rewinds the clock.
	fixed := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	future := fixed.Add(5 * time.Minute)
	m := ports.NewMockClockPort(fixed)
	got := m.Since(future)
	if got != -5*time.Minute {
		t.Errorf("expected -5m, got %v", got)
	}
}
