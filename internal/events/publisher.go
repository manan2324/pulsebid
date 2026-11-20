package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"pulsebid/internal/platform/tracing"
)

var (
	// ErrBufferFull indicates that the publisher's internal buffer capacity has been reached.
	ErrBufferFull = errors.New("event buffer full")
)

// Publisher defines the contract for producing auction, impression, and click events.
type Publisher interface {
	// Publish synchronously produces critical events (impression, click) to Kafka with AllISRAcks durability.
	Publish(ctx context.Context, ev *Event) error

	// TryPublishAsync submits best-effort events (auction) to a bounded async channel.
	// Returns false and increments drop counter if the channel is full.
	TryPublishAsync(ev *Event) bool

	// Flush flushes all buffered records to Kafka.
	Flush(ctx context.Context) error

	// Close gracefully stops workers and closes the Kafka client.
	Close() error
}

// PublisherMetrics holds Prometheus metrics for the event publishing subsystem (Section 13.1).
type PublisherMetrics struct {
	EventsPublished *prometheus.CounterVec
	BufferDepth     prometheus.Gauge
}

// NewPublisherMetrics creates and optionally registers publisher metrics.
func NewPublisherMetrics(reg prometheus.Registerer) *PublisherMetrics {
	m := &PublisherMetrics{
		EventsPublished: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "events_published_total",
				Help: "Total events published, partitioned by type (impression, click, auction) and result (ok, error, dropped).",
			},
			[]string{"type", "result"},
		),
		BufferDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "event_buffer_depth",
				Help: "Current depth of the in-flight auction event publisher queue.",
			},
		),
	}
	if reg != nil {
		reg.MustRegister(m.EventsPublished, m.BufferDepth)
	}
	return m
}

// PublisherConfig configures the Franz-go Kafka event publisher (Section 10.4).
type PublisherConfig struct {
	Brokers          []string
	Topic            string
	MaxBuffered      int
	AsyncChannelSize int
	Linger           time.Duration
	BatchMaxBytes    int32
	DeliveryTimeout  time.Duration
}

// DefaultPublisherConfig provides standard production defaults per Section 10.4.
func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{
		Brokers:          []string{"localhost:9092"},
		Topic:            "pulsebid.events.v1",
		MaxBuffered:      100000,
		AsyncChannelSize: 10000,
		Linger:           5 * time.Millisecond,
		BatchMaxBytes:    1024 * 1024,
		DeliveryTimeout:  30 * time.Second,
	}
}

// KafkaPublisher publishes durable events to Kafka using franz-go (Section 10.4).
type KafkaPublisher struct {
	cl          *kgo.Client
	topic       string
	maxBuffered int
	auctionChan chan *Event
	metrics     *PublisherMetrics
	logger      *slog.Logger
	wg          sync.WaitGroup
	closeOnce   sync.Once
	closed      chan struct{}
}

