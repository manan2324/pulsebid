// Package tracing constructs explicitly owned OpenTelemetry providers.
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// New owns its exporter and batch worker; callers must call Shutdown.
// Local OTLP uses plaintext gRPC, matching the Jaeger development endpoint.
func New(ctx context.Context, service, version, endpoint string, ratio float64) (*sdktrace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure(), otlptracegrpc.WithTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}
	return NewProvider(service, version, ratio, sdktrace.WithBatcher(exporter)), nil
}

// NewProvider creates an isolated provider, also usable with in-memory test exporters.
func NewProvider(service, version string, ratio float64, options ...sdktrace.TracerProviderOption) *sdktrace.TracerProvider {
	options = append(options,
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service), attribute.String("service.version", version))),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))))
	return sdktrace.NewTracerProvider(options...)
}

// Propagator carries W3C trace and baggage context without global registration.
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// HTTP instruments a handler with the supplied provider.
func HTTP(provider *sdktrace.TracerProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "HTTP", otelhttp.WithTracerProvider(provider), otelhttp.WithPropagators(Propagator()),
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				if r.Pattern != "" {
					return r.Pattern
				}
				return operation
			}))
	}
}
