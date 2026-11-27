package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"pulsebid/internal/events"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines persistence operations for event deduplication and rollups.
type Store interface {
	ProcessBatch(ctx context.Context, evs []*events.Event) (newCount, dupCount int, err error)
	DeleteOldProcessedEvents(ctx context.Context, cutoff time.Time, batchLimit int) (int64, error)
	Ping(ctx context.Context) error
	Close()
}

// PGStore implements Store using PostgreSQL and pgxpool.
type PGStore struct {
	pool    *pgxpool.Pool
	metrics *Metrics
	logger  *slog.Logger
}

// NewPGStore wraps an existing pgxpool.Pool.
func NewPGStore(pool *pgxpool.Pool, metrics *Metrics, logger *slog.Logger) *PGStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &PGStore{
		pool:    pool,
		metrics: metrics,
		logger:  logger,
	}
}

// NewPGStoreFromURL creates a pgxpool.Pool connected to databaseURL.
func NewPGStoreFromURL(ctx context.Context, databaseURL string, metrics *Metrics, logger *slog.Logger) (*PGStore, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 32
	cfg.MinConns = 4
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return NewPGStore(pool, metrics, logger), nil
}

// Ping checks database connectivity.
func (s *PGStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close closes the underlying connection pool.
func (s *PGStore) Close() {
	s.pool.Close()
}

// Pool returns the underlying pgxpool.Pool for query operations.
func (s *PGStore) Pool() *pgxpool.Pool {
	return s.pool
}

const insertProcessedEventsSQL = `
INSERT INTO processed_events (event_id, event_type)
SELECT * FROM unnest($1::uuid[], $2::text[])
ON CONFLICT (event_id) DO NOTHING
RETURNING event_id;
`

const upsertHourlyStatsSQL = `
INSERT INTO hourly_stats (campaign_id, hour_start, impressions, clicks, spend_micros, bids, wins)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (campaign_id, hour_start) DO UPDATE SET
  impressions  = hourly_stats.impressions  + EXCLUDED.impressions,
  clicks       = hourly_stats.clicks       + EXCLUDED.clicks,
  spend_micros = hourly_stats.spend_micros + EXCLUDED.spend_micros,
  bids         = hourly_stats.bids         + EXCLUDED.bids,
  wins         = hourly_stats.wins         + EXCLUDED.wins,
  updated_at   = now();
`

// ProcessBatch runs deduplication and hourly rollup upserts in a single database transaction (Section 10.5).
func (s *PGStore) ProcessBatch(ctx context.Context, evs []*events.Event) (newCount, dupCount int, err error) {
	if len(evs) == 0 {
		return 0, 0, nil
	}

	// 1. Gather unique event IDs in batch for ledger insert
	seenInBatch := make(map[string]struct{}, len(evs))
	ids := make([]string, 0, len(evs))
	types := make([]string, 0, len(evs))

	for _, ev := range evs {
		if ev.EventID == "" {
			continue
		}
		if _, seen := seenInBatch[ev.EventID]; !seen {
			seenInBatch[ev.EventID] = struct{}{}
			ids = append(ids, ev.EventID)
			types = append(types, ev.Type)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 2. Insert into processed_events; returns only newly inserted IDs
	newIDs := make(map[string]struct{}, len(ids))
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, insertProcessedEventsSQL, ids, types)
		if err != nil {
			return 0, 0, fmt.Errorf("insert processed_events: %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return 0, 0, fmt.Errorf("scan new event_id: %w", err)
			}
			newIDs[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			return 0, 0, fmt.Errorf("iterate new event_id rows: %w", err)
		}
	}

	// 3. Separate new events vs duplicates and aggregate rollups
	rollups := make(map[RollupKey]*RollupData)
	countedInBatch := make(map[string]struct{}, len(evs))
	var newEvents []*events.Event

	for _, ev := range evs {
		if _, isNew := newIDs[ev.EventID]; isNew {
			if _, already := countedInBatch[ev.EventID]; !already {
				countedInBatch[ev.EventID] = struct{}{}
				newCount++
				newEvents = append(newEvents, ev)
				if s.metrics != nil {
					s.metrics.EventsTotal.WithLabelValues(ev.Type, "new").Inc()
				}
				continue
			}
		}
		dupCount++
		if s.metrics != nil {
			s.metrics.EventsTotal.WithLabelValues(ev.Type, "duplicate").Inc()
		}
	}

	// 4. In-memory aggregation of new events by (campaign_id, hour_start)
	for _, ev := range newEvents {
		hourStart := ev.OccurredAt.UTC().Truncate(time.Hour)

		switch ev.Type {
		case events.TypeImpression:
			if ev.CampaignID != "" {
				k := RollupKey{CampaignID: ev.CampaignID, HourStart: hourStart}
				r := getOrCreateRollup(rollups, k)
				r.Impressions++
				r.SpendMicros += ev.SpendMicros
			}
		case events.TypeClick:
			if ev.CampaignID != "" {
				k := RollupKey{CampaignID: ev.CampaignID, HourStart: hourStart}
				r := getOrCreateRollup(rollups, k)
				r.Clicks++
			}
		case events.TypeAuction:
			// For auction events: for each participant bids += 1; winner wins += 1 (Section 10.5)
			for _, participantID := range ev.Participants {
				if participantID != "" {
					k := RollupKey{CampaignID: participantID, HourStart: hourStart}
					r := getOrCreateRollup(rollups, k)
					r.Bids++
				}
			}
			winnerID := ev.WinnerCampaignID
			if winnerID == "" {
				winnerID = ev.CampaignID
			}
			if winnerID != "" {
				k := RollupKey{CampaignID: winnerID, HourStart: hourStart}
				r := getOrCreateRollup(rollups, k)
				r.Wins++
			}
		}
	}

	// 5. Sort rollup keys to prevent deadlocks across concurrent aggregators (Section 10.5)
	if len(rollups) > 0 {
		keys := make([]RollupKey, 0, len(rollups))
		for k := range rollups {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].CampaignID != keys[j].CampaignID {
				return keys[i].CampaignID < keys[j].CampaignID
			}
			return keys[i].HourStart.Before(keys[j].HourStart)
		})

		batch := &pgx.Batch{}
		for _, k := range keys {
			r := rollups[k]
			batch.Queue(upsertHourlyStatsSQL, k.CampaignID, k.HourStart, r.Impressions, r.Clicks, r.SpendMicros, r.Bids, r.Wins)
		}

		br := tx.SendBatch(ctx, batch)
		for i := 0; i < len(keys); i++ {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return 0, 0, fmt.Errorf("exec rollup batch item %d: %w", i, err)
			}
		}
		if err := br.Close(); err != nil {
			return 0, 0, fmt.Errorf("close rollup batch: %w", err)
		}
	}

	// 6. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit batch transaction: %w", err)
	}

	return newCount, dupCount, nil
}

// DeleteOldProcessedEvents cleans up processed_events older than cutoff in batches of batchLimit (Section 8).
func (s *PGStore) DeleteOldProcessedEvents(ctx context.Context, cutoff time.Time, batchLimit int) (int64, error) {
	if batchLimit <= 0 {
		batchLimit = 10000
	}
	const deleteSQL = `
DELETE FROM processed_events
WHERE ctid IN (
    SELECT ctid FROM processed_events
    WHERE processed_at < $1
    LIMIT $2
);
`
	var totalDeleted int64
	for {
		res, err := s.pool.Exec(ctx, deleteSQL, cutoff, batchLimit)
		if err != nil {
			return totalDeleted, fmt.Errorf("delete old processed_events: %w", err)
		}
		deleted := res.RowsAffected()
		totalDeleted += deleted
		if deleted < int64(batchLimit) {
			break
		}
	}
	return totalDeleted, nil
}

func getOrCreateRollup(m map[RollupKey]*RollupData, k RollupKey) *RollupData {
	r, ok := m[k]
	if !ok {
		r = &RollupData{}
		m[k] = r
	}
	return r
}
