package bidder_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"pulsebid/internal/bidder"
)

func newTestServer(t *testing.T, cfg bidder.Config) *bidder.Server {
	t.Helper()
	s, err := bidder.NewServer("bidder-1", 9101, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("failed to create bidder server: %v", err)
	}
	return s
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*bidder.Config)
		wantErr bool
	}{
		{"valid defaults", func(_ *bidder.Config) {}, false},
		{"negative latency", func(c *bidder.Config) { c.LatencyMS = -1 }, true},
		{"negative jitter", func(c *bidder.Config) { c.JitterMS = -1 }, true},
		{"error rate > 1", func(c *bidder.Config) { c.ErrorRate = 1.1 }, true},
		{"negative error rate", func(c *bidder.Config) { c.ErrorRate = -0.1 }, true},
		{"timeout rate > 1", func(c *bidder.Config) { c.TimeoutRate = 1.5 }, true},
		{"no bid rate > 1", func(c *bidder.Config) { c.NoBidRate = 1.5 }, true},
		{"invalid bid mode", func(c *bidder.Config) { c.BidMode = "invalid_mode" }, true},
		{"invalid shade <= 0", func(c *bidder.Config) { c.BidShade = 0 }, true},
		{"invalid shade > 1", func(c *bidder.Config) { c.BidShade = 1.5 }, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := bidder.DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigApplyAllFields(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()

	lat := 20
	jit := 30
	errRate := 0.1
	toRate := 0.05
	nbRate := 0.15
	mode := "shade"
	shade := 0.75
	seed := int64(12345)

	patch := bidder.ConfigPatch{
		LatencyMS:   &lat,
		JitterMS:    &jit,
		ErrorRate:   &errRate,
		TimeoutRate: &toRate,
		NoBidRate:   &nbRate,
		BidMode:     &mode,
		BidShade:    &shade,
		Seed:        &seed,
	}

	updated, err := cfg.Apply(patch)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if updated.LatencyMS != 20 || updated.JitterMS != 30 || updated.ErrorRate != 0.1 ||
		updated.TimeoutRate != 0.05 || updated.NoBidRate != 0.15 || updated.BidMode != "shade" ||
		updated.BidShade != 0.75 || updated.Seed != 12345 {
		t.Errorf("config not updated as expected: %+v", updated)
	}
}

func TestBidder_FixedMode(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 0
	cfg.BidMode = bidder.BidModeFixed

	s := newTestServer(t, cfg)
	handler := s.Handler()

	reqBody := bidder.BidRequest{
		RequestID: "req-1",
		TmaxMS:    100,
		Imp: bidder.Impression{
			ID:          "1",
			PlacementID: "banner-top",
			W:           300,
			H:           250,
			Bidfloor:    0.50,
		},
		Candidates: []bidder.Candidate{
			{CampaignID: "camp-1", CreativeID: "cr-1", MaxBidCPM: 3.00},
			{CampaignID: "camp-2", CreativeID: "cr-2", MaxBidCPM: 2.00},
			{CampaignID: "camp-3", CreativeID: "cr-3", MaxBidCPM: 0.40}, // below floor!
		},
	}

	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var bidResp bidder.BidResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &bidResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(bidResp.Bids) != 2 {
		t.Fatalf("expected 2 bids (camp-3 below floor), got %d", len(bidResp.Bids))
	}

	// In fixed mode, price == max_bid_cpm
	if bidResp.Bids[0].Price != 3.00 {
		t.Errorf("camp-1 price: expected 3.00, got %f", bidResp.Bids[0].Price)
	}
	if bidResp.Bids[1].Price != 2.00 {
		t.Errorf("camp-2 price: expected 2.00, got %f", bidResp.Bids[1].Price)
	}
}

func TestBidder_ShadeMode(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 0
	cfg.BidMode = bidder.BidModeShade
	cfg.BidShade = 0.8

	s := newTestServer(t, cfg)
	handler := s.Handler()

	reqBody := bidder.BidRequest{
		RequestID: "req-1",
		Imp: bidder.Impression{
			Bidfloor: 1.00,
		},
		Candidates: []bidder.Candidate{
			{CampaignID: "camp-1", CreativeID: "cr-1", MaxBidCPM: 5.00}, // 5.0 * 0.8 = 4.0
			{CampaignID: "camp-2", CreativeID: "cr-2", MaxBidCPM: 1.10}, // 1.1 * 0.8 = 0.88 < floor => dropped
		},
	}

	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var bidResp bidder.BidResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &bidResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(bidResp.Bids) != 1 {
		t.Fatalf("expected 1 bid, got %d", len(bidResp.Bids))
	}
	if bidResp.Bids[0].Price != 4.00 {
		t.Errorf("expected shaded price 4.00, got %f", bidResp.Bids[0].Price)
	}
}

func TestBidder_RandomModeDeterminism(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 0
	cfg.BidMode = bidder.BidModeRandom
	cfg.Seed = 42

	s := newTestServer(t, cfg)
	handler := s.Handler()

	reqBody := bidder.BidRequest{
		RequestID: "repeatable-req-123",
		Imp: bidder.Impression{
			Bidfloor: 1.00,
		},
		Candidates: []bidder.Candidate{
			{CampaignID: "camp-alpha", CreativeID: "cr-1", MaxBidCPM: 4.00},
		},
	}

	// Make 2 identical requests
	raw, _ := json.Marshal(reqBody)
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	var b1 bidder.BidResponse
	_ = json.Unmarshal(rec1.Body.Bytes(), &b1)

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	var b2 bidder.BidResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &b2)

	if len(b1.Bids) != 1 || len(b2.Bids) != 1 {
		t.Fatalf("expected 1 bid in both responses")
	}

	// Must be exactly identical
	if b1.Bids[0].Price != b2.Bids[0].Price {
		t.Errorf("expected deterministic identical price: %f vs %f", b1.Bids[0].Price, b2.Bids[0].Price)
	}

	// Must be bounded within [floor, max_bid]
	if b1.Bids[0].Price < 1.00 || b1.Bids[0].Price > 4.00 {
		t.Errorf("price %f outside [1.00, 4.00]", b1.Bids[0].Price)
	}

	// Change request ID -> price should differ
	reqBody.RequestID = "different-req-456"
	rawDiff, _ := json.Marshal(reqBody)
	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(rawDiff))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	var b3 bidder.BidResponse
	_ = json.Unmarshal(rec3.Body.Bytes(), &b3)

	if b3.Bids[0].Price == b1.Bids[0].Price {
		t.Logf("different request ID yielded same price (unlikely but possible): %f", b3.Bids[0].Price)
	}
}

