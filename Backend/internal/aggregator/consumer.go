package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"pulsebid/internal/events"
	"pulsebid/internal/platform/tracing"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Consumer polls events from Kafka, decodes them, forwards malformed messages to DLQ,
// and processes valid events through the deduplicating store in batches (Section 10.5).
type Consumer struct {
	cl                                       *kgo.Client
	store                                    Store
	dlq                                      DLQProducer
	cfg                                      Config
	metrics                                  *Metrics
	logger                                   *slog.Logger
	tracer                                   trace.Tracer
	failpointBeforeDBCommit                  func(evs []*events.Event) error
	failpointAfterDBCommitBeforeOffsetCommit func(evs []*events.Event) error
}

// NewConsumer initializes a consumer with franz-go client and store.
func NewConsumer(cl *kgo.Client, store Store, dlq DLQProducer, cfg Config, metrics *Metrics, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{
		cl:      cl,
		store:   store,
		dlq:     dlq,
		cfg:     cfg,
		metrics: metrics,
		logger:  logger,
		tracer:  otel.Tracer("pulsebid/internal/aggregator"),
	}
}

// SetFailpointBeforeDBCommit injects a failure callback right before DB commit.
func (c *Consumer) SetFailpointBeforeDBCommit(fn func(evs []*events.Event) error) {
	c.failpointBeforeDBCommit = fn
}

// SetFailpointAfterDBCommitBeforeOffsetCommit injects a failure callback after DB commit but before offset commit.
func (c *Consumer) SetFailpointAfterDBCommitBeforeOffsetCommit(fn func(evs []*events.Event) error) {
	c.failpointAfterDBCommitBeforeOffsetCommit = fn
}

// Run executes the continuous poll-process-commit consumer loop until ctx is canceled (Section 10.5).
func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("starting event aggregator consumer loop",
		"group", c.cfg.ConsumerGroup,
		"topic", c.cfg.KafkaTopicEvents,
		"batch_size", c.cfg.BatchSize,
		"flush_interval_ms", c.cfg.FlushIntervalMS,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer context done, stopping")
			return nil
		default:
		}

		fetches := c.cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				if fe.Err == context.Canceled {
					return nil
				}
				c.logger.Error("fetch error from kafka", "topic", fe.Topic, "partition", fe.Partition, "error", fe.Err)
			}
		}

		// Update partition lag metrics
		if c.metrics != nil {
			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				if len(p.Records) > 0 {
					last := p.Records[len(p.Records)-1]
					lag := p.HighWatermark - (last.Offset + 1)
					if lag < 0 {
						lag = 0
					}
					c.metrics.ConsumerLag.WithLabelValues(strconv.Itoa(int(p.Partition))).Set(float64(lag))
				}
			})
		}

		numRecords := fetches.NumRecords()
		if numRecords == 0 {
			continue
		}

		// Collect records
		records := make([]*kgo.Record, 0, numRecords)
		fetches.EachRecord(func(r *kgo.Record) {
			records = append(records, r)
		})

		// Process in chunks up to cfg.BatchSize
		for i := 0; i < len(records); i += c.cfg.BatchSize {
			end := i + c.cfg.BatchSize
			if end > len(records) {
				end = len(records)
			}
			chunk := records[i:end]

			if err := c.processChunk(ctx, chunk); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("process chunk: %w", err)
			}
		}
	}
}

