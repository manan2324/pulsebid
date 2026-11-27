package aggregator

import (
	"context"
	"testing"
	"time"

	"pulsebid/internal/events"

	"github.com/stretchr/testify/require"
)

type mockStoreForRetention struct {
	deletedCount int64
	called       bool
}

func (m *mockStoreForRetention) ProcessBatch(_ context.Context, _ []*events.Event) (newCount, dupCount int, err error) {
	return 0, 0, nil
}

func (m *mockStoreForRetention) DeleteOldProcessedEvents(_ context.Context, _ time.Time, _ int) (int64, error) {
	m.called = true
	m.deletedCount += 5
	return 5, nil
}

func (m *mockStoreForRetention) Ping(_ context.Context) error {
	return nil
}

func (m *mockStoreForRetention) Close() {}

func TestRetentionJob_Run(t *testing.T) {
	mock := &mockStoreForRetention{}
	job := NewRetentionJob(mock, 24*time.Hour, 10*time.Millisecond, nil)
	require.NotNil(t, job)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := job.Run(ctx)
	require.NoError(t, err)
	require.True(t, mock.called)
	require.GreaterOrEqual(t, mock.deletedCount, int64(5))
}

func TestStore_PingAndHelpers(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()
	require.NoError(t, store.Ping(ctx))

	// Invalid URL
	_, err := NewPGStoreFromURL(ctx, "invalid-url", nil, nil)
	require.Error(t, err)
}
