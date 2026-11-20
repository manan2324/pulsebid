// Package clock provides injectable UTC time sources.
package clock

import (
	"sync"
	"time"
)

// Clock supplies the current time to business logic.
type Clock interface{ Now() time.Time }

// Real is the system clock.
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a concurrency-safe controllable clock.
type Fake struct {
	mu  sync.RWMutex
	now time.Time
}

// NewFake creates a clock at the supplied instant.
func NewFake(now time.Time) *Fake { return &Fake{now: now.UTC()} }

// Now returns the stored UTC instant.
func (f *Fake) Now() time.Time { f.mu.RLock(); defer f.mu.RUnlock(); return f.now }

// Set changes the current instant.
func (f *Fake) Set(now time.Time) { f.mu.Lock(); defer f.mu.Unlock(); f.now = now.UTC() }

// Advance moves the clock by a duration.
func (f *Fake) Advance(d time.Duration) { f.mu.Lock(); defer f.mu.Unlock(); f.now = f.now.Add(d) }
