package aggregator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfig_LoadAndValidate(t *testing.T) {
	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, ":8083", cfg.HTTPAddr)
	require.Equal(t, ":8093", cfg.Common.AdminAddr)
	require.Equal(t, "pulsebid.events.v1", cfg.KafkaTopicEvents)
	require.Equal(t, "pulsebid.events.dlq.v1", cfg.KafkaTopicDLQ)
	require.Equal(t, "pulsebid-aggregator", cfg.ConsumerGroup)
	require.Equal(t, 500, cfg.BatchSize)
	require.Equal(t, 200, cfg.FlushIntervalMS)
	require.Equal(t, 168*time.Hour, cfg.DedupRetention)
}

func TestConfig_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "invalid HTTP_ADDR",
			mutate: func(c *Config) {
				c.HTTPAddr = "invalid-no-port"
			},
			wantErr: "invalid HTTP_ADDR",
		},
		{
			name: "empty DATABASE_URL",
			mutate: func(c *Config) {
				c.DatabaseURL = ""
			},
			wantErr: "DATABASE_URL must not be empty",
		},
		{
			name: "empty KAFKA_BROKERS",
			mutate: func(c *Config) {
				c.KafkaBrokers = nil
			},
			wantErr: "KAFKA_BROKERS must not be empty",
		},
		{
			name: "empty KAFKA_TOPIC_EVENTS",
			mutate: func(c *Config) {
				c.KafkaTopicEvents = ""
			},
			wantErr: "KAFKA_TOPIC_EVENTS must not be empty",
		},
		{
			name: "empty KAFKA_TOPIC_DLQ",
			mutate: func(c *Config) {
				c.KafkaTopicDLQ = ""
			},
			wantErr: "KAFKA_TOPIC_DLQ must not be empty",
		},
		{
			name: "empty CONSUMER_GROUP",
			mutate: func(c *Config) {
				c.ConsumerGroup = ""
			},
			wantErr: "CONSUMER_GROUP must not be empty",
		},
		{
			name: "negative BATCH_SIZE",
			mutate: func(c *Config) {
				c.BatchSize = 0
			},
			wantErr: "BATCH_SIZE must be positive",
		},
		{
			name: "negative FLUSH_INTERVAL_MS",
			mutate: func(c *Config) {
				c.FlushIntervalMS = -1
			},
			wantErr: "FLUSH_INTERVAL_MS must be positive",
		},
		{
			name: "negative DEDUP_RETENTION",
			mutate: func(c *Config) {
				c.DedupRetention = 0
			},
			wantErr: "DEDUP_RETENTION must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig()
			require.NoError(t, err)
			tt.mutate(&cfg)
			err = cfg.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
