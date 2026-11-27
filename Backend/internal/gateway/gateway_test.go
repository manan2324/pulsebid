package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/budget"
	"pulsebid/internal/platform/clock"
)

func setupTestGateway(_ *testing.T, transport http.RoundTripper, enableTest bool) (*Handler, *SnapshotCache) {
	cfg := Config{
		HTTPAddr:           ":8080",
		PublicBaseURL:      "http://localhost:8080",
		BidderDeadlineMS:   100,
		RequestMaxBytes:    16384,
		BidderMaxInflight:  512,
		CBMinRequests:      20,
		CBFailureRatio:     0.5,
		CBOpenFor:          5 * time.Second,
		MaxAuctionAttempts: 3,
	}
	cfg.EnableTestEndpoints = enableTest

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	fanout := NewFanout(cfg, m, logger, transport)
	h := NewHandler(cfg, snapCache, fanout, StubReserver{}, NoopEventPublisher{}, m, logger, clk)

	return h, snapCache
}

func TestHandler_HappyPath200(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-1", CreativeID: "cr-1", Price: 2.50}},
		})
	}

	h, snapCache := setupTestGateway(t, transport, true)

	// Populate snapshot
	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "bidder-1", EndpointUrl: "http://fake/bid"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-1",
				AdvertiserId:    "adv-1",
				BidderId:        "bidder-1",
				MaxBidCpmMicros: 3_000_000,
				Targeting: &catalogv1.Targeting{
					Countries:    []string{"IN"},
					DeviceTypes:  []string{"mobile"},
					PlacementIds: []string{"top-banner"},
				},
				Creatives: []*catalogv1.Creative{
					{
						Id:         "cr-1",
						Width:      300,
						Height:     250,
						Markup:     `<a href="{click_url}"><img src="ad.png"/></a><img src="{impression_url}"/>`,
						LandingUrl: "https://example.com",
					},
				},
			},
		},
	})

	body := `{"id":"req-123","imp":{"id":"imp-1","placement_id":"top-banner","w":300,"h":250,"bidfloor":0.50},"device":{"type":"mobile"},"geo":{"country":"IN"}}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "trace-req-123")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Contains(t, res.Header.Get("Server-Timing"), "total;dur=")
	assert.Contains(t, res.Header.Get("X-PulseBid-Debug"), "eligible=1;responded=1;timed_out=0;errors=0;valid_bids=1;attempts=1")

	var bidResp BidResponse
	err := json.NewDecoder(res.Body).Decode(&bidResp)
	require.NoError(t, err)

	assert.Equal(t, "req-123", bidResp.RequestID)
	_, err = uuid.Parse(bidResp.BidID)
	assert.NoError(t, err, "bid_id must be valid UUID")
	assert.Equal(t, "camp-1", bidResp.CampaignID)
	assert.Equal(t, "cr-1", bidResp.CreativeID)
	assert.Equal(t, "adv-1", bidResp.AdvertiserID)
	assert.Equal(t, "bidder-1", bidResp.BidderID)
	assert.Equal(t, "USD", bidResp.Currency)
	assert.Equal(t, 300, bidResp.W)
	assert.Equal(t, 250, bidResp.H)
	// Clearing CPM: single bid 2.50 with floor 0.50 -> min(2.50, max(0, 0.50) + 0.01) = 0.51
	assert.Equal(t, 0.51, bidResp.Price)

	assert.Contains(t, bidResp.ImpressionURL, "http://localhost:8080/v1/events/impression?t=")
	assert.Contains(t, bidResp.ClickURL, "http://localhost:8080/v1/events/click?t=")
	assert.NotContains(t, bidResp.Adm, "{click_url}")
	assert.NotContains(t, bidResp.Adm, "{impression_url}")
	assert.Contains(t, bidResp.Adm, bidResp.ClickURL)
	assert.Contains(t, bidResp.Adm, bidResp.ImpressionURL)
}

func TestHandler_NoFillReasons(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid/nobid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	transport.handlers["/bid/low"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-low", CreativeID: "cr-1", Price: 0.40}},
		})
	}

	h, snapCache := setupTestGateway(t, transport, true)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "b-nobid", EndpointUrl: "http://fake/bid/nobid"},
			{Id: "b-low", EndpointUrl: "http://fake/bid/low"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-geo",
				BidderId:        "b-nobid",
				MaxBidCpmMicros: 2_000_000,
				Targeting:       &catalogv1.Targeting{Countries: []string{"US"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250}},
			},
			{
				Id:              "camp-low",
				BidderId:        "b-low",
				MaxBidCpmMicros: 2_000_000,
				Targeting:       &catalogv1.Targeting{Countries: []string{"IN"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250}},
			},
		},
	})

	// Case 1: no_eligible (country DE matches neither US nor IN)
	req1 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"r1","imp":{"id":"i1","placement_id":"p1"},"geo":{"country":"DE"}}`))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusNoContent, rec1.Code)
	assert.Equal(t, "no_eligible", rec1.Header().Get("X-PulseBid-Reason"))

	// Case 2: no_bids (matches camp-geo with b-nobid which returns 204)
	req2 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"r2","imp":{"id":"i1","placement_id":"p1"},"geo":{"country":"US"}}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNoContent, rec2.Code)
	assert.Equal(t, "no_bids", rec2.Header().Get("X-PulseBid-Reason"))

	// Case 3: below_floor (matches camp-low which bids 0.40, floor is 1.00)
	req3 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"r3","imp":{"id":"i1","placement_id":"p1","bidfloor":1.00},"geo":{"country":"IN"}}`))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusNoContent, rec3.Code)
	assert.Equal(t, "below_floor", rec3.Header().Get("X-PulseBid-Reason"))
}

func TestHandler_TimeOfDayHeaderOverride(t *testing.T) {
	transport := &fakeBidderTransport{handlers: make(map[string]http.HandlerFunc)}
	transport.handlers["/bid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-hours", CreativeID: "cr-1", Price: 2.00}},
		})
	}

	h, snapCache := setupTestGateway(t, transport, true)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{{Id: "b1", EndpointUrl: "http://fake/bid"}},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-hours",
				BidderId:        "b1",
				MaxBidCpmMicros: 2_000_000,
				Targeting: &catalogv1.Targeting{
					HoursOfDay: []int32{3},
					Timezone:   "UTC",
				},
				Creatives: []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250}},
			},
		},
	})

	// Match at 03:30 UTC
	reqMatch := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"r1","imp":{"id":"i1","placement_id":"p1"}}`))
	reqMatch.Header.Set("Content-Type", "application/json")
	reqMatch.Header.Set("X-PulseBid-Test-Time", "2026-11-18T03:30:00Z")
	recMatch := httptest.NewRecorder()
	h.ServeHTTP(recMatch, reqMatch)
	assert.Equal(t, http.StatusOK, recMatch.Code)

	// Mismatch at 10:00 UTC
	reqMiss := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"r2","imp":{"id":"i1","placement_id":"p1"}}`))
	reqMiss.Header.Set("Content-Type", "application/json")
	reqMiss.Header.Set("X-PulseBid-Test-Time", "2026-11-18T10:00:00Z")
	recMiss := httptest.NewRecorder()
	h.ServeHTTP(recMiss, reqMiss)
	assert.Equal(t, http.StatusNoContent, recMiss.Code)
	assert.Equal(t, "no_eligible", recMiss.Header().Get("X-PulseBid-Reason"))
}

func TestHandler_ErrorNegatives(t *testing.T) {
	h, snapCache := setupTestGateway(t, nil, false)
	snapCache.Set(&catalogv1.GetSnapshotResponse{Version: 1})

	// 1. Wrong Content-Type => 415
	req415 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"r1"}`))
	req415.Header.Set("Content-Type", "text/plain")
	rec415 := httptest.NewRecorder()
	h.ServeHTTP(rec415, req415)
	assert.Equal(t, http.StatusUnsupportedMediaType, rec415.Code)

	// 2. Body too large => 413
	req413 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{"id":"`+strings.Repeat("a", 20000)+`"}`))
	req413.Header.Set("Content-Type", "application/json")
	rec413 := httptest.NewRecorder()
	h.ServeHTTP(rec413, req413)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec413.Code)

	// 3. Invalid JSON => 400
	req400 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(`{malformed`))
	req400.Header.Set("Content-Type", "application/json")
	rec400 := httptest.NewRecorder()
	h.ServeHTTP(rec400, req400)
	assert.Equal(t, http.StatusBadRequest, rec400.Code)
}

func BenchmarkBidHandler_Filled(b *testing.B) {
	transport := &fakeBidderTransport{handlers: make(map[string]http.HandlerFunc)}
	transport.handlers["/bid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-1", CreativeID: "cr-1", Price: 2.00}},
		})
	}

	cfg := Config{
		HTTPAddr:          ":8080",
		PublicBaseURL:     "http://localhost:8080",
		BidderDeadlineMS:  100,
		RequestMaxBytes:   16384,
		BidderMaxInflight: 512,
		CBMinRequests:     20,
		CBFailureRatio:    0.5,
		CBOpenFor:         5 * time.Second,
	}
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version:     1,
		GeneratedAt: timestamppb.Now(),
		Bidders:     []*catalogv1.Bidder{{Id: "b1", EndpointUrl: "http://fake/bid"}},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-1",
				BidderId:        "b1",
				MaxBidCpmMicros: 3_000_000,
				Targeting: &catalogv1.Targeting{
					Countries:    []string{"IN"},
					DeviceTypes:  []string{"mobile"},
					PlacementIds: []string{"top-banner"},
				},
				Creatives: []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<ad/>"}},
			},
		},
	})

	fanout := NewFanout(cfg, m, logger, transport)
	h := NewHandler(cfg, snapCache, fanout, StubReserver{}, NoopEventPublisher{}, m, logger, clk)

	payload := `{"id":"req-1","imp":{"id":"i1","placement_id":"top-banner","w":300,"h":250,"bidfloor":0.50},"device":{"type":"mobile"},"geo":{"country":"IN"}}`

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
}

type mockReserverFunc func(ctx context.Context, c budget.CampaignBudget, costMicros int64, now time.Time) (budget.Result, error)

type customMockReserver struct {
	fn mockReserverFunc
}

func (m customMockReserver) Reserve(ctx context.Context, c budget.CampaignBudget, costMicros int64, now time.Time) (budget.Result, error) {
	if m.fn != nil {
		return m.fn(ctx, c, costMicros, now)
	}
	return budget.Reserved, nil
}
func (m customMockReserver) Release(context.Context, string, int64, time.Time) error { return nil }
func (m customMockReserver) Spent(context.Context, string, time.Time) (int64, error) { return 0, nil }
func (m customMockReserver) Reset(context.Context, string, time.Time) error          { return nil }

func TestHandler_ReAuctionOnBudgetExhaustion(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid1"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-1", CreativeID: "cr-1", Price: 3.00}},
		})
	}
	transport.handlers["/bid2"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-2", CreativeID: "cr-2", Price: 2.00}},
		})
	}

	cfg := Config{
		HTTPAddr:            ":8080",
		PublicBaseURL:       "http://localhost:8080",
		BidderDeadlineMS:    1000, // Allow race-instrumented scheduling; this test verifies re-auction logic.
		RequestMaxBytes:     16384,
		BidderMaxInflight:   512,
		CBMinRequests:       20,
		CBFailureRatio:      0.5,
		CBOpenFor:           5 * time.Second,
		MaxAuctionAttempts:  3,
		EnableTestEndpoints: true,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "bidder-1", EndpointUrl: "http://fake/bid1"},
			{Id: "bidder-2", EndpointUrl: "http://fake/bid2"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:                "camp-1",
				AdvertiserId:      "adv-1",
				BidderId:          "bidder-1",
				DailyBudgetMicros: 10_000,
				MaxBidCpmMicros:   3_000_000,
				Targeting: &catalogv1.Targeting{
					Countries:    []string{"IN"},
					DeviceTypes:  []string{"mobile"},
					PlacementIds: []string{"top-banner"},
				},
				Creatives: []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>Ad1</div>"}},
			},
			{
				Id:                "camp-2",
				AdvertiserId:      "adv-2",
				BidderId:          "bidder-2",
				DailyBudgetMicros: 10_000,
				MaxBidCpmMicros:   2_000_000,
				Targeting: &catalogv1.Targeting{
					Countries:    []string{"IN"},
					DeviceTypes:  []string{"mobile"},
					PlacementIds: []string{"top-banner"},
				},
				Creatives: []*catalogv1.Creative{{Id: "cr-2", Width: 300, Height: 250, Markup: "<div>Ad2</div>"}},
			},
		},
	})

	fanout := NewFanout(cfg, m, logger, transport)

	// camp-1 is exhausted, camp-2 succeeds
	reserver := customMockReserver{
		fn: func(_ context.Context, c budget.CampaignBudget, _ int64, _ time.Time) (budget.Result, error) {
			if c.ID == "camp-1" {
				return budget.Exhausted, nil
			}
			return budget.Reserved, nil
		},
	}

	h := NewHandler(cfg, snapCache, fanout, reserver, NoopEventPublisher{}, m, logger, clk)

	// Attempt 1: both bid; camp-1 wins 1st price auction with 2.01, but is exhausted.
	// Re-auction drops camp-1; camp-2 wins 2nd price auction with clearing price 1.01 (floor 1.00 + 0.01).
	payload := `{"id":"req-1","imp":{"id":"i1","placement_id":"top-banner","w":300,"h":250,"bidfloor":1.00},"device":{"type":"mobile"},"geo":{"country":"IN"}}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp BidResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "camp-2", resp.CampaignID)
	assert.Equal(t, 1.01, resp.Price)
	assert.Contains(t, rec.Header().Get("X-PulseBid-Debug"), "attempts=2")

	// camp-1 should now be marked exhausted in cache
	assert.True(t, h.exhausted.IsExhausted("camp-1", time.Now()))

	// Second request: camp-1 is filtered at targeting, only bidder-2 is called
	req2 := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(payload))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	var resp2 BidResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	assert.Equal(t, "camp-2", resp2.CampaignID)
	assert.Contains(t, rec2.Header().Get("X-PulseBid-Debug"), "eligible=1")
}

