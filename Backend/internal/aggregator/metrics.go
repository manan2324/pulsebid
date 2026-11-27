package aggregator

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the aggregator metrics specified in Section 13.1.
type Metrics struct {
	BatchSize     prometheus.Histogram
	BatchDuration prometheus.Histogram
	EventsTotal   *prometheus.CounterVec
	ConsumerLag   *prometheus.GaugeVec
}

// NewMetrics initializes and registers aggregator metrics with the given registry.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		BatchSize: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "aggregator_batch_size",
				Help:    "Number of events processed in a single consumer batch.",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000},
			},
		),
		BatchDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "aggregator_batch_duration_seconds",
				Help:    "Total time spent processing and persisting a consumer batch in seconds.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
		),
		EventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aggregator_events_total",
				Help: "Total events consumed by event-aggregator partitioned by type and result (new, duplicate, dlq).",
			},
			[]string{"type", "result"},
		),
		ConsumerLag: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "aggregator_consumer_lag",
				Help: "Current Kafka consumer lag per partition.",
			},
			[]string{"partition"},
		),
	}

	if reg != nil {
		reg.MustRegister(m.BatchSize, m.BatchDuration, m.EventsTotal, m.ConsumerLag)
	}

	return m
}
