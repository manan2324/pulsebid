package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/twmb/franz-go/pkg/kgo"
)

// DLQProducer forwards unprocessable records to the dead-letter topic (Section 10.5).
type DLQProducer interface {
	PublishDLQ(ctx context.Context, rec *kgo.Record, errReason string) error
	Close()
}

// KafkaDLQProducer publishes malformed records to Kafka DLQ topic with error metadata headers.
type KafkaDLQProducer struct {
	cl      *kgo.Client
	topic   string
	metrics *Metrics
	logger  *slog.Logger
}

// NewKafkaDLQProducer creates a new KafkaDLQProducer.
func NewKafkaDLQProducer(cl *kgo.Client, topic string, metrics *Metrics, logger *slog.Logger) *KafkaDLQProducer {
	if topic == "" {
		topic = "pulsebid.events.dlq.v1"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &KafkaDLQProducer{
		cl:      cl,
		topic:   topic,
		metrics: metrics,
		logger:  logger,
	}
}

// PublishDLQ sends the malformed record with diagnostic headers to the DLQ topic.
func (p *KafkaDLQProducer) PublishDLQ(ctx context.Context, rec *kgo.Record, errReason string) error {
	headers := make([]kgo.RecordHeader, 0, len(rec.Headers)+4)
	for _, h := range rec.Headers {
		headers = append(headers, kgo.RecordHeader{Key: h.Key, Value: h.Value})
	}
	headers = append(headers,
		kgo.RecordHeader{Key: "error", Value: []byte(errReason)},
		kgo.RecordHeader{Key: "original_topic", Value: []byte(rec.Topic)},
		kgo.RecordHeader{Key: "original_partition", Value: []byte(strconv.Itoa(int(rec.Partition)))},
		kgo.RecordHeader{Key: "original_offset", Value: []byte(strconv.FormatInt(rec.Offset, 10))},
	)

	dlqRec := &kgo.Record{
		Topic:   p.topic,
		Key:     rec.Key,
		Value:   rec.Value,
		Headers: headers,
	}

	res := p.cl.ProduceSync(ctx, dlqRec)
	if err := res.FirstErr(); err != nil {
		p.logger.Error("failed to publish to dlq", "topic", p.topic, "error", err, "reason", errReason)
		return fmt.Errorf("produce dlq record: %w", err)
	}

	if p.metrics != nil {
		p.metrics.EventsTotal.WithLabelValues("unknown", "dlq").Inc()
	}

	p.logger.Warn("forwarded malformed record to dlq", "topic", p.topic, "partition", rec.Partition, "offset", rec.Offset, "reason", errReason)
	return nil
}

// Close closes the producer if needed.
func (p *KafkaDLQProducer) Close() {
	// If the client is shared with consumer, closing is managed by the owner.
}
