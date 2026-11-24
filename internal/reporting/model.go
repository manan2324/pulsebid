package reporting

import (
	"time"

	"github.com/google/uuid"
)

// Granularity defines the temporal grouping for reporting metrics (Section 11.3).
type Granularity string

// Supported report aggregation granularities.
const (
	GranularityHour  Granularity = "hour"
	GranularityDay   Granularity = "day"
	GranularityTotal Granularity = "total"
)

// QueryParams holds validated parameters for report queries.
type QueryParams struct {
	From        time.Time
	To          time.Time
	Granularity Granularity
	CampaignIDs []uuid.UUID
}

// ReportRow represents aggregated metrics for a single campaign and temporal bucket.
type ReportRow struct {
	CampaignID  uuid.UUID `json:"campaign_id"`
	Bucket      *string   `json:"bucket,omitempty"`
	Impressions int64     `json:"impressions"`
	Clicks      int64     `json:"clicks"`
	CTR         float64   `json:"ctr"`
	Spend       float64   `json:"spend"`
	Currency    string    `json:"currency"`
	Bids        int64     `json:"bids"`
	Wins        int64     `json:"wins"`
	WinRate     float64   `json:"win_rate"`
}

// Totals represents summary metrics aggregated across all rows in the report.
type Totals struct {
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	CTR         float64 `json:"ctr"`
	Spend       float64 `json:"spend"`
	Bids        int64   `json:"bids"`
	Wins        int64   `json:"wins"`
	WinRate     float64 `json:"win_rate"`
}

// ReportResponse is the top-level JSON envelope returned by the Reporting API (Section 11.3).
type ReportResponse struct {
	From        string      `json:"from"`
	To          string      `json:"to"`
	Granularity string      `json:"granularity"`
	Data        []ReportRow `json:"data"`
	Totals      Totals      `json:"totals"`
}

// RawStatsRow holds database-retrieved aggregation aggregates before conversion into presentation units.
type RawStatsRow struct {
	CampaignID  uuid.UUID
	Bucket      *time.Time
	Impressions int64
	Clicks      int64
	SpendMicros int64
	Bids        int64
	Wins        int64
}
