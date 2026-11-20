package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/budget"
	"pulsebid/internal/events"
	"pulsebid/internal/events/token"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/ids"
	"pulsebid/internal/platform/logging"

	"go.opentelemetry.io/otel"
)

// Handler processes incoming /v1/bid HTTP requests.
type Handler struct {
	cfg       Config
	snapshot  *SnapshotCache
	targeting *TargetingEngine
	fanout    *Fanout
	reserver  Reserver
	exhausted *ExhaustedCache
	events    EventPublisher
	metrics   *Metrics
	logger    *slog.Logger
	clk       clock.Clock
}

// TargetingEngine wraps campaign targeting evaluation.
type TargetingEngine struct{}

// NewHandler constructs a fully configured bid handler.
func NewHandler(
	cfg Config,
	snapshot *SnapshotCache,
	fanout *Fanout,
	reserver Reserver,
	events EventPublisher,
	m *Metrics,
	logger *slog.Logger,
	clk clock.Clock,
) *Handler {
	if clk == nil {
		clk = clock.Real{}
	}
	if reserver == nil {
		reserver = StubReserver{}
	}
	if events == nil {
		events = NoopEventPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.EventTokenSecret == "" {
		cfg.EventTokenSecret = "local-development-only-replace-before-deploying"
	}
	if cfg.EventTokenTTL <= 0 {
		cfg.EventTokenTTL = 24 * time.Hour
	}
	return &Handler{
		cfg:       cfg,
		snapshot:  snapshot,
		targeting: &TargetingEngine{},
		fanout:    fanout,
		reserver:  reserver,
		exhausted: NewExhaustedCache(5 * time.Second),
		events:    events,
		metrics:   m,
		logger:    logger,
		clk:       clk,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 1. Decode and validate request payload
	_, valSpan := otel.Tracer("pulsebid/gateway").Start(r.Context(), "bid.validate")
	var req BidRequest
	decodeErr := httpx.DecodeJSON(w, r, &req, h.cfg.RequestMaxBytes, false)
	if decodeErr == nil {
		decodeErr = req.Validate()
	}
	valSpan.End()
	if decodeErr != nil {
		h.recordOutcome(r.Context(), "invalid", start, req.ID)
		httpx.WriteError(w, r, decodeErr)
		return
	}

	// 2. Determine "now" time (support test override if enabled)
	now := h.clk.Now().UTC()
	if h.cfg.EnableTestEndpoints {
		if testTimeHdr := r.Header.Get("X-PulseBid-Test-Time"); testTimeHdr != "" {
			if t, err := time.Parse(time.RFC3339, testTimeHdr); err == nil {
				now = t.UTC()
			}
		}
		if r.Header.Get("X-PulseBid-Clear-Exhausted") == "true" {
			h.exhausted.Clear()
		}
	}

	// 3. Compute fanout deadline anchored to request arrival
	deadlineMS := h.cfg.BidderDeadlineMS
	if req.TMax > 0 {
		tmax := req.TMax
		if tmax < 10 {
			tmax = 10
		} else if tmax > 1000 {
			tmax = 1000
		}
		if tmax < deadlineMS {
			deadlineMS = tmax
		}
	}
	deadline := start.Add(time.Duration(deadlineMS) * time.Millisecond)

	// 4. Retrieve current snapshot
	snap := h.snapshot.Get()
	if snap == nil {
		h.recordOutcome(r.Context(), "error", start, req.ID)
		h.writeNoFill(w, r, "error", "", start)
		return
	}

	// 5. Evaluate targeting
	_, tgtSpan := otel.Tracer("pulsebid/gateway").Start(r.Context(), "bid.targeting")
	isExhausted := func(id string) bool {
		return h.exhausted.IsExhausted(id, now)
	}
	groups := GroupCandidates(snap.Campaigns, &req, now, isExhausted)
	tgtSpan.End()
	eligibleCount := len(groups)
	if eligibleCount == 0 {
		debug := ""
		if h.cfg.EnableTestEndpoints {
			debug = "eligible=0;responded=0;timed_out=0;errors=0;valid_bids=0;attempts=0"
		}
		h.recordOutcome(r.Context(), "no_eligible", start, req.ID)
		h.writeNoFill(w, r, "no_eligible", debug, start)
		return
	}

	// 6. Fan out to candidate bidders in parallel
	fanoutCtx, fanoutSpan := otel.Tracer("pulsebid/gateway").Start(r.Context(), "bid.fanout")
	results := h.fanout.Run(fanoutCtx, deadline, groups, &req, snap.Bidders)
	fanoutSpan.End()

	// 7. Aggregate bids and statistics
	var (
		allValidBids      []Bid
		respondedCount    int
		timedOutCount     int
		errorsCount       int
		validBidsCount    int
		hadBidsBelowFloor bool
	)

	floorMicros := req.FloorMicros()

	for _, res := range results {
		switch res.Outcome {
		case "bid":
			respondedCount++
			for _, b := range res.Bids {
				validBidsCount++
				if b.CPMMicros >= floorMicros {
					allValidBids = append(allValidBids, b)
				} else {
					hadBidsBelowFloor = true
				}
			}
		case "nobid":
			respondedCount++
		case "timeout":
			timedOutCount++
		case "error", "circuit_open", "rejected":
			errorsCount++
		}
	}

	// Any eligible candidate bidder that did not deliver a result before deadline timed out.
	if missing := eligibleCount - (respondedCount + timedOutCount + errorsCount); missing > 0 {
		timedOutCount += missing
	}

	formatDebug := func(attempts int) string {
		if !h.cfg.EnableTestEndpoints {
			return ""
		}
		return fmt.Sprintf("eligible=%d;responded=%d;timed_out=%d;errors=%d;valid_bids=%d;attempts=%d",
			eligibleCount, respondedCount, timedOutCount, errorsCount, validBidsCount, attempts)
	}

	// 8. Run auction
	if len(allValidBids) == 0 {
		allCandidatesBelowFloor := len(groups) > 0
		for _, cands := range groups {
			for _, cand := range cands {
				if cand.Campaign.MaxBidCPMMicros >= floorMicros {
					allCandidatesBelowFloor = false
					break
				}
			}
			if !allCandidatesBelowFloor {
				break
			}
		}

		reason := "no_bids"
		if hadBidsBelowFloor || allCandidatesBelowFloor {
			reason = "below_floor"
		}
		h.recordOutcome(r.Context(), reason, start, req.ID)
		h.writeNoFill(w, r, reason, formatDebug(0), start)
		return
	}

	// 8 & 9. Auction + Budget reserve loop (Section 12.4)
	bids := make([]Bid, len(allValidBids))
	copy(bids, allValidBids)

	var participants []string
	seenParticipants := make(map[string]struct{})
	for _, b := range allValidBids {
		if _, exists := seenParticipants[b.CampaignID]; !exists {
			seenParticipants[b.CampaignID] = struct{}{}
			participants = append(participants, b.CampaignID)
		}
	}

	maxAttempts := h.cfg.MaxAuctionAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var auctionRes AuctionResult
	var finalCostMicros int64
	attemptsMade := 0

	_, aucSpan := otel.Tracer("pulsebid/gateway").Start(r.Context(), "bid.auction")
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptsMade = attempt
		res, ok := RunAuction(bids, floorMicros)
		if !ok {
			aucSpan.End()
			if attempt > 1 {
				if len(allValidBids) > 0 {
					blockedBidID, _ := ids.New()
					h.events.TryPublishAsync(&events.Event{
						SchemaVersion:    events.CurrentSchemaVersion,
						EventID:          events.EventID(events.TypeAuction, blockedBidID),
						Type:             events.TypeAuction,
						OccurredAt:       now,
						RequestID:        req.ID,
						BidID:            blockedBidID,
						CampaignID:       "",
						SpendMicros:      0,
						Participants:     participants,
						WinnerCampaignID: "",
					})
				}
				h.recordOutcome(r.Context(), "budget", start, req.ID)
				h.writeNoFill(w, r, "budget", formatDebug(attemptsMade), start)
				return
			}
			h.recordOutcome(r.Context(), "below_floor", start, req.ID)
			h.writeNoFill(w, r, "below_floor", formatDebug(attemptsMade), start)
			return
		}

		costMicros := (res.ClearingCPM + 999) / 1000 // ceil to nearest micro

		camp := snap.CampaignsByID[res.Winner.CampaignID]
		pacingStr := "even"
		var dailyBudget int64 = 1_000_000_000
		if camp != nil {
			if camp.PacingMode == catalogv1.PacingMode_PACING_MODE_ASAP {
				pacingStr = "asap"
			}
			dailyBudget = camp.DailyBudgetMicros
		}
		campBudget := budget.CampaignBudget{
			ID:                  res.Winner.CampaignID,
			DailyBudgetMicros:   dailyBudget,
			PacingMode:          pacingStr,
			PacingBurstFraction: h.cfg.PacingBurstFraction,
		}

		reserveTimeout := time.Duration(h.cfg.RedisOpTimeoutMS) * time.Millisecond
		if reserveTimeout <= 0 {
			reserveTimeout = 10 * time.Millisecond
		}
		if dl, ok := r.Context().Deadline(); ok {
			remaining := time.Until(dl)
			if remaining <= 0 {
				aucSpan.End()
				h.recordOutcome(r.Context(), "error", start, req.ID)
				h.writeNoFill(w, r, "error", formatDebug(attemptsMade), start)
				return
			}
			if remaining < reserveTimeout {
				reserveTimeout = remaining
			}
		}

		reserveCtx, cancel := context.WithTimeout(r.Context(), reserveTimeout)
		bCtx, bSpan := otel.Tracer("pulsebid/gateway").Start(reserveCtx, "bid.budget_reserve")
		resResult, err := h.reserver.Reserve(bCtx, campBudget, costMicros, now)
		bSpan.End()
		cancel()

		if err != nil {
			aucSpan.End()
			h.recordOutcome(r.Context(), "error", start, req.ID)
			h.writeNoFill(w, r, "error", formatDebug(attemptsMade), start)
			return
		}

		switch resResult {
		case budget.Reserved:
			auctionRes = res
			finalCostMicros = costMicros
			goto AuctionDone
		case budget.Exhausted:
			h.exhausted.Mark(res.Winner.CampaignID, now)
			bids = removeCampaignBids(bids, res.Winner.CampaignID)
		case budget.Paced:
			bids = removeCampaignBids(bids, res.Winner.CampaignID)
		}
	}

	aucSpan.End()
	if len(allValidBids) > 0 {
		blockedBidID, _ := ids.New()
		h.events.TryPublishAsync(&events.Event{
			SchemaVersion:    events.CurrentSchemaVersion,
			EventID:          events.EventID(events.TypeAuction, blockedBidID),
			Type:             events.TypeAuction,
			OccurredAt:       now,
			RequestID:        req.ID,
			BidID:            blockedBidID,
			CampaignID:       "",
			SpendMicros:      0,
			Participants:     participants,
			WinnerCampaignID: "",
		})
	}
	h.recordOutcome(r.Context(), "budget", start, req.ID)
	h.writeNoFill(w, r, "budget", formatDebug(attemptsMade), start)
	return

AuctionDone:
	aucSpan.End()
	if h.metrics != nil && h.metrics.AuctionClearingPriceMicros != nil {
		h.metrics.AuctionClearingPriceMicros.Observe(float64(auctionRes.ClearingCPM))
	}
	_, respSpan := otel.Tracer("pulsebid/gateway").Start(r.Context(), "bid.respond")
	defer respSpan.End()

	debugHeader := formatDebug(attemptsMade)

	// 10. Build 200 response
	bidID, err := ids.New()
	if err != nil {
		h.recordOutcome(r.Context(), "error", start, req.ID)
		h.writeNoFill(w, r, "error", debugHeader, start)
		return
	}

	landingURL := auctionRes.Winner.LandingURL
	if landingURL == "" && snap != nil {
		if camp := snap.CampaignsByID[auctionRes.Winner.CampaignID]; camp != nil {
			for _, cr := range camp.Creatives {
				if cr.GetId() == auctionRes.Winner.CreativeID {
					landingURL = cr.GetLandingUrl()
					break
				}
			}
		}
	}
	if landingURL == "" {
		landingURL = "https://example.com"
	}

	tokPayload := token.Payload{
		BidID:        bidID,
		RequestID:    req.ID,
		CampaignID:   auctionRes.Winner.CampaignID,
		CreativeID:   auctionRes.Winner.CreativeID,
		AdvertiserID: auctionRes.Winner.AdvertiserID,
		BidderID:     auctionRes.Winner.BidderID,
		PriceMicros:  finalCostMicros,
		LandingURL:   landingURL,
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(h.cfg.EventTokenTTL).Unix(),
	}

	signedToken, err := token.Sign([]byte(h.cfg.EventTokenSecret), tokPayload)
	if err != nil {
		h.recordOutcome(r.Context(), "error", start, req.ID)
		h.writeNoFill(w, r, "error", debugHeader, start)
		return
	}

	impressionURL := fmt.Sprintf("%s/v1/events/impression?t=%s", h.cfg.PublicBaseURL, signedToken)
	clickURL := fmt.Sprintf("%s/v1/events/click?t=%s", h.cfg.PublicBaseURL, signedToken)

	adm := strings.ReplaceAll(auctionRes.Winner.Adm, "{impression_url}", impressionURL)
	adm = strings.ReplaceAll(adm, "{click_url}", clickURL)

	// Emit auction event for the filled auction
	if len(participants) > 0 {
		h.events.TryPublishAsync(&events.Event{
			SchemaVersion:    events.CurrentSchemaVersion,
			EventID:          events.EventID(events.TypeAuction, bidID),
			Type:             events.TypeAuction,
			OccurredAt:       now,
			RequestID:        req.ID,
			BidID:            bidID,
			CampaignID:       auctionRes.Winner.CampaignID,
			CreativeID:       auctionRes.Winner.CreativeID,
			AdvertiserID:     auctionRes.Winner.AdvertiserID,
			BidderID:         auctionRes.Winner.BidderID,
			SpendMicros:      finalCostMicros,
			Participants:     participants,
			WinnerCampaignID: auctionRes.Winner.CampaignID,
		})
	}

	resp := BidResponse{
		RequestID:     req.ID,
		BidID:         bidID,
		CampaignID:    auctionRes.Winner.CampaignID,
		CreativeID:    auctionRes.Winner.CreativeID,
		AdvertiserID:  auctionRes.Winner.AdvertiserID,
		BidderID:      auctionRes.Winner.BidderID,
		Price:         float64(auctionRes.ClearingCPM) / 1e6,
		Currency:      "USD",
		Adm:           adm,
		ImpressionURL: impressionURL,
		ClickURL:      clickURL,
		W:             auctionRes.Winner.Width,
		H:             auctionRes.Winner.Height,
	}

	durMS := float64(time.Since(start).Microseconds()) / 1000.0
	w.Header().Set("Server-Timing", fmt.Sprintf("total;dur=%.1f", durMS))
	if h.cfg.EnableTestEndpoints && debugHeader != "" {
		w.Header().Set("X-PulseBid-Debug", debugHeader)
	}

	h.recordOutcome(r.Context(), "filled", start, req.ID)
	httpx.WriteJSON(w, r, http.StatusOK, resp)
}

func (h *Handler) writeNoFill(w http.ResponseWriter, r *http.Request, reason, debug string, start time.Time) {
	durMS := float64(time.Since(start).Microseconds()) / 1000.0
	w.Header().Set("Server-Timing", fmt.Sprintf("total;dur=%.1f", durMS))
	w.Header().Set("X-PulseBid-Reason", reason)
	if h.cfg.EnableTestEndpoints && debug != "" {
		w.Header().Set("X-PulseBid-Debug", debug)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) recordOutcome(ctx context.Context, outcome string, start time.Time, reqID string) {
	dur := time.Since(start)
	if h.metrics != nil {
		h.metrics.BidRequestsTotal.WithLabelValues(outcome).Inc()
		h.metrics.BidDurationSeconds.WithLabelValues(outcome).Observe(dur.Seconds())
	}
	if h.logger != nil {
		logCtx := ctx
		if reqID != "" && logging.RequestID(logCtx) == "" {
			logCtx = logging.WithRequestID(logCtx, reqID)
		}
		durMS := dur.Milliseconds()
		if dur > 110*time.Millisecond || outcome == "error" {
			h.logger.WarnContext(logCtx, "bid processed", "outcome", outcome, "duration_ms", durMS)
		} else {
			h.logger.DebugContext(logCtx, "bid processed", "outcome", outcome, "duration_ms", durMS)
		}
	}
}

func removeCampaignBids(bids []Bid, campaignID string) []Bid {
	filtered := make([]Bid, 0, len(bids))
	for _, b := range bids {
		if b.CampaignID != campaignID {
			filtered = append(filtered, b)
		}
	}
	return filtered
}
