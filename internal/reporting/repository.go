package reporting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxRowLimit = 50000

// Repository defines the data access contract for reporting queries.
type Repository interface {
	QueryStats(ctx context.Context, params QueryParams) ([]RawStatsRow, error)
}

// PostgresRepository implements Repository using a PostgreSQL connection pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository instantiates a new PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// QueryStats queries the hourly_stats table aggregated by the requested granularity (hour, day, total).
func (r *PostgresRepository) QueryStats(ctx context.Context, params QueryParams) ([]RawStatsRow, error) {
	var query string
	switch params.Granularity {
	case GranularityHour:
		query = `
			SELECT
				campaign_id,
				date_trunc('hour', hour_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket,
				COALESCE(SUM(impressions), 0) AS impressions,
				COALESCE(SUM(clicks), 0) AS clicks,
				COALESCE(SUM(spend_micros), 0) AS spend_micros,
				COALESCE(SUM(bids), 0) AS bids,
				COALESCE(SUM(wins), 0) AS wins
			FROM hourly_stats
			WHERE hour_start >= $1 AND hour_start < $2
			  AND ($3::uuid[] IS NULL OR campaign_id = ANY($3))
			GROUP BY campaign_id, bucket
			ORDER BY bucket ASC, campaign_id ASC
			LIMIT 50001;
		`
	case GranularityDay:
		query = `
			SELECT
				campaign_id,
				date_trunc('day', hour_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket,
				COALESCE(SUM(impressions), 0) AS impressions,
				COALESCE(SUM(clicks), 0) AS clicks,
				COALESCE(SUM(spend_micros), 0) AS spend_micros,
				COALESCE(SUM(bids), 0) AS bids,
				COALESCE(SUM(wins), 0) AS wins
			FROM hourly_stats
			WHERE hour_start >= $1 AND hour_start < $2
			  AND ($3::uuid[] IS NULL OR campaign_id = ANY($3))
			GROUP BY campaign_id, bucket
			ORDER BY bucket ASC, campaign_id ASC
			LIMIT 50001;
		`
	case GranularityTotal:
		query = `
			SELECT
				campaign_id,
				COALESCE(SUM(impressions), 0) AS impressions,
				COALESCE(SUM(clicks), 0) AS clicks,
				COALESCE(SUM(spend_micros), 0) AS spend_micros,
				COALESCE(SUM(bids), 0) AS bids,
				COALESCE(SUM(wins), 0) AS wins
			FROM hourly_stats
			WHERE hour_start >= $1 AND hour_start < $2
			  AND ($3::uuid[] IS NULL OR campaign_id = ANY($3))
			GROUP BY campaign_id
			ORDER BY campaign_id ASC
			LIMIT 50001;
		`
	default:
		return nil, fmt.Errorf("unsupported granularity: %s", params.Granularity)
	}

	var campaignFilter []uuid.UUID
	if len(params.CampaignIDs) > 0 {
		campaignFilter = params.CampaignIDs
	}

	rows, err := r.pool.Query(ctx, query, params.From, params.To, campaignFilter)
	if err != nil {
		return nil, fmt.Errorf("querying stats: %w", err)
	}
	defer rows.Close()

	var result []RawStatsRow
	for rows.Next() {
		var rRow RawStatsRow
		if params.Granularity == GranularityTotal {
			if err := rows.Scan(
				&rRow.CampaignID,
				&rRow.Impressions,
				&rRow.Clicks,
				&rRow.SpendMicros,
				&rRow.Bids,
				&rRow.Wins,
			); err != nil {
				return nil, fmt.Errorf("scanning total stats row: %w", err)
			}
		} else {
			var bTime time.Time
			if err := rows.Scan(
				&rRow.CampaignID,
				&bTime,
				&rRow.Impressions,
				&rRow.Clicks,
				&rRow.SpendMicros,
				&rRow.Bids,
				&rRow.Wins,
			); err != nil {
				return nil, fmt.Errorf("scanning bucket stats row: %w", err)
			}
			utcTime := bTime.UTC()
			rRow.Bucket = &utcTime
		}
		result = append(result, rRow)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stats rows: %w", err)
	}

	return result, nil
}
