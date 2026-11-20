package aggregator

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"pulsebid/internal/events"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func getTestDatabaseURL() string {
	if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		return envURL
	}
	// Try port 15432 (local Windows override in .env)
	conn, err := net.DialTimeout("tcp", "localhost:15432", 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return "postgres://pulsebid:pulsebid@localhost:15432/pulsebid?sslmode=disable"
	}
	// Try standard 5432
	conn, err = net.DialTimeout("tcp", "localhost:5432", 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return "postgres://pulsebid:pulsebid@localhost:5432/pulsebid?sslmode=disable"
	}
	return ""
}

func setupTestStore(t *testing.T) (*PGStore, *pgxpool.Pool) {
	t.Helper()
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("PostgreSQL not reachable for store integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	store, err := NewPGStoreFromURL(ctx, dbURL, m, nil)
	if err != nil {
		t.Skip("failed to connect to postgres: " + err.Error())
	}

	return store, store.pool
}

func cleanupTestData(ctx context.Context, pool *pgxpool.Pool, prefix string) {
	_, _ = pool.Exec(ctx, "DELETE FROM processed_events WHERE event_type LIKE $1", prefix+"%")
	_, _ = pool.Exec(ctx, "DELETE FROM hourly_stats WHERE campaign_id::text LIKE $1", prefix+"%")
}

func TestStore_EmptyBatch(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()
	newCount, dupCount, err := store.ProcessBatch(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 0, newCount)
	require.Equal(t, 0, dupCount)

	newCount, dupCount, err = store.ProcessBatch(ctx, []*events.Event{})
	require.NoError(t, err)
	require.Equal(t, 0, newCount)
	require.Equal(t, 0, dupCount)
}

func TestStore_SingleEventAndDuplicates(t *testing.T) {
	store, pool := setupTestStore(t)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testCampID := uuid.New().String()
	bidID := uuid.New().String()
	evID := events.EventID(events.TypeImpression, bidID)
	now := time.Date(2026, 11, 19, 14, 15, 0, 0, time.UTC)
	hourStart := time.Date(2026, 11, 19, 14, 0, 0, 0, time.UTC)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM processed_events WHERE event_id = $1", evID)
		_, _ = pool.Exec(ctx, "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	ev := &events.Event{
		SchemaVersion: events.CurrentSchemaVersion,
		EventID:       evID,
		Type:          events.TypeImpression,
		OccurredAt:    now,
		RequestID:     "req-1",
		BidID:         bidID,
		CampaignID:    testCampID,
		SpendMicros:   1510,
	}

	// 1. First write: counted as new
	newCount, dupCount, err := store.ProcessBatch(ctx, []*events.Event{ev})
	require.NoError(t, err)
	require.Equal(t, 1, newCount)
	require.Equal(t, 0, dupCount)

	// Verify DB state
	var pCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id = $1", evID).Scan(&pCount)
	require.NoError(t, err)
	require.Equal(t, 1, pCount)

	var imps, spend int64
	err = pool.QueryRow(ctx, "SELECT impressions, spend_micros FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", testCampID, hourStart).Scan(&imps, &spend)
	require.NoError(t, err)
	require.Equal(t, int64(1), imps)
	require.Equal(t, int64(1510), spend)

	// 2. Second and Third write with identical event: counted as duplicate
	newCount, dupCount, err = store.ProcessBatch(ctx, []*events.Event{ev})
	require.NoError(t, err)
	require.Equal(t, 0, newCount)
	require.Equal(t, 1, dupCount)

	newCount, dupCount, err = store.ProcessBatch(ctx, []*events.Event{ev})
	require.NoError(t, err)
	require.Equal(t, 0, newCount)
	require.Equal(t, 1, dupCount)

	// Verify stats remain exactly 1 impression and 1510 micros
	err = pool.QueryRow(ctx, "SELECT impressions, spend_micros FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", testCampID, hourStart).Scan(&imps, &spend)
	require.NoError(t, err)
	require.Equal(t, int64(1), imps)
	require.Equal(t, int64(1510), spend)
}

func TestStore_BatchWithInternalDuplicates(t *testing.T) {
	store, pool := setupTestStore(t)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testCampID := uuid.New().String()
	bidID := uuid.New().String()
	evID := events.EventID(events.TypeImpression, bidID)
	now := time.Date(2026, 11, 19, 14, 20, 0, 0, time.UTC)
	hourStart := time.Date(2026, 11, 19, 14, 0, 0, 0, time.UTC)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM processed_events WHERE event_id = $1", evID)
		_, _ = pool.Exec(ctx, "DELETE FROM hourly_stats WHERE campaign_id = $1", testCampID)
	}()

	ev := &events.Event{
		SchemaVersion: events.CurrentSchemaVersion,
		EventID:       evID,
		Type:          events.TypeImpression,
		OccurredAt:    now,
		RequestID:     "req-dup",
		BidID:         bidID,
		CampaignID:    testCampID,
		SpendMicros:   2000,
	}

	// Batch has 3 identical copies
	newCount, dupCount, err := store.ProcessBatch(ctx, []*events.Event{ev, ev, ev})
	require.NoError(t, err)
	require.Equal(t, 1, newCount)
	require.Equal(t, 2, dupCount)

	var imps, spend int64
	err = pool.QueryRow(ctx, "SELECT impressions, spend_micros FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", testCampID, hourStart).Scan(&imps, &spend)
	require.NoError(t, err)
	require.Equal(t, int64(1), imps)
	require.Equal(t, int64(2000), spend)
}

