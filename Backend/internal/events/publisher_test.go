package events

import (
	"context"
	"encoding/json"
	"net"
	"pulsebid/internal/platform/testutil"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/trace"
)

func isKafkaAvailable(addr string) bool {
	conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestPublisher_DefaultConfig(t *testing.T) {
	cfg := DefaultPublisherConfig()
	require.Equal(t, "pulsebid.events.v1", cfg.Topic)
	require.Equal(t, 100000, cfg.MaxBuffered)
	require.Equal(t, 10000, cfg.AsyncChannelSize)
	require.Equal(t, 5*time.Millisecond, cfg.Linger)
}

func TestPublisher_NoopPublisher(t *testing.T) {
	noop := NoopPublisher{}
	require.NoError(t, noop.Publish(context.Background(), &Event{}))
	require.True(t, noop.TryPublishAsync(context.Background(), &Event{}))
	require.NoError(t, noop.Flush(context.Background()))
	require.NoError(t, noop.Close())
}

func TestPublisher_KafkaHeaderCarrier(t *testing.T) {
	headers := []kgo.RecordHeader{
		{Key: "foo", Value: []byte("bar")},
	}
	carrier := &kafkaHeaderCarrier{headers: &headers}

	require.Equal(t, "bar", carrier.Get("foo"))
	require.Equal(t, "bar", carrier.Get("FOO")) // case-insensitive
	require.Empty(t, carrier.Get("non-existent"))

	carrier.Set("foo", "updated")
	require.Equal(t, "updated", carrier.Get("foo"))

	carrier.Set("new-key", "new-val")
	require.Equal(t, "new-val", carrier.Get("new-key"))

	keys := carrier.Keys()
	require.Len(t, keys, 2)
	require.Contains(t, keys, "foo")
	require.Contains(t, keys, "new-key")
}

func TestPublisher_BuildRecord_KeyFallbacks(t *testing.T) {
	p := &KafkaPublisher{topic: "test-topic"}

	// 1. CampaignID present -> key is CampaignID
	ev1 := &Event{Type: TypeImpression, CampaignID: "c-1", RequestID: "r-1", BidID: "b-1"}
	rec1, err := p.buildRecord(context.Background(), ev1)
	require.NoError(t, err)
	require.Equal(t, "c-1", string(rec1.Key))

	// 2. CampaignID empty, RequestID present -> key is RequestID
	ev2 := &Event{Type: TypeAuction, CampaignID: "", RequestID: "r-1", BidID: "b-1"}
	rec2, err := p.buildRecord(context.Background(), ev2)
	require.NoError(t, err)
	require.Equal(t, "r-1", string(rec2.Key))

	// 3. CampaignID empty, RequestID empty -> key is BidID
	ev3 := &Event{Type: TypeAuction, CampaignID: "", RequestID: "", BidID: "b-1"}
	rec3, err := p.buildRecord(context.Background(), ev3)
	require.NoError(t, err)
	require.Equal(t, "b-1", string(rec3.Key))
}

func TestPublisher_AsyncCapturesTraceAndOwnsPayload(t *testing.T) {
	p := &KafkaPublisher{
		topic: "test-topic", auctionChan: make(chan auctionItem, 1),
		metrics: NewPublisherMetrics(nil), closed: make(chan struct{}),
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
	})
	ctx, cancel := context.WithCancel(trace.ContextWithSpanContext(context.Background(), sc))
	ev := &Event{Type: TypeAuction, BidID: "original", Participants: []string{"campaign"}}
	require.True(t, p.TryPublishAsync(ctx, ev))
	cancel()
	ev.BidID = "mutated"
	ev.Participants[0] = "mutated"
	item := <-p.auctionChan
	carrier := &kafkaHeaderCarrier{headers: &item.record.Headers}
	require.Contains(t, carrier.Get("traceparent"), sc.TraceID().String())
	var captured Event
	require.NoError(t, json.Unmarshal(item.record.Value, &captured))
	require.Equal(t, "original", captured.BidID)
	require.Equal(t, []string{"campaign"}, captured.Participants)
}

func TestPublisher_AuctionDropOnFull(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewPublisherMetrics(reg)

	p := &KafkaPublisher{
		topic:       "test-topic",
		auctionChan: make(chan auctionItem, 2),
		metrics:     metrics,
		closed:      make(chan struct{}),
	}

	ev1 := &Event{Type: TypeAuction, BidID: "bid-1"}
	ev2 := &Event{Type: TypeAuction, BidID: "bid-2"}
	ev3 := &Event{Type: TypeAuction, BidID: "bid-3"}

	require.True(t, p.TryPublishAsync(context.Background(), ev1))
	require.True(t, p.TryPublishAsync(context.Background(), ev2))

	// Third event should drop because buffer is full (capacity 2)
	require.False(t, p.TryPublishAsync(context.Background(), ev3))

	metricFamilies, err := reg.Gather()
	require.NoError(t, err)

	var droppedFound bool
	for _, mf := range metricFamilies {
		if mf.GetName() == "events_published_total" {
			for _, m := range mf.GetMetric() {
				var isAuction, isDropped bool
				for _, l := range m.GetLabel() {
					if l.GetName() == "type" && l.GetValue() == TypeAuction {
						isAuction = true
					}
					if l.GetName() == "result" && l.GetValue() == "dropped" {
						isDropped = true
					}
				}
				if isAuction && isDropped {
					droppedFound = true
					require.Equal(t, float64(1), m.GetCounter().GetValue())
				}
			}
		}
	}
	require.True(t, droppedFound, "events_published_total{type=auction, result=dropped} should be 1")

	// When closed, TryPublishAsync should also return false
	close(p.closed)
	require.False(t, p.TryPublishAsync(context.Background(), ev1))
}

