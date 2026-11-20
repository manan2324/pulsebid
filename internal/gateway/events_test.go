package gateway

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/events"
	"pulsebid/internal/events/token"
	"pulsebid/internal/platform/clock"
)

type capturingEventPublisher struct {
	mu           sync.Mutex
	published    []*events.Event
	publishError error
}

func (c *capturingEventPublisher) Publish(ctx context.Context, ev *events.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.publishError != nil {
		return c.publishError
	}
	c.published = append(c.published, ev)
	return nil
}

func (c *capturingEventPublisher) TryPublishAsync(ev *events.Event) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.published = append(c.published, ev)
	return true
}

func (c *capturingEventPublisher) Flush(context.Context) error { return nil }
func (c *capturingEventPublisher) Close() error                { return nil }

func setupEventsGateway(t *testing.T, pub EventPublisher, enableTest bool) *Handler {
	cfg := Config{
		HTTPAddr:            ":8080",
		PublicBaseURL:       "http://localhost:8080",
		EventTokenSecret:    "test-secret-key-for-events-gateway-1234",
		EventTokenTTL:       24 * time.Hour,
		EnableTestEndpoints: enableTest,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	clk := clock.Real{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	snapCache := NewSnapshotCache(m, logger, clk)
	fanout := NewFanout(cfg, m, logger, nil)
	return NewHandler(cfg, snapCache, fanout, StubReserver{}, pub, m, logger, clk)
}

func TestEvents_Impression_Success(t *testing.T) {
	pub := &capturingEventPublisher{}
	h := setupEventsGateway(t, pub, true)

	now := time.Now().UTC()
	payload := token.Payload{
		BidID:        "0193f0a2-1234-7000-8000-000000000001",
		RequestID:    "req-imp-1",
		CampaignID:   "camp-1",
		CreativeID:   "cr-1",
		AdvertiserID: "adv-1",
		BidderID:     "bidder-1",
		PriceMicros:  1510,
		LandingURL:   "https://example.com/target",
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(24 * time.Hour).Unix(),
	}

	tok, err := token.Sign([]byte(h.cfg.EventTokenSecret), payload)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/v1/events/impression?t="+tok, nil)
	rec := httptest.NewRecorder()

	h.HandleImpression(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "image/gif", res.Header.Get("Content-Type"))
	assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
	assert.Equal(t, transparent1x1GIF, rec.Body.Bytes())

	require.Len(t, pub.published, 1)
	ev := pub.published[0]
	assert.Equal(t, events.TypeImpression, ev.Type)
	assert.Equal(t, events.EventID(events.TypeImpression, payload.BidID), ev.EventID)
	assert.Equal(t, int64(1510), ev.SpendMicros)
	assert.Equal(t, "req-imp-1", ev.RequestID)
	assert.Equal(t, "camp-1", ev.CampaignID)
}

func TestEvents_Click_Success(t *testing.T) {
	pub := &capturingEventPublisher{}
	h := setupEventsGateway(t, pub, true)

	now := time.Now().UTC()
	payload := token.Payload{
		BidID:        "0193f0a2-1234-7000-8000-000000000002",
		RequestID:    "req-click-1",
		CampaignID:   "camp-2",
		CreativeID:   "cr-2",
		AdvertiserID: "adv-2",
		BidderID:     "bidder-2",
		PriceMicros:  2000,
		LandingURL:   "https://example.com/clicked-landing",
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(24 * time.Hour).Unix(),
	}

	tok, err := token.Sign([]byte(h.cfg.EventTokenSecret), payload)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/v1/events/click?t="+tok, nil)
	rec := httptest.NewRecorder()

	h.HandleClick(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, "https://example.com/clicked-landing", res.Header.Get("Location"))
	assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))

	require.Len(t, pub.published, 1)
	ev := pub.published[0]
	assert.Equal(t, events.TypeClick, ev.Type)
	assert.Equal(t, events.EventID(events.TypeClick, payload.BidID), ev.EventID)
	assert.Equal(t, int64(0), ev.SpendMicros) // clicks have 0 spend
	assert.Equal(t, "req-click-1", ev.RequestID)
}

