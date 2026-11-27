// Package lifecycle owns service workers and ordered shutdown hooks.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// Group cancels its context on the first fatal worker error or parent cancellation.
// Workers must exit on cancellation or through a registered shutdown hook.
type Group struct {
	group *errgroup.Group
	ctx   context.Context
}

// New creates an owner for all long-running service goroutines.
func New(ctx context.Context) *Group {
	group, ctx := errgroup.WithContext(ctx)
	return &Group{group: group, ctx: ctx}
}

// Go starts a named worker. Context cancellation during shutdown is expected.
func (g *Group) Go(name string, work func(context.Context) error) {
	g.group.Go(func() error {
		err := work(g.ctx)
		if err == nil || (errors.Is(err, context.Canceled) && g.ctx.Err() != nil) {
			return nil
		}
		return fmt.Errorf("%s: %w", name, err)
	})
}

// Hook must honor its deadline and perform any required forced cleanup itself.
type Hook struct {
	Name string
	Stop func(context.Context) error
}

// Run waits for shutdown, closes readiness, allows a drain delay, then invokes
// hooks in order under a fresh shared deadline. Register listener drains first,
// work flushes next, pools next, and telemetry shutdown last.
func (g *Group) Run(drain func(), delay, timeout time.Duration, hooks ...Hook) error {
	<-g.ctx.Done()
	drain()
	if delay > 0 {
		timer := time.NewTimer(delay)
		<-timer.C
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(g.ctx), timeout)
	defer cancel()
	var result error
	for _, hook := range hooks {
		if err := hook.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown %s: %w", hook.Name, err))
		}
	}
	return errors.Join(result, g.group.Wait())
}
