package bidder

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Supported bid modes.
const (
	BidModeFixed  = "fixed"
	BidModeShade  = "shade"
	BidModeRandom = "random"
)

// ErrInvalidConfig is returned when runtime configuration fails validation.
var ErrInvalidConfig = errors.New("invalid bidder configuration")

// BidRequest represents the JSON payload sent by the gateway to POST /bid.
type BidRequest struct {
	RequestID  string      `json:"request_id"`
	TmaxMS     int         `json:"tmax_ms"`
	Imp        Impression  `json:"imp"`
	Device     Device      `json:"device"`
	Geo        Geo         `json:"geo"`
	Candidates []Candidate `json:"candidates"`
}

// Impression describes the ad slot to be filled.
type Impression struct {
	ID          string  `json:"id"`
	PlacementID string  `json:"placement_id"`
	W           int     `json:"w"`
	H           int     `json:"h"`
	Bidfloor    float64 `json:"bidfloor"`
}

// Device describes the user agent device.
type Device struct {
	Type string `json:"type"`
	OS   string `json:"os"`
}

// Geo contains geographical context.
type Geo struct {
	Country string `json:"country"`
}

// Candidate represents a campaign/creative candidate nominated by gateway targeting.
type Candidate struct {
	CampaignID string  `json:"campaign_id"`
	CreativeID string  `json:"creative_id"`
	MaxBidCPM  float64 `json:"max_bid_cpm"`
}

// BidResponse represents the JSON response returned by POST /bid on success (200).
type BidResponse struct {
	Bids []Bid `json:"bids"`
}

// Bid represents a single price offer for a candidate.
type Bid struct {
	CampaignID string  `json:"campaign_id"`
	CreativeID string  `json:"creative_id"`
	Price      float64 `json:"price"`
}

// Config controls simulated DSP behavior at runtime.
type Config struct {
	LatencyMS   int     `json:"latency_ms"`
	JitterMS    int     `json:"jitter_ms"`
	ErrorRate   float64 `json:"error_rate"`
	TimeoutRate float64 `json:"timeout_rate"`
	NoBidRate   float64 `json:"no_bid_rate"`
	BidMode     string  `json:"bid_mode"`
	BidShade    float64 `json:"bid_shade"`
	Seed        int64   `json:"seed"`
}

// ConfigPatch represents a partial update sent to POST /admin/config.
type ConfigPatch struct {
	LatencyMS   *int     `json:"latency_ms,omitempty"`
	JitterMS    *int     `json:"jitter_ms,omitempty"`
	ErrorRate   *float64 `json:"error_rate,omitempty"`
	TimeoutRate *float64 `json:"timeout_rate,omitempty"`
	NoBidRate   *float64 `json:"no_bid_rate,omitempty"`
	BidMode     *string  `json:"bid_mode,omitempty"`
	BidShade    *float64 `json:"bid_shade,omitempty"`
	Seed        *int64   `json:"seed,omitempty"`
}

// DefaultConfig returns the Section 7.5 initial defaults for a mock bidder.
func DefaultConfig() Config {
	return Config{
		LatencyMS:   10,
		JitterMS:    15,
		ErrorRate:   0,
		TimeoutRate: 0,
		NoBidRate:   0.05,
		BidMode:     BidModeFixed,
		BidShade:    0.9,
		Seed:        1,
	}
}

// Validate ensures all configuration parameters fall within acceptable bounds.
func (c Config) Validate() error {
	if c.LatencyMS < 0 {
		return fmt.Errorf("%w: latency_ms must be non-negative", ErrInvalidConfig)
	}
	if c.JitterMS < 0 {
		return fmt.Errorf("%w: jitter_ms must be non-negative", ErrInvalidConfig)
	}
	if c.ErrorRate < 0 || c.ErrorRate > 1 || math.IsNaN(c.ErrorRate) {
		return fmt.Errorf("%w: error_rate must be between 0.0 and 1.0", ErrInvalidConfig)
	}
	if c.TimeoutRate < 0 || c.TimeoutRate > 1 || math.IsNaN(c.TimeoutRate) {
		return fmt.Errorf("%w: timeout_rate must be between 0.0 and 1.0", ErrInvalidConfig)
	}
	if c.NoBidRate < 0 || c.NoBidRate > 1 || math.IsNaN(c.NoBidRate) {
		return fmt.Errorf("%w: no_bid_rate must be between 0.0 and 1.0", ErrInvalidConfig)
	}
	mode := strings.ToLower(strings.TrimSpace(c.BidMode))
	switch mode {
	case BidModeFixed, BidModeShade, BidModeRandom:
	default:
		return fmt.Errorf("%w: bid_mode must be fixed, shade, or random", ErrInvalidConfig)
	}
	if c.BidShade <= 0 || c.BidShade > 1 || math.IsNaN(c.BidShade) {
		return fmt.Errorf("%w: bid_shade must be between 0.0 and 1.0", ErrInvalidConfig)
	}
	return nil
}

// Apply applies a partial patch to the configuration.
func (c Config) Apply(patch ConfigPatch) (Config, error) {
	updated := c
	if patch.LatencyMS != nil {
		updated.LatencyMS = *patch.LatencyMS
	}
	if patch.JitterMS != nil {
		updated.JitterMS = *patch.JitterMS
	}
	if patch.ErrorRate != nil {
		updated.ErrorRate = *patch.ErrorRate
	}
	if patch.TimeoutRate != nil {
		updated.TimeoutRate = *patch.TimeoutRate
	}
	if patch.NoBidRate != nil {
		updated.NoBidRate = *patch.NoBidRate
	}
	if patch.BidMode != nil {
		updated.BidMode = strings.ToLower(strings.TrimSpace(*patch.BidMode))
	}
	if patch.BidShade != nil {
		updated.BidShade = *patch.BidShade
	}
	if patch.Seed != nil {
		updated.Seed = *patch.Seed
	}
	if err := updated.Validate(); err != nil {
		return c, err
	}
	return updated, nil
}
