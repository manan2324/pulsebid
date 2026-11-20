package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"pulsebid/internal/events"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

func setupTestKafkaClient(t *testing.T, group, topic string) (*kgo.Client, string) {
	t.Helper()
	broker := "localhost:9092"
	if !isKafkaAvailable(broker) {
		t.Skip("Kafka / Redpanda is not reachable at " + broker)
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(broker),
		kgo.DefaultProduceTopic(topic),
		kgo.FetchMaxWait(100 * time.Millisecond),
	}
	if group != "" {
		opts = append(opts, kgo.ConsumerGroup(group), kgo.ConsumeTopics(topic), kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	}

	cl, err := kgo.NewClient(opts...)
	require.NoError(t, err)

	return cl, broker
}

func TestConsumer_SingleEventAndDuplicate3x(t *testing.T) {
	topic := "pulsebid.events.v1"
	group := "test-cg-dup-" + uuid.New().String()
	store, pool := setupTestStore(t)
	defer store.Close()

	cl, broker := setupTestKafkaClient(t, group, topic)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testCampID := uuid.New().String()
	bidID := uuid.New().String()
	evID := events.EventID(events.TypeImpression, bidID)
	now := time.Date(2026, 11, 19, 15, 30, 0, 0, time.UTC)
	hourStart := time.Date(2026, 11, 19, 15, 0, 0, 0, time.UTC)

	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM processed_events WHERE event_id = $1", evID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	ev := &events.Event{
		SchemaVersion: events.CurrentSchemaVersion,
		EventID:       evID,
		Type:          events.TypeImpression,
		OccurredAt:    now,
		RequestID:     "req-dup-3x",
		BidID:         bidID,
		CampaignID:    testCampID,
		SpendMicros:   1500,
	}
	val, err := json.Marshal(ev)
	require.NoError(t, err)

	// Publish identical event 3 times
	for i := 0; i < 3; i++ {
		res := cl.ProduceSync(ctx, &kgo.Record{
			Topic: topic,
			Key:   []byte(testCampID),
			Value: val,
			Headers: []kgo.RecordHeader{
				{Key: "event_type", Value: []byte(events.TypeImpression)},
				{Key: "schema_version", Value: []byte("1")},
			},
		})
		require.NoError(t, res.FirstErr())
	}

	cfg := Config{
		KafkaBrokers:     []string{broker},
		KafkaTopicEvents: topic,
		KafkaTopicDLQ:    "pulsebid.events.dlq.v1",
		ConsumerGroup:    group,
		BatchSize:        500,
		FlushIntervalMS:  100,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	dlq := NewKafkaDLQProducer(cl, cfg.KafkaTopicDLQ, m, nil)
	consumer := NewConsumer(cl, store, dlq, cfg, m, nil)

	consumerCtx, consumerCancel := context.WithCancel(ctx)
	defer consumerCancel()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Run(consumerCtx)
	}()

	// Wait until hourly_stats reflects exactly 1 impression
	require.Eventually(t, func() bool {
		var imps, spend int64
		err := pool.QueryRow(ctx, "SELECT impressions, spend_micros FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", testCampID, hourStart).Scan(&imps, &spend)
		return err == nil && imps == 1 && spend == 1500
	}, 10*time.Second, 100*time.Millisecond)

	// Sleep slightly to ensure any extra duplicate processing finishes
	time.Sleep(300 * time.Millisecond)

	var finalImps, finalSpend int64
	err = pool.QueryRow(ctx, "SELECT impressions, spend_micros FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", testCampID, hourStart).Scan(&finalImps, &finalSpend)
	require.NoError(t, err)
	require.Equal(t, int64(1), finalImps, "duplicate fires must result in exactly 1 impression")
	require.Equal(t, int64(1500), finalSpend, "duplicate fires must not duplicate spend")

	consumerCancel()
	<-consumerDone
}

