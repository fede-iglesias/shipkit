package adapters

import (
	"testing"
	"time"
)

// TestNewRealClock verifies the constructor returns a non-nil adapter.
func TestNewRealClock(t *testing.T) {
	a := NewRealClock()
	if a == nil {
		t.Fatal("NewRealClock returned nil")
	}
}

// TestRealClockAdapter_NowUTC verifies that NowUTC returns a time close to the
// actual wall clock (within 1 second).
func TestRealClockAdapter_NowUTC(t *testing.T) {
	a := NewRealClock()
	before := time.Now().UTC()
	got := a.NowUTC()
	after := time.Now().UTC()

	if got.Before(before) || got.After(after) {
		t.Errorf("NowUTC = %v; want between %v and %v", got, before, after)
	}
}

// TestRealClockAdapter_Since verifies that Since returns a non-negative
// duration for a past timestamp.
func TestRealClockAdapter_Since(t *testing.T) {
	a := NewRealClock()
	past := time.Now().Add(-1 * time.Second)
	d := a.Since(past)
	if d < 0 {
		t.Errorf("Since(past) = %v; want non-negative", d)
	}
}
