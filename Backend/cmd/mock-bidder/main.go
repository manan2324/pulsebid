// Command mock-bidder runs simulated DSP bidders for testing and load tests.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pulsebid/internal/bidder"
	"pulsebid/internal/platform/config"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/lifecycle"
	"pulsebid/internal/platform/logging"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"

	"go.opentelemetry.io/otel"
)

func main() {
	if err := run(); err != nil {
		logging.New(os.Stdout, "mock-bidder", "dev", slog.LevelInfo).Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var countFlag int
	var basePortFlag int
	var adminAddrFlag string

	flag.IntVar(&countFlag, "count", 0, "number of mock bidders to run")
	flag.IntVar(&basePortFlag, "base-port", 0, "starting port for mock bidders")
	flag.StringVar(&adminAddrFlag, "admin-addr", "", "address for mock-bidder admin listener")
	flag.Parse()

	count := 8
	if countFlag > 0 {
		count = countFlag
	} else if envCount := os.Getenv("BIDDER_COUNT"); envCount != "" {
		if c, err := strconv.Atoi(envCount); err == nil && c > 0 {
			count = c
		}
	}

	basePort := 9101
	if basePortFlag > 0 {
		basePort = basePortFlag
	} else if envBase := os.Getenv("BIDDER_BASE_PORT"); envBase != "" {
		if p, err := strconv.Atoi(envBase); err == nil && p > 0 {
			basePort = p
		}
	}

	adminAddr := ":8094"
	if adminAddrFlag != "" {
		adminAddr = adminAddrFlag
	} else if envAdmin := os.Getenv("ADMIN_ADDR"); envAdmin != "" {
		adminAddr = envAdmin
	}

	common, err := config.LoadCommon("mock-bidder", adminAddr)
	if err != nil {
		return fmt.Errorf("load common config: %w", err)
	}

	level, err := common.Level()
	if err != nil {
		return err
	}
	logger := logging.New(os.Stdout, common.ServiceName, "dev", level)

	provider, err := tracing.New(ctx, common.ServiceName, "dev", common.OTLPEndpoint, common.TraceSampleRatio)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(tracing.Propagator())

	m := metrics.New(common.ServiceName)
	ready := health.New(nil, time.Second)

	initialCfg := bidder.DefaultConfig()
	loadEnvConfig(&initialCfg)

	listenCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type bidderInstance struct {
		server   *bidder.Server
		httpSrv  *http.Server
		listener net.Listener
	}

	instances := make([]bidderInstance, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("bidder-%d", i+1)
		port := basePort + i
		addr := fmt.Sprintf(":%d", port)

		bServer, err := bidder.NewServer(id, port, initialCfg, logger)
		if err != nil {
			return fmt.Errorf("initialize bidder %s: %w", id, err)
		}

		ln, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", addr)
		if err != nil {
			// Clean up already opened listeners
			for _, inst := range instances {
				_ = inst.listener.Close()
			}
			return fmt.Errorf("listen bidder %s on %s: %w", id, addr, err)
		}

		httpSrv := httpx.NewServer(addr, tracing.HTTP(provider)(bServer.Handler()), logger)
		instances = append(instances, bidderInstance{
			server:   bServer,
			httpSrv:  httpSrv,
			listener: ln,
		})
	}

	// Admin listener for platform smoke testing and centralized observability
	adminLn, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", common.AdminAddr)
	if err != nil {
		for _, inst := range instances {
			_ = inst.listener.Close()
		}
		return fmt.Errorf("listen admin %s: %w", common.AdminAddr, err)
	}
	adminSrv := httpx.NewServer(common.AdminAddr, httpx.AdminHandler(ready, logger, m, provider), logger)

	group := lifecycle.New(ctx)

	for _, inst := range instances {
		inst := inst
		group.Go(inst.server.ID(), func(context.Context) error {
			err := inst.httpSrv.Serve(inst.listener)
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})
	}

	group.Go("admin", func(context.Context) error {
		err := adminSrv.Serve(adminLn)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	ready.SetReady(true)
	logger.InfoContext(ctx, "service ready",
		"bidders_count", count,
		"base_port", basePort,
		"admin_addr", common.AdminAddr,
	)

	hooks := make([]lifecycle.Hook, 0, len(instances)+2)
	for _, inst := range instances {
		inst := inst
		hooks = append(hooks, lifecycle.Hook{
			Name: inst.server.ID(),
			Stop: func(ctx context.Context) error {
				return httpx.Shutdown(ctx, inst.httpSrv)
			},
		})
	}
	hooks = append(hooks, lifecycle.Hook{
		Name: "admin",
		Stop: func(ctx context.Context) error {
			return httpx.Shutdown(ctx, adminSrv)
		},
	})
	hooks = append(hooks, lifecycle.Hook{
		Name: "tracing",
		Stop: provider.Shutdown,
	})

	err = group.Run(func() {
		ready.Drain()
		for _, inst := range instances {
			inst.server.SetDraining(true)
		}
		logger.InfoContext(context.WithoutCancel(ctx), "service draining")
	}, common.ShutdownDelay, common.ShutdownTimeout, hooks...)

	logger.InfoContext(context.WithoutCancel(ctx), "service stopped")
	return err
}

func loadEnvConfig(cfg *bidder.Config) {
	if val := os.Getenv("LATENCY_MS"); val != "" {
		if v, err := strconv.Atoi(val); err == nil && v >= 0 {
			cfg.LatencyMS = v
		}
	}
	if val := os.Getenv("JITTER_MS"); val != "" {
		if v, err := strconv.Atoi(val); err == nil && v >= 0 {
			cfg.JitterMS = v
		}
	}
	if val := os.Getenv("ERROR_RATE"); val != "" {
		if v, err := strconv.ParseFloat(val, 64); err == nil && v >= 0 && v <= 1 {
			cfg.ErrorRate = v
		}
	}
	if val := os.Getenv("TIMEOUT_RATE"); val != "" {
		if v, err := strconv.ParseFloat(val, 64); err == nil && v >= 0 && v <= 1 {
			cfg.TimeoutRate = v
		}
	}
	if val := os.Getenv("NO_BID_RATE"); val != "" {
		if v, err := strconv.ParseFloat(val, 64); err == nil && v >= 0 && v <= 1 {
			cfg.NoBidRate = v
		}
	}
	if val := os.Getenv("BID_MODE"); val != "" {
		v := strings.ToLower(strings.TrimSpace(val))
		if v == bidder.BidModeFixed || v == bidder.BidModeShade || v == bidder.BidModeRandom {
			cfg.BidMode = v
		}
	}
	if val := os.Getenv("BID_SHADE"); val != "" {
		if v, err := strconv.ParseFloat(val, 64); err == nil && v > 0 && v <= 1 {
			cfg.BidShade = v
		}
	}
	if val := os.Getenv("SEED"); val != "" {
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.Seed = v
		}
	}
}
