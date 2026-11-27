package gateway

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus collectors specific to the bid-gateway service.
type Metrics struct {
	BidRequestsTotal           *prometheus.CounterVec
	BidDurationSeconds         *prometheus.HistogramVec
	BidderRequestsTotal        *prometheus.CounterVec
	BidderLatencySecs          *prometheus.HistogramVec
	BidderInflight             *prometheus.GaugeVec
	BidsInvalidTotal           *prometheus.CounterVec
	AuctionClearingPriceMicros prometheus.Histogram
	SnapshotVersion            prometheus.Gauge
	SnapshotAgeSeconds         prometheus.Gauge
	SnapshotCampaigns          prometheus.Gauge
}

// NewMetrics initializes and registers gateway metrics with the given registry.
func NewMetrics(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		BidRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bid_requests_total",
				Help: "Total number of bid requests processed by the gateway.",
			},
			[]string{"outcome"},
		),
		BidDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bid_duration_seconds",
				Help:    "Duration of bid requests processed by the gateway.",
				Buckets: []float64{.001, .0025, .005, .01, .02, .035, .05, .075, .1, .12, .15, .25, .5, 1},
			},
			[]string{"outcome"},
		),
		BidderRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bidder_requests_total",
				Help: "Outbound bidder requests by bidder and outcome.",
			},
			[]string{"bidder_id", "outcome"},
		),
		BidderLatencySecs: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "bidder_latency_seconds",
				Help:    "Latency of outbound requests to bidders.",
				Buckets: []float64{.001, .0025, .005, .01, .02, .035, .05, .075, .1, .12, .15, .25, .5, 1},
			},
			[]string{"bidder_id"},
		),
		BidderInflight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bidder_inflight",
				Help: "Current number of in-flight requests to this bidder.",
			},
			[]string{"bidder_id"},
		),
		BidsInvalidTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "bids_invalid_total",
				Help: "Count of invalid bids dropped during post-validation.",
			},
			[]string{"reason"},
		),
		AuctionClearingPriceMicros: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "auction_clearing_price_micros",
				Help:    "Clearing prices of winning bids in micros.",
				Buckets: []float64{100_000, 500_000, 1_000_000, 2_000_000, 3_000_000, 5_000_000, 10_000_000, 20_000_000},
			},
		),
		SnapshotVersion: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "snapshot_version",
				Help: "Current version of the loaded catalog snapshot.",
			},
		),
		SnapshotAgeSeconds: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "snapshot_age_seconds",
				Help: "Age in seconds of the currently loaded snapshot since generated_at.",
			},
		),
		SnapshotCampaigns: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "snapshot_campaigns",
				Help: "Number of active campaigns in currently loaded catalog snapshot.",
			},
		),
	}

	reg.MustRegister(
		m.BidRequestsTotal,
		m.BidDurationSeconds,
		m.BidderRequestsTotal,
		m.BidderLatencySecs,
		m.BidderInflight,
		m.BidsInvalidTotal,
		m.AuctionClearingPriceMicros,
		m.SnapshotVersion,
		m.SnapshotAgeSeconds,
		m.SnapshotCampaigns,
	)
	return m
}