func (c *Consumer) processChunk(ctx context.Context, chunk []*kgo.Record) error {
	batchStart := time.Now()
	validEvents := make([]*events.Event, 0, len(chunk))
	var links []trace.Link

	// 1. Decode records; malformed records go to DLQ
	for _, rec := range chunk {
		carrier := &kafkaRecordHeaderCarrier{headers: rec.Headers}
		recCtx := tracing.Propagator().Extract(ctx, carrier)
		if sc := trace.SpanContextFromContext(recCtx); sc.IsValid() {
			links = append(links, trace.Link{SpanContext: sc})
		}

		var ev events.Event
		if err := json.Unmarshal(rec.Value, &ev); err != nil {
			if c.dlq != nil {
				if err := c.publishDLQ(ctx, rec, fmt.Sprintf("malformed_json: %v", err)); err != nil {
					return fmt.Errorf("publish dead-letter record: %w", err)
				}
			}
			continue
		}
		if ev.SchemaVersion != events.CurrentSchemaVersion {
			if c.dlq != nil {
				if err := c.publishDLQ(ctx, rec, fmt.Sprintf("unsupported_schema_version: %d", ev.SchemaVersion)); err != nil {
					return fmt.Errorf("publish dead-letter record: %w", err)
				}
			}
			continue
		}
		if ev.EventID == "" {
			if c.dlq != nil {
				if err := c.publishDLQ(ctx, rec, "missing_event_id"); err != nil {
					return fmt.Errorf("publish dead-letter record: %w", err)
				}
			}
			continue
		}
		if _, err := uuid.Parse(ev.EventID); err != nil {
			if c.dlq != nil {
				if err := c.publishDLQ(ctx, rec, fmt.Sprintf("invalid_event_id_uuid: %v", err)); err != nil {
					return fmt.Errorf("publish dead-letter record: %w", err)
				}
			}
			continue
		}
		if ev.Type != events.TypeImpression && ev.Type != events.TypeClick && ev.Type != events.TypeAuction {
			if c.dlq != nil {
				if err := c.publishDLQ(ctx, rec, fmt.Sprintf("unknown_event_type: %q", ev.Type)); err != nil {
					return fmt.Errorf("publish dead-letter record: %w", err)
				}
			}
			continue
		}
		if ev.CampaignID != "" {
			if _, err := uuid.Parse(ev.CampaignID); err != nil {
				if c.dlq != nil {
					if err := c.publishDLQ(ctx, rec, fmt.Sprintf("invalid_campaign_id_uuid: %v", err)); err != nil {
						return fmt.Errorf("publish dead-letter record: %w", err)
					}
				}
				continue
			}
		}
		if ev.WinnerCampaignID != "" {
			if _, err := uuid.Parse(ev.WinnerCampaignID); err != nil {
				if c.dlq != nil {
					if err := c.publishDLQ(ctx, rec, fmt.Sprintf("invalid_winner_campaign_id_uuid: %v", err)); err != nil {
						return fmt.Errorf("publish dead-letter record: %w", err)
					}
				}
				continue
			}
		}
		var invalidParticipant bool
		for _, p := range ev.Participants {
			if p != "" {
				if _, err := uuid.Parse(p); err != nil {
					invalidParticipant = true
					break
				}
			}
		}
		if invalidParticipant {
			if c.dlq != nil {
				if err := c.publishDLQ(ctx, rec, "invalid_participant_uuid"); err != nil {
					return fmt.Errorf("publish dead-letter record: %w", err)
				}
			}
			continue
		}

		validEvents = append(validEvents, &ev)
	}

	spanOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	if len(links) > 0 {
		spanOpts = append(spanOpts, trace.WithLinks(links...))
	}
	ctx, span := c.tracer.Start(ctx, "aggregator.process_batch", spanOpts...)
	defer span.End()

	// Testing failpoint hook before DB commit
	if c.failpointBeforeDBCommit != nil {
		if err := c.failpointBeforeDBCommit(validEvents); err != nil {
			return fmt.Errorf("failpoint_before_db_commit: %w", err)
		}
	}

	// 2. Process valid events with retry on database error (cap 5s backoff)
	if len(validEvents) > 0 {
		backoff := 50 * time.Millisecond
		maxBackoff := 5 * time.Second
		for {
			_, _, err := c.store.ProcessBatch(ctx, validEvents)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Error("failed to process event batch in db, retrying...", "error", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	// Testing failpoint hook after DB commit before offset commit
	if c.failpointAfterDBCommitBeforeOffsetCommit != nil {
		if err := c.failpointAfterDBCommitBeforeOffsetCommit(validEvents); err != nil {
			return fmt.Errorf("failpoint_after_db_commit: %w", err)
		}
	}

	// 3. Commit offsets only after DB transaction commits (Section 10.5 & ADR-6)
	if err := c.cl.CommitRecords(ctx, chunk...); err != nil {
		c.logger.Error("failed to commit kafka offsets", "error", err)
		return fmt.Errorf("commit records: %w", err)
	}

	if c.metrics != nil {
		c.metrics.BatchSize.Observe(float64(len(chunk)))
		c.metrics.BatchDuration.Observe(time.Since(batchStart).Seconds())
	}

	return nil
}

// publishDLQ bounds broker waits; returning an error leaves source offsets
// uncommitted, so an unavailable dead-letter topic cannot silently lose data.
func (c *Consumer) publishDLQ(ctx context.Context, rec *kgo.Record, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.dlq.PublishDLQ(ctx, rec, reason)
}

type kafkaRecordHeaderCarrier struct {
	headers []kgo.RecordHeader
}

func (c *kafkaRecordHeaderCarrier) Get(key string) string {
	for _, h := range c.headers {
		if strings.EqualFold(h.Key, key) {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaRecordHeaderCarrier) Set(key, value string) {
	for i, h := range c.headers {
		if strings.EqualFold(h.Key, key) {
			c.headers[i].Value = []byte(value)
			return
		}
	}
	c.headers = append(c.headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func (c *kafkaRecordHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for _, h := range c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}
