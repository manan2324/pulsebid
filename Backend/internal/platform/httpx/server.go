package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/felixge/httpsnoop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"
)

// NewServer configures bounded header/body/idle reads and a private mux.
// A write timeout is omitted for pprof's long-running profiles; individual API
// handlers can use Timeout and cooperative context deadlines.
func NewServer(address string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError)}
}

// Shutdown drains accepted requests and force-closes connections at the deadline.
func Shutdown(ctx context.Context, server *http.Server) error {
	err := server.Shutdown(ctx)
	if err != nil {
		return errors.Join(err, server.Close())
	}
	return nil
}

// Stack instruments a mux in an order that retains route patterns, span context,
// request IDs, and recovered response status in both metrics and logs.
func Stack(mux *http.ServeMux, logger *slog.Logger, m *metrics.Metrics, provider *sdktrace.TracerProvider) http.Handler {
	return Chain(mux, RequestID, tracing.HTTP(provider), Logging(logger), m.Middleware, Recover)
}

// AdminHandler exposes operations endpoints on a dedicated server only.
func AdminHandler(ready *health.Readiness, logger *slog.Logger, m *metrics.Metrics, provider *sdktrace.TracerProvider) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := ready.Check(r.Context()); err != nil {
			WriteError(w, r, &Error{Code: "unavailable", Message: "service is not ready", Cause: err})
			return
		}
		WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", m.Handler())
	mux.Handle("GET /debug/pprof/", profileHandler(pprof.Index))
	mux.Handle("GET /debug/pprof/cmdline", profileHandler(pprof.Cmdline))
	mux.Handle("GET /debug/pprof/profile", profileHandler(pprof.Profile))
	mux.Handle("GET /debug/pprof/symbol", profileHandler(pprof.Symbol))
	mux.Handle("POST /debug/pprof/symbol", profileHandler(pprof.Symbol))
	mux.Handle("GET /debug/pprof/trace", profileHandler(pprof.Trace))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, &Error{Code: "not_found", Message: "route not found"})
	})
	return Stack(mux, logger, m, provider)
}

// profileHandler preserves streaming profiles but normalizes pprof's error
// responses to the common JSON envelope without buffering successful profiles.
func profileHandler(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failed := 0
		wrapped := httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(original httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					if code >= 400 {
						failed = code
						return
					}
					original(code)
				}
			},
			Write: func(original httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(data []byte) (int, error) {
					if failed != 0 {
						return len(data), nil
					}
					return original(data)
				}
			},
			Flush: func(original httpsnoop.FlushFunc) httpsnoop.FlushFunc {
				return func() {
					if failed == 0 {
						original()
					}
				}
			},
		})
		next(wrapped, r)
		if failed != 0 {
			w.Header().Del("Content-Length")
			code, message := "internal", "profiling failed"
			switch failed {
			case http.StatusBadRequest:
				code, message = "invalid_request", "invalid profiling parameters"
			case http.StatusNotFound:
				code, message = "not_found", "profile not found"
			}
			WriteError(w, r, &Error{Code: code, Message: message})
		}
	})
}
