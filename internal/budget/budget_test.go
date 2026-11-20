package budget

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestRedis(t *testing.T) redis.UniversalClient {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping Redis test: Redis not reachable at %s: %v", addr, err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func TestComputePaceCap(t *testing.T) {
	dailyBudget := int64(100_000)

	// ASAP mode always returns full daily budget
	assert.Equal(t, dailyBudget, ComputePaceCap(dailyBudget, "asap", 0.05, time.Now()))

	// Even mode at UTC midnight (0s) with 0.05 burst
	tMidnight := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	expectedMidnight := int64(math.Floor(float64(dailyBudget) * 0.05))
	assert.Equal(t, expectedMidnight, ComputePaceCap(dailyBudget, "even", 0.05, tMidnight))

	// Even mode at noon (12:00:00 = 43200s, 0.5) with 0.05 burst => 0.55
	tNoon := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	expectedNoon := int64(math.Floor(float64(dailyBudget) * 0.55))
	assert.Equal(t, expectedNoon, ComputePaceCap(dailyBudget, "even", 0.05, tNoon))

	// Even mode at 23:59:59 (near 1.0) => 1.0 => daily budget
	tEndOfDay := time.Date(2026, 11, 18, 23, 59, 59, 0, time.UTC)
	assert.Equal(t, dailyBudget, ComputePaceCap(dailyBudget, "even", 0.05, tEndOfDay))
}

func TestRedisReserver_ExactCapAndRelease(t *testing.T) {
	client := getTestRedis(t)
	reg := prometheus.NewRegistry()
	reserver := NewRedisReserver(client, 100*time.Millisecond, 0.05, reg)

	ctx := context.Background()
	campID := "camp-test-exact-cap"
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	// Clean up any stale data
	require.NoError(t, reserver.Reset(ctx, campID, now))

	camp := CampaignBudget{
		ID:                campID,
		DailyBudgetMicros: 10_000, // 10,000 micros = $0.01
		PacingMode:        "asap",
	}

	// 1. Initial spent should be 0
	spent, err := reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spent)

	// 2. Reserve 3 x 3000 = 9000
	for i := 0; i < 3; i++ {
		res, err := reserver.Reserve(ctx, camp, 3000, now)
		require.NoError(t, err)
		assert.Equal(t, Reserved, res)
	}

	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(9000), spent)

	// 3. 4th reserve of 3000 exceeds 10,000 cap -> Exhausted
	res, err := reserver.Reserve(ctx, camp, 3000, now)
	require.NoError(t, err)
	assert.Equal(t, Exhausted, res)

	// Spent remains 9000
	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(9000), spent)

	// 4. Reserve remaining 1000 exactly hits 10,000 cap
	res, err = reserver.Reserve(ctx, camp, 1000, now)
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)

	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(10_000), spent)

	// 5. Any further reservation is Exhausted
	res, err = reserver.Reserve(ctx, camp, 1, now)
	require.NoError(t, err)
	assert.Equal(t, Exhausted, res)

	// 6. Release 2000
	require.NoError(t, reserver.Release(ctx, campID, 2000, now))
	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(8000), spent)

	// Now can reserve 2000 again
	res, err = reserver.Reserve(ctx, camp, 2000, now)
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)
	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(10_000), spent)

	// 7. Release clamp to 0: release 50,000 when spent is 10,000
	require.NoError(t, reserver.Release(ctx, campID, 50_000, now))
	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spent)

	// 8. Key TTL verification
	key := BudgetKey(campID, now)
	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 48*time.Hour)

	// 9. Reset deletes key
	require.NoError(t, reserver.Reset(ctx, campID, now))
	spent, err = reserver.Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spent)
}

