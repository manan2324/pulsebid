// Package gateway implements the core bid-gateway logic, including request validation,
// in-memory targeting, bidder fan-out, second-price auction, and response generation.
package gateway

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"pulsebid/internal/platform/config"
)

// Config defines the operational and behavioral parameters of bid-gateway.
type Config struct {
	config.Common
	HTTPAddr                string        `env:"HTTP_ADDR" envDefault:":8080"`
	PublicBaseURL           string        `env:"PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`
	CampaignGRPCAddr        string        `env:"CAMPAIGN_GRPC_ADDR" envDefault:"localhost:8082"`
	SnapshotRefreshInterval time.Duration `env:"SNAPSHOT_REFRESH_INTERVAL" envDefault:"2s"`
	BidderDeadlineMS        int           `env:"BIDDER_DEADLINE_MS" envDefault:"100"`
	RequestMaxBytes         int64         `env:"REQUEST_MAX_BYTES" envDefault:"16384"`
	RedisAddr               string        `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisOpTimeoutMS        int           `env:"REDIS_OP_TIMEOUT_MS" envDefault:"10"`
	PacingBurstFraction     float64       `env:"PACING_BURST_FRACTION" envDefault:"0.02"`
	KafkaBrokers            []string      `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
	KafkaTopicEvents        string        `env:"KAFKA_TOPIC_EVENTS" envDefault:"pulsebid.events.v1"`
	EventBufferSize         int           `env:"EVENT_BUFFER_SIZE" envDefault:"100000"`
	EventTokenSecret        string        `env:"EVENT_TOKEN_SECRET" envDefault:"local-development-only-replace-before-deploying"`
	EventTokenTTL           time.Duration `env:"EVENT_TOKEN_TTL" envDefault:"24h"`
	BidderMaxInflight       int           `env:"BIDDER_MAX_INFLIGHT" envDefault:"512"`
	CBMinRequests           uint32        `env:"CB_MIN_REQUESTS" envDefault:"20"`
	CBFailureRatio          float64       `env:"CB_FAILURE_RATIO" envDefault:"0.5"`
	CBOpenFor               time.Duration `env:"CB_OPEN_FOR" envDefault:"5s"`
	MaxAuctionAttempts      int           `env:"MAX_AUCTION_ATTEMPTS" envDefault:"3"`
}

// LoadConfig parses environment variables into Config with default fallbacks.
func LoadConfig() (Config, error) {
	cfg := Config{
		Common: config.Common{
			ServiceName: "bid-gateway",
			AdminAddr:   ":8090",
		},
	}
	if err := config.Parse(&cfg); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

// Validate ensures all operational parameters are usable.
func (c Config) Validate() error {
	if err := c.Common.Validate(); err != nil {
		return err
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("HTTP_ADDR must be host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("HTTP_ADDR has invalid port")
	}
	if strings.TrimSpace(c.CampaignGRPCAddr) == "" {
		return fmt.Errorf("CAMPAIGN_GRPC_ADDR is required")
	}
	if c.SnapshotRefreshInterval <= 0 {
		return fmt.Errorf("SNAPSHOT_REFRESH_INTERVAL must be positive")
	}
	if c.BidderDeadlineMS <= 0 {
		return fmt.Errorf("BIDDER_DEADLINE_MS must be positive")
	}
	if c.RequestMaxBytes <= 0 {
		return fmt.Errorf("REQUEST_MAX_BYTES must be positive")
	}
	if c.BidderMaxInflight <= 0 {
		return fmt.Errorf("BIDDER_MAX_INFLIGHT must be positive")
	}
	if c.CBFailureRatio < 0 || c.CBFailureRatio > 1 {
		return fmt.Errorf("CB_FAILURE_RATIO must be between 0 and 1")
	}
	if c.CBOpenFor <= 0 {
		return fmt.Errorf("CB_OPEN_FOR must be positive")
	}
	if c.MaxAuctionAttempts <= 0 {
		return fmt.Errorf("MAX_AUCTION_ATTEMPTS must be positive")
	}
	return nil
}
