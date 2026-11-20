package budget

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

//go:embed reserve.lua
var reserveLua string

//go:embed release.lua
var releaseLua string

// TTLSeconds is 48 hours in seconds per Section 9.1.
const TTLSeconds = 172800

// Result represents the outcome of a budget reservation attempt.
type Result int

const (
	// Reserved indicates budget was successfully reserved.
	Reserved Result = iota
	// Exhausted indicates the campaign has reached its daily budget cap (-1).
	Exhausted
	// Paced indicates the campaign is paced out for this time window (-2).
	Paced
)

func (r Result) String() string {
	switch r {
	case Reserved:
		return "reserved"
	case Exhausted:
		return "exhausted"
	case Paced:
		return "paced"
	default:
		return "unknown"
	}
}

// CampaignBudget encapsulates budget constraints and pacing rules for a campaign.
type CampaignBudget struct {
	ID                  string
	DailyBudgetMicros   int64
	PacingMode          string // "asap" or "even"
	PacingBurstFraction float64
}

// Reserver defines atomic budget reservation across replicas.
type Reserver interface {
	Reserve(ctx context.Context, c CampaignBudget, costMicros int64, now time.Time) (Result, error)
	Release(ctx context.Context, campaignID string, costMicros int64, now time.Time) error
	Spent(ctx context.Context, campaignID string, now time.Time) (int64, error)
	Reset(ctx context.Context, campaignID string, now time.Time) error
}

// BudgetKey constructs the Redis spend key pb:budget:{campaign_id}:{YYYYMMDD} (UTC).
func BudgetKey(campaignID string, now time.Time) string {
	return fmt.Sprintf("pb:budget:%s:%s", campaignID, now.UTC().Format("20060102"))
}

// ComputePaceCap calculates the current spend cap for the day based on pacing mode.
func ComputePaceCap(dailyBudget int64, mode string, burstFraction float64, now time.Time) int64 {
	if mode == "asap" {
		return dailyBudget
	}

	nowUTC := now.UTC()
	secondsIntoUTCDay := float64(nowUTC.Hour()*3600+nowUTC.Minute()*60+nowUTC.Second()) + float64(nowUTC.Nanosecond())/1e9
	burst := burstFraction
	if burst <= 0 {
		burst = 0.05
	}

	frac := (secondsIntoUTCDay / 86400.0) + burst
	if frac > 1.0 {
		frac = 1.0
	}

	paceCap := int64(math.Floor(float64(dailyBudget) * frac))
	if paceCap > dailyBudget {
		paceCap = dailyBudget
	}
	return paceCap
}

// Metrics holds Prometheus collectors for budget reservation.
type Metrics struct {
	ReserveTotal    *prometheus.CounterVec
	ReserveDuration prometheus.Histogram
}

// NewMetrics initializes and registers budget metrics if a registry is provided.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	total := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "budget_reserve_total",
			Help: "Total budget reservation attempts by result outcome",
		},
		[]string{"result"},
	)
	duration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "budget_reserve_duration_seconds",
			Help:    "Latency of Redis budget reservation operations",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
	)

	if reg != nil {
		if err := reg.Register(total); err != nil {
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				total = are.ExistingCollector.(*prometheus.CounterVec)
			}
		}
		if err := reg.Register(duration); err != nil {
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				duration = are.ExistingCollector.(prometheus.Histogram)
			}
		}
	}

	return &Metrics{
		ReserveTotal:    total,
		ReserveDuration: duration,
	}
}

// RedisReserver implements Reserver using Redis and atomic Lua scripts.
type RedisReserver struct {
	client               redis.UniversalClient
	reserveScript        *redis.Script
	releaseScript        *redis.Script
	opTimeout            time.Duration
	defaultBurstFraction float64
	metrics              *Metrics
}

// NewRedisReserver creates a new RedisReserver.
func NewRedisReserver(
	client redis.UniversalClient,
	opTimeout time.Duration,
	defaultBurstFraction float64,
	reg prometheus.Registerer,
) *RedisReserver {
	if opTimeout <= 0 {
		opTimeout = 10 * time.Millisecond
	}
	if defaultBurstFraction <= 0 {
		defaultBurstFraction = 0.05
	}
	return &RedisReserver{
		client:               client,
		reserveScript:        redis.NewScript(reserveLua),
		releaseScript:        redis.NewScript(releaseLua),
		opTimeout:            opTimeout,
		defaultBurstFraction: defaultBurstFraction,
		metrics:              NewMetrics(reg),
	}
}

func (r *RedisReserver) recordOutcome(outcome string) {
	if r.metrics != nil && r.metrics.ReserveTotal != nil {
		r.metrics.ReserveTotal.WithLabelValues(outcome).Inc()
	}
}

// Reserve attempts to reserve costMicros against the campaign's daily budget.
func (r *RedisReserver) Reserve(ctx context.Context, c CampaignBudget, costMicros int64, now time.Time) (Result, error) {
	start := time.Now()
	defer func() {
		if r.metrics != nil && r.metrics.ReserveDuration != nil {
			r.metrics.ReserveDuration.Observe(time.Since(start).Seconds())
		}
	}()

	if costMicros <= 0 {
		r.recordOutcome("reserved")
		return Reserved, nil
	}
	if c.DailyBudgetMicros <= 0 {
		r.recordOutcome("exhausted")
		return Exhausted, nil
	}

	hardCap := c.DailyBudgetMicros
	burst := c.PacingBurstFraction
	if burst <= 0 {
		burst = r.defaultBurstFraction
	}
	paceCap := ComputePaceCap(hardCap, c.PacingMode, burst, now)

	key := BudgetKey(c.ID, now)
	res, err := r.reserveScript.Run(ctx, r.client, []string{key}, costMicros, hardCap, paceCap, TTLSeconds).Int64()
	if err != nil {
		r.recordOutcome("error")
		return 0, err
	}

	switch res {
	case -1:
		r.recordOutcome("exhausted")
		return Exhausted, nil
	case -2:
		r.recordOutcome("paced")
		return Paced, nil
	default:
		r.recordOutcome("reserved")
		return Reserved, nil
	}
}

// Release issues a compensation refund (DECRBY clamped to 0).
func (r *RedisReserver) Release(ctx context.Context, campaignID string, costMicros int64, now time.Time) error {
	if costMicros <= 0 {
		return nil
	}
	key := BudgetKey(campaignID, now)
	return r.releaseScript.Run(ctx, r.client, []string{key}, costMicros).Err()
}

// Spent returns the current micros spent today for the given campaign.
func (r *RedisReserver) Spent(ctx context.Context, campaignID string, now time.Time) (int64, error) {
	key := BudgetKey(campaignID, now)
	val, err := r.client.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Reset resets the spend today for the given campaign to 0 by deleting the key.
func (r *RedisReserver) Reset(ctx context.Context, campaignID string, now time.Time) error {
	key := BudgetKey(campaignID, now)
	return r.client.Del(ctx, key).Err()
}
