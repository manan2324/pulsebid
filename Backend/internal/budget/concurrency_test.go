package budget

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisReserver_ConcurrencyOverspendProof(t *testing.T) {
	client := getTestRedis(t)

	// Simulate 3 gateway instances with distinct reserver instances connected to the same Redis
	gateways := make([]*RedisReserver, 3)
	for i := 0; i < 3; i++ {
		// This test proves atomic cap math under 600 racing callers. Give the
		// instrumented integration run time to schedule them; timeout behavior
		// is checked separately by TestRedisOperationsHaveOwnDeadline.
		gateways[i] = NewRedisReserver(client, 2*time.Second, 0.05, nil)
	}

	campID := fmt.Sprintf("camp-concurrent-%d", time.Now().UnixNano())
	now := time.Date(2026, 11, 18, 14, 0, 0, 0, time.UTC)

	const costMicros = int64(100)
	const targetReservations = int64(100)
	const dailyBudget = targetReservations * costMicros // 10,000 micros

	camp := CampaignBudget{
		ID:                campID,
		DailyBudgetMicros: dailyBudget,
		PacingMode:        "asap",
	}

	ctx := context.Background()
	require.NoError(t, gateways[0].Reset(ctx, campID, now))
	t.Cleanup(func() {
		_ = gateways[0].Reset(context.Background(), campID, now)
	})

	const goroutinesPerGateway = 200
	totalGoroutines := len(gateways) * goroutinesPerGateway // 600

	var accepted atomic.Int64
	var exhausted atomic.Int64
	var errCount atomic.Int64

	startCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(totalGoroutines)

	for gwIdx, gw := range gateways {
		for i := 0; i < goroutinesPerGateway; i++ {
			go func(reserver *RedisReserver, _ int) {
				defer wg.Done()
				<-startCh // wait for release of starting gate

				resCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				res, err := reserver.Reserve(resCtx, camp, costMicros, now)
				if err != nil {
					errCount.Add(1)
					return
				}
				switch res {
				case Reserved:
					accepted.Add(1)
				case Exhausted:
					exhausted.Add(1)
				default:
					errCount.Add(1)
				}
			}(gw, gwIdx*goroutinesPerGateway+i)
		}
	}

	// Unleash all 600 goroutines simultaneously
	close(startCh)
	wg.Wait()

	assert.Equal(t, int64(0), errCount.Load(), "no Redis communication errors should occur")
	assert.Equal(t, targetReservations, accepted.Load(), "exactly 100 reservations must be accepted")
	assert.Equal(t, int64(totalGoroutines)-targetReservations, exhausted.Load(), "remaining 500 reservations must be exhausted")

	spent, err := gateways[0].Spent(ctx, campID, now)
	require.NoError(t, err)
	assert.Equal(t, dailyBudget, spent, "Redis spend must exactly match 100 * costMicros = 10,000")
}
