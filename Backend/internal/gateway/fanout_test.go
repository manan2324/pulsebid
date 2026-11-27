package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
)

type fakeBidderTransport struct {
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
}

func (f *fakeBidderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	handler, ok := f.handlers[req.URL.Path]
	f.mu.Unlock()

	if !ok {
		// Fallback handler if path not specifically registered
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(http.NoBody),
			Header:     make(http.Header),
		}, nil
	}

	rec := httptest.NewRecorder()
	handler(rec, req)
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return rec.Result(), nil
}

func TestFanout_BiddersMixAndDeadline(t *testing.T) {
	defer goleak.VerifyNone(t)

	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}

	// 1. Fast bidder: returns 200 immediately
	transport.handlers["/bid/fast"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{{CampaignID: "camp-fast", CreativeID: "cr1", Price: 2.50}},
		})
	}

	// 2. Error bidder: returns 500
	transport.handlers["/bid/error"] = func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}

	// 3. No-bid bidder: returns 204
	transport.handlers["/bid/nobid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	// 4. Hanging bidder: waits for context cancellation
	transport.handlers["/bid/hang"] = func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}

	// 5. Late bidder: sleeps longer than deadline
	transport.handlers["/bid/late"] = func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(outboundBidResponse{
				Bids: []RawBid{{CampaignID: "camp-late", CreativeID: "cr1", Price: 3.00}},
			})
		case <-r.Context().Done():
		}
	}

	cfg := Config{
		BidderDeadlineMS:  50,
		BidderMaxInflight: 10,
		CBMinRequests:     20,
		CBFailureRatio:    0.5,
		CBOpenFor:         5 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fanout := NewFanout(cfg, nil, logger, transport)

	bidders := map[string]*catalogv1.Bidder{
		"b-fast":  {Id: "b-fast", EndpointUrl: "http://fake/bid/fast"},
		"b-error": {Id: "b-error", EndpointUrl: "http://fake/bid/error"},
		"b-nobid": {Id: "b-nobid", EndpointUrl: "http://fake/bid/nobid"},
		"b-hang":  {Id: "b-hang", EndpointUrl: "http://fake/bid/hang"},
		"b-late":  {Id: "b-late", EndpointUrl: "http://fake/bid/late"},
	}

	groups := map[string][]Candidate{
		"b-fast":  {{Campaign: &CompiledCampaign{ID: "camp-fast", MaxBidCPMMicros: 3_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}}},
		"b-error": {{Campaign: &CompiledCampaign{ID: "camp-err", MaxBidCPMMicros: 3_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}}},
		"b-nobid": {{Campaign: &CompiledCampaign{ID: "camp-nobid", MaxBidCPMMicros: 3_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}}},
		"b-hang":  {{Campaign: &CompiledCampaign{ID: "camp-hang", MaxBidCPMMicros: 3_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}}},
		"b-late":  {{Campaign: &CompiledCampaign{ID: "camp-late", MaxBidCPMMicros: 3_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}}},
	}

	req := &BidRequest{ID: "req-1", Imp: Impression{ID: "i1"}}

	start := time.Now()
	deadline := start.Add(50 * time.Millisecond)
	results := fanout.Run(context.Background(), deadline, groups, req, bidders)
	elapsed := time.Since(start)

	// Proving latency ≈ deadline when bidders hang:
	// Should complete right around 50ms (allow generous upper bound for scheduler jitter, but assert bounded)
	assert.Less(t, elapsed, 150*time.Millisecond, "fanout must not hang indefinitely")

	// Verify outcomes
	outcomes := make(map[string]string)
	for _, res := range results {
		outcomes[res.BidderID] = res.Outcome
	}

	assert.Equal(t, "bid", outcomes["b-fast"])
	assert.Equal(t, "error", outcomes["b-error"])
	assert.Equal(t, "nobid", outcomes["b-nobid"])
	// b-hang and b-late could either be timed out or dropped when deadline fires
	if o, ok := outcomes["b-hang"]; ok {
		assert.Equal(t, "timeout", o)
	}
	if o, ok := outcomes["b-late"]; ok {
		assert.Equal(t, "timeout", o)
	}
}

func TestFanout_BulkheadRejection(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	gate := make(chan struct{})
	transport.handlers["/bid/slow"] = func(w http.ResponseWriter, _ *http.Request) {
		<-gate
		w.WriteHeader(http.StatusNoContent)
	}

	cfg := Config{
		BidderDeadlineMS:  500,
		BidderMaxInflight: 1, // Only 1 inflight allowed
		CBMinRequests:     20,
		CBFailureRatio:    0.5,
		CBOpenFor:         5 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fanout := NewFanout(cfg, nil, logger, transport)

	bidders := map[string]*catalogv1.Bidder{
		"b1": {Id: "b1", EndpointUrl: "http://fake/bid/slow"},
	}
	cands := []Candidate{
		{Campaign: &CompiledCampaign{ID: "c1", MaxBidCPMMicros: 2_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}},
	}
	req := &BidRequest{ID: "r1", Imp: Impression{ID: "i1"}}

	// Launch first request that occupies the bulkhead
	doneFirst := make(chan struct{})
	go func() {
		defer close(doneFirst)
		_ = fanout.callOne(context.Background(), "b1", bidders["b1"], cands, req)
	}()

	time.Sleep(10 * time.Millisecond) // Let first request acquire semaphore

	// Second request should be rejected by bulkhead
	res := fanout.callOne(context.Background(), "b1", bidders["b1"], cands, req)
	assert.Equal(t, "rejected", res.Outcome)

	close(gate)
	<-doneFirst
}

func TestFanout_DropInvalidBids(t *testing.T) {
	transport := &fakeBidderTransport{
		handlers: make(map[string]http.HandlerFunc),
	}
	transport.handlers["/bid"] = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outboundBidResponse{
			Bids: []RawBid{
				{CampaignID: "unknown-c", CreativeID: "cr1", Price: 1.00}, // Unknown campaign
				{CampaignID: "c1", CreativeID: "cr1", Price: -0.50},       // Negative price
				{CampaignID: "c1", CreativeID: "cr1", Price: 5.00},        // Exceeds max bid 2.00
				{CampaignID: "c1", CreativeID: "cr1", Price: 1.1234567},   // > 6 decimals
				{CampaignID: "c1", CreativeID: "cr1", Price: 1.50},        // Valid
			},
		})
	}

	cfg := Config{
		BidderDeadlineMS:  500,
		BidderMaxInflight: 10,
		CBMinRequests:     20,
		CBFailureRatio:    0.5,
		CBOpenFor:         5 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fanout := NewFanout(cfg, nil, logger, transport)

	bidders := map[string]*catalogv1.Bidder{
		"b1": {Id: "b1", EndpointUrl: "http://fake/bid"},
	}
	cands := []Candidate{
		{Campaign: &CompiledCampaign{ID: "c1", MaxBidCPMMicros: 2_000_000}, Creative: &catalogv1.Creative{Id: "cr1"}},
	}
	req := &BidRequest{ID: "r1", Imp: Impression{ID: "i1"}}

	res := fanout.callOne(context.Background(), "b1", bidders["b1"], cands, req)
	assert.Equal(t, "bid", res.Outcome)
	assert.Len(t, res.Bids, 1)
	assert.Equal(t, int64(1_500_000), res.Bids[0].CPMMicros)
}