func TestRedisReserver_EvenPacing(t *testing.T) {
	client := getTestRedis(t)
	reserver := NewRedisReserver(client, 100*time.Millisecond, 0.05, nil)

	ctx := context.Background()
	campID := "camp-test-even-pacing"
	// Midnight UTC: pace cap is 5% of 10,000 = 500
	nowMidnight := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	require.NoError(t, reserver.Reset(ctx, campID, nowMidnight))

	camp := CampaignBudget{
		ID:                  campID,
		DailyBudgetMicros:   10_000,
		PacingMode:          "even",
		PacingBurstFraction: 0.05,
	}

	// Reserve 300 <= 500 pace cap -> Reserved
	res, err := reserver.Reserve(ctx, camp, 300, nowMidnight)
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)

	// Reserve 300: 300 + 300 = 600 > 500 pace cap (but <= 10,000 hard cap) -> Paced
	res, err = reserver.Reserve(ctx, camp, 300, nowMidnight)
	require.NoError(t, err)
	assert.Equal(t, Paced, res)

	// Spend remains 300
	spent, err := reserver.Spent(ctx, campID, nowMidnight)
	require.NoError(t, err)
	assert.Equal(t, int64(300), spent)

	// Fast forward time to Noon: pace cap is 55% of 10,000 = 5500
	nowNoon := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	// Now reserve 300: 300 + 300 = 600 <= 5500 -> Reserved
	res, err = reserver.Reserve(ctx, camp, 300, nowNoon)
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)

	spent, err = reserver.Spent(ctx, campID, nowNoon)
	require.NoError(t, err)
	assert.Equal(t, int64(600), spent)
}

func TestRedisReserver_DayRollover(t *testing.T) {
	client := getTestRedis(t)
	reserver := NewRedisReserver(client, 100*time.Millisecond, 0.05, nil)

	ctx := context.Background()
	campID := "camp-test-day-rollover"
	day1 := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	require.NoError(t, reserver.Reset(ctx, campID, day1))
	require.NoError(t, reserver.Reset(ctx, campID, day2))

	camp := CampaignBudget{
		ID:                campID,
		DailyBudgetMicros: 5000,
		PacingMode:        "asap",
	}

	// Spend all budget on Day 1
	res, err := reserver.Reserve(ctx, camp, 5000, day1)
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)

	spentDay1, err := reserver.Spent(ctx, campID, day1)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), spentDay1)

	// Day 2 has zero spend initially
	spentDay2, err := reserver.Spent(ctx, campID, day2)
	require.NoError(t, err)
	assert.Equal(t, int64(0), spentDay2)

	// Can reserve on Day 2
	res, err = reserver.Reserve(ctx, camp, 2500, day2)
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)

	spentDay2, err = reserver.Spent(ctx, campID, day2)
	require.NoError(t, err)
	assert.Equal(t, int64(2500), spentDay2)

	// Day 1 spend is unaffected
	spentDay1, err = reserver.Spent(ctx, campID, day1)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), spentDay1)
}

func TestRedisReserver_FailClosedOnClosedClient(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	reserver := NewRedisReserver(client, 50*time.Millisecond, 0.05, nil)
	// Close client immediately
	_ = client.Close()

	camp := CampaignBudget{
		ID:                "camp-closed-client",
		DailyBudgetMicros: 10_000,
		PacingMode:        "asap",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := reserver.Reserve(ctx, camp, 1000, time.Now())
	require.Error(t, err, "must return error on closed Redis client so gateway can fail closed")
}

func TestBudget_EdgeCasesAndCoverage(t *testing.T) {
	// String representations
	assert.Equal(t, "reserved", Reserved.String())
	assert.Equal(t, "exhausted", Exhausted.String())
	assert.Equal(t, "paced", Paced.String())
	assert.Equal(t, "unknown", Result(999).String())

	// ComputePaceCap fallbacks
	assert.Equal(t, int64(5000), ComputePaceCap(100_000, "even", 0, time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, int64(100_000), ComputePaceCap(100_000, "even", 2.0, time.Now()))

	// NewRedisReserver defaults
	rDefault := NewRedisReserver(nil, 0, 0, nil)
	assert.Equal(t, 10*time.Millisecond, rDefault.opTimeout)
	assert.Equal(t, 0.05, rDefault.defaultBurstFraction)

	// NewMetrics already registered handling
	reg := prometheus.NewRegistry()
	m1 := NewMetrics(reg)
	require.NotNil(t, m1)
	m2 := NewMetrics(reg)
	require.NotNil(t, m2)

	// In-memory zero cost and zero budget paths
	r := NewRedisReserver(nil, 10*time.Millisecond, 0.05, nil)
	ctx := context.Background()
	res, err := r.Reserve(ctx, CampaignBudget{ID: "c1", DailyBudgetMicros: 1000}, 0, time.Now())
	require.NoError(t, err)
	assert.Equal(t, Reserved, res)

	res, err = r.Reserve(ctx, CampaignBudget{ID: "c1", DailyBudgetMicros: 0}, 100, time.Now())
	require.NoError(t, err)
	assert.Equal(t, Exhausted, res)

	require.NoError(t, r.Release(ctx, "c1", 0, time.Now()))
}
