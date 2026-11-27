package metrics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/metrics"
)

func TestIndependentRegistriesAndPools(t *testing.T) {
	t.Parallel()
	first, second := metrics.New("first"), metrics.New("second")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig("postgres://test:test@localhost:1/test?connect_timeout=1")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()
	client := redis.NewClient(&redis.Options{Addr: "localhost:1"})
	defer func() { require.NoError(t, client.Close()) }()
	require.NoError(t, first.RegisterPGX(pool))
	require.NoError(t, first.RegisterRedis(client))
	require.Error(t, first.RegisterRedis(client))
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	first.Handler().ServeHTTP(w, req)
	require.Contains(t, w.Body.String(), `db_pool_acquired{service="first"} 0`)
	require.Contains(t, w.Body.String(), `db_pool_idle{service="first"} 0`)
	require.Contains(t, w.Body.String(), `redis_pool_idle{service="first"} 0`)
	w = httptest.NewRecorder()
	second.Handler().ServeHTTP(w, req)
	require.NotContains(t, w.Body.String(), "db_pool_acquired")
}

func TestMiddlewarePreservesStreaming(t *testing.T) {
	t.Parallel()
	m := metrics.New("stream")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		flusher.Flush()
		_, err := w.Write([]byte("streamed"))
		require.NoError(t, err)
	})
	handler := m.Middleware(mux)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "GET", "/items/private-id", nil))
	require.Equal(t, http.StatusAccepted, w.Code)
	require.True(t, w.Flushed)
	require.Equal(t, "streamed", w.Body.String())
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), "CUSTOM", "/another-private-id", nil))
	w = httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), "GET", "/metrics", nil))
	require.Contains(t, w.Body.String(), `route="GET /items/{id}"`)
	require.Contains(t, w.Body.String(), `method="OTHER"`)
	require.Contains(t, w.Body.String(), `route="unmatched"`)
	require.False(t, strings.Contains(w.Body.String(), "private-id"))
}
