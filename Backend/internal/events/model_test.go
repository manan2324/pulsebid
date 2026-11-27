package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEventID_Deterministic(t *testing.T) {
	bidID := "0193f0a2-1234-7000-8000-000000000001"

	id1 := EventID(TypeImpression, bidID)
	id2 := EventID(TypeImpression, bidID)
	require.Equal(t, id1, id2, "same event type and bid_id must produce identical event_id")

	parsed, err := uuid.Parse(id1)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(5), parsed.Version(), "must be UUIDv5")

	// Different event type on same bid_id produces distinct event_id
	clickID := EventID(TypeClick, bidID)
	require.NotEqual(t, id1, clickID)

	// Different bid_id produces distinct event_id
	otherBidID := "0193f0a2-1234-7000-8000-000000000002"
	otherImpID := EventID(TypeImpression, otherBidID)
	require.NotEqual(t, id1, otherImpID)
}

func TestEvent_JSONSerialization(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 15, 30, 123000000, time.UTC)
	bidID := "0193f0a2-1234-7000-8000-000000000001"

	ev := Event{
		SchemaVersion:    CurrentSchemaVersion,
		EventID:          EventID(TypeImpression, bidID),
		Type:             TypeImpression,
		OccurredAt:       now,
		RequestID:        "req-123",
		BidID:            bidID,
		CampaignID:       "camp-1",
		CreativeID:       "cr-1",
		AdvertiserID:     "adv-1",
		BidderID:         "bidder-1",
		SpendMicros:      1510,
		Participants:     []string{"camp-1", "camp-2"},
		WinnerCampaignID: "camp-1",
	}

	data, err := json.Marshal(ev)
	require.NoError(t, err)

	var unmarshaled map[string]any
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	require.Equal(t, float64(1), unmarshaled["schema_version"])
	require.Equal(t, ev.EventID, unmarshaled["event_id"])
	require.Equal(t, "impression", unmarshaled["type"])
	require.Equal(t, "req-123", unmarshaled["request_id"])
	require.Equal(t, float64(1510), unmarshaled["spend_micros"])
	require.Equal(t, "camp-1", unmarshaled["winner_campaign_id"])
}
