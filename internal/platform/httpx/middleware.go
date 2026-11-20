package httpx

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"pulsebid/internal/platform/ids"
	"pulsebid/internal/platform/logging"
)

// Middleware wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware in request order, outermost first.
func Chain(next http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		next = middleware[i](next)
	}
	return next
}

// RequestID accepts bounded printable correlation IDs or generates a UUIDv7.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		valid := len(id) > 0 && len(id) <= 128
		for _, c := range id {
			if c < 33 || c > 126 {
				valid = false
				break
			}
		}
		if !valid {
			var err error
			id, err = ids.New()
			if err != nil {
				WriteError(w, r, err)
				return
			}
		}
		w.Header().Set("X-Request-Id", id)
		ctx := logging.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recover shields callers from panic contents and does not append an error to an
// already committed response. Streaming failures abort the connection instead.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		committed := false
		wrapped := httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(original httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					if code >= 200 {
						committed = true
					}
					original(code)
				}
			},
			Write: func(original httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(p []byte) (int, error) { committed = true; return original(p) }
			},
			Flush: func(original httpsnoop.FlushFunc) httpsnoop.FlushFunc { return func() { committed = true; original() } },
			ReadFrom: func(original httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
				return func(reader io.Reader) (int64, error) { committed = true; return original(reader) }
			},
		})
		defer func() {
			if recovered := recover(); recovered != nil {
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				logging.FromContext(r.Context()).ErrorContext(r.Context(), "HTTP handler panic")
				if committed {
					panic(http.ErrAbortHandler)
				}
				WriteError(w, r, &Error{Code: "internal"})
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

// Logging injects the logger and records only route templates and safe metadata.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(logging.WithLogger(r.Context(), logger))
			measured := httpsnoop.CaptureMetrics(next, w, r)
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			durMS := float64(measured.Duration) / float64(time.Millisecond)
			if route == "POST /v1/bid" || strings.HasSuffix(route, "/v1/bid") {
				if measured.Code >= 400 || measured.Duration > 110*time.Millisecond {
					logger.WarnContext(r.Context(), "HTTP request", "route", route, "status", measured.Code, "latency_ms", durMS)
				} else {
					logger.DebugContext(r.Context(), "HTTP request", "route", route, "status", measured.Code, "latency_ms", durMS)
				}
			} else {
				logger.InfoContext(r.Context(), "HTTP request", "route", route, "status", measured.Code, "latency_ms", durMS)
			}
		})
	}
}

// APIKey hashes keys before constant-time comparison, including different lengths.
// An empty configured key list always denies access; no keys are logged.
func APIKey(keys []string) Middleware {
	hashes := make([][sha256.Size]byte, 0, len(keys))
	for _, key := range keys {
		if key = strings.TrimSpace(key); key != "" {
			hashes = append(hashes, sha256.Sum256([]byte(key)))
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			supplied := sha256.Sum256([]byte(r.Header.Get("X-API-Key")))
			matched := 0
			for _, expected := range hashes {
				matched |= subtle.ConstantTimeCompare(supplied[:], expected[:])
			}
			if matched != 1 {
				WriteError(w, r, &Error{Code: "unauthorized", Message: "valid API key required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout supplies a cooperative handler deadline without spawning an unowned
// goroutine. Handlers must pass the context through all blocking work.
func Timeout(duration time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()
			child := r.WithContext(ctx)
			next.ServeHTTP(w, child)
			r.Pattern = child.Pattern
		})
	}
}