func TestHandler_BudgetExhaustionAllAttempts(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-1", CreativeID: "cr-1", Price: 2.50}},
		})
	}

	cfg := Config{
		HTTPAddr:            ":8080",
		PublicBaseURL:       "http://localhost:8080",
		BidderDeadlineMS:    100,
		RequestMaxBytes:     16384,
		BidderMaxInflight:   512,
		CBMinRequests:       20,
		CBFailureRatio:      0.5,
		CBOpenFor:           5 * time.Second,
		MaxAuctionAttempts:  3,
		EnableTestEndpoints: true,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{{Id: "bidder-1", EndpointUrl: "http://fake/bid"}},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:                "camp-1",
				BidderId:          "bidder-1",
				DailyBudgetMicros: 10_000,
				MaxBidCpmMicros:   3_000_000,
				Targeting: &catalogv1.Targeting{
					Countries:    []string{"IN"},
					DeviceTypes:  []string{"mobile"},
					PlacementIds: []string{"top-banner"},
				},
				Creatives: []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>Ad1</div>"}},
			},
		},
	})

	fanout := NewFanout(cfg, m, logger, transport)
	reserver := customMockReserver{
		fn: func(_ context.Context, _ budget.CampaignBudget, _ int64, _ time.Time) (budget.Result, error) {
			return budget.Exhausted, nil
		},
	}

	h := NewHandler(cfg, snapCache, fanout, reserver, NoopEventPublisher{}, m, logger, clk)

	payload := `{"id":"req-1","imp":{"id":"i1","placement_id":"top-banner","w":300,"h":250,"bidfloor":1.00},"device":{"type":"mobile"},"geo":{"country":"IN"}}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "budget", rec.Header().Get("X-PulseBid-Reason"))
}

func TestHandler_FailClosedOnRedisError(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-1", CreativeID: "cr-1", Price: 2.50}},
		})
	}

	cfg := Config{
		HTTPAddr:            ":8080",
		PublicBaseURL:       "http://localhost:8080",
		BidderDeadlineMS:    100,
		RequestMaxBytes:     16384,
		BidderMaxInflight:   512,
		CBMinRequests:       20,
		CBFailureRatio:      0.5,
		CBOpenFor:           5 * time.Second,
		MaxAuctionAttempts:  3,
		EnableTestEndpoints: true,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{{Id: "bidder-1", EndpointUrl: "http://fake/bid"}},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:                "camp-1",
				BidderId:          "bidder-1",
				DailyBudgetMicros: 10_000,
				MaxBidCpmMicros:   3_000_000,
				Targeting: &catalogv1.Targeting{
					Countries:    []string{"IN"},
					DeviceTypes:  []string{"mobile"},
					PlacementIds: []string{"top-banner"},
				},
				Creatives: []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>Ad1</div>"}},
			},
		},
	})

	fanout := NewFanout(cfg, m, logger, transport)
	reserver := customMockReserver{
		fn: func(_ context.Context, _ budget.CampaignBudget, _ int64, _ time.Time) (budget.Result, error) {
			return 0, assert.AnError
		},
	}

	h := NewHandler(cfg, snapCache, fanout, reserver, NoopEventPublisher{}, m, logger, clk)

	payload := `{"id":"req-1","imp":{"id":"i1","placement_id":"top-banner","w":300,"h":250,"bidfloor":1.00},"device":{"type":"mobile"},"geo":{"country":"IN"}}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/bid", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "error", rec.Header().Get("X-PulseBid-Reason"))
}
