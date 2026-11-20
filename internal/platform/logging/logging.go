// Package logging supplies structured, context-aware JSON logging.
package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type loggerKey struct{}
type requestIDKey struct{}

// New creates an independent logger without modifying slog's global default.
func New(out io.Writer, service, version string, level slog.Level) *slog.Logger {
	base := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level})
	return slog.New(&contextHandler{next: base}).With("service", service, "version", version)
}

// WithLogger associates a logger with a request or worker context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the injected logger, or an inert logger when none is set.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return New(io.Discard, "", "", slog.LevelInfo)
}

// WithRequestID attaches a correlation ID shared by logging and HTTP helpers.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the request's correlation ID, or the empty string.
func RequestID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey{}).(string); return id }

type contextHandler struct{ next slog.Handler }

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	record = record.Clone()
	record.Time = record.Time.UTC()
	span := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if span.IsValid() {
		traceID, spanID = span.TraceID().String(), span.SpanID().String()
	}
	record.AddAttrs(slog.String("request_id", RequestID(ctx)), slog.String("trace_id", traceID), slog.String("span_id", spanID))
	return h.next.Handle(ctx, record)
}
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{next: h.next.WithAttrs(attrs)}
}
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{next: h.next.WithGroup(name)}
}
