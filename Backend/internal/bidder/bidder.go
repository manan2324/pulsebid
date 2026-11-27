// Package bidder implements configurable mock demand partners for exchange tests.
package bidder

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	mrand "math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"pulsebid/internal/platform/ids"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const maxRequestBodyBytes = 64 * 1024 // 64 KiB

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Server implements an independent mock bidder (DSP) HTTP service.
type Server struct {
	id       string
	port     int
	cfg      atomic.Pointer[Config]
	logger   *slog.Logger
	draining atomic.Bool
	reg      *prometheus.Registry

	reqCounter   *prometheus.CounterVec
	durationHist *prometheus.HistogramVec
	bidsCounter  *prometheus.CounterVec

	randMu sync.Mutex
	rng    *mrand.Rand
}

// NewServer constructs a mock bidder instance with its own metrics registry and configuration.
func NewServer(id string, port int, initial Config, logger *slog.Logger) (*Server, error) {
	if err := initial.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	reg := prometheus.NewRegistry()
	reqCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "http_server_requests_total",
		Help:        "Total HTTP requests handled by this bidder.",
		ConstLabels: prometheus.Labels{"bidder_id": id},
	}, []string{"method", "route", "status"})

	durationHist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "http_server_request_duration_seconds",
		Help:        "HTTP request latency in seconds.",
		ConstLabels: prometheus.Labels{"bidder_id": id},
		Buckets:     []float64{.001, .0025, .005, .01, .02, .035, .05, .075, .1, .12, .15, .25, .5, 1},
	}, []string{"route"})

	bidsCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "bidder_decisions_total",
		Help:        "Total bid decisions by outcome.",
		ConstLabels: prometheus.Labels{"bidder_id": id},
	}, []string{"outcome"})

	reg.MustRegister(reqCounter, durationHist, bidsCounter)

	var seed int64
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		// #nosec G115 -- A random seed intentionally uses all 64 bits, including the sign bit.
		seed = int64(binary.LittleEndian.Uint64(b[:]))
	} else {
		seed = time.Now().UnixNano()
	}

	s := &Server{
		id:           id,
		port:         port,
		logger:       logger.With("bidder_id", id),
		reg:          reg,
		reqCounter:   reqCounter,
		durationHist: durationHist,
		bidsCounter:  bidsCounter,
		rng:          mrand.New(mrand.NewSource(seed)), // #nosec G404 -- simulated traffic behavior
	}
	s.cfg.Store(&initial)
	return s, nil
}

// ID returns this bidder's slug identifier (e.g. "bidder-1").
func (s *Server) ID() string {
	return s.id
}

// Port returns the TCP port this bidder is configured to listen on.
func (s *Server) Port() int {
	return s.port
}

// CurrentConfig returns an immutable copy of the bidder's active runtime configuration.
func (s *Server) CurrentConfig() Config {
	return *s.cfg.Load()
}

// SetDraining marks the server as shutting down (causing /readyz to return 503).
func (s *Server) SetDraining(draining bool) {
	s.draining.Store(draining)
}

// Handler returns the HTTP handler registering all mock-bidder routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /bid", s.handleBid)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /admin/config", s.handleGetConfig)
	mux.HandleFunc("POST /admin/config", s.handlePostConfig)
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID, _ = ids.New()
		}
		w.Header().Set("X-Request-Id", reqID)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		mux.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		dur := time.Since(start).Seconds()
		s.durationHist.WithLabelValues(route).Observe(dur)
		s.reqCounter.WithLabelValues(r.Method, route, fmt.Sprintf("%d", rec.statusCode)).Inc()
	})
}

func (s *Server) randomFloat() float64 {
	s.randMu.Lock()
	defer s.randMu.Unlock()
	return s.rng.Float64()
}

