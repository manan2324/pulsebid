package gateway

import (
	"log/slog"
	"net/http"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/metrics"
)

// NewServer builds the standard HTTP ServeMux for the gateway.
func NewServer(bidHandler *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/bid", bidHandler)
	mux.HandleFunc("GET /v1/events/impression", bidHandler.HandleImpression)
	mux.HandleFunc("GET /v1/events/click", bidHandler.HandleClick)
	return mux
}

// Stack applies standard middleware (RequestID, Recover, Logging, Metrics, Tracing) to the mux.
func Stack(mux *http.ServeMux, logger *slog.Logger, m *metrics.Metrics, provider *sdktrace.TracerProvider) http.Handler {
	return httpx.Stack(mux, logger, m, provider)
}
