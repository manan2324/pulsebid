package config_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/config"
)

func valid() config.Common {
	return config.Common{ServiceName: "test", LogLevel: "info", AdminAddr: ":8090", OTLPEndpoint: "localhost:4317", TraceSampleRatio: .01, ShutdownDelay: 5 * time.Second, ShutdownTimeout: 15 * time.Second}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	require.NoError(t, valid().Validate())
	for _, tc := range []struct {
		name   string
		change func(*config.Common)
	}{
		{"name", func(c *config.Common) { c.ServiceName = " " }},
		{"level", func(c *config.Common) { c.LogLevel = "unknown" }},
		{"address", func(c *config.Common) { c.AdminAddr = "bad" }},
		{"port", func(c *config.Common) { c.AdminAddr = ":65536" }},
		{"port zero", func(c *config.Common) { c.AdminAddr = ":0" }},
		{"endpoint", func(c *config.Common) { c.OTLPEndpoint = ":4317" }},
		{"ratio negative", func(c *config.Common) { c.TraceSampleRatio = -1 }},
		{"ratio high", func(c *config.Common) { c.TraceSampleRatio = 1.1 }},
		{"ratio nan", func(c *config.Common) { c.TraceSampleRatio = math.NaN() }},
		{"ratio infinite", func(c *config.Common) { c.TraceSampleRatio = math.Inf(1) }},
		{"delay", func(c *config.Common) { c.ShutdownDelay = -1 }},
		{"timeout", func(c *config.Common) { c.ShutdownTimeout = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) { t.Parallel(); cfg := valid(); tc.change(&cfg); require.Error(t, cfg.Validate()) })
	}
}

func TestEnvironment(t *testing.T) {
	// Setenv tests must not run in parallel with each other.
	for key, value := range map[string]string{"SERVICE_NAME": "campaign-service", "ADMIN_ADDR": ":8091", "LOG_LEVEL": "debug", "SHUTDOWN_DELAY": "0s", "SHUTDOWN_TIMEOUT": "2s", "OTEL_EXPORTER_OTLP_ENDPOINT": "localhost:4317", "OTEL_TRACES_SAMPLER_ARG": "1", "ENABLE_TEST_ENDPOINTS": "true"} {
		t.Setenv(key, value)
	}
	cfg, err := config.LoadCommon("fallback", ":8090")
	require.NoError(t, err)
	require.Equal(t, "campaign-service", cfg.ServiceName)
	require.Equal(t, ":8091", cfg.AdminAddr)
	require.Zero(t, cfg.ShutdownDelay)
	require.Equal(t, 2*time.Second, cfg.ShutdownTimeout)
	require.True(t, cfg.EnableTestEndpoints)
	t.Setenv("SHUTDOWN_TIMEOUT", "bad")
	_, err = config.LoadCommon("fallback", ":8090")
	require.Error(t, err)
	t.Setenv("SHUTDOWN_TIMEOUT", "-1s")
	_, err = config.LoadCommon("fallback", ":8090")
	require.Error(t, err)
}
