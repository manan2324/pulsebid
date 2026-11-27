package budget

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type stalledRedisHook struct{}

func (stalledRedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (stalledRedisHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, _ redis.Cmder) error {
		<-ctx.Done()
		return ctx.Err()
	}
}
func (stalledRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestRedisOperationsHaveOwnDeadline(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", ContextTimeoutEnabled: true})
	defer func() { _ = client.Close() }()
	client.AddHook(stalledRedisHook{})
	reserver := NewRedisReserver(client, 5*time.Millisecond, 0.05, nil)
	operations := map[string]func(context.Context) error{
		"reserve": func(ctx context.Context) error {
			_, err := reserver.Reserve(ctx, CampaignBudget{ID: "test", DailyBudgetMicros: 100, PacingMode: "asap"}, 1, time.Now())
			return err
		},
		"release": func(ctx context.Context) error { return reserver.Release(ctx, "test", 1, time.Now()) },
		"spent":   func(ctx context.Context) error { _, err := reserver.Spent(ctx, "test", time.Now()); return err },
		"reset":   func(ctx context.Context) error { return reserver.Reset(ctx, "test", time.Now()) },
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			require.ErrorIs(t, operation(context.Background()), context.DeadlineExceeded)
			require.Less(t, time.Since(start), time.Second)
		})
	}
}