func (s *Server) randomIntn(n int) int {
	if n <= 0 {
		return 0
	}
	s.randMu.Lock()
	defer s.randMu.Unlock()
	return s.rng.Intn(n)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if s.draining.Load() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"draining"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.CurrentConfig()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handlePostConfig(w http.ResponseWriter, r *http.Request) {
	var patch ConfigPatch
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&patch); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"code":"invalid_request","message":%q}}`, err.Error()), http.StatusBadRequest)
		return
	}

	for {
		oldCfg := s.cfg.Load()
		newCfg, err := oldCfg.Apply(patch)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":{"code":"invalid_request","message":%q}}`, err.Error()), http.StatusBadRequest)
			return
		}
		if s.cfg.CompareAndSwap(oldCfg, &newCfg) {
			s.logger.InfoContext(r.Context(), "runtime configuration updated",
				"latency_ms", newCfg.LatencyMS,
				"error_rate", newCfg.ErrorRate,
				"timeout_rate", newCfg.TimeoutRate,
				"bid_mode", newCfg.BidMode,
			)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(newCfg)
			return
		}
	}
}

func (s *Server) handleBid(w http.ResponseWriter, r *http.Request) {
	var req BidRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err := decoder.Decode(&req); err != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"malformed JSON request"}}`))
		s.bidsCounter.WithLabelValues("error").Inc()
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()

	cfg := s.CurrentConfig()

	// 1. Timeout simulation: hang until request context is canceled or up to 2 seconds.
	if cfg.TimeoutRate > 0 && s.randomFloat() < cfg.TimeoutRate {
		s.bidsCounter.WithLabelValues("timeout").Inc()
		t := time.NewTimer(2 * time.Second)
		defer t.Stop()
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		case <-t.C:
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
	}

	// 2. Error rate simulation: return HTTP 500.
	if cfg.ErrorRate > 0 && s.randomFloat() < cfg.ErrorRate {
		s.bidsCounter.WithLabelValues("error").Inc()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"simulated bidder failure"}}`))
		return
	}

	// 3. Latency + Jitter delay simulation.
	totalDelay := time.Duration(cfg.LatencyMS) * time.Millisecond
	if cfg.JitterMS > 0 {
		jitter := s.randomIntn(cfg.JitterMS + 1)
		totalDelay += time.Duration(jitter) * time.Millisecond
	}
	if totalDelay > 0 {
		t := time.NewTimer(totalDelay)
		defer t.Stop()
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusRequestTimeout)
			return
		case <-t.C:
		}
	}

	// 4. No-bid rate simulation: return 204.
	if (cfg.NoBidRate >= 1.0) || (cfg.BidMode != BidModeFixed && cfg.NoBidRate > 0 && s.randomFloat() < cfg.NoBidRate) {
		s.bidsCounter.WithLabelValues("nobid").Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// 5. Evaluate candidate bids.
	if len(req.Candidates) == 0 {
		s.bidsCounter.WithLabelValues("nobid").Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	floor := math.Max(0, req.Imp.Bidfloor)
	bids := make([]Bid, 0, len(req.Candidates))

	for _, cand := range req.Candidates {
		if cand.MaxBidCPM < floor {
			continue
		}

		var price float64
		switch cfg.BidMode {
		case BidModeFixed:
			price = cand.MaxBidCPM
		case BidModeShade:
			price = cand.MaxBidCPM * cfg.BidShade
		case BidModeRandom:
			// Deterministic seed per (seed + hash(request_id + campaign_id))
			h := fnv.New64a()
			_, _ = h.Write([]byte(req.RequestID))
			_, _ = h.Write([]byte(cand.CampaignID))
			// #nosec G115 -- Deterministic seed mixing intentionally wraps modulo 2^64.
			subSeed := cfg.Seed + int64(h.Sum64())
			subRng := mrand.New(mrand.NewSource(subSeed)) // #nosec G404 -- deterministic test seed

			span := cand.MaxBidCPM - floor
			if span > 0 {
				price = floor + subRng.Float64()*span
			} else {
				price = floor
			}
		}

		// Ensure precision is capped to 6 decimal places without float drift.
		price = math.Round(price*1e6) / 1e6

		if price >= floor && price <= cand.MaxBidCPM {
			bids = append(bids, Bid{
				CampaignID: cand.CampaignID,
				CreativeID: cand.CreativeID,
				Price:      price,
			})
		}
	}

	if len(bids) == 0 {
		s.bidsCounter.WithLabelValues("nobid").Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.bidsCounter.WithLabelValues("bid").Inc()
	respBytes, err := json.Marshal(BidResponse{Bids: bids})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}
