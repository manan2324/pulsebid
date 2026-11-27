// Package grpcx provides traced gRPC servers and clients with bounded calls.
package grpcx

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"pulsebid/internal/platform/tracing"
)

// NewServer registers the standard health service, initially NOT_SERVING.
func NewServer(provider trace.TracerProvider, options ...grpc.ServerOption) (*grpc.Server, *health.Server) {
	options = append(options, grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(tracing.Propagator()))))
	server := grpc.NewServer(options...)
	ready := health.NewServer()
	ready.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(server, ready)
	return server, ready
}

// NewClient requires an explicit transport security choice. Unary calls receive
// a five-second cap; streams require their caller to supply a deadline.
func NewClient(target string, transport credentials.TransportCredentials, provider trace.TracerProvider) (*grpc.ClientConn, error) {
	if transport == nil {
		return nil, fmt.Errorf("gRPC transport credentials are required")
	}
	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(transport),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithTracerProvider(provider), otelgrpc.WithPropagators(tracing.Propagator()))),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return invoke(ctx, method, req, reply, cc, opts...)
		}),
		grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			if _, ok := ctx.Deadline(); !ok {
				return nil, fmt.Errorf("gRPC stream requires a context deadline")
			}
			return streamer(ctx, desc, cc, method, opts...)
		}))
	if err != nil {
		return nil, fmt.Errorf("create gRPC client: %w", err)
	}
	return connection, nil
}

// Shutdown waits for graceful completion and force-stops on the deadline.
// The helper goroutine is always joined before returning.
func Shutdown(ctx context.Context, server *grpc.Server) error {
	done := make(chan struct{})
	go func() { defer close(done); server.GracefulStop() }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		server.Stop()
		<-done
		return ctx.Err()
	}
}
