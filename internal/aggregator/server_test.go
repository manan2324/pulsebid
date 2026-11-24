package aggregator

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"pulsebid/internal/platform/metrics"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestServer_Healthz(t *testing.T) {
	mux := NewServer(nil, nil)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/healthz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestServer_Stack(t *testing.T) {
	mux := NewServer(nil, nil)
	m := metrics.New("test-agg")
	provider := sdktrace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()

	handler := Stack(mux, slog.Default(), m, provider)
	require.NotNil(t, handler)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