func TestStore_MixedBatchAcrossHours(t *testing.T) {
	store, pool := setupTestStore(t)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	campA := uuid.New().String()
	campB := uuid.New().String()

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM hourly_stats WHERE campaign_id IN ($1, $2)", campA, campB)
	}()

	h10 := time.Date(2026, 11, 19, 10, 15, 0, 0, time.UTC)
	h11 := time.Date(2026, 11, 19, 11, 15, 0, 0, time.UTC)

	var batch []*events.Event

	// Generate 1000 events:
	// - 300 impressions for campA at h10 (spend 1000 each = 300,000 micros)
	// - 200 clicks for campA at h10
	// - 100 auctions at h10: participants [campA, campB], winner campA
	// - 200 impressions for campB at h11 (spend 2000 each = 400,000 micros)
	// - 100 clicks for campB at h11
	// - 100 auctions at h11: participants [campA, campB], winner campB
	for i := 0; i < 300; i++ {
		bidID := fmt.Sprintf("b-imp-a-%d", i)
		batch = append(batch, &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       events.EventID(events.TypeImpression, bidID),
			Type:          events.TypeImpression,
			OccurredAt:    h10,
			BidID:         bidID,
			CampaignID:    campA,
			SpendMicros:   1000,
		})
	}
	for i := 0; i < 200; i++ {
		bidID := fmt.Sprintf("b-clk-a-%d", i)
		batch = append(batch, &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       events.EventID(events.TypeClick, bidID),
			Type:          events.TypeClick,
			OccurredAt:    h10,
			BidID:         bidID,
			CampaignID:    campA,
			SpendMicros:   0,
		})
	}
	for i := 0; i < 100; i++ {
		bidID := fmt.Sprintf("b-auc-10-%d", i)
		batch = append(batch, &events.Event{
			SchemaVersion:    events.CurrentSchemaVersion,
			EventID:          events.EventID(events.TypeAuction, bidID),
			Type:             events.TypeAuction,
			OccurredAt:       h10,
			BidID:            bidID,
			Participants:     []string{campA, campB},
			WinnerCampaignID: campA,
		})
	}
	for i := 0; i < 200; i++ {
		bidID := fmt.Sprintf("b-imp-b-%d", i)
		batch = append(batch, &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       events.EventID(events.TypeImpression, bidID),
			Type:          events.TypeImpression,
			OccurredAt:    h11,
			BidID:         bidID,
			CampaignID:    campB,
			SpendMicros:   2000,
		})
	}
	for i := 0; i < 100; i++ {
		bidID := fmt.Sprintf("b-clk-b-%d", i)
		batch = append(batch, &events.Event{
			SchemaVersion: events.CurrentSchemaVersion,
			EventID:       events.EventID(events.TypeClick, bidID),
			Type:          events.TypeClick,
			OccurredAt:    h11,
			BidID:         bidID,
			CampaignID:    campB,
			SpendMicros:   0,
		})
	}
	for i := 0; i < 100; i++ {
		bidID := fmt.Sprintf("b-auc-11-%d", i)
		batch = append(batch, &events.Event{
			SchemaVersion:    events.CurrentSchemaVersion,
			EventID:          events.EventID(events.TypeAuction, bidID),
			Type:             events.TypeAuction,
			OccurredAt:       h11,
			BidID:            bidID,
			Participants:     []string{campA, campB},
			WinnerCampaignID: campB,
		})
	}

	require.Len(t, batch, 1000)

	newCount, dupCount, err := store.ProcessBatch(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, 1000, newCount)
	require.Equal(t, 0, dupCount)

	// Verify CampA at 10:00:
	// impressions: 300, clicks: 200, spend: 300,000, bids: 100, wins: 100
	var imps, clks, spnd, bids, wins int64
	err = pool.QueryRow(ctx, "SELECT impressions, clicks, spend_micros, bids, wins FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", campA, time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)).Scan(&imps, &clks, &spnd, &bids, &wins)
	require.NoError(t, err)
	require.Equal(t, int64(300), imps)
	require.Equal(t, int64(200), clks)
	require.Equal(t, int64(300000), spnd)
	require.Equal(t, int64(100), bids)
	require.Equal(t, int64(100), wins)

	// Verify CampB at 10:00:
	// bids: 100, wins: 0
	err = pool.QueryRow(ctx, "SELECT impressions, clicks, spend_micros, bids, wins FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", campB, time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)).Scan(&imps, &clks, &spnd, &bids, &wins)
	require.NoError(t, err)
	require.Equal(t, int64(0), imps)
	require.Equal(t, int64(0), clks)
	require.Equal(t, int64(0), spnd)
	require.Equal(t, int64(100), bids)
	require.Equal(t, int64(0), wins)

	// Verify CampB at 11:00:
	// impressions: 200, clicks: 100, spend: 400,000, bids: 100, wins: 100
	err = pool.QueryRow(ctx, "SELECT impressions, clicks, spend_micros, bids, wins FROM hourly_stats WHERE campaign_id = $1 AND hour_start = $2", campB, time.Date(2026, 11, 19, 11, 0, 0, 0, time.UTC)).Scan(&imps, &clks, &spnd, &bids, &wins)
	require.NoError(t, err)
	require.Equal(t, int64(200), imps)
	require.Equal(t, int64(100), clks)
	require.Equal(t, int64(400000), spnd)
	require.Equal(t, int64(100), bids)
	require.Equal(t, int64(100), wins)

	// Cleanup test events
	for _, ev := range batch {
		_, _ = pool.Exec(ctx, "DELETE FROM processed_events WHERE event_id = $1", ev.EventID)
	}
}

func TestStore_RetentionCleanup(t *testing.T) {
	store, pool := setupTestStore(t)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oldID := uuid.New().String()
	newID := uuid.New().String()

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	olderTime := cutoff.Add(-1 * time.Hour)
	newerTime := cutoff.Add(1 * time.Hour)

	_, err := pool.Exec(ctx, "INSERT INTO processed_events (event_id, event_type, processed_at) VALUES ($1, $2, $3), ($4, $5, $6)",
		oldID, "impression", olderTime,
		newID, "impression", newerTime,
	)
	require.NoError(t, err)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM processed_events WHERE event_id IN ($1, $2)", oldID, newID)
	}()

	deleted, err := store.DeleteOldProcessedEvents(ctx, cutoff, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))

	// Verify oldID deleted, newID preserved
	var countOld, countNew int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id = $1", oldID).Scan(&countOld))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM processed_events WHERE event_id = $1", newID).Scan(&countNew))
	require.Equal(t, 0, countOld)
	require.Equal(t, 1, countNew)
}
