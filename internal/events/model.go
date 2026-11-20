package events

import (
	"time"

	"github.com/google/uuid"
)

// Fixed namespace UUID for PulseBid deterministic event UUIDv5 generation (ADR-5).
var NamespacePulseBid = uuid.MustParse("0193f000-0000-7000-8000-000000000000")

const (
	TypeImpression = "impression"
	TypeClick      = "click"
	TypeAuction    = "auction"

	CurrentSchemaVersion = 1
)

// Event is the canonical event structure emitted to Kafka (Section 10.2).
type Event struct {
	SchemaVersion    int       `json:"schema_version"`
	EventID          string    `json:"event_id"`
	Type             string    `json:"type"`
	OccurredAt       time.Time `json:"occurred_at"`
	RequestID        string    `json:"request_id"`
	BidID            string    `json:"bid_id"`
	CampaignID       string    `json:"campaign_id,omitempty"`
	CreativeID       string    `json:"creative_id,omitempty"`
	AdvertiserID     string    `json:"advertiser_id,omitempty"`
	BidderID         string    `json:"bidder_id,omitempty"`
	SpendMicros      int64     `json:"spend_micros"`
	Participants     []string  `json:"participants,omitempty"`
	WinnerCampaignID string    `json:"winner_campaign_id,omitempty"`
}

// EventID computes a deterministic UUIDv5 using NamespacePulseBid over "<type>:<bid_id>".
// Duplicate pixel fires or Kafka redeliveries yield identical event IDs (ADR-5).
func EventID(eventType, bidID string) string {
	name := eventType + ":" + bidID
	return uuid.NewSHA1(NamespacePulseBid, []byte(name)).String()
}
