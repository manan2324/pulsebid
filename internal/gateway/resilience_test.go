package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/budget"
	"pulsebid/internal/events"
	"pulsebid/internal/events/token"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/health"
)

type failingEventPublisher struct {
	mu           sync.Mutex
	droppedCount int
}

func (f *failingEventPublisher) Publish(ctx context.Context, ev *events.Event) error {
	return errors.New("kafka broker connection refused")
}

func (f *failingEventPublisher) TryPublishAsync(ev *events.Event) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.droppedCount++
	return false // dropped
}

func (f *failingEventPublisher) Flush(context.Context) error { return nil }
func (f *failingEventPublisher) Close() error                { return nil }

type failingReserver struct{}

func (f failingReserver) Reserve(ctx context.Context, c budget.CampaignBudget, costMicros int64, now time.Time) (budget.Result, error) {
	return 0, errors.New("redis connection timeout")
}
func (f failingReserver) Release(ctx context.Context, campaignID string, costMicros int64, now time.Time) error {
	return errors.New("redis connection timeout")
}
func (f failingReserver) Spent(ctx context.Context, campaignID string, now time.Time) (int64, error) {
	return 0, errors.New("redis connection timeout")
}
func (f failingReserver) Reset(ctx context.Context, campaignID string, now time.Time) error {
	return errors.New("redis connection timeout")
}

// 1. Failure Scenario (a): One bidder slow (> deadline), dropped at deadline, others unaffected
func TestResilience_OneBidderSlow(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	// b-slow sleeps 500ms
	transport.handlers["/bid/slow"] = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(outboundBidResponse{
				Bids: []RawBid{{CampaignID: "camp-slow", CreativeID: "cr-1", Price: 3.50}},
			})
		case <-r.Context().Done():
		}
	}
	// b-fast responds immediately with $2.00
	transport.handlers["/bid/fast"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-fast", CreativeID: "cr-1", Price: 2.00}},
		})
	}

	h, snapCache := setupTestGateway(t, transport, true)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "b-slow", EndpointUrl: "http://fake/bid/slow"},
			{Id: "b-fast", EndpointUrl: "http://fake/bid/fast"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-slow",
				AdvertiserId:    "adv-1",
				BidderId:        "b-slow",
				MaxBidCpmMicros: 4_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>slow</div>", LandingUrl: "https://slow.com"}},
			},
			{
				Id:              "camp-fast",
				AdvertiserId:    "adv-1",
				BidderId:        "b-fast",
				MaxBidCpmMicros: 3_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>fast</div>", LandingUrl: "https://fast.com"}},
			},
		},
	})

	body := `{"id":"req-slow-1","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.5}}`
	req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	start := time.Now()
	h.ServeHTTP(w, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Less(t, elapsed, 200*time.Millisecond, "slow bidder must be dropped at deadline without blocking gateway")

	debug := w.Header().Get("X-PulseBid-Debug")
	assert.Contains(t, debug, "timed_out=1", "timed_out count must reflect slow bidder")

	var resp BidResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "camp-fast", resp.CampaignID, "fast bidder must win despite slow bidder")
}

// 2. Failure Scenario (b): One bidder returns 5xx/garbage, counted as error, bid ignored
func TestResilience_OneBidderErrors(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid/err"] = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	transport.handlers["/bid/fast"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-fast", CreativeID: "cr-1", Price: 2.00}},
		})
	}

	h, snapCache := setupTestGateway(t, transport, true)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "b-err", EndpointUrl: "http://fake/bid/err"},
			{Id: "b-fast", EndpointUrl: "http://fake/bid/fast"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-err",
				AdvertiserId:    "adv-1",
				BidderId:        "b-err",
				MaxBidCpmMicros: 4_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>err</div>", LandingUrl: "https://err.com"}},
			},
			{
				Id:              "camp-fast",
				AdvertiserId:    "adv-1",
				BidderId:        "b-fast",
				MaxBidCpmMicros: 3_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>fast</div>", LandingUrl: "https://fast.com"}},
			},
		},
	})

	body := `{"id":"req-err-1","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.5}}`
	req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	debug := w.Header().Get("X-PulseBid-Debug")
	assert.Contains(t, debug, "errors=1")

	var resp BidResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "camp-fast", resp.CampaignID)
}

