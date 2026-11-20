// Package health manages readiness gates and bounded dependency checks.
package health

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"time"
)

// Check must obey its context deadline and return nil when a dependency is usable.
type Check func(context.Context) error

// Readiness starts unready. Once draining, it can never become ready again.
type Readiness struct {
	state   atomic.Int32
	checks  map[string]Check
	names   []string
	timeout time.Duration
}

// New copies checks, which are immutable after construction.
func New(checks map[string]Check, timeout time.Duration) *Readiness {
	if timeout <= 0 {
		timeout = time.Second
	}
	r := &Readiness{checks: make(map[string]Check, len(checks)), timeout: timeout}
	for name, check := range checks {
		r.checks[name] = check
		r.names = append(r.names, name)
	}
	sort.Strings(r.names)
	return r
}

// SetReady opens or closes the startup gate unless shutdown has begun.
func (r *Readiness) SetReady(ready bool) {
	target := int32(0)
	if ready {
		target = 1
	}
	for {
		old := r.state.Load()
		if old == 2 || r.state.CompareAndSwap(old, target) {
			return
		}
	}
}

// Drain permanently closes the readiness gate.
func (r *Readiness) Drain() { r.state.Store(2) }

// Check evaluates dependencies under one deadline and rechecks the shutdown gate.
func (r *Readiness) Check(ctx context.Context) error {
	if r.state.Load() != 1 {
		return fmt.Errorf("service is not ready")
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	for _, name := range r.names {
		check := r.checks[name]
		if check == nil {
			return fmt.Errorf("readiness check %s is nil", name)
		}
		if err := check(ctx); err != nil {
			return fmt.Errorf("readiness check %s: %w", name, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.state.Load() != 1 {
		return fmt.Errorf("service is draining")
	}
	return nil
}
