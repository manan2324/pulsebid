// Package aggregator consumes exchange events into idempotent hourly rollups.
package aggregator

import (
	"fmt"
	"net"
	"strings"
	"time"

	"pulsebid/internal/platform/config"
)

// Config defines configuration parameters for event-aggregator (Section 7.4).
type Config struct {
	config.Common
	HTTPAddr         string        `env:"HTTP_ADDR" envDefault:":8083"`
	DatabaseURL      string        `env:"DATABASE_URL" envDefault:"postgres://pulsebid:pulsebid@localhost:5432/pulsebid?sslmode=disable"`
	KafkaBrokers     []string      `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
	KafkaTopicEvents string        `env:"KAFKA_TOPIC_EVENTS" envDefault:"pulsebid.events.v1"`
	KafkaTopicDLQ    string        `env:"KAFKA_TOPIC_DLQ" envDefault:"pulsebid.events.dlq.v1"`
	ConsumerGroup    string        `env:"CONSUMER_GROUP" envDefault:"pulsebid-aggregator"`
	BatchSize        int           `env:"BATCH_SIZE" envDefault:"500"`
	FlushIntervalMS  int           `env:"FLUSH_INTERVAL_MS" envDefault:"200"`
	DedupRetention   time.Duration `env:"DEDUP_RETENTION" envDefault:"168h"`
	APIKeys          string        `env:"API_KEYS" envDefault:"dev-key"`
}

// LoadConfig parses environment variables into Config with default fallbacks.
func LoadConfig() (Config, error) {
	cfg := Config{
		Common: config.Common{
			ServiceName: "event-aggregator",
			AdminAddr:   ":8093",
		},
	}
	if err := config.Parse(&cfg); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

// Validate ensures all required parameters are non-zero and valid.
func (c Config) Validate() error {
	if err := c.Common.Validate(); err != nil {
		return err
	}
	if _, _, err := net.SplitHostPort(c.HTTPAddr); err != nil {
		return fmt.Errorf("invalid HTTP_ADDR %q: %w", c.HTTPAddr, err)
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	if len(c.KafkaBrokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS must not be empty")
	}
	if strings.TrimSpace(c.KafkaTopicEvents) == "" {
		return fmt.Errorf("KAFKA_TOPIC_EVENTS must not be empty")
	}
	if strings.TrimSpace(c.KafkaTopicDLQ) == "" {
		return fmt.Errorf("KAFKA_TOPIC_DLQ must not be empty")
	}
	if strings.TrimSpace(c.ConsumerGroup) == "" {
		return fmt.Errorf("CONSUMER_GROUP must not be empty")
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("BATCH_SIZE must be positive, got %d", c.BatchSize)
	}
	if c.FlushIntervalMS <= 0 {
		return fmt.Errorf("FLUSH_INTERVAL_MS must be positive, got %d", c.FlushIntervalMS)
	}
	if c.DedupRetention <= 0 {
		return fmt.Errorf("DEDUP_RETENTION must be positive, got %v", c.DedupRetention)
	}
	return nil
}
