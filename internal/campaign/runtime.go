package campaign

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/budget"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/config"
	"pulsebid/internal/platform/grpcx"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/lifecycle"
	"pulsebid/internal/platform/logging"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"

	"go.opentelemetry.io/otel"
)

// Config holds campaign-specific settings; secrets are never logged.
type Config struct {
	HTTPAddr            string   `env:"HTTP_ADDR" envDefault:":8081"`
	GRPCAddr            string   `env:"GRPC_ADDR" envDefault:":8082"`
	DatabaseURL         string   `env:"DATABASE_URL" envDefault:"postgres://pulsebid:pulsebid@localhost:5432/pulsebid?sslmode=disable"`
	APIKeys             []string `env:"API_KEYS" envDefault:"dev-key" envSeparator:","`
	RedisAddr           string   `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	EnableTestEndpoints bool     `env:"ENABLE_TEST_ENDPOINTS" envDefault:"false"`
}

// LoadConfig parses campaign-specific settings without reading a dotenv file.
func LoadConfig() (Config, error) {
	var c Config
	if err := config.Parse(&c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

// Validate rejects unusable listeners and an empty authentication configuration.
func (c Config) Validate() error {
	for _, listener := range []struct{ name, address string }{{"HTTP_ADDR", c.HTTPAddr}, {"GRPC_ADDR", c.GRPCAddr}} {
		_, port, err := net.SplitHostPort(listener.address)
		if err != nil {
			return fmt.Errorf("%s must be host:port", listener.name)
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("%s has an invalid port", listener.name)
		}
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	for _, key := range c.APIKeys {
		if strings.TrimSpace(key) != "" {
			return nil
		}
	}
	return fmt.Errorf("API_KEYS must contain at least one nonempty key")
}

// Run hosts REST, internal gRPC, and the DB-aware admin listener.
func Run(ctx context.Context, common config.Common, cfg Config, out io.Writer) (result error) {
	if err := common.Validate(); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	level, err := common.Level()
	if err != nil {
		return err
	}
	logger := logging.New(out, common.ServiceName, "dev", level)
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("invalid DATABASE_URL")
	}
	poolCfg.ConnConfig.ConnectTimeout = 5 * time.Second
	poolCfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolCfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	poolCfg.ConnConfig.RuntimeParams["lock_timeout"] = "4000"
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("initialize database pool: %w", err)
	}
	defer pool.Close()
	provider, err := tracing.New(ctx, common.ServiceName, "dev", common.OTLPEndpoint, common.TraceSampleRatio)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(tracing.Propagator())
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), common.ShutdownTimeout)
		defer cancel()
		result = errors.Join(result, provider.Shutdown(cleanup))
	}()
	m := metrics.New(common.ServiceName)
	if err = m.RegisterPGX(pool); err != nil {
		return err
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	defer rdb.Close()
	_ = m.RegisterRedis(rdb)
	budgetReserver := budget.NewRedisReserver(rdb, 100*time.Millisecond, 0.05, nil)

	ready := health.New(map[string]health.Check{
		"postgres": pool.Ping,
		"redis": func(ctx context.Context) error {
			return rdb.Ping(ctx).Err()
		},
	}, time.Second)
	store := NewStore(pool)
	service := NewService(store).WithBudget(budgetReserver, cfg.EnableTestEndpoints)
	snapshots := NewSnapshotService(store, clock.Real{})
	mux := service.Handler(cfg.APIKeys)
	mux.Handle("GET /v1/snapshot", httpx.APIKey(cfg.APIKeys)(snapshots.Handler()))
	api := httpx.NewServer(cfg.HTTPAddr, httpx.Stack(mux, logger, m, provider), logger)
	api.WriteTimeout = 15 * time.Second
	admin := httpx.NewServer(common.AdminAddr, httpx.AdminHandler(ready, logger, m, provider), logger)
	listenCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	apiListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen API: %w", err)
	}
	adminListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", common.AdminAddr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen admin: %w", err), apiListener.Close())
	}
	grpcListener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.GRPCAddr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen gRPC: %w", err), apiListener.Close(), adminListener.Close())
	}
	grpcServer, grpcHealth := grpcx.NewServer(provider)
	catalogv1.RegisterCatalogServiceServer(grpcServer, NewCatalogServer(snapshots))
	reflection.Register(grpcServer)
	group := lifecycle.New(ctx)
	group.Go("catalog gRPC", func(context.Context) error {
		err := grpcServer.Serve(grpcListener)
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	})
	for _, pair := range []struct {
		name     string
		server   *http.Server
		listener net.Listener
	}{{"campaign API", api, apiListener}, {"campaign admin", admin, adminListener}} {
		group.Go(pair.name, func(context.Context) error {
			err := pair.server.Serve(pair.listener)
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})
	}
	ready.SetReady(true)
	group.Go("gRPC health", func(ctx context.Context) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			state := healthpb.HealthCheckResponse_SERVING
			if err := ready.Check(ctx); err != nil {
				state = healthpb.HealthCheckResponse_NOT_SERVING
			}
			grpcHealth.SetServingStatus("", state)
			grpcHealth.SetServingStatus(catalogv1.CatalogService_ServiceDesc.ServiceName, state)
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}
	})
	logger.InfoContext(ctx, "service ready", "http_addr", cfg.HTTPAddr, "grpc_addr", cfg.GRPCAddr, "admin_addr", common.AdminAddr)
	result = group.Run(func() {
		ready.Drain()
		grpcHealth.Shutdown()
		logger.InfoContext(context.WithoutCancel(ctx), "service draining")
	}, common.ShutdownDelay, common.ShutdownTimeout,
		lifecycle.Hook{Name: "API", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, api) }},
		lifecycle.Hook{Name: "admin", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, admin) }},
		lifecycle.Hook{Name: "gRPC", Stop: func(ctx context.Context) error { return grpcx.Shutdown(ctx, grpcServer) }},
		lifecycle.Hook{Name: "redis", Stop: func(context.Context) error { return rdb.Close() }},
		lifecycle.Hook{Name: "postgres", Stop: func(context.Context) error { pool.Close(); return nil }},
		lifecycle.Hook{Name: "tracing", Stop: provider.Shutdown})
	logger.InfoContext(context.WithoutCancel(ctx), "service stopped")
	return result
}
