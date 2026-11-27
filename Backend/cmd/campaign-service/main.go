// Command campaign-service runs the PulseBid campaign-service process.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"pulsebid/internal/campaign"
	"pulsebid/internal/platform/config"
	"pulsebid/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		logging.New(os.Stdout, "campaign-service", "dev", slog.LevelInfo).Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadCommon("campaign-service", ":8091")
	if err != nil {
		return err
	}
	serviceCfg, err := campaign.LoadConfig()
	if err != nil {
		return err
	}
	return campaign.Run(ctx, cfg, serviceCfg, os.Stdout)
}
