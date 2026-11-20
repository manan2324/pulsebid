package gateway

import (
	"errors"
	"net/http"
	"time"

	"pulsebid/internal/events"
	"pulsebid/internal/events/token"
	"pulsebid/internal/platform/httpx"
)

// 43-byte valid 1x1 transparent GIF89a.
var transparent1x1GIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff,
	0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b,
}

// HandleImpression tracks ad impressions, validates the signed event token, publishes to Kafka,
// and returns a transparent 1x1 GIF with Cache-Control: no-store (Section 11.1 & Section 1295).
func (h *Handler) HandleImpression(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("t")
	if tokenStr == "" {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_token", Message: "missing event token"})
		return
	}

	now := h.clk.Now().UTC()
	if h.cfg.EnableTestEndpoints {
		if testTimeHdr := r.Header.Get("X-PulseBid-Test-Time"); testTimeHdr != "" {
			if t, err := time.Parse(time.RFC3339, testTimeHdr); err == nil {
				now = t.UTC()
			}
		}
	}

	payload, err := token.Verify([]byte(h.cfg.EventTokenSecret), tokenStr, now)
	if err != nil {
		if errors.Is(err, token.ErrExpiredToken) {
			httpx.WriteError(w, r, &httpx.Error{Code: "token_expired", Message: "event token has expired"})
			return
		}
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_token", Message: "invalid event token"})
		return
	}

	ev := &events.Event{
		SchemaVersion: events.CurrentSchemaVersion,
		EventID:       events.EventID(events.TypeImpression, payload.BidID),
		Type:          events.TypeImpression,
		OccurredAt:    now,
		RequestID:     payload.RequestID,
		BidID:         payload.BidID,
		CampaignID:    payload.CampaignID,
		CreativeID:    payload.CreativeID,
		AdvertiserID:  payload.AdvertiserID,
		BidderID:      payload.BidderID,
		SpendMicros:   payload.PriceMicros,
	}

	if err := h.events.Publish(r.Context(), ev); err != nil {
		h.logger.Error("failed to publish impression event", "bid_id", payload.BidID, "error", err)
		httpx.WriteError(w, r, &httpx.Error{Code: "unavailable", Message: "event publishing unavailable", Cause: err})
		return
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(transparent1x1GIF)
}

// HandleClick tracks ad clicks, validates the signed event token, publishes to Kafka,
// and issues an HTTP 302 Found redirect to the token's landing URL (Section 11.1 & Section 1295).
func (h *Handler) HandleClick(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("t")
	if tokenStr == "" {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_token", Message: "missing event token"})
		return
	}

	now := h.clk.Now().UTC()
	if h.cfg.EnableTestEndpoints {
		if testTimeHdr := r.Header.Get("X-PulseBid-Test-Time"); testTimeHdr != "" {
			if t, err := time.Parse(time.RFC3339, testTimeHdr); err == nil {
				now = t.UTC()
			}
		}
	}

	payload, err := token.Verify([]byte(h.cfg.EventTokenSecret), tokenStr, now)
	if err != nil {
		if errors.Is(err, token.ErrExpiredToken) {
			httpx.WriteError(w, r, &httpx.Error{Code: "token_expired", Message: "event token has expired"})
			return
		}
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_token", Message: "invalid event token"})
		return
	}

	if payload.LandingURL == "" {
		httpx.WriteError(w, r, &httpx.Error{Code: "invalid_token", Message: "token missing landing url"})
		return
	}

	ev := &events.Event{
		SchemaVersion: events.CurrentSchemaVersion,
		EventID:       events.EventID(events.TypeClick, payload.BidID),
		Type:          events.TypeClick,
		OccurredAt:    now,
		RequestID:     payload.RequestID,
		BidID:         payload.BidID,
		CampaignID:    payload.CampaignID,
		CreativeID:    payload.CreativeID,
		AdvertiserID:  payload.AdvertiserID,
		BidderID:      payload.BidderID,
		SpendMicros:   0,
	}

	if err := h.events.Publish(r.Context(), ev); err != nil {
		h.logger.Error("failed to publish click event", "bid_id", payload.BidID, "error", err)
		httpx.WriteError(w, r, &httpx.Error{Code: "unavailable", Message: "event publishing unavailable", Cause: err})
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, payload.LandingURL, http.StatusFound)
}
