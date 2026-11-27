package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/platform/money"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/semaphore"
)

// RawBid represents a bid response from a DSP.
type RawBid struct {
	CampaignID string  `json:"campaign_id"`
	CreativeID string  `json:"creative_id"`
	Price      float64 `json:"price"`
}

// BidderResult records the outcome of querying a single bidder.
type BidderResult struct {
	BidderID string
	Bids     []Bid
	RawBids  []RawBid
	Outcome  string // bid | nobid | timeout | error | circuit_open | rejected
	Latency  time.Duration
}

// Fanout orchestrates parallel calls to external bidders with isolation and circuit breakers.
type Fanout struct {
	tracer         trace.Tracer
	client         *http.Client
	metrics        *Metrics
	logger         *slog.Logger
	maxInflight    int
	cbMinRequests  uint32
	cbFailureRatio float64
	cbOpenFor      time.Duration

	mu        sync.RWMutex
	breakers  map[string]*gobreaker.TwoStepCircuitBreaker[any]
	bulkheads map[string]*semaphore.Weighted
}

// NewFanout initializes a new Fanout engine.
func NewFanout(cfg Config, m *Metrics, logger *slog.Logger, transport http.RoundTripper) *Fanout {
	if transport == nil {
		transport = &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:   false,
			MaxIdleConns:        4096,
			MaxIdleConnsPerHost: 512,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
		}
	}
	return &Fanout{
		tracer:         otel.Tracer("pulsebid/gateway"),
		client:         &http.Client{Transport: transport},
		metrics:        m,
		logger:         logger,
		maxInflight:    cfg.BidderMaxInflight,
		cbMinRequests:  cfg.CBMinRequests,
		cbFailureRatio: cfg.CBFailureRatio,
		cbOpenFor:      cfg.CBOpenFor,
		breakers:       make(map[string]*gobreaker.TwoStepCircuitBreaker[any]),
		bulkheads:      make(map[string]*semaphore.Weighted),
	}
}

func (f *Fanout) getBreaker(bidderID string) *gobreaker.TwoStepCircuitBreaker[any] {
	f.mu.RLock()
	cb, ok := f.breakers[bidderID]
	f.mu.RUnlock()
	if ok {
		return cb
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	cb, ok = f.breakers[bidderID]
	if !ok {
		cb = gobreaker.NewTwoStepCircuitBreaker[any](gobreaker.Settings{
			Name:        bidderID,
			MaxRequests: 1,
			Timeout:     f.cbOpenFor,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.Requests >= f.cbMinRequests &&
					(float64(counts.TotalFailures)/float64(counts.Requests)) >= f.cbFailureRatio
			},
			OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
				if f.logger != nil {
					f.logger.Warn("bidder circuit breaker state transition",
						"bidder_id", name,
						"from", from.String(),
						"to", to.String(),
					)
				}
			},
		})
		f.breakers[bidderID] = cb
	}
	return cb
}

