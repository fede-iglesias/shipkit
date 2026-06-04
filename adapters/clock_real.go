package adapters

import "time"

// RealClockAdapter is the production implementation of
// [github.com/fede-iglesias/shipkit/ports.ClockPort]. It delegates to the
// standard library time package.
//
// All lifecycle verbs that need the current time accept a ClockPort rather
// than calling time.Now() directly. This makes tests fully deterministic by
// injecting a [github.com/fede-iglesias/shipkit/ports.MockClockPort] with a
// fixed timestamp.
type RealClockAdapter struct{}

// NewRealClock returns a RealClockAdapter backed by the system wall clock.
func NewRealClock() *RealClockAdapter { return &RealClockAdapter{} }

// NowUTC returns the current UTC time. Equivalent to time.Now().UTC().
func (a *RealClockAdapter) NowUTC() time.Time { return time.Now().UTC() }

// Since returns the duration elapsed since t. Equivalent to time.Since(t).
func (a *RealClockAdapter) Since(t time.Time) time.Duration { return time.Since(t) }
