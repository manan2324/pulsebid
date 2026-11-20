package gateway

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/config"
)

func TestConfig_Validation(t *testing.T) {
	valid := Config{
		Common: config.Common{
			ServiceName:     "bid-gateway",
			LogLevel:        "info",
			AdminAddr:       ":8090",
			OTLPEndpoint:    "localhost:4317",
			ShutdownTimeout: 15 * time.Second,
		},
		HTTPAddr:                ":8080",
		CampaignGRPCAddr:        "localhost:8082",
		SnapshotRefreshInterval: 2 * time.Second,
		BidderDeadlineMS:        100,
		RequestMaxBytes:         16384,
		BidderMaxInflight:       512,
		CBMinRequests:           20,
		CBFailureRatio:          0.5,
		CBOpenFor:               5 * time.Second,
		MaxAuctionAttempts:      3,
	}

	require.NoError(t, valid.Validate())

	// Invalid HTTP port
	badPort := valid
	badPort.HTTPAddr = "invalid"
	assert.Error(t, badPort.Validate())

	// Missing campaign gRPC addr
	badGRPC := valid
	badGRPC.CampaignGRPCAddr = ""
	assert.Error(t, badGRPC.Validate())

	// Non-positive intervals/deadlines
	badInterval := valid
	badInterval.SnapshotRefreshInterval = 0
	assert.Error(t, badInterval.Validate())

	badDeadline := valid
	badDeadline.BidderDeadlineMS = -1
	assert.Error(t, badDeadline.Validate())

	badMaxBytes := valid
	badMaxBytes.RequestMaxBytes = 0
	assert.Error(t, badMaxBytes.Validate())

	badInflight := valid
	badInflight.BidderMaxInflight = 0
	assert.Error(t, badInflight.Validate())

	badRatio := valid
	badRatio.CBFailureRatio = 1.5
	assert.Error(t, badRatio.Validate())

	badCBOpen := valid
	badCBOpen.CBOpenFor = 0
	assert.Error(t, badCBOpen.Validate())

	badAttempts := valid
	badAttempts.MaxAuctionAttempts = 0
	assert.Error(t, badAttempts.Validate())
}

func TestConfig_LoadConfig(t *testing.T) {
	os.Setenv("HTTP_ADDR", ":8888")
	defer os.Unsetenv("HTTP_ADDR")

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, ":8888", cfg.HTTPAddr)
}