func TestBidder_FloorFilterNoFill(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 0
	cfg.BidMode = bidder.BidModeFixed

	s := newTestServer(t, cfg)
	handler := s.Handler()

	// Floor is 5.00, max bid is 3.00 => no bid => 204
	reqBody := bidder.BidRequest{
		RequestID: "req-1",
		Imp: bidder.Impression{
			Bidfloor: 5.00,
		},
		Candidates: []bidder.Candidate{
			{CampaignID: "camp-1", CreativeID: "cr-1", MaxBidCPM: 3.00},
		},
	}

	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d", rec.Code)
	}
}

func TestBidder_EmptyCandidates(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 0

	s := newTestServer(t, cfg)
	handler := s.Handler()

	reqBody := bidder.BidRequest{
		RequestID:  "req-empty",
		Candidates: []bidder.Candidate{},
	}

	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for empty candidates, got %d", rec.Code)
	}
}

func TestBidder_MalformedJSON(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	s := newTestServer(t, cfg)
	handler := s.Handler()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader([]byte(`{"request_id":`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestBidder_AdminConfigCRUD(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	s := newTestServer(t, cfg)
	handler := s.Handler()

	// 1. GET /admin/config
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/config", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}

	var readCfg bidder.Config
	if err := json.Unmarshal(rec1.Body.Bytes(), &readCfg); err != nil {
		t.Fatalf("failed to decode config: %v", err)
	}
	if readCfg.LatencyMS != 10 || readCfg.BidMode != "fixed" {
		t.Errorf("unexpected initial config: %+v", readCfg)
	}

	// 2. POST /admin/config partial update
	newLatency := 500
	newMode := "shade"
	patch := bidder.ConfigPatch{
		LatencyMS: &newLatency,
		BidMode:   &newMode,
	}
	patchRaw, _ := json.Marshal(patch)
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/config", bytes.NewReader(patchRaw))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for patch, got %d", rec2.Code)
	}

	var updatedCfg bidder.Config
	_ = json.Unmarshal(rec2.Body.Bytes(), &updatedCfg)
	if updatedCfg.LatencyMS != 500 || updatedCfg.BidMode != "shade" || updatedCfg.JitterMS != 15 {
		t.Errorf("unexpected updated config: %+v", updatedCfg)
	}

	// Verify server reflects updated config internally
	if s.CurrentConfig().LatencyMS != 500 {
		t.Errorf("server didn't update config internally")
	}

	// 3. POST invalid config (e.g. error_rate > 1)
	badRate := 2.5
	badPatch := bidder.ConfigPatch{ErrorRate: &badRate}
	badRaw, _ := json.Marshal(badPatch)
	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/config", bytes.NewReader(badRaw))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad config, got %d", rec3.Code)
	}
}