// 3. Failure Scenario (c): One bidder hangs indefinitely, context cancelled at deadline, no leaks
func TestResilience_OneBidderHangs(t *testing.T) {
	defer goleak.VerifyNone(t)

	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid/hang"] = func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}
	transport.handlers["/bid/fast"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-fast", CreativeID: "cr-1", Price: 2.00}},
		})
	}

	h, snapCache := setupTestGateway(t, transport, true)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "b-hang", EndpointUrl: "http://fake/bid/hang"},
			{Id: "b-fast", EndpointUrl: "http://fake/bid/fast"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-hang",
				AdvertiserId:    "adv-1",
				BidderId:        "b-hang",
				MaxBidCpmMicros: 4_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>hang</div>"}},
			},
			{
				Id:              "camp-fast",
				AdvertiserId:    "adv-1",
				BidderId:        "b-fast",
				MaxBidCpmMicros: 3_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>fast</div>"}},
			},
		},
	})

	body := `{"id":"req-hang-1","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.5}}`
	req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// 4. Failure Scenario (d): All bidders down or failing, returns 204 no_bids
func TestResilience_AllBiddersDown(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid/err1"] = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	transport.handlers["/bid/err2"] = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	h, snapCache := setupTestGateway(t, transport, true)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{
			{Id: "b1", EndpointUrl: "http://fake/bid/err1"},
			{Id: "b2", EndpointUrl: "http://fake/bid/err2"},
		},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "c1",
				AdvertiserId:    "adv-1",
				BidderId:        "b1",
				MaxBidCpmMicros: 2_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250}},
			},
			{
				Id:              "c2",
				AdvertiserId:    "adv-1",
				BidderId:        "b2",
				MaxBidCpmMicros: 2_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250}},
			},
		},
	})

	body := `{"id":"req-all-fail","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.5}}`
	req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "no_bids", w.Header().Get("X-PulseBid-Reason"))
}

// 5. Circuit Breaker: Tripping to Open on failure threshold, instant circuit_open, half-open probing & recovery
func TestResilience_CircuitBreaker_TripAndHalfOpen(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}

	var mu sync.Mutex
	returnError := true
	transport.handlers["/bid/cb"] = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if returnError {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "c1", CreativeID: "cr1", Price: 2.00}},
		})
	}

	cfg := Config{
		BidderDeadlineMS:  100,
		BidderMaxInflight: 10,
		CBMinRequests:     4, // Trip after 4 requests with >= 50% failures
		CBFailureRatio:    0.5,
		CBOpenFor:         80 * time.Millisecond,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fanout := NewFanout(cfg, m, logger, transport)

	bidder := &catalogv1.Bidder{Id: "b-cb", EndpointUrl: "http://fake/bid/cb"}
	cands := []Candidate{
		{Campaign: &CompiledCampaign{ID: "c1", MaxBidCPMMicros: 3_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}},
	}
	req := &BidRequest{ID: "r-cb", Imp: Impression{ID: "i1"}}

	// Step 1: Issue 4 failing requests to trip the circuit breaker
	for i := 0; i < 4; i++ {
		res := fanout.callOne(context.Background(), "b-cb", bidder, cands, req)
		assert.Equal(t, "error", res.Outcome)
	}

	// Verify breaker is tripped: 5th call must immediately return circuit_open without I/O
	start := time.Now()
	res := fanout.callOne(context.Background(), "b-cb", bidder, cands, req)
	elapsed := time.Since(start)

	assert.Equal(t, "circuit_open", res.Outcome, "open circuit breaker must return circuit_open outcome")
	assert.Less(t, elapsed, 2*time.Millisecond, "circuit_open outcome must be instant without network I/O")

	// Step 2: Transition from Open to Half-Open by waiting > CBOpenFor (80ms)
	time.Sleep(100 * time.Millisecond)

	// Heal the backend: return success now
	mu.Lock()
	returnError = false
	mu.Unlock()

	// Step 3: Probe request in Half-Open state
	resProbe := fanout.callOne(context.Background(), "b-cb", bidder, cands, req)
	assert.Equal(t, "bid", resProbe.Outcome, "half-open probe should execute and succeed")

	// Breaker should now be Closed again: subsequent requests succeed
	resClosed := fanout.callOne(context.Background(), "b-cb", bidder, cands, req)
	assert.Equal(t, "bid", resClosed.Outcome, "circuit breaker should be fully closed")
}

