package gateway

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"
)

type mockCatalogClient struct {
	responses []*catalogv1.GetSnapshotResponse
	errors    []error
	idx       int
}

func (m *mockCatalogClient) GetSnapshot(_ context.Context, _ *catalogv1.GetSnapshotRequest, _ ...grpc.CallOption) (*catalogv1.GetSnapshotResponse, error) {
	if m.idx < len(m.errors) && m.errors[m.idx] != nil {
		err := m.errors[m.idx]
		m.idx++
		return nil, err
	}
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return resp, nil
	}
	return nil, errors.New("no more responses")
}

func TestSnapshotCache_PollAndRefresher(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache := NewSnapshotCache(m, logger, clock.Real{})

	assert.False(t, cache.IsReady())
	assert.Error(t, cache.CheckReady(context.Background()))

	mock := &mockCatalogClient{
		responses: []*catalogv1.GetSnapshotResponse{
			{
				Version:     1,
				GeneratedAt: timestamppb.Now(),
				Campaigns: []*catalogv1.Campaign{
					{Id: "c1", BidderId: "b1"},
				},
				Bidders: []*catalogv1.Bidder{
					{Id: "b1", EndpointUrl: "http://b1"},
				},
			},
			{
				Version:   1,
				Unchanged: true,
			},
			nil, // error
		},
		errors: []error{
			nil,
			nil,
			errors.New("gRPC unavailable"),
		},
	}

	// 1. Initial poll
	err := cache.Poll(context.Background(), mock)
	require.NoError(t, err)
	assert.True(t, cache.IsReady())
	assert.NoError(t, cache.CheckReady(context.Background()))
	assert.Equal(t, int64(1), cache.Get().Version)

	// 2. Unchanged response
	err = cache.Poll(context.Background(), mock)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cache.Get().Version)

	// 3. Outage (error) -> stale-while-error: keeps previous snapshot
	err = cache.Poll(context.Background(), mock)
	require.Error(t, err)
	assert.True(t, cache.IsReady())
	assert.Equal(t, int64(1), cache.Get().Version)

	// 4. RunRefresher terminates cleanly
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cache.RunRefresher(ctx, mock, 10*time.Millisecond)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
}

func TestStubsAndServerHelpers(t *testing.T) {
	reserver := StubReserver{}
	assert.NoError(t, reserver.Release(context.Background(), "c1", 100, time.Now()))
	spent, err := reserver.Spent(context.Background(), "c1", time.Now())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), spent)

	publisher := NoopEventPublisher{}
	assert.True(t, publisher.TryPublishAsync(context.Background(), nil))
	assert.NoError(t, publisher.Flush(context.Background()))
	assert.NoError(t, publisher.Close())

	h, _ := setupTestGateway(t, nil, false)
	mux := NewServer(h)
	require.NotNil(t, mux)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := metrics.New("test")
	provider := tracing.NewProvider("test", "dev", 0)

	stacked := Stack(mux, logger, m, provider)
	require.NotNil(t, stacked)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/healthz", nil)
	stacked.ServeHTTP(rec, req)
	// /healthz is on admin server, so on public API mux it returns 404
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
