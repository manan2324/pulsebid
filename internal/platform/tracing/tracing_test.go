package tracing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"pulsebid/internal/platform/tracing"
)

func TestParentBasedSamplingAndResource(t *testing.T) {
	t.Parallel()
	exporter := tracetest.NewInMemoryExporter()
	provider := tracing.NewProvider("test-service", "v1", 0, sdktrace.WithSyncer(exporter))
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()
	_, root := provider.Tracer("test").Start(context.Background(), "unsampled")
	require.False(t, root.IsRecording())
	root.End()
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	require.NoError(t, err)
	parent := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true})
	_, child := provider.Tracer("test").Start(trace.ContextWithRemoteSpanContext(context.Background(), parent), "sampled child")
	require.True(t, child.IsRecording())
	child.End()
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, traceID, spans[0].SpanContext.TraceID())
	attributes := map[string]string{}
	for _, attribute := range spans[0].Resource.Attributes() {
		attributes[string(attribute.Key)] = attribute.Value.AsString()
	}
	require.Equal(t, "test-service", attributes["service.name"])
	require.Equal(t, "v1", attributes["service.version"])
}

func TestHTTPHonorsRemoteSampling(t *testing.T) {
	t.Parallel()
	exporter := tracetest.NewInMemoryExporter()
	provider := tracing.NewProvider("http-test", "dev", 1, sdktrace.WithSyncer(exporter))
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()
	var recorded bool
	handler := tracing.HTTP(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded = trace.SpanFromContext(r.Context()).IsRecording()
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		parent string
		record bool
	}{
		{"00-0123456789abcdef0123456789abcdef-0123456789abcdef-00", false},
		{"00-0123456789abcdef0123456789abcdef-0123456789abcdef-01", true},
		{"", true},
	} {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/probe", nil)
		req.Pattern = "GET /probe"
		req.Header.Set("traceparent", tc.parent)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		require.Equal(t, http.StatusNoContent, response.Code)
		require.Equal(t, tc.record, recorded)
	}
	require.Len(t, exporter.GetSpans(), 2)
	require.Equal(t, "GET /probe", exporter.GetSpans()[0].Name)
	require.Equal(t, "0123456789abcdef0123456789abcdef", exporter.GetSpans()[0].SpanContext.TraceID().String())
}

func TestExporterLifecycleWithoutCollector(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	provider, err := tracing.New(ctx, "test", "dev", "localhost:1", 0)
	require.NoError(t, err)
	require.NoError(t, provider.Shutdown(ctx))
}