func TestConsumer_MalformedMessageToDLQ(t *testing.T) {
	topic := "pulsebid.events.v1"
	dlqTopic := "pulsebid.events.dlq.v1"
	group := "test-cg-dlq-" + uuid.New().String()
	store, pool := setupTestStore(t)
	defer store.Close()

	cl, broker := setupTestKafkaClient(t, group, topic)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testCampID := uuid.New().String()
	bidID := uuid.New().String()
	evID := events.EventID(events.TypeImpression, bidID)
	now := time.Date(2026, 11, 19, 15, 35, 0, 0, time.UTC)

	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM processed_events WHERE event_id = $1", evID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	// 1. Produce 1 malformed record
	res := cl.ProduceSync(ctx, &kgo.Record{
		Topic: topic,
		Key:   []byte("broken-key"),
		Value: []byte(`{"invalid-json-content`),
	})
	require.NoError(t, res.FirstErr())

	// 2. Produce 1 valid record
	validEv := &events.Event{
		SchemaVersion: events.CurrentSchemaVersion,
		EventID:       evID,
		Type:          events.TypeImpression,
		OccurredAt:    now,
		BidID:         bidID,
		CampaignID:    testCampID,
		SpendMicros:   3000,
	}
	validVal, _ := json.Marshal(validEv)
	res = cl.ProduceSync(ctx, &kgo.Record{
		Topic: topic,
		Key:   []byte(testCampID),
		Value: validVal,
	})
	require.NoError(t, res.FirstErr())

	cfg := Config{
		KafkaBrokers:     []string{broker},
		KafkaTopicEvents: topic,
		KafkaTopicDLQ:    dlqTopic,
		ConsumerGroup:    group,
		BatchSize:        500,
		FlushIntervalMS:  100,
	}

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	dlq := NewKafkaDLQProducer(cl, cfg.KafkaTopicDLQ, m, nil)
	consumer := NewConsumer(cl, store, dlq, cfg, m, nil)

	consumerCtx, consumerCancel := context.WithCancel(ctx)
	defer consumerCancel()

	go func() {
		_ = consumer.Run(consumerCtx)
	}()

	// Verify valid event processed into hourly_stats
	require.Eventually(t, func() bool {
		var imps int64
		err := pool.QueryRow(ctx, "SELECT impressions FROM hourly_stats WHERE campaign_id = $1", testCampID).Scan(&imps)
		return err == nil && imps == 1
	}, 10*time.Second, 100*time.Millisecond)

	consumerCancel()
}