func TestPublisher_BufferLimitCheck(t *testing.T) {
	// If maxBuffered is set to 1 and we simulate a client with 1 buffered record
	// Publish should fail-fast with ErrBufferFull
	cl, err := kgo.NewClient(
		kgo.SeedBrokers("127.0.0.1:9999"), // non-existent broker
		kgo.MaxBufferedRecords(1),
	)
	require.NoError(t, err)
	defer cl.Close()

	reg := prometheus.NewRegistry()
	p := NewKafkaPublisherWithClient(cl, "test-topic", 1, 10, reg, nil)
	defer func() { _ = p.Close() }()

	// Fill the franz-go buffer with 1 record so BufferedProduceRecords() >= 1
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	rec := &kgo.Record{Topic: "test-topic", Value: []byte("fill")}
	cl.Produce(ctx, rec, func(*kgo.Record, error) {})

	// Now Publish with maxBuffered=1 should return ErrBufferFull immediately
	ev := &Event{Type: TypeImpression, BidID: "bid-1"}
	err = p.Publish(context.Background(), ev)
	require.ErrorIs(t, err, ErrBufferFull)
}

func TestPublisher_IntegrationWithRedpanda(t *testing.T) {
	broker := "localhost:9092"
	if !isKafkaAvailable(broker) {
		testutil.Unavailable(t, "%s", "Kafka / Redpanda is not reachable at "+broker)
	}

	topic := testutil.KafkaTopic(t, "localhost:9092")
	reg := prometheus.NewRegistry()
	cfg := PublisherConfig{
		Brokers:          []string{broker},
		Topic:            topic,
		MaxBuffered:      1000,
		AsyncChannelSize: 100,
		Linger:           5 * time.Millisecond,
		BatchMaxBytes:    1024 * 1024,
		DeliveryTimeout:  5 * time.Second,
	}

	pub, err := NewKafkaPublisher(cfg, reg, nil)
	require.NoError(t, err)
	defer func() { _ = pub.Close() }()

	bidID := uuid.NewString()
	campID := "0193f0a2-ca00-7000-8000-000000000001"
	reqID := "req-redpanda-test-1"

	// 1. Publish Impression Event
	impEv := &Event{
		SchemaVersion: CurrentSchemaVersion,
		EventID:       EventID(TypeImpression, bidID),
		Type:          TypeImpression,
		OccurredAt:    time.Now().UTC(),
		RequestID:     reqID,
		BidID:         bidID,
		CampaignID:    campID,
		SpendMicros:   1510,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pub.Publish(ctx, impEv)
	require.NoError(t, err)

	// 2. Publish Click Event
	clickEv := &Event{
		SchemaVersion: CurrentSchemaVersion,
		EventID:       EventID(TypeClick, bidID),
		Type:          TypeClick,
		OccurredAt:    time.Now().UTC(),
		RequestID:     reqID,
		BidID:         bidID,
		CampaignID:    campID,
		SpendMicros:   0,
	}
	err = pub.Publish(ctx, clickEv)
	require.NoError(t, err)

	// 3. Publish Auction Event (async)
	auctionEv := &Event{
		SchemaVersion:    CurrentSchemaVersion,
		EventID:          EventID(TypeAuction, bidID),
		Type:             TypeAuction,
		OccurredAt:       time.Now().UTC(),
		RequestID:        reqID,
		BidID:            bidID,
		CampaignID:       campID,
		SpendMicros:      1510,
		Participants:     []string{campID},
		WinnerCampaignID: campID,
	}
	require.True(t, pub.TryPublishAsync(ctx, auctionEv))

	// Flush to ensure delivery
	err = pub.Flush(ctx)
	require.NoError(t, err)

	// Consume and verify from Redpanda
	consumerCl, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	require.NoError(t, err)
	defer consumerCl.Close()

	pollCtx, pollCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pollCancel()

	fetches := consumerCl.PollFetches(pollCtx)
	require.NoError(t, fetches.Err())

	var foundImp, foundClick, foundAuction bool
	fetches.EachRecord(func(r *kgo.Record) {
		var ev Event
		if err := json.Unmarshal(r.Value, &ev); err == nil && ev.BidID == bidID {
			require.Equal(t, campID, string(r.Key), "record key must be campaign_id")

			var eventTypeHdr, schemaVerHdr string
			for _, h := range r.Headers {
				if h.Key == "event_type" {
					eventTypeHdr = string(h.Value)
				}
				if h.Key == "schema_version" {
					schemaVerHdr = string(h.Value)
				}
			}
			require.Equal(t, "1", schemaVerHdr)

			switch ev.Type {
			case TypeImpression:
				foundImp = true
				require.Equal(t, TypeImpression, eventTypeHdr)
				require.Equal(t, EventID(TypeImpression, bidID), ev.EventID)
				require.Equal(t, int64(1510), ev.SpendMicros)
			case TypeClick:
				foundClick = true
				require.Equal(t, TypeClick, eventTypeHdr)
				require.Equal(t, EventID(TypeClick, bidID), ev.EventID)
				require.Equal(t, int64(0), ev.SpendMicros)
			case TypeAuction:
				foundAuction = true
				require.Equal(t, TypeAuction, eventTypeHdr)
				require.Equal(t, EventID(TypeAuction, bidID), ev.EventID)
			}
		}
	})

	require.True(t, foundImp, "impression event should be delivered to Redpanda")
	require.True(t, foundClick, "click event should be delivered to Redpanda")
	require.True(t, foundAuction, "auction event should be delivered to Redpanda")

	// 4. Duplicate impression test: firing impression again produces identical event_id (ADR-5)
	dupImpID := EventID(TypeImpression, bidID)
	require.Equal(t, impEv.EventID, dupImpID, "duplicate impression must produce identical event_id")
}
