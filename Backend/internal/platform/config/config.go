// Package config loads and validates service environment configuration.
package config

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Common contains the configuration shared by all commands.
type Common struct {
	ServiceName         string        `env:"SERVICE_NAME"`
	LogLevel            string        `env:"LOG_LEVEL" envDefault:"info"`
	AdminAddr           string        `env:"ADMIN_ADDR"`
	OTLPEndpoint        string        `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"localhost:4317"`
	TraceSampleRatio    float64       `env:"OTEL_TRACES_SAMPLER_ARG" envDefault:"0.01"`
	ShutdownDelay       time.Duration `env:"SHUTDOWN_DELAY" envDefault:"5s"`
	ShutdownTimeout     time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
	EnableTestEndpoints bool          `env:"ENABLE_TEST_ENDPOINTS" envDefault:"false"`
}

// Parse populates any service config using the same environment parser.
func Parse(target any) error {
	if err := env.Parse(target); err != nil {
		return fmt.Errorf("parse environment: %w", err)
	}
	return nil
}

// LoadCommon applies command-specific defaults before parsing environment values.
func LoadCommon(service, adminAddr string) (Common, error) {
	cfg := Common{ServiceName: service, AdminAddr: adminAddr}
	if err := Parse(&cfg); err != nil {
		return Common{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Common{}, err
	}
	return cfg, nil
}

// Validate fails startup on invalid operational settings.
func (c Common) Validate() error {
	if strings.TrimSpace(c.ServiceName) == "" {
		return fmt.Errorf("SERVICE_NAME must not be empty")
	}
	if _, err := c.Level(); err != nil {
		return err
	}
	if err := validateAddress(c.AdminAddr, false); err != nil {
		return fmt.Errorf("ADMIN_ADDR: %w", err)
	}
	if err := validateAddress(c.OTLPEndpoint, true); err != nil {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT: %w", err)
	}
	if math.IsNaN(c.TraceSampleRatio) || math.IsInf(c.TraceSampleRatio, 0) || c.TraceSampleRatio < 0 || c.TraceSampleRatio > 1 {
		return fmt.Errorf("OTEL_TRACES_SAMPLER_ARG must be between 0 and 1")
	}
	if c.ShutdownDelay < 0 {
		return fmt.Errorf("SHUTDOWN_DELAY must be nonnegative")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}

// Level returns the configured structured log threshold.
func (c Common) Level() (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	return level, nil
}

func validateAddress(address string, requireHost bool) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("expected host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 || (requireHost && strings.TrimSpace(host) == "") {
		return fmt.Errorf("invalid host or port")
	}
	return nil
}
