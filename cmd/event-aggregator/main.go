// Command event-aggregator runs the PulseBid event aggregation consumer service (Section 10.5).
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
	"strings"
	"syscall"
	"time"

	"pulsebid/internal/aggregator"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/lifecycle"
	"pulsebid/internal/platform/logging"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"
	"pulsebid/internal/reporting"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
)

func main() {
	if err := run(os.Stdout); err != nil {
		logging.New(os.Stdout, "event-aggregator", "dev", slog.LevelInfo).Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(out io.Writer) (result error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := aggregator.LoadConfig()
	if err != nil {
		return err
	}

	level, err := cfg.Level()
	if err != nil {
		return err
	}
	logger := logging.New(out, cfg.ServiceName, "dev", level)

	provider, err := tracing.New(ctx, cfg.ServiceName, "dev", cfg.OTLPEndpoint, cfg.TraceSampleRatio)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(tracing.Propagator())
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		result = errors.Join(result, provider.Shutdown(cleanup))
	}()

	m := metrics.New("event-aggregator")
	aggMetrics := aggregator.NewMetrics(m.Registry)

	// Connect PostgreSQL store
	store, err := aggregator.NewPGStoreFromURL(ctx, cfg.DatabaseURL, aggMetrics, logger)
	if err != nil {
		return fmt.Errorf("init pg store: %w", err)
	}
	defer store.Close()
	_ = m.RegisterPGX(store.Pool())

	// Initialize Kafka client for consumer group + DLQ producer
	kgoOpts := []kgo.Opt{
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.ConsumerGroup(cfg.ConsumerGroup),
		kgo.ConsumeTopics(cfg.KafkaTopicEvents),
		kgo.DisableAutoCommit(),
		kgo.FetchMaxWait(time.Duration(cfg.FlushIntervalMS) * time.Millisecond),
	}
	cl, err := kgo.NewClient(kgoOpts...)
	if err != nil {
		return fmt.Errorf("create kafka consumer client: %w", err)
	}
	defer cl.Close()

	dlq := aggregator.NewKafkaDLQProducer(cl, cfg.KafkaTopicDLQ, aggMetrics, logger)
	consumer := aggregator.NewConsumer(cl, store, dlq, cfg, aggMetrics, logger)
	retentionJob := aggregator.NewRetentionJob(store, cfg.DedupRetention, time.Hour, logger)

	reportingRepo := reporting.NewPostgresRepository(store.Pool())
	reportingSvc := reporting.NewService(reportingRepo)
	apiKeys := strings.Split(cfg.APIKeys, ",")

	mux := aggregator.NewServer(reportingSvc, apiKeys)
	apiServer := httpx.NewServer(cfg.HTTPAddr, aggregator.Stack(mux, logger, m, provider), logger)
	apiServer.WriteTimeout = 15 * time.Second

	ready := health.New(map[string]health.Check{
		"postgres": store.Ping,
		"kafka": func(ctx context.Context) error {
			return cl.Ping(ctx)
		},
	}, time.Second)
	adminServer := httpx.NewServer(cfg.AdminAddr, httpx.AdminHandler(ready, logger, m, provider), logger)

	listenCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	apiListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen API: %w", err)
	}

	adminListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.AdminAddr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen admin: %w", err), apiListener.Close())
	}

	group := lifecycle.New(ctx)
	group.Go("kafka consumer", consumer.Run)
	group.Go("retention worker", retentionJob.Run)

	for _, pair := range []struct {
		name     string
		server   *http.Server
		listener net.Listener
	}{
		{"aggregator API", apiServer, apiListener},
		{"aggregator admin", adminServer, adminListener},
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
	logger.InfoContext(ctx, "event-aggregator ready",
		"http_addr", cfg.HTTPAddr,
		"admin_addr", cfg.AdminAddr,
		"kafka_brokers", cfg.KafkaBrokers,
		"topic", cfg.KafkaTopicEvents,
	)

	result = group.Run(func() {
		ready.Drain()
		logger.InfoContext(context.WithoutCancel(ctx), "event-aggregator draining")
	}, cfg.ShutdownDelay, cfg.ShutdownTimeout,
		lifecycle.Hook{Name: "API", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, apiServer) }},
		lifecycle.Hook{Name: "admin", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, adminServer) }},
		lifecycle.Hook{Name: "kafka", Stop: func(_ context.Context) error {
			cl.Close()
			return nil
		}},
		lifecycle.Hook{Name: "tracing", Stop: provider.Shutdown},
	)

	logger.InfoContext(context.WithoutCancel(ctx), "event-aggregator stopped")
	return result
}
