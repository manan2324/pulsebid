package aggregator

import (
	"log/slog"
	"net/http"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/reporting"
)

// NewServer sets up the HTTP router for event-aggregator including reporting routes.
func NewServer(reportingSvc *reporting.Service, apiKeys []string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	if reportingSvc != nil {
		reporting.RegisterRoutes(mux, reportingSvc, apiKeys)
	}
	return mux
}

// Stack applies standard middleware (RequestID, Recover, Logging, Metrics, Tracing) to the mux.
func Stack(mux *http.ServeMux, logger *slog.Logger, m *metrics.Metrics, provider *sdktrace.TracerProvider) http.Handler {
	return httpx.Stack(mux, logger, m, provider)
}