// 6. Bulkhead: Rejection on excess concurrent requests
func TestResilience_Bulkhead_Rejection(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}

	gate := make(chan struct{})
	transport.handlers["/bid/bulkhead"] = func(w http.ResponseWriter, r *http.Request) {
		<-gate
		w.WriteHeader(http.StatusNoContent)
	}

	cfg := Config{
		BidderDeadlineMS:  500,
		BidderMaxInflight: 2, // Maximum 2 concurrent in-flight requests
		CBMinRequests:     20,
		CBFailureRatio:    0.5,
		CBOpenFor:         5 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fanout := NewFanout(cfg, nil, logger, transport)

	bidder := &catalogv1.Bidder{Id: "b-bh", EndpointUrl: "http://fake/bid/bulkhead"}
	cands := []Candidate{
		{Campaign: &CompiledCampaign{ID: "c1", MaxBidCPMMicros: 2_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}},
	}
	req := &BidRequest{ID: "r-bh", Imp: Impression{ID: "i1"}}

	// Launch 2 requests that fill the bulkhead
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_ = fanout.callOne(context.Background(), "b-bh", bidder, cands, req)
		}()
	}

	time.Sleep(15 * time.Millisecond) // Let both workers acquire the semaphore

	// 3rd concurrent request must be rejected instantly by bulkhead
	start := time.Now()
	res := fanout.callOne(context.Background(), "b-bh", bidder, cands, req)
	elapsed := time.Since(start)

	assert.Equal(t, "rejected", res.Outcome, "bulkhead overflow must return rejected outcome")
	assert.Less(t, elapsed, 2*time.Millisecond, "bulkhead rejection must be non-blocking")

	close(gate)
	wg.Wait()
}

// 7. Failure Scenario (f): Kafka down: bidding unaffected, auction events dropped, impression/click return 503
func TestResilience_KafkaDown(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid"] = func(w http.ResponseWriter, r *http.Request) {
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
		EventTokenSecret:    "test-secret-at-least-32-bytes-long!",
		EventTokenTTL:       24 * time.Hour,
		EnableTestEndpoints: true,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	fanout := NewFanout(cfg, m, logger, transport)
	failingPub := &failingEventPublisher{}
	h := NewHandler(cfg, snapCache, fanout, StubReserver{}, failingPub, m, logger, clk)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{{Id: "bidder-1", EndpointUrl: "http://fake/bid"}},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-1",
				AdvertiserId:    "adv-1",
				BidderId:        "bidder-1",
				MaxBidCpmMicros: 3_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives: []*catalogv1.Creative{
					{Id: "cr-1", Width: 300, Height: 250, Markup: "<div>ad</div>", LandingUrl: "https://example.com"},
				},
			},
		},
	})

	// (a) Bidding unaffected: POST /v1/bid still returns 200 filled
	body := `{"id":"req-k-1","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.5}}`
	req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "bidding must be unaffected when Kafka is down")

	var bidResp BidResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bidResp))
	assert.Equal(t, 1, failingPub.droppedCount, "auction event should be dropped gracefully")

	// Generate a valid signed event token
	tokPayload := token.Payload{
		BidID:        "bid-123",
		RequestID:    "req-k-1",
		CampaignID:   "camp-1",
		CreativeID:   "cr-1",
		AdvertiserID: "adv-1",
		BidderID:     "bidder-1",
		PriceMicros:  1_000_000,
		LandingURL:   "https://example.com",
		IssuedAt:     time.Now().Unix(),
		ExpiresAt:    time.Now().Add(24 * time.Hour).Unix(),
	}
	signedTok, err := token.Sign([]byte(cfg.EventTokenSecret), tokPayload)
	require.NoError(t, err)

	// (b) Impression returns 503 unavailable
	impReq := httptest.NewRequest("GET", "/v1/events/impression?t="+signedTok, nil)
	impW := httptest.NewRecorder()
	h.HandleImpression(impW, impReq)
	assert.Equal(t, http.StatusServiceUnavailable, impW.Code, "impression must return 503 when Kafka is down")

	// (c) Click returns 503 unavailable
	clickReq := httptest.NewRequest("GET", "/v1/events/click?t="+signedTok, nil)
	clickW := httptest.NewRecorder()
	h.HandleClick(clickW, clickReq)
	assert.Equal(t, http.StatusServiceUnavailable, clickW.Code, "click must return 503 when Kafka is down")
}

