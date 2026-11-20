// Command bid-gateway runs the PulseBid bid-gateway service.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/redis/go-redis/v9"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/budget"
	"pulsebid/internal/events"
	"pulsebid/internal/gateway"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/grpcx"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/lifecycle"
	"pulsebid/internal/platform/logging"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"
)

func main() {
	if err := run(os.Stdout); err != nil {
		logging.New(os.Stdout, "bid-gateway", "dev", slog.LevelInfo).Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(out io.Writer) (result error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := gateway.LoadConfig()
	if err != nil {
		return err
	}

	level, err := cfg.Common.Level()
	if err != nil {
		return err
	}
	logger := logging.New(out, cfg.Common.ServiceName, "dev", level)

	provider, err := tracing.New(ctx, cfg.Common.ServiceName, "dev", cfg.Common.OTLPEndpoint, cfg.Common.TraceSampleRatio)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(tracing.Propagator())
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Common.ShutdownTimeout)
		defer cancel()
		result = errors.Join(result, provider.Shutdown(cleanup))
	}()

	m := metrics.New(cfg.Common.ServiceName)
	gwMetrics := gateway.NewMetrics(m.Registry)

	grpcConn, err := grpcx.NewClient(cfg.CampaignGRPCAddr, insecure.NewCredentials(), provider)
	if err != nil {
		return fmt.Errorf("connect to campaign-service gRPC: %w", err)
	}
	defer grpcConn.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		PoolSize:     512,
		MinIdleConns: 64,
		PoolTimeout:  500 * time.Millisecond,
	})
	defer rdb.Close()
	_ = m.RegisterRedis(rdb)

	budgetReserver := budget.NewRedisReserver(
		rdb,
		time.Duration(cfg.RedisOpTimeoutMS)*time.Millisecond,
		cfg.PacingBurstFraction,
		m.Registry,
	)

	pubCfg := events.PublisherConfig{
		Brokers:          cfg.KafkaBrokers,
		Topic:            cfg.KafkaTopicEvents,
		MaxBuffered:      cfg.EventBufferSize,
		AsyncChannelSize: 10000,
		Linger:           5 * time.Millisecond,
		BatchMaxBytes:    1024 * 1024,
		DeliveryTimeout:  30 * time.Second,
	}
	eventPublisher, err := events.NewKafkaPublisher(pubCfg, m.Registry, logger)
	if err != nil {
		return fmt.Errorf("create kafka event publisher: %w", err)
	}
	defer eventPublisher.Close()

	catalogClient := catalogv1.NewCatalogServiceClient(grpcConn)
	snapshotCache := gateway.NewSnapshotCache(gwMetrics, logger, clock.Real{})
	fanout := gateway.NewFanout(cfg, gwMetrics, logger, nil)
	bidHandler := gateway.NewHandler(cfg, snapshotCache, fanout, budgetReserver, eventPublisher, gwMetrics, logger, clock.Real{})

	mux := gateway.NewServer(bidHandler)
	apiServer := httpx.NewServer(cfg.HTTPAddr, gateway.Stack(mux, logger, m, provider), logger)
	apiServer.WriteTimeout = 15 * time.Second

	ready := health.New(map[string]health.Check{
		"snapshot": snapshotCache.CheckReady,
		"redis": func(ctx context.Context) error {
			return rdb.Ping(ctx).Err()
		},
	}, time.Second)
	adminServer := httpx.NewServer(cfg.Common.AdminAddr, httpx.AdminHandler(ready, logger, m, provider), logger)

	listenCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	apiListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen API: %w", err)
	}

	adminListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.Common.AdminAddr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen admin: %w", err), apiListener.Close())
	}

	group := lifecycle.New(ctx)
	group.Go("snapshot refresher", func(ctx context.Context) error {
		return snapshotCache.RunRefresher(ctx, catalogClient, cfg.SnapshotRefreshInterval)
	})

	for _, pair := range []struct {
		name     string
		server   *http.Server
		listener net.Listener
	}{
		{"gateway API", apiServer, apiListener},
		{"gateway admin", adminServer, adminListener},
	} {
		group.Go(pair.name, func(context.Context) error {
			err := pair.server.Serve(pair.listener)
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})
	}

	ready.SetReady(true)
	logger.InfoContext(ctx, "gateway ready", "http_addr", cfg.HTTPAddr, "admin_addr", cfg.Common.AdminAddr, "campaign_grpc", cfg.CampaignGRPCAddr)

	result = group.Run(func() {
		ready.Drain()
		logger.InfoContext(context.WithoutCancel(ctx), "gateway draining")
	}, cfg.Common.ShutdownDelay, cfg.Common.ShutdownTimeout,
		lifecycle.Hook{Name: "API", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, apiServer) }},
		lifecycle.Hook{Name: "admin", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, adminServer) }},
		lifecycle.Hook{Name: "grpc_conn", Stop: func(context.Context) error { return grpcConn.Close() }},
		lifecycle.Hook{Name: "redis", Stop: func(context.Context) error { return rdb.Close() }},
		lifecycle.Hook{Name: "tracing", Stop: provider.Shutdown})

	logger.InfoContext(context.WithoutCancel(ctx), "gateway stopped")
	return result
}