func (f *Fanout) getBulkhead(bidderID string) *semaphore.Weighted {
	f.mu.RLock()
	bh, ok := f.bulkheads[bidderID]
	f.mu.RUnlock()
	if ok {
		return bh
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	bh, ok = f.bulkheads[bidderID]
	if !ok {
		bh = semaphore.NewWeighted(int64(f.maxInflight))
		f.bulkheads[bidderID] = bh
	}
	return bh
}

// Run fans out to all candidate groups in parallel, waiting at most until deadline.
func (f *Fanout) Run(ctx context.Context, deadline time.Time, groups map[string][]Candidate, req *BidRequest, bidders map[string]*catalogv1.Bidder) []BidderResult {
	if len(groups) == 0 {
		return nil
	}

	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	results := make(chan BidderResult, len(groups))
	for id, cands := range groups {
		bidder := bidders[id]
		if bidder == nil {
			results <- BidderResult{BidderID: id, Outcome: "error"}
			continue
		}
		go func(bidderID string, bidderInfo *catalogv1.Bidder, cs []Candidate) {
			results <- f.callOne(ctx, bidderID, bidderInfo, cs, req)
		}(id, bidder, cands)
	}

	out := make([]BidderResult, 0, len(groups))
	for len(out) < len(groups) {
		select {
		case r := <-results:
			out = append(out, r)
		case <-ctx.Done():
			return out
		}
	}
	return out
}

type bidderReqCandidate struct {
	CampaignID string  `json:"campaign_id"`
	CreativeID string  `json:"creative_id"`
	MaxBidCPM  float64 `json:"max_bid_cpm"`
}

type bidderReqBanner struct {
	W int `json:"w"`
	H int `json:"h"`
}

type bidderReqImp struct {
	ID       string           `json:"id"`
	Banner   *bidderReqBanner `json:"banner,omitempty"`
	BidFloor float64          `json:"bidfloor"`
}

type outboundBidRequest struct {
	ID         string               `json:"id"`
	Imp        bidderReqImp         `json:"imp"`
	Device     *Device              `json:"device,omitempty"`
	Geo        *Geo                 `json:"geo,omitempty"`
	Candidates []bidderReqCandidate `json:"candidates"`
}

type outboundBidResponse struct {
	Bids []RawBid `json:"bids"`
}

func (f *Fanout) callOne(ctx context.Context, bidderID string, bidder *catalogv1.Bidder, cands []Candidate, req *BidRequest) (res BidderResult) {
	start := time.Now()

	callCtx, span := f.tracer.Start(ctx, "bidder.call",
		trace.WithAttributes(attribute.String("bidder.id", bidderID)),
	)
	defer func() {
		if res.Outcome != "" {
			span.SetAttributes(attribute.String("outcome", res.Outcome))
		}
		span.End()
	}()

	// 1. Circuit breaker check
	cb := f.getBreaker(bidderID)
	done, err := cb.Allow()
	if err != nil {
		f.record(bidderID, "circuit_open", time.Since(start))
		return BidderResult{BidderID: bidderID, Outcome: "circuit_open", Latency: time.Since(start)}
	}

	// 2. Bulkhead check
	bh := f.getBulkhead(bidderID)
	if !bh.TryAcquire(1) {
		done(nil) // rejected does not count as breaker failure
		f.record(bidderID, "rejected", time.Since(start))
		return BidderResult{BidderID: bidderID, Outcome: "rejected", Latency: time.Since(start)}
	}
	if f.metrics != nil && f.metrics.BidderInflight != nil {
		f.metrics.BidderInflight.WithLabelValues(bidderID).Inc()
	}
	defer func() {
		if f.metrics != nil && f.metrics.BidderInflight != nil {
			f.metrics.BidderInflight.WithLabelValues(bidderID).Dec()
		}
		bh.Release(1)
	}()

	// 3. Compute per-bidder timeout
	callTimeout := 5 * time.Second
	if deadline, ok := callCtx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			done(nil) // Caller context expired before invocation, not an upstream bidder failure
			f.record(bidderID, "timeout", time.Since(start))
			return BidderResult{BidderID: bidderID, Outcome: "timeout", Latency: time.Since(start)}
		}
		callTimeout = remaining
	}

	if bidder.GetTimeoutMs() > 0 {
		bt := time.Duration(bidder.GetTimeoutMs()) * time.Millisecond
		if bt < callTimeout {
			callTimeout = bt
		}
	}

	reqCtx, cancel := context.WithTimeout(callCtx, callTimeout)

	// 4. Construct payload
	candList := make([]bidderReqCandidate, 0, len(cands))
	for _, c := range cands {
		candList = append(candList, bidderReqCandidate{
			CampaignID: c.Campaign.ID,
			CreativeID: c.Creative.GetId(),
			MaxBidCPM:  float64(c.Campaign.MaxBidCPMMicros) / 1e6,
		})
	}

	outImp := bidderReqImp{
		ID:       req.Imp.ID,
		BidFloor: float64(req.FloorMicros()) / 1e6,
	}
	if len(cands) > 0 && cands[0].Creative.GetWidth() > 0 && cands[0].Creative.GetHeight() > 0 {
		outImp.Banner = &bidderReqBanner{
			W: int(cands[0].Creative.GetWidth()),
			H: int(cands[0].Creative.GetHeight()),
		}
	}

	outReq := outboundBidRequest{
		ID:         req.ID,
		Imp:        outImp,
		Device:     req.Device,
		Geo:        req.Geo,
		Candidates: candList,
	}

	// A transport may finish closing a request body after Do returns an error.
	// Give each request its own bytes; returning a pooled bytes.Buffer here can
	// overwrite a body that an asynchronous transport writer still owns.
	payload, err := json.Marshal(outReq)
	if err != nil {
		cancel()
		done(err)
		f.record(bidderID, "error", time.Since(start))
		return BidderResult{BidderID: bidderID, Outcome: "error", Latency: time.Since(start)}
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", bidder.GetEndpointUrl(), bytes.NewReader(payload))
	if err != nil {
		cancel()
		done(err)
		f.record(bidderID, "error", time.Since(start))
		return BidderResult{BidderID: bidderID, Outcome: "error", Latency: time.Since(start)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-Id", req.ID)
	otel.GetTextMapPropagator().Inject(reqCtx, propagation.HeaderCarrier(httpReq.Header))

	// 5. Send HTTP request
	httpResp, err := f.client.Do(httpReq)
	latency := time.Since(start)

	if err != nil {
		timedOut := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || reqCtx.Err() != nil
		cancel()
		if errors.Is(callCtx.Err(), context.Canceled) {
			done(nil) // A caller cancellation is not a bidder failure.
		} else {
			done(err)
		}
		outcome := "error"
		if timedOut {
			outcome = "timeout"
		}
		f.record(bidderID, outcome, latency)
		return BidderResult{BidderID: bidderID, Outcome: outcome, Latency: latency}
	}

	const maxResponseBytes = 64 * 1024
	bodyBytes, readErr := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes+1))
	if len(bodyBytes) > maxResponseBytes {
		readErr = fmt.Errorf("bidder response exceeds %d bytes", maxResponseBytes)
	}
	if readErr == nil {
		readErr = reqCtx.Err()
	}
	_ = httpResp.Body.Close()
	timedOut := errors.Is(readErr, context.DeadlineExceeded) || reqCtx.Err() != nil
	cancel()
	latency = time.Since(start)
	if readErr != nil {
		if errors.Is(callCtx.Err(), context.Canceled) {
			done(nil)
		} else {
			done(readErr)
		}
		outcome := "error"
		if timedOut {
			outcome = "timeout"
		}
		f.record(bidderID, outcome, latency)
		return BidderResult{BidderID: bidderID, Outcome: outcome, Latency: latency}
	}

	if httpResp.StatusCode == http.StatusNoContent {
		done(nil)
		f.record(bidderID, "nobid", latency)
		return BidderResult{BidderID: bidderID, Outcome: "nobid", Latency: latency}
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		done(fmt.Errorf("bidder returned HTTP %d", httpResp.StatusCode))
		f.record(bidderID, "error", latency)
		return BidderResult{BidderID: bidderID, Outcome: "error", Latency: latency}
	}

	// 6. Decode response body
	var respPayload outboundBidResponse
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &respPayload); err != nil {
			if errors.Is(callCtx.Err(), context.Canceled) {
				done(nil)
			} else {
				done(err)
			}
			outcome := "error"
			if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
				outcome = "timeout"
			}
			f.record(bidderID, outcome, latency)
			return BidderResult{BidderID: bidderID, Outcome: outcome, Latency: latency}
		}
	}

	done(nil)

	if len(respPayload.Bids) == 0 {
		f.record(bidderID, "nobid", latency)
		return BidderResult{BidderID: bidderID, Outcome: "nobid", Latency: latency}
	}

	// 7. Post-validate bids
	candLookup := make(map[string]Candidate, len(cands))
	for _, c := range cands {
		candLookup[c.Campaign.ID+":"+c.Creative.GetId()] = c
	}

	validBids := make([]Bid, 0, len(respPayload.Bids))
	for _, raw := range respPayload.Bids {
		cand, ok := candLookup[raw.CampaignID+":"+raw.CreativeID]
		if !ok {
			f.dropInvalid("not_in_candidate_list")
			continue
		}
		if raw.Price <= 0 {
			f.dropInvalid("price_non_positive")
			continue
		}
		// Validate at most 6 decimal places and exact bounds
		priceStr := strconv.FormatFloat(raw.Price, 'f', -1, 64)
		cpmMicros, err := money.FromDecimalString(priceStr)
		if err != nil || int64(cpmMicros) <= 0 || int64(cpmMicros) > cand.Campaign.MaxBidCPMMicros {
			f.dropInvalid("price_invalid_or_exceeds_max")
			continue
		}

		validBids = append(validBids, Bid{
			CampaignID:   cand.Campaign.ID,
			CreativeID:   cand.Creative.GetId(),
			AdvertiserID: cand.Campaign.AdvertiserID,
			BidderID:     bidderID,
			CPMMicros:    int64(cpmMicros),
			Adm:          cand.Creative.GetMarkup(),
			LandingURL:   cand.Creative.GetLandingUrl(),
			Width:        int(cand.Creative.GetWidth()),
			Height:       int(cand.Creative.GetHeight()),
		})
	}

	outcome := "bid"
	if len(validBids) == 0 {
		outcome = "nobid"
	}
	f.record(bidderID, outcome, latency)

	return BidderResult{
		BidderID: bidderID,
		Bids:     validBids,
		RawBids:  respPayload.Bids,
		Outcome:  outcome,
		Latency:  latency,
	}
}

func (f *Fanout) record(bidderID, outcome string, latency time.Duration) {
	if f.metrics != nil {
		f.metrics.BidderRequestsTotal.WithLabelValues(bidderID, outcome).Inc()
		f.metrics.BidderLatencySecs.WithLabelValues(bidderID).Observe(latency.Seconds())
	}
}

func (f *Fanout) dropInvalid(reason string) {
	if f.metrics != nil && f.metrics.BidsInvalidTotal != nil {
		f.metrics.BidsInvalidTotal.WithLabelValues(reason).Inc()
	}
}