// NewKafkaPublisher initializes a franz-go client with Section 10.4 configurations.
func NewKafkaPublisher(cfg PublisherConfig, reg prometheus.Registerer, logger *slog.Logger) (*KafkaPublisher, error) {
	if cfg.Topic == "" {
		cfg.Topic = "pulsebid.events.v1"
	}
	if cfg.MaxBuffered <= 0 {
		cfg.MaxBuffered = 100000
	}
	if cfg.AsyncChannelSize <= 0 {
		cfg.AsyncChannelSize = 10000
	}
	if cfg.Linger <= 0 {
		cfg.Linger = 5 * time.Millisecond
	}
	if cfg.BatchMaxBytes <= 0 {
		cfg.BatchMaxBytes = 1024 * 1024
	}
	if cfg.DeliveryTimeout <= 0 {
		cfg.DeliveryTimeout = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.DefaultProduceTopic(cfg.Topic),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(cfg.Linger),
		kgo.ProducerBatchMaxBytes(cfg.BatchMaxBytes),
		kgo.RecordDeliveryTimeout(cfg.DeliveryTimeout),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		kgo.MaxBufferedRecords(cfg.MaxBuffered),
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	p := &KafkaPublisher{
		cl:          cl,
		topic:       cfg.Topic,
		maxBuffered: cfg.MaxBuffered,
		auctionChan: make(chan *Event, cfg.AsyncChannelSize),
		metrics:     NewPublisherMetrics(reg),
		logger:      logger,
		closed:      make(chan struct{}),
	}

	p.wg.Add(1)
	go p.runAuctionWorker()

	return p, nil
}

// NewKafkaPublisherWithClient wraps an existing kgo.Client for testing.
func NewKafkaPublisherWithClient(cl *kgo.Client, topic string, maxBuffered, asyncChanSize int, reg prometheus.Registerer, logger *slog.Logger) *KafkaPublisher {
	if topic == "" {
		topic = "pulsebid.events.v1"
	}
	if maxBuffered <= 0 {
		maxBuffered = 100000
	}
	if asyncChanSize <= 0 {
		asyncChanSize = 10000
	}
	if logger == nil {
		logger = slog.Default()
	}

	p := &KafkaPublisher{
		cl:          cl,
		topic:       topic,
		maxBuffered: maxBuffered,
		auctionChan: make(chan *Event, asyncChanSize),
		metrics:     NewPublisherMetrics(reg),
		logger:      logger,
		closed:      make(chan struct{}),
	}

	p.wg.Add(1)
	go p.runAuctionWorker()

	return p
}

// Publish writes an impression or click event synchronously with AllISRAcks durability.
func (p *KafkaPublisher) Publish(ctx context.Context, ev *Event) error {
	if p.maxBuffered > 0 && p.cl.BufferedProduceRecords() >= int64(p.maxBuffered) {
		p.metrics.EventsPublished.WithLabelValues(ev.Type, "error").Inc()
		return ErrBufferFull
	}

	rec, err := p.buildRecord(ctx, ev)
	if err != nil {
		p.metrics.EventsPublished.WithLabelValues(ev.Type, "error").Inc()
		return err
	}

	res := p.cl.ProduceSync(ctx, rec)
	if err := res.FirstErr(); err != nil {
		p.metrics.EventsPublished.WithLabelValues(ev.Type, "error").Inc()
		p.logger.Error("failed to publish event to kafka", "type", ev.Type, "event_id", ev.EventID, "error", err)
		return err
	}

	p.metrics.EventsPublished.WithLabelValues(ev.Type, "ok").Inc()
	return nil
}

// TryPublishAsync enqueues an auction event to the bounded async channel.
// Returns false and increments drop counter if the channel is full (ADR-7).
func (p *KafkaPublisher) TryPublishAsync(ev *Event) bool {
	select {
	case <-p.closed:
		p.metrics.EventsPublished.WithLabelValues(ev.Type, "dropped").Inc()
		return false
	case p.auctionChan <- ev:
		p.metrics.BufferDepth.Set(float64(len(p.auctionChan)))
		return true
	default:
		p.metrics.EventsPublished.WithLabelValues(ev.Type, "dropped").Inc()
		return false
	}
}

// Flush flushes all records to Kafka.
func (p *KafkaPublisher) Flush(ctx context.Context) error {
	return p.cl.Flush(ctx)
}

// Close gracefully stops the async worker and closes the Kafka client.
func (p *KafkaPublisher) Close() error {
	p.closeOnce.Do(func() {
		close(p.closed)
		p.wg.Wait()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.cl.Flush(ctx)
		p.cl.Close()
	})
	return nil
}

func (p *KafkaPublisher) runAuctionWorker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.closed:
			for {
				select {
				case ev := <-p.auctionChan:
					p.publishAuctionRecord(ev)
				default:
					return
				}
			}
		case ev := <-p.auctionChan:
			p.publishAuctionRecord(ev)
		}
	}
}

func (p *KafkaPublisher) publishAuctionRecord(ev *Event) {
	p.metrics.BufferDepth.Set(float64(len(p.auctionChan)))
	rec, err := p.buildRecord(context.Background(), ev)
	if err != nil {
		p.metrics.EventsPublished.WithLabelValues(ev.Type, "error").Inc()
		return
	}
	p.cl.Produce(context.Background(), rec, func(r *kgo.Record, err error) {
		if err != nil {
			p.metrics.EventsPublished.WithLabelValues(ev.Type, "error").Inc()
			p.logger.Error("failed to deliver auction event to kafka", "event_id", ev.EventID, "error", err)
		} else {
			p.metrics.EventsPublished.WithLabelValues(ev.Type, "ok").Inc()
		}
	})
}

func (p *KafkaPublisher) buildRecord(ctx context.Context, ev *Event) (*kgo.Record, error) {
	data, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	key := ev.CampaignID
	if key == "" {
		if ev.RequestID != "" {
			key = ev.RequestID
		} else {
			key = ev.BidID
		}
	}

	headers := []kgo.RecordHeader{
		{Key: "event_type", Value: []byte(ev.Type)},
		{Key: "schema_version", Value: []byte(strconv.Itoa(ev.SchemaVersion))},
	}

	// Trace context propagation (Section 10.2 & Section 1296)
	tracing.Propagator().Inject(ctx, &kafkaHeaderCarrier{headers: &headers})

	return &kgo.Record{
		Topic:     p.topic,
		Key:       []byte(key),
		Value:     data,
		Headers:   headers,
		Timestamp: ev.OccurredAt,
	}, nil
}

type kafkaHeaderCarrier struct {
	headers *[]kgo.RecordHeader
}

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if strings.EqualFold(h.Key, key) {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, val string) {
	for i, h := range *c.headers {
		if strings.EqualFold(h.Key, key) {
			(*c.headers)[i].Value = []byte(val)
			return
		}
	}
	*c.headers = append(*c.headers, kgo.RecordHeader{Key: key, Value: []byte(val)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(*c.headers))
	for i, h := range *c.headers {
		keys[i] = h.Key
	}
	return keys
}

// NoopPublisher satisfies Publisher for testing or when Kafka publishing is disabled.
type NoopPublisher struct{}

func (n NoopPublisher) Publish(context.Context, *Event) error     { return nil }
func (n NoopPublisher) TryPublishAsync(*Event) bool                { return true }
func (n NoopPublisher) PublishAuction(context.Context, any) error { return nil }
func (n NoopPublisher) Flush(context.Context) error               { return nil }
func (n NoopPublisher) Close() error                              { return nil }
