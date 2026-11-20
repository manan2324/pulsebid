package aggregator

import "time"

// RollupKey identifies an hourly rollup bucket per campaign.
type RollupKey struct {
	CampaignID string
	HourStart  time.Time
}

// RollupData contains aggregated metrics for an hourly bucket.
type RollupData struct {
	Impressions int64
	Clicks      int64
	SpendMicros int64
	Bids        int64
	Wins        int64
}

// BatchResult summarizes the outcome of processing a batch of records.
type BatchResult struct {
	Total     int
	New       int
	Duplicate int
	DLQ       int
}
