package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/logging"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"
)

func request(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestErrors(t *testing.T) {
	t.Parallel()
	for code, status := range map[string]int{"invalid_request": 400, "invalid_token": 400, "unauthorized": 401, "not_found": 404, "conflict": 409, "payload_too_large": 413, "unsupported_media_type": 415, "token_expired": 410, "unavailable": 503, "internal": 500, "unknown": 500} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, status, httpx.Status(code))
			cause := fmt.Errorf("wrapped: %w", &httpx.Error{Code: code, Message: "safe message", Details: []httpx.Detail{{Field: "amount", Issue: "required"}}, Cause: errors.New("database secret")})
			handler := httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, cause) }))
			w := request(handler, "GET", "/", "", map[string]string{"X-Request-Id": "req-1"})
			require.Equal(t, status, w.Code)
			require.Equal(t, "req-1", w.Header().Get("X-Request-Id"))
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			var body struct {
				Error httpx.Error `json:"error"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, "req-1", body.Error.RequestID)
			if status == 500 {
				require.Equal(t, "internal", body.Error.Code)
				require.Equal(t, "internal server error", body.Error.Message)
			} else {
				require.Equal(t, code, body.Error.Code)
				require.Len(t, body.Error.Details, 1)
			}
			require.NotContains(t, w.Body.String(), "database secret")
		})
	}
	for _, err := range []error{errors.New("secret"), context.Canceled, context.DeadlineExceeded} {
		handler := httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, err) }))
		w := request(handler, "GET", "/", "", nil)
		require.NotEmpty(t, w.Header().Get("X-Request-Id"))
		require.NotContains(t, w.Body.String(), "secret")
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			require.Equal(t, 503, w.Code)
		} else {
			require.Equal(t, 500, w.Code)
		}
	}
	sentinel := errors.New("domain error")
	require.ErrorIs(t, &httpx.Error{Cause: sentinel}, sentinel)
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, contentType string
		limit                   int64
		strict                  bool
		status                  int
	}{
		{"valid", `{"amount":1}`, "application/json", 64, true, 204},
		{"charset", `{"amount":1}`, "application/json; charset=utf-8", 64, true, 204},
		{"lenient", `{"amount":1,"extra":true}`, "application/json", 64, false, 204},
		{"strict", `{"amount":1,"extra":true}`, "application/json", 64, true, 400},
		{"exact limit", `{"amount":1}`, "application/json", 12, true, 204},
		{"over limit", `{"amount":12}`, "application/json", 12, true, 413},
		{"trailing oversized", `{"amount":1}` + strings.Repeat(" ", 100), "application/json", 20, true, 413},
		{"malformed oversized", strings.Repeat("x", 100), "application/json", 20, true, 413},
		{"no type", `{}`, "", 64, true, 415},
		{"wrong type", `{}`, "text/plain", 64, true, 415},
		{"bad type", `{}`, "application/json; broken", 64, true, 415},
		{"empty", "", "application/json", 64, true, 400},
		{"null", "null", "application/json", 64, true, 400},
		{"broken", "{", "application/json", 64, true, 400},
		{"multiple", "{} {}", "application/json", 64, true, 400},
		{"trailing invalid", "{} x", "application/json", 64, true, 400},
		{"wrong shape", "[]", "application/json", 64, true, 400},
		{"bad limit", "{}", "application/json", 0, true, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Amount json.Number `json:"amount"`
				}
				if err := httpx.DecodeJSON(w, r, &body, tc.limit, tc.strict); err != nil {
					httpx.WriteError(w, r, err)
					return
				}
				httpx.WriteJSON(w, r, 204, nil)
			}))
			w := request(handler, "POST", "/", tc.body, map[string]string{"Content-Type": tc.contentType})
			require.Equal(t, tc.status, w.Code, w.Body.String())
		})
	}
}

func TestAPIKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, key string
		keys      []string
		status    int
	}{
		{"first", "key-a", []string{"key-a", "key-b"}, 204}, {"second", "key-b", []string{"key-a", "key-b"}, 204},
		{"trim config", "key-a", []string{" key-a "}, 204}, {"missing", "", []string{"key-a"}, 401},
		{"wrong", "key-c", []string{"key-a"}, 401}, {"short", "key", []string{"key-a"}, 401},
		{"long", "key-a-extra", []string{"key-a"}, 401}, {"empty config", "", []string{""}, 401}, {"no keys", "anything", nil, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { httpx.WriteJSON(w, r, 204, nil) }), httpx.RequestID, httpx.APIKey(tc.keys))
			w := request(h, "GET", "/", "", map[string]string{"X-API-Key": tc.key})
			require.Equal(t, tc.status, w.Code)
			require.NotEmpty(t, w.Header().Get("X-Request-Id"))
		})
	}
}

func TestRequestIDAndRecovery(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", strings.Repeat("a", 129), "bad id", "bad\nid", "valid-id"} {
		h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, w.Header().Get("X-Request-Id"), logging.RequestID(r.Context()))
			panic("private panic")
		}), httpx.RequestID, httpx.Recover)
		w := request(h, "GET", "/", "", map[string]string{"X-Request-Id": id})
		require.Equal(t, 500, w.Code)
		require.NotContains(t, w.Body.String(), "private panic")
		if id == "valid-id" {
			require.Equal(t, id, w.Header().Get("X-Request-Id"))
		} else {
			require.Len(t, w.Header().Get("X-Request-Id"), 36)
		}
	}
	h := httpx.Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); panic("after headers") }))
	require.PanicsWithValue(t, http.ErrAbortHandler, func() { request(h, "GET", "/", "", nil) })
	h = httpx.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	require.PanicsWithValue(t, http.ErrAbortHandler, func() { request(h, "GET", "/", "", nil) })
}

func TestAdminMetricsTracingAndReadiness(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	exporter := tracetest.NewInMemoryExporter()
	provider := tracing.NewProvider("test", "v1", 1, sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	logger := logging.New(&logs, "test", "v1", slog.LevelInfo)
	m := metrics.New("test")
	ready := health.New(nil, time.Second)
	h := httpx.AdminHandler(ready, logger, m, provider)
	for _, path := range []string{"/healthz", "/metrics", "/debug/pprof/", "/debug/pprof/goroutine?debug=1"} {
		w := request(h, "GET", path, "", nil)
		require.Equal(t, 200, w.Code)
		require.NotEmpty(t, w.Header().Get("X-Request-Id"))
	}
	require.Equal(t, 503, request(h, "GET", "/readyz", "", nil).Code)
	ready.SetReady(true)
	require.Equal(t, 200, request(h, "GET", "/readyz", "", nil).Code)
	ready.Drain()
	require.Equal(t, 503, request(h, "GET", "/readyz", "", nil).Code)
	w := request(h, "GET", "/metrics", "", nil)
	require.Contains(t, w.Body.String(), "http_server_requests_total")
	require.Contains(t, w.Body.String(), `route="GET /healthz"`)
	require.Contains(t, w.Body.String(), "go_goroutines")
	require.Equal(t, 404, request(h, "GET", "/unknown", "", nil).Code)
	missingProfile := request(h, "GET", "/debug/pprof/missing-profile", "", nil)
	require.Equal(t, 404, missingProfile.Code)
	require.Equal(t, "application/json", missingProfile.Header().Get("Content-Type"))
	require.Contains(t, missingProfile.Body.String(), `"not_found"`)
	var found bool
	for _, span := range exporter.GetSpans() {
		if span.Name == "GET /healthz" {
			found = true
		}
	}
	require.True(t, found)
	require.Contains(t, logs.String(), `"request_id"`)
	require.Contains(t, logs.String(), `"trace_id"`)
}

func TestRouteCardinalityAndPropagation(t *testing.T) {
	t.Parallel()
	m := metrics.New("test")
	var logs bytes.Buffer
	exporter := tracetest.NewInMemoryExporter()
	provider := tracing.NewProvider("test", "dev", 1, sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.PathValue("id"))
		httpx.WriteJSON(w, r, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /panic", func(http.ResponseWriter, *http.Request) { panic("secret") })
	h := httpx.Stack(mux, logging.New(&logs, "test", "dev", slog.LevelInfo), m, provider)
	for _, id := range []string{"customer-123", "customer-456"} {
		require.Equal(t, 200, request(h, "GET", "/items/"+id+"?secret=private", "", map[string]string{"traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"}).Code)
	}
	require.Equal(t, 500, request(h, "GET", "/panic", "", nil).Code)
	require.Equal(t, 404, request(h, "UNUSUAL", "/missing-123", "", nil).Code)
	w := request(m.Handler(), "GET", "/metrics", "", nil)
	require.Contains(t, w.Body.String(), `route="GET /items/{id}"`)
	require.Contains(t, w.Body.String(), `status="500"`)
	require.Contains(t, w.Body.String(), `method="OTHER"`)
	for _, secret := range []string{"customer-123", "customer-456", "private", "missing-123"} {
		require.NotContains(t, w.Body.String(), secret)
		require.NotContains(t, logs.String(), secret)
	}
	spans := exporter.GetSpans()
	require.Equal(t, "0123456789abcdef0123456789abcdef", spans[0].SpanContext.TraceID().String())
	require.Equal(t, "0123456789abcdef", spans[0].Parent.SpanID().String())
}

func TestTimeoutAndEncodingFailure(t *testing.T) {
	t.Parallel()
	handler := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		httpx.WriteError(w, r, r.Context().Err())
	}), httpx.RequestID, httpx.Timeout(time.Millisecond))
	require.Equal(t, 503, request(handler, "GET", "/", "", nil).Code)
	handler = httpx.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { httpx.WriteJSON(w, r, 200, make(chan int)) }))
	w := request(handler, "GET", "/", "", nil)
	require.Equal(t, 500, w.Code)
	require.Contains(t, w.Body.String(), `"internal"`)
}

func TestServerShutdownDrainsRequest(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	logger := logging.New(io.Discard, "test", "dev", slog.LevelInfo)
	server := httpx.NewServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		httpx.WriteJSON(w, r, 200, "finished")
	}), logger)
	require.Positive(t, server.ReadHeaderTimeout)
	require.Positive(t, server.IdleTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", server.Addr)
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	response := make(chan error, 1)
	go func() {
		req, err := http.NewRequestWithContext(ctx, "GET", "http://"+listener.Addr().String(), nil)
		if err != nil {
			response <- err
			return
		}
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			_, err = io.Copy(io.Discard, res.Body)
			err = errors.Join(err, res.Body.Close())
			if res.StatusCode != 200 {
				err = fmt.Errorf("status %d", res.StatusCode)
			}
		}
		response <- err
	}()
	<-entered
	stopped := make(chan error, 1)
	go func() { stopped <- httpx.Shutdown(ctx, server) }()
	select {
	case err := <-stopped:
		t.Fatalf("shutdown did not drain: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-response)
	require.NoError(t, <-stopped)
	require.ErrorIs(t, <-served, http.ErrServerClosed)
}
