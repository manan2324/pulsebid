package grpcx_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"pulsebid/internal/platform/grpcx"
	"pulsebid/internal/platform/tracing"
)

func TestHealthTracingAndDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exporter := tracetest.NewInMemoryExporter()
	provider := tracing.NewProvider("grpc-test", "dev", 1, sdktrace.WithSyncer(exporter))
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()
	deadlines := make(chan time.Duration, 4)
	server, health := grpcx.NewServer(provider, grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, status.Error(codes.Internal, "missing deadline")
		}
		deadlines <- time.Until(deadline)
		return handler(ctx, req)
	}))
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		err := <-served
		require.True(t, err == nil || errors.Is(err, grpc.ErrServerStopped))
	})
	client, err := grpcx.NewClient(listener.Addr().String(), insecure.NewCredentials(), provider)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()
	parentCtx, span := provider.Tracer("test").Start(ctx, "parent")
	defer span.End()
	response, err := healthpb.NewHealthClient(client).Check(parentCtx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, response.Status)
	health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	response, err = healthpb.NewHealthClient(client).Check(parentCtx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, response.Status)
	require.LessOrEqual(t, <-deadlines, 5*time.Second)
	require.LessOrEqual(t, <-deadlines, 5*time.Second)
	for _, record := range exporter.GetSpans() {
		require.Equal(t, span.SpanContext().TraceID(), record.SpanContext.TraceID())
	}
	_, err = healthpb.NewHealthClient(client).Watch(context.Background(), &healthpb.HealthCheckRequest{})
	require.ErrorContains(t, err, "deadline")
	watchCtx, stopWatch := context.WithCancel(ctx)
	stream, err := healthpb.NewHealthClient(client).Watch(watchCtx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)
	stopWatch()
	require.NoError(t, grpcx.Shutdown(ctx, server))
	_, err = grpcx.NewClient(listener.Addr().String(), nil, provider)
	require.Error(t, err)
}

func TestForcedShutdown(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	provider := tracing.NewProvider("test", "dev", 0)
	defer func() { require.NoError(t, provider.Shutdown(context.Background())) }()
	server, _ := grpcx.NewServer(provider)
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		err := <-served
		require.True(t, err == nil || errors.Is(err, grpc.ErrServerStopped))
	}()
	client, err := grpcx.NewClient(listener.Addr().String(), insecure.NewCredentials(), provider)
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()
	stream, err := healthpb.NewHealthClient(client).Watch(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)
	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer stop()
	require.ErrorIs(t, grpcx.Shutdown(shutdownCtx, server), context.DeadlineExceeded)
	_, err = stream.Recv()
	require.Error(t, err)
}
