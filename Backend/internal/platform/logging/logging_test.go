package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"pulsebid/internal/platform/logging"
)

func TestContextFields(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := logging.New(&output, "test", "v1", slog.LevelInfo)
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	require.NoError(t, err)
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID}))
	ctx = logging.WithLogger(logging.WithRequestID(ctx, "req-1"), logger)
	logging.FromContext(ctx).DebugContext(ctx, "hidden")
	require.Empty(t, output.String())
	logging.FromContext(ctx).With("outcome", "ok").InfoContext(ctx, "test message")
	var line map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &line))
	for key, want := range map[string]string{"service": "test", "version": "v1", "request_id": "req-1", "trace_id": traceID.String(), "span_id": spanID.String(), "outcome": "ok"} {
		require.Equal(t, want, line[key])
	}
	output.Reset()
	logger.Info("background")
	require.NoError(t, json.Unmarshal(output.Bytes(), &line))
	require.Equal(t, "", line["request_id"])
	require.Equal(t, "", line["trace_id"])
	output.Reset()
	logger.WithGroup("job").InfoContext(ctx, "grouped")
	require.Contains(t, output.String(), `"job"`)
	logging.FromContext(context.Background()).Info("discarded")
	output.Reset()
	record := slog.NewRecord(time.Date(2026, 11, 18, 10, 0, 0, 0, time.FixedZone("local", 19800)), slog.LevelInfo, "UTC timestamp", 0)
	require.NoError(t, logger.Handler().Handle(ctx, record))
	require.NoError(t, json.Unmarshal(output.Bytes(), &line))
	require.Equal(t, "2026-11-18T04:30:00Z", line["time"])
}
