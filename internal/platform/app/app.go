// Package app wires the Phase 1 operational shell shared by all commands.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"pulsebid/internal/platform/config"
	"pulsebid/internal/platform/health"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/lifecycle"
	"pulsebid/internal/platform/logging"
	"pulsebid/internal/platform/metrics"
	"pulsebid/internal/platform/tracing"
)

// Run owns the admin server and telemetry workers until ctx is canceled.
// Domain dependencies and public listeners are added in their assigned phases.
func Run(ctx context.Context, cfg config.Common, version string, out io.Writer) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	level, err := cfg.Level()
	if err != nil {
		return err
	}
	logger := logging.New(out, cfg.ServiceName, version, level)
	provider, err := tracing.New(ctx, cfg.ServiceName, version, cfg.OTLPEndpoint, cfg.TraceSampleRatio)
	if err != nil {
		return err
	}
	ready := health.New(nil, time.Second)
	m := metrics.New(cfg.ServiceName)
	server := httpx.NewServer(cfg.AdminAddr, httpx.AdminHandler(ready, logger, m, provider), logger)
	listenCtx, cancelListen := context.WithTimeout(ctx, 5*time.Second)
	listener, err := (&net.ListenConfig{}).Listen(listenCtx, "tcp", cfg.AdminAddr)
	cancelListen()
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		return errors.Join(fmt.Errorf("listen admin: %w", err), provider.Shutdown(closeCtx))
	}
	group := lifecycle.New(ctx)
	group.Go("admin HTTP", func(context.Context) error {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	ready.SetReady(true)
	logger.InfoContext(ctx, "service ready", "admin_addr", listener.Addr().String())
	err = group.Run(func() { ready.Drain(); logger.InfoContext(context.WithoutCancel(ctx), "service draining") }, cfg.ShutdownDelay, cfg.ShutdownTimeout,
		lifecycle.Hook{Name: "admin HTTP", Stop: func(ctx context.Context) error { return httpx.Shutdown(ctx, server) }},
		lifecycle.Hook{Name: "tracing", Stop: provider.Shutdown})
	logger.InfoContext(context.WithoutCancel(ctx), "service stopped")
	return err
}