// 8. Failure Scenario (g): Campaign-service down, gateway keeps last snapshot; /readyz only fails if no snapshot ever loaded
type failingCatalogClient struct{}

func (f *failingCatalogClient) GetSnapshot(ctx context.Context, in *catalogv1.GetSnapshotRequest, opts ...grpc.CallOption) (*catalogv1.GetSnapshotResponse, error) {
	return nil, status.Error(codes.Unavailable, "campaign service unavailable")
}

func TestResilience_CampaignServiceDown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	snapCache := NewSnapshotCache(nil, logger, clock.Real{})

	// Case 1: Uninitialized cache fails readiness
	assert.False(t, snapCache.IsReady())
	assert.Error(t, snapCache.CheckReady(context.Background()))

	// Case 2: Load initial snapshot
	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version:   5,
		Campaigns: []*catalogv1.Campaign{{Id: "c-initial", MaxBidCpmMicros: 2_000_000}},
	})
	assert.True(t, snapCache.IsReady())
	assert.NoError(t, snapCache.CheckReady(context.Background()))

	// Case 3: Poll fails with campaign service down -> retains stale snapshot, readiness remains OK
	err := snapCache.Poll(context.Background(), &failingCatalogClient{})
	assert.Error(t, err)

	current := snapCache.Get()
	require.NotNil(t, current)
	assert.Equal(t, int64(5), current.Version, "must keep serving last snapshot")
	assert.NoError(t, snapCache.CheckReady(context.Background()), "readiness must stay healthy when serving stale snapshot")
}

// 9. Failure Scenario (e): Redis down, fails closed with 204 error
func TestResilience_RedisDown(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid"] = func(w http.ResponseWriter, r *http.Request) {
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
		RedisOpTimeoutMS:    10,
		EnableTestEndpoints: true,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clk := clock.Real{}

	snapCache := NewSnapshotCache(m, logger, clk)
	fanout := NewFanout(cfg, m, logger, transport)
	h := NewHandler(cfg, snapCache, fanout, failingReserver{}, NoopEventPublisher{}, m, logger, clk)

	snapCache.Set(&catalogv1.GetSnapshotResponse{
		Version: 1,
		Bidders: []*catalogv1.Bidder{{Id: "bidder-1", EndpointUrl: "http://fake/bid"}},
		Campaigns: []*catalogv1.Campaign{
			{
				Id:              "camp-1",
				AdvertiserId:    "adv-1",
				BidderId:        "bidder-1",
				MaxBidCpmMicros: 3_000_000,
				Targeting:       &catalogv1.Targeting{PlacementIds: []string{"p1"}},
				Creatives:       []*catalogv1.Creative{{Id: "cr-1", Width: 300, Height: 250}},
			},
		},
	})

	body := `{"id":"req-redis-down","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.5}}`
	req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// Section 14.1: Redis down/slow -> 204 error (fail closed)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "error", w.Header().Get("X-PulseBid-Reason"))
}

// 10. Graceful Shutdown: /readyz flips to 503 first so upstream load balancer drains traffic cleanly
func TestResilience_GracefulShutdown(t *testing.T) {
	ready := health.New(map[string]health.Check{
		"check": func(ctx context.Context) error { return nil },
	}, time.Second)

	ready.SetReady(true)

	// Step 1: Health check is ready
	assert.NoError(t, ready.Check(context.Background()), "readyz check must pass when healthy")

	// Step 2: On shutdown signal, ready.Drain() flips /readyz to fail immediately
	ready.Drain()
	assert.Error(t, ready.Check(context.Background()), "readyz check must fail when draining")
}
