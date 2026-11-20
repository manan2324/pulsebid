package reporting

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func getTestDatabaseURL() string {
	url := os.Getenv("DATABASE_URL")
	if url != "" {
		return url
	}
	return "postgres://pulsebid:pulsebid@localhost:15432/pulsebid?sslmode=disable"
}

func setupTestDB(t *testing.T) (*pgxpool.Pool, *PostgresRepository) {
	ctx := context.Background()
	dsn := getTestDatabaseURL()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Skipf("cannot parse db dsn %s: %v", dsn, err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skipf("cannot connect to db: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("cannot ping db: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	return pool, NewPostgresRepository(pool)
}

func TestPostgresRepository_Queries(t *testing.T) {
	pool, repo := setupTestDB(t)
	ctx := context.Background()

	campA := uuid.MustParse("0193f0a2-ca00-7000-8000-0000000000aa")
	campB := uuid.MustParse("0193f0a2-ca00-7000-8000-0000000000bb")

	t1 := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 10, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

	// Clean up any pre-existing rows for test IDs
	_, _ = pool.Exec(ctx, "DELETE FROM hourly_stats WHERE campaign_id IN ($1, $2)", campA, campB)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM hourly_stats WHERE campaign_id IN ($1, $2)", campA, campB)
	})

	// Insert test fixtures
	insertSQL := `
		INSERT INTO hourly_stats (campaign_id, hour_start, impressions, clicks, spend_micros, bids, wins)
		VALUES
			($1, $2, 100, 5, 100000, 200, 105),
			($1, $3, 150, 10, 150000, 300, 155),
			($4, $3, 50, 2, 50000, 100, 50),
			($1, $5, 200, 15, 200000, 400, 210);
	`
	_, err := pool.Exec(ctx, insertSQL, campA, t1, t2, campB, t3)
	require.NoError(t, err)

	from := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	t.Run("granularity hour", func(t *testing.T) {
		rows, err := repo.QueryStats(ctx, QueryParams{
			From:        from,
			To:          to,
			Granularity: GranularityHour,
			CampaignIDs: []uuid.UUID{campA},
		})
		require.NoError(t, err)
		require.Len(t, rows, 3)

		require.Equal(t, campA, rows[0].CampaignID)
		require.Equal(t, t1, *rows[0].Bucket)
		require.Equal(t, int64(100), rows[0].Impressions)
		require.Equal(t, int64(5), rows[0].Clicks)

		require.Equal(t, t2, *rows[1].Bucket)
		require.Equal(t, int64(150), rows[1].Impressions)

		require.Equal(t, t3, *rows[2].Bucket)
		require.Equal(t, int64(200), rows[2].Impressions)
	})

	t.Run("granularity day", func(t *testing.T) {
		rows, err := repo.QueryStats(ctx, QueryParams{
			From:        from,
			To:          to,
			Granularity: GranularityDay,
			CampaignIDs: []uuid.UUID{campA},
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)

		day1 := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
		day2 := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

		require.Equal(t, day1, *rows[0].Bucket)
		require.Equal(t, int64(250), rows[0].Impressions) // 100 + 150
		require.Equal(t, int64(15), rows[0].Clicks)       // 5 + 10

		require.Equal(t, day2, *rows[1].Bucket)
		require.Equal(t, int64(200), rows[1].Impressions)
		require.Equal(t, int64(15), rows[1].Clicks)
	})

	t.Run("granularity total", func(t *testing.T) {
		rows, err := repo.QueryStats(ctx, QueryParams{
			From:        from,
			To:          to,
			Granularity: GranularityTotal,
			CampaignIDs: []uuid.UUID{campA, campB},
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)

		// Ordered by campaign_id asc
		require.Nil(t, rows[0].Bucket)
		require.Equal(t, campA, rows[0].CampaignID)
		require.Equal(t, int64(450), rows[0].Impressions) // 100 + 150 + 200
		require.Equal(t, int64(30), rows[0].Clicks)
		require.Equal(t, int64(450000), rows[0].SpendMicros)

		require.Nil(t, rows[1].Bucket)
		require.Equal(t, campB, rows[1].CampaignID)
		require.Equal(t, int64(50), rows[1].Impressions)
		require.Equal(t, int64(2), rows[1].Clicks)
	})

	t.Run("empty range", func(t *testing.T) {
		rows, err := repo.QueryStats(ctx, QueryParams{
			From:        time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			To:          time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
			Granularity: GranularityTotal,
			CampaignIDs: []uuid.UUID{campA},
		})
		require.NoError(t, err)
		require.Empty(t, rows)
	})
}