func TestBidder_ErrorRateBehavior(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.ErrorRate = 1.0 // 100% error rate

	s := newTestServer(t, cfg)
	handler := s.Handler()

	reqBody := bidder.BidRequest{
		RequestID: "req-err",
		Candidates: []bidder.Candidate{
			{CampaignID: "camp-1", CreativeID: "cr-1", MaxBidCPM: 2.0},
		},
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when error_rate=1.0, got %d", rec.Code)
	}
}

func TestBidder_NoBidRateBehavior(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 1.0 // 100% no-bid rate

	s := newTestServer(t, cfg)
	handler := s.Handler()

	reqBody := bidder.BidRequest{
		RequestID: "req-nobid",
		Candidates: []bidder.Candidate{
			{CampaignID: "camp-1", CreativeID: "cr-1", MaxBidCPM: 2.0},
		},
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 when no_bid_rate=1.0, got %d", rec.Code)
	}
}

func TestBidder_TimeoutRateBehavior(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.TimeoutRate = 1.0 // 100% timeout rate

	s := newTestServer(t, cfg)
	handler := s.Handler()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader([]byte(`{"candidates":[{"campaign_id":"c1","creative_id":"cr1","max_bid_cpm":1.0}]}`)))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	duration := time.Since(start)

	if duration < 40*time.Millisecond {
		t.Errorf("request finished too quickly: %v", duration)
	}
}

func TestBidder_HealthzAndReadyz(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	s := newTestServer(t, cfg)
	handler := s.Handler()

	// /healthz
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("healthz check failed: %d", rec1.Code)
	}

	// /readyz
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("readyz check failed: %d", rec2.Code)
	}

	// drain
	s.SetDraining(true)
	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusServiceUnavailable {
		t.Errorf("draining readyz check expected 503, got %d", rec3.Code)
	}

	// /metrics
	req4 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Errorf("metrics check failed: %d", rec4.Code)
	}
	body, _ := io.ReadAll(rec4.Body)
	if !bytes.Contains(body, []byte("http_server_requests_total")) {
		t.Errorf("metrics response missing http_server_requests_total")
	}
}

func TestBidder_ConcurrentRequestsAndUpdates(t *testing.T) {
	t.Parallel()
	cfg := bidder.DefaultConfig()
	cfg.LatencyMS = 0
	cfg.JitterMS = 0
	cfg.NoBidRate = 0

	s := newTestServer(t, cfg)
	handler := s.Handler()

	var wg sync.WaitGroup
	// 50 concurrent bid requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			reqBody := bidder.BidRequest{
				RequestID: "req-conc",
				Candidates: []bidder.Candidate{
					{CampaignID: "camp-1", CreativeID: "cr-1", MaxBidCPM: 3.00},
				},
			}
			raw, _ := json.Marshal(reqBody)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/bid", bytes.NewReader(raw))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
				t.Errorf("concurrent bid failed: %d", rec.Code)
			}
		}(i)
	}

	// Concurrent config patches
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mode := "fixed"
			if idx%2 == 0 {
				mode = "shade"
			}
			patch := bidder.ConfigPatch{BidMode: &mode}
			raw, _ := json.Marshal(patch)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/config", bytes.NewReader(raw))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}(i)
	}

	wg.Wait()
}
