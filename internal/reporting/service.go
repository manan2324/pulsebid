package reporting

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Report input validation errors.
var (
	ErrMissingFrom        = errors.New("missing required parameter: from")
	ErrMissingTo          = errors.New("missing required parameter: to")
	ErrInvalidRange       = errors.New("from must be strictly before to")
	ErrInvalidGranularity = errors.New("granularity must be hour, day, or total")
	ErrRangeTooLarge      = errors.New("range exceeds maximum allowed duration")
	ErrTooManyCampaigns   = errors.New("maximum of 100 campaign_id filters allowed")
	ErrRowLimitExceeded   = errors.New("result exceeds hard cap of 50,000 rows")
)

// Maximum time ranges for each reporting granularity.
const (
	MaxHourRangeDuration = 31 * 24 * time.Hour
	MaxDayRangeDuration  = 366 * 24 * time.Hour
	MaxCampaignFilters   = 100
)

// Service coordinates validation, database queries, and metric calculations for reporting.
type Service struct {
	repo Repository
}

// NewService constructs a Service with the given repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GenerateReport validates query parameters, executes aggregation, and formats the report response.
func (s *Service) GenerateReport(ctx context.Context, params QueryParams) (*ReportResponse, error) {
	if params.From.IsZero() {
		return nil, ErrMissingFrom
	}
	if params.To.IsZero() {
		return nil, ErrMissingTo
	}
	if !params.From.Before(params.To) {
		return nil, ErrInvalidRange
	}

	if params.Granularity == "" {
		params.Granularity = GranularityTotal
	}
	switch params.Granularity {
	case GranularityHour:
		if params.To.Sub(params.From) > MaxHourRangeDuration {
			return nil, fmt.Errorf("%w: hour granularity maximum range is 31 days", ErrRangeTooLarge)
		}
	case GranularityDay, GranularityTotal:
		if params.To.Sub(params.From) > MaxDayRangeDuration {
			return nil, fmt.Errorf("%w: maximum range is 366 days", ErrRangeTooLarge)
		}
	default:
		return nil, ErrInvalidGranularity
	}

	if len(params.CampaignIDs) > MaxCampaignFilters {
		return nil, ErrTooManyCampaigns
	}

	rawRows, err := s.repo.QueryStats(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("retrieving stats: %w", err)
	}

	if len(rawRows) > MaxRowLimit {
		return nil, ErrRowLimitExceeded
	}

	data := make([]ReportRow, 0, len(rawRows))
	var totImps, totClicks, totSpendMicros, totBids, totWins int64

	for _, r := range rawRows {
		var ctr float64
		if r.Impressions > 0 {
			ctr = float64(r.Clicks) / float64(r.Impressions)
		}

		var winRate float64
		if r.Bids > 0 {
			winRate = float64(r.Wins) / float64(r.Bids)
		}

		spend := float64(r.SpendMicros) / 1_000_000.0

		var bucketStr *string
		if r.Bucket != nil {
			s := r.Bucket.UTC().Format(time.RFC3339)
			bucketStr = &s
		}

		data = append(data, ReportRow{
			CampaignID:  r.CampaignID,
			Bucket:      bucketStr,
			Impressions: r.Impressions,
			Clicks:      r.Clicks,
			CTR:         ctr,
			Spend:       spend,
			Currency:    "USD",
			Bids:        r.Bids,
			Wins:        r.Wins,
			WinRate:     winRate,
		})

		totImps += r.Impressions
		totClicks += r.Clicks
		totSpendMicros += r.SpendMicros
		totBids += r.Bids
		totWins += r.Wins
	}

	var totCTR float64
	if totImps > 0 {
		totCTR = float64(totClicks) / float64(totImps)
	}

	var totWinRate float64
	if totBids > 0 {
		totWinRate = float64(totWins) / float64(totBids)
	}

	totSpend := float64(totSpendMicros) / 1_000_000.0

	return &ReportResponse{
		From:        params.From.UTC().Format(time.RFC3339),
		To:          params.To.UTC().Format(time.RFC3339),
		Granularity: string(params.Granularity),
		Data:        data,
		Totals: Totals{
			Impressions: totImps,
			Clicks:      totClicks,
			CTR:         totCTR,
			Spend:       totSpend,
			Bids:        totBids,
			Wins:        totWins,
			WinRate:     totWinRate,
		},
	}, nil
}
