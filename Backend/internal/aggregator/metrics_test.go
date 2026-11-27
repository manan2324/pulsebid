package aggregator

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestMetrics_Registration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	require.NotNil(t, m)

	m.BatchSize.Observe(10)
	m.BatchDuration.Observe(0.05)
	m.EventsTotal.WithLabelValues("impression", "new").Inc()
	m.EventsTotal.WithLabelValues("impression", "duplicate").Inc()
	m.EventsTotal.WithLabelValues("unknown", "dlq").Inc()
	m.ConsumerLag.WithLabelValues("0").Set(42)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	require.True(t, names["aggregator_batch_size"])
	require.True(t, names["aggregator_batch_duration_seconds"])
	require.True(t, names["aggregator_events_total"])
	require.True(t, names["aggregator_consumer_lag"])
}
