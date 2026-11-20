package aggregator

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

func isKafkaAvailable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestDLQ_Publish(t *testing.T) {
	broker := "localhost:9092"
	if !isKafkaAvailable(broker) {
		t.Skip("Kafka / Redpanda is not reachable at " + broker)
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.DefaultProduceTopic("pulsebid.events.dlq.v1"),
	)
	require.NoError(t, err)
	defer cl.Close()

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	dlq := NewKafkaDLQProducer(cl, "pulsebid.events.dlq.v1", m, nil)

	rec := &kgo.Record{
		Topic:     "pulsebid.events.v1",
		Partition: 2,
		Offset:    104,
		Key:       []byte("test-key"),
		Value:     []byte(`{invalid-json`),
		Headers: []kgo.RecordHeader{
			{Key: "custom-hdr", Value: []byte("val")},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = dlq.PublishDLQ(ctx, rec, "malformed_json: unexpected character")
	require.NoError(t, err)

	// Verify DLQ metric incremented
	mfs, err := reg.Gather()
	require.NoError(t, err)
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "aggregator_events_total" {
			for _, metric := range mf.GetMetric() {
				var hasUnknown, hasDLQ bool
				for _, label := range metric.GetLabel() {
					if label.GetName() == "type" && label.GetValue() == "unknown" {
						hasUnknown = true
					}
					if label.GetName() == "result" && label.GetValue() == "dlq" {
						hasDLQ = true
					}
				}
				if hasUnknown && hasDLQ {
					found = true
					require.Equal(t, float64(1), metric.GetCounter().GetValue())
				}
			}
		}
	}
	require.True(t, found, "dlq metric should be incremented")
}
