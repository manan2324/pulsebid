package clock_test

import (
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/clock"
	"sync"
	"testing"
	"time"
)

func TestClocks(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 18, 10, 0, 0, 0, time.FixedZone("test", 19800))
	fake := clock.NewFake(start)
	require.Equal(t, time.UTC, fake.Now().Location())
	require.True(t, fake.Now().Equal(start))
	var workers sync.WaitGroup
	for range 20 {
		workers.Add(1)
		go func() { defer workers.Done(); fake.Advance(time.Second); _ = fake.Now() }()
	}
	workers.Wait()
	require.Equal(t, start.UTC().Add(20*time.Second), fake.Now())
	fake.Set(start)
	require.Equal(t, start.UTC(), fake.Now())
	require.WithinDuration(t, time.Now(), clock.Real{}.Now(), time.Second)
}
