package health_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/health"
)

func TestReadiness(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ready := health.New(nil, 0)
	require.Error(t, ready.Check(ctx))
	ready.SetReady(true)
	require.NoError(t, ready.Check(ctx))
	ready.SetReady(false)
	require.Error(t, ready.Check(ctx))
	ready.SetReady(true)
	require.NoError(t, ready.Check(ctx))
	ready.Drain()
	ready.SetReady(true)
	require.Error(t, ready.Check(ctx))
}

func TestChecks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		check health.Check
	}{
		{"failure", func(context.Context) error { return errors.New("down") }},
		{"deadline", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
		{"late success", func(ctx context.Context) error { <-ctx.Done(); return nil }},
		{"nil", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ready := health.New(map[string]health.Check{"db": tc.check}, time.Millisecond)
			ready.SetReady(true)
			require.Error(t, ready.Check(context.Background()))
		})
	}
	checks := map[string]health.Check{"db": func(ctx context.Context) error { _, ok := ctx.Deadline(); require.True(t, ok); return nil }}
	ready := health.New(checks, time.Second)
	delete(checks, "db")
	ready.SetReady(true)
	require.NoError(t, ready.Check(context.Background()))
}

func TestDrainDuringCheck(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	ready := health.New(map[string]health.Check{"db": func(context.Context) error { close(entered); <-release; return nil }}, time.Second)
	ready.SetReady(true)
	result := make(chan error, 1)
	go func() { result <- ready.Check(context.Background()) }()
	<-entered
	ready.Drain()
	close(release)
	require.Error(t, <-result)
	var workers sync.WaitGroup
	for range 30 {
		workers.Add(1)
		go func() { defer workers.Done(); ready.SetReady(true); ready.Drain() }()
	}
	workers.Wait()
	require.Error(t, ready.Check(context.Background()))
}