func TestEvents_MissingToken(t *testing.T) {
	h := setupEventsGateway(t, &capturingEventPublisher{}, true)

	// Impression without token
	req := httptest.NewRequest("GET", "/v1/events/impression", nil)
	rec := httptest.NewRecorder()
	h.HandleImpression(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_token")

	// Click without token
	req2 := httptest.NewRequest("GET", "/v1/events/click", nil)
	rec2 := httptest.NewRecorder()
	h.HandleClick(rec2, req2)
	assert.Equal(t, http.StatusBadRequest, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "invalid_token")
}

func TestEvents_TamperedToken(t *testing.T) {
	h := setupEventsGateway(t, &capturingEventPublisher{}, true)

	req := httptest.NewRequest("GET", "/v1/events/impression?t=tampered-garbage-token", nil)
	rec := httptest.NewRecorder()
	h.HandleImpression(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_token")
}

func TestEvents_ExpiredToken(t *testing.T) {
	h := setupEventsGateway(t, &capturingEventPublisher{}, true)

	issuedAt := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	payload := token.Payload{
		BidID:        "0193f0a2-1234-7000-8000-000000000003",
		RequestID:    "req-exp",
		CampaignID:   "camp-1",
		LandingURL:   "https://example.com",
		IssuedAt:     issuedAt.Unix(),
		ExpiresAt:    issuedAt.Add(24 * time.Hour).Unix(),
	}

	tok, err := token.Sign([]byte(h.cfg.EventTokenSecret), payload)
	require.NoError(t, err)

	// In test mode, send X-PulseBid-Test-Time 48h in future
	futureTime := issuedAt.Add(48 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/v1/events/impression?t="+tok, nil)
	req.Header.Set("X-PulseBid-Test-Time", futureTime)
	rec := httptest.NewRecorder()

	h.HandleImpression(rec, req)
	assert.Equal(t, http.StatusGone, rec.Code)
	assert.Contains(t, rec.Body.String(), "token_expired")

	// Same for click
	req2 := httptest.NewRequest("GET", "/v1/events/click?t="+tok, nil)
	req2.Header.Set("X-PulseBid-Test-Time", futureTime)
	rec2 := httptest.NewRecorder()

	h.HandleClick(rec2, req2)
	assert.Equal(t, http.StatusGone, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "token_expired")
}

func TestEvents_PublisherFails_503(t *testing.T) {
	pub := &capturingEventPublisher{
		publishError: errors.New("kafka down"),
	}
	h := setupEventsGateway(t, pub, true)

	now := time.Now().UTC()
	payload := token.Payload{
		BidID:        "0193f0a2-1234-7000-8000-000000000004",
		RequestID:    "req-down",
		CampaignID:   "camp-1",
		LandingURL:   "https://example.com",
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(24 * time.Hour).Unix(),
	}

	tok, err := token.Sign([]byte(h.cfg.EventTokenSecret), payload)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/v1/events/impression?t="+tok, nil)
	rec := httptest.NewRecorder()
	h.HandleImpression(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "unavailable")
}

func TestEvents_DuplicateImpressionProducesIdenticalEventID(t *testing.T) {
	pub := &capturingEventPublisher{}
	h := setupEventsGateway(t, pub, true)

	now := time.Now().UTC()
	payload := token.Payload{
		BidID:        "0193f0a2-1234-7000-8000-000000000005",
		RequestID:    "req-dup",
		CampaignID:   "camp-1",
		LandingURL:   "https://example.com",
		PriceMicros:  1200,
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(24 * time.Hour).Unix(),
	}

	tok, err := token.Sign([]byte(h.cfg.EventTokenSecret), payload)
	require.NoError(t, err)

	// Fire 1st impression
	req1 := httptest.NewRequest("GET", "/v1/events/impression?t="+tok, nil)
	rec1 := httptest.NewRecorder()
	h.HandleImpression(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	// Fire 2nd impression
	req2 := httptest.NewRequest("GET", "/v1/events/impression?t="+tok, nil)
	rec2 := httptest.NewRecorder()
	h.HandleImpression(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	require.Len(t, pub.published, 2)
	assert.Equal(t, pub.published[0].EventID, pub.published[1].EventID, "duplicate impression fire must yield identical event_id")
}