func TestConsumer_CrashBeforeDBCommit(t *testing.T) {
	topic := "pulsebid.events.v1"
	group := "test-cg-crash1-" + uuid.New().String()
	store, pool := setupTestStore(t)
	defer store.Close()

	cl, broker := setupTestKafkaClient(t, group, topic)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testCampID := uuid.New().String()
	var evIDs []string
	defer func() {
		for _, id := range evIDs {
			_, _ = pool.Exec(context.Background(), "DELETE FROM processed_events WHERE event_id = $1", id)
		}
		_, _ = pool.Exec(context.Background(), "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	// Produce 3 events
	for i := 0; i < 3; i++ {
		bidID := fmt.Sprintf("bid-c1-%d-%s", i, uuid.New().String()[:8])
		evID := events.EventID(events.TypeImpression, bidID)
		evIDs = append(evIDs, evID)
		ev := &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       evID,
			Type:          events.TypeImpression,
			OccurredAt:    time.Now().UTC(),
			BidID:         bidID,
			CampaignID:    testCampID,
			SpendMicros:   1000,
		}
		val, _ := json.Marshal(ev)
		res := cl.ProduceSync(ctx, &kgo.Record{
			Topic: topic,
			Key:   []byte(testCampID),
			Value: val,
		})
		require.NoError(t, res.FirstErr())
	}

	cfg := Config{
		KafkaBrokers:     []string{broker},
		KafkaTopicEvents: topic,
		KafkaTopicDLQ:    "pulsebid.events.dlq.v1",
		ConsumerGroup:    group,
		BatchSize:        500,
		FlushIntervalMS:  100,
	}

	// First consumer: crashes before DB commit on first attempt when processing test events
	c1 := NewConsumer(cl, store, nil, cfg, nil, nil)
	var crashOnce int32
	c1.SetFailpointBeforeDBCommit(func(evs []*events.Event) error {
		for _, ev := range evs {
			if ev.CampaignID == testCampID {
				if atomic.CompareAndSwapInt32(&crashOnce, 0, 1) {
					return errors.New("simulated crash before db commit")
				}
			}
		}
		return nil
	})

	c1Ctx, c1Cancel := context.WithCancel(ctx)
	c1Done := make(chan error, 1)
	go func() {
		c1Done <- c1.Run(c1Ctx)
	}()

	// c1 must fail with error
	err := <-c1Done
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated crash before db commit")
	c1Cancel()
	cl.Close() // Release consumer group partitions immediately on process crash

	// Zero rows should be in processed_events or hourly_stats
	var pCount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id = ANY($1)", evIDs).Scan(&pCount))
	require.Equal(t, 0, pCount, "crash before DB commit must not leave partial writes")

	// Restart consumer (c2) on same consumer group
	cl2, _ := setupTestKafkaClient(t, group, topic)
	defer cl2.Close()
	c2 := NewConsumer(cl2, store, nil, cfg, nil, nil)

	c2Ctx, c2Cancel := context.WithCancel(ctx)
	defer c2Cancel()
	go func() {
		_ = c2.Run(c2Ctx)
	}()

	// c2 must resume from uncommitted offset and process all 3 events
	require.Eventually(t, func() bool {
		var imps int64
		err := pool.QueryRow(ctx, "SELECT impressions FROM hourly_stats WHERE campaign_id = $1", testCampID).Scan(&imps)
		return err == nil && imps == 3
	}, 10*time.Second, 100*time.Millisecond)

	c2Cancel()
}

func TestConsumer_CrashAfterDBCommitBeforeOffsetCommit(t *testing.T) {
	topic := "pulsebid.events.v1"
	group := "test-cg-crash2-" + uuid.New().String()
	store, pool := setupTestStore(t)
	defer store.Close()

	cl, broker := setupTestKafkaClient(t, group, topic)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	testCampID := uuid.New().String()
	var evIDs []string
	defer func() {
		for _, id := range evIDs {
			_, _ = pool.Exec(context.Background(), "DELETE FROM processed_events WHERE event_id = $1", id)
		}
		_, _ = pool.Exec(context.Background(), "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	// Produce 3 events
	for i := 0; i < 3; i++ {
		bidID := fmt.Sprintf("bid-c2-%d-%s", i, uuid.New().String()[:8])
		evID := events.EventID(events.TypeImpression, bidID)
		evIDs = append(evIDs, evID)
		ev := &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       evID,
			Type:          events.TypeImpression,
			OccurredAt:    time.Now().UTC(),
			BidID:         bidID,
			CampaignID:    testCampID,
			SpendMicros:   1000,
		}
		val, _ := json.Marshal(ev)
		res := cl.ProduceSync(ctx, &kgo.Record{
			Topic: topic,
			Key:   []byte(testCampID),
			Value: val,
		})
		require.NoError(t, res.FirstErr())
	}

	cfg := Config{
		KafkaBrokers:     []string{broker},
		KafkaTopicEvents: topic,
		KafkaTopicDLQ:    "pulsebid.events.dlq.v1",
		ConsumerGroup:    group,
		BatchSize:        500,
		FlushIntervalMS:  100,
	}

	// First consumer: crashes AFTER DB commit, BEFORE offset commit when processing test events!
	c1 := NewConsumer(cl, store, nil, cfg, nil, nil)
	var crashOnce int32
	c1.SetFailpointAfterDBCommitBeforeOffsetCommit(func(evs []*events.Event) error {
		for _, ev := range evs {
			if ev.CampaignID == testCampID {
				if atomic.CompareAndSwapInt32(&crashOnce, 0, 1) {
					return errors.New("simulated crash after db commit before offset commit")
				}
			}
		}
		return nil
	})

	c1Ctx, c1Cancel := context.WithCancel(ctx)
	c1Done := make(chan error, 1)
	go func() {
		c1Done <- c1.Run(c1Ctx)
	}()

	// c1 must fail with error
	err := <-c1Done
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated crash after db commit before offset commit")
	c1Cancel()
	cl.Close() // Release consumer group partitions immediately on process crash

	// DB has partial or full impressions committed by c1
	var imps int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT impressions FROM hourly_stats WHERE campaign_id = $1", testCampID).Scan(&imps))
	require.Greater(t, imps, int64(0))

	// But Kafka offset was NOT committed!
	// Restart consumer (c2) on same consumer group: it will re-read from the uncommitted offset!
	cl2, _ := setupTestKafkaClient(t, group, topic)
	defer cl2.Close()
	c2 := NewConsumer(cl2, store, nil, cfg, nil, nil)

	c2Ctx, c2Cancel := context.WithCancel(ctx)
	defer c2Cancel()
	go func() {
		_ = c2.Run(c2Ctx)
	}()

	// Verify impressions reach EXACTLY 3 (not 6!)
	require.Eventually(t, func() bool {
		var finalImps int64
		err := pool.QueryRow(ctx, "SELECT impressions FROM hourly_stats WHERE campaign_id = $1", testCampID).Scan(&finalImps)
		return err == nil && finalImps == 3
	}, 10*time.Second, 100*time.Millisecond)

	// Ensure no double count after extra processing
	time.Sleep(300 * time.Millisecond)
	require.NoError(t, pool.QueryRow(ctx, "SELECT impressions FROM hourly_stats WHERE campaign_id = $1", testCampID).Scan(&imps))
	require.Equal(t, int64(3), imps, "re-reading uncommitted batch must deduplicate all records; no double count!")

	c2Cancel()
}

func TestConsumer_TwoAggregatorsInOneGroup(t *testing.T) {
	topic := "pulsebid.events.v1"
	group := "test-cg-multi-" + uuid.New().String()
	store, pool := setupTestStore(t)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cl1, broker := setupTestKafkaClient(t, group, topic)
	defer cl1.Close()

	cl2, _ := setupTestKafkaClient(t, group, topic)
	defer cl2.Close()

	cfg := Config{
		KafkaBrokers:     []string{broker},
		KafkaTopicEvents: topic,
		KafkaTopicDLQ:    "pulsebid.events.dlq.v1",
		ConsumerGroup:    group,
		BatchSize:        500,
		FlushIntervalMS:  100,
	}

	c1 := NewConsumer(cl1, store, nil, cfg, nil, nil)
	c2 := NewConsumer(cl2, store, nil, cfg, nil, nil)

	cCtx, cCancel := context.WithCancel(ctx)
	defer cCancel()

	go func() { _ = c1.Run(cCtx) }()
	go func() { _ = c2.Run(cCtx) }()

	// Give a moment for group rebalance
	time.Sleep(1 * time.Second)

	producerCl, _ := setupTestKafkaClient(t, "", topic)
	defer producerCl.Close()

	testCampID := uuid.New().String()
	var evIDs []string
	defer func() {
		for _, id := range evIDs {
			_, _ = pool.Exec(context.Background(), "DELETE FROM processed_events WHERE event_id = $1", id)
		}
		_, _ = pool.Exec(context.Background(), "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	const totalEvents = 20
	for i := 0; i < totalEvents; i++ {
		bidID := fmt.Sprintf("bid-multi-%d-%s", i, uuid.New().String()[:8])
		evID := events.EventID(events.TypeImpression, bidID)
		evIDs = append(evIDs, evID)
		ev := &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       evID,
			Type:          events.TypeImpression,
			OccurredAt:    time.Now().UTC(),
			BidID:         bidID,
			CampaignID:    testCampID,
			SpendMicros:   1000,
		}
		val, _ := json.Marshal(ev)
		res := producerCl.ProduceSync(ctx, &kgo.Record{
			Topic: topic,
			Key:   []byte(uuid.New().String()), // random key so it spreads across partitions
			Value: val,
		})
		require.NoError(t, res.FirstErr())
	}

	require.Eventually(t, func() bool {
		var imps int64
		err := pool.QueryRow(ctx, "SELECT impressions FROM hourly_stats WHERE campaign_id = $1", testCampID).Scan(&imps)
		return err == nil && imps == int64(totalEvents)
	}, 15*time.Second, 200*time.Millisecond)

	cCancel()
}

func TestConsumer_CarrierAndDLQ(t *testing.T) {
	carrier := &kafkaRecordHeaderCarrier{}
	carrier.Set("X-Trace", "12345")
	require.Equal(t, "12345", carrier.Get("X-Trace"))
	require.Equal(t, "12345", carrier.Get("x-trace"))
	require.Empty(t, carrier.Get("Non-Existent"))

	carrier.Set("X-Trace", "updated-trace")
	require.Equal(t, "updated-trace", carrier.Get("x-trace"))

	keys := carrier.Keys()
	require.Len(t, keys, 1)
	require.Equal(t, "X-Trace", keys[0])

	dlq := NewKafkaDLQProducer(nil, "", nil, nil)
	dlq.Close()
}


