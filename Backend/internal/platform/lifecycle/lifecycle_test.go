package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/lifecycle"
)

func TestShutdownOrdering(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	group := lifecycle.New(ctx)
	serverStopped := make(chan struct{})
	workerDone := make(chan struct{})
	group.Go("server", func(context.Context) error { <-serverStopped; return nil })
	group.Go("worker", func(ctx context.Context) error { defer close(workerDone); <-ctx.Done(); return ctx.Err() })
	var order []string
	var drainTime time.Time
	cancel()
	err := group.Run(func() { order = append(order, "unready"); drainTime = time.Now() }, 10*time.Millisecond, time.Second,
		lifecycle.Hook{Name: "listeners", Stop: func(ctx context.Context) error {
			require.NoError(t, ctx.Err())
			require.GreaterOrEqual(t, time.Since(drainTime), 10*time.Millisecond)
			order = append(order, "listeners")
			close(serverStopped)
			return nil
		}},
		lifecycle.Hook{Name: "flush", Stop: func(context.Context) error { order = append(order, "flush"); return nil }},
		lifecycle.Hook{Name: "pools", Stop: func(context.Context) error { order = append(order, "pools"); return nil }},
		lifecycle.Hook{Name: "telemetry", Stop: func(context.Context) error { order = append(order, "telemetry"); return nil }},
	)
	require.NoError(t, err)
	<-workerDone
	require.Equal(t, []string{"unready", "listeners", "flush", "pools", "telemetry"}, order)
}

func TestFatalWorkerAndHookFailure(t *testing.T) {
	t.Parallel()
	group := lifecycle.New(context.Background())
	fatal, flush := errors.New("fatal"), errors.New("flush failed")
	group.Go("listener", func(context.Context) error { return fatal })
	closed := false
	err := group.Run(func() {}, 0, time.Second, lifecycle.Hook{Name: "flush", Stop: func(context.Context) error { return flush }}, lifecycle.Hook{Name: "close", Stop: func(context.Context) error { closed = true; return nil }})
	require.ErrorIs(t, err, fatal)
	require.ErrorIs(t, err, flush)
	require.True(t, closed)
}

func TestSharedShutdownDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	group := lifecycle.New(ctx)
	lastCalled := false
	err := group.Run(func() {}, 0, time.Millisecond, lifecycle.Hook{Name: "slow", Stop: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}, lifecycle.Hook{Name: "cleanup", Stop: func(ctx context.Context) error { lastCalled = true; return ctx.Err() }})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, lastCalled)
}
