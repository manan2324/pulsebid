package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) QueryStats(ctx context.Context, params QueryParams) ([]RawStatsRow, error) {
	args := m.Called(ctx, params)
	if rows, ok := args.Get(0).([]RawStatsRow); ok {
		return rows, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestService_GenerateReport_Validation(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("missing from", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			To: now,
		})
		require.ErrorIs(t, err, ErrMissingFrom)
	})

	t.Run("missing to", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From: now,
		})
		require.ErrorIs(t, err, ErrMissingTo)
	})

	t.Run("from equals to", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From: now,
			To:   now,
		})
		require.ErrorIs(t, err, ErrInvalidRange)
	})

	t.Run("from after to", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From: now.Add(time.Hour),
			To:   now,
		})
		require.ErrorIs(t, err, ErrInvalidRange)
	})

	t.Run("invalid granularity", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From:        now,
			To:          now.Add(time.Hour),
			Granularity: "minute",
		})
		require.ErrorIs(t, err, ErrInvalidGranularity)
	})

	t.Run("hour granularity exceeds 31 days", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From:        now,
			To:          now.Add(32 * 24 * time.Hour),
			Granularity: GranularityHour,
		})
		require.ErrorIs(t, err, ErrRangeTooLarge)
	})

	t.Run("day granularity exceeds 366 days", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From:        now,
			To:          now.Add(367 * 24 * time.Hour),
			Granularity: GranularityDay,
		})
		require.ErrorIs(t, err, ErrRangeTooLarge)
	})

	t.Run("total granularity exceeds 366 days", func(t *testing.T) {
		_, err := svc.GenerateReport(ctx, QueryParams{
			From:        now,
			To:          now.Add(367 * 24 * time.Hour),
			Granularity: GranularityTotal,
		})
		require.ErrorIs(t, err, ErrRangeTooLarge)
	})

	t.Run("too many campaign filters", func(t *testing.T) {
		ids := make([]uuid.UUID, 101)
		for i := range ids {
			ids[i] = uuid.New()
		}
		_, err := svc.GenerateReport(ctx, QueryParams{
			From:        now,
			To:          now.Add(24 * time.Hour),
			Granularity: GranularityTotal,
			CampaignIDs: ids,
		})
		require.ErrorIs(t, err, ErrTooManyCampaigns)
	})
}

func TestService_GenerateReport_RowLimitExceeded(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()
	from := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	params := QueryParams{
		From:        from,
		To:          to,
		Granularity: GranularityTotal,
	}

	overflow := make([]RawStatsRow, MaxRowLimit+1)
	mockRepo.On("QueryStats", ctx, params).Return(overflow, nil)

	_, err := svc.GenerateReport(ctx, params)
	require.ErrorIs(t, err, ErrRowLimitExceeded)
	mockRepo.AssertExpectations(t)
}

func TestService_GenerateReport_MathAndCalculations(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()

	from := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	bTime1 := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	bTime2 := time.Date(2026, 11, 18, 11, 0, 0, 0, time.UTC)

	camp1 := uuid.MustParse("0193f0a2-ca00-7000-8000-000000000001")
	camp2 := uuid.MustParse("0193f0a2-ca00-7000-8000-000000000002")

	params := QueryParams{
		From:        from,
		To:          to,
		Granularity: GranularityHour,
	}

	rawRows := []RawStatsRow{
		{
			CampaignID:  camp1,
			Bucket:      &bTime1,
			Impressions: 1200,
			Clicks:      36,
			SpendMicros: 1812000,
			Bids:        5000,
			Wins:        1210,
		},
		{
			CampaignID:  camp2,
			Bucket:      &bTime2,
			Impressions: 800,
			Clicks:      16,
			SpendMicros: 808000,
			Bids:        4000,
			Wins:        800,
		},
		{
			CampaignID:  camp1,
			Bucket:      &bTime2,
			Impressions: 0, // tests division by zero safeguard
			Clicks:      0,
			SpendMicros: 0,
			Bids:        0, // tests division by zero safeguard
			Wins:        0,
		},
	}

	mockRepo.On("QueryStats", ctx, params).Return(rawRows, nil)

	resp, err := svc.GenerateReport(ctx, params)
	require.NoError(t, err)
	require.Equal(t, from.Format(time.RFC3339), resp.From)
	require.Equal(t, to.Format(time.RFC3339), resp.To)
	require.Equal(t, "hour", resp.Granularity)
	require.Len(t, resp.Data, 3)

	// Row 1 checks
	r1 := resp.Data[0]
	require.Equal(t, camp1, r1.CampaignID)
	require.NotNil(t, r1.Bucket)
	require.Equal(t, "2026-11-18T10:00:00Z", *r1.Bucket)
	require.Equal(t, int64(1200), r1.Impressions)
	require.Equal(t, int64(36), r1.Clicks)
	require.Equal(t, 0.03, r1.CTR)
	require.Equal(t, 1.812, r1.Spend)
	require.Equal(t, "USD", r1.Currency)
	require.Equal(t, int64(5000), r1.Bids)
	require.Equal(t, int64(1210), r1.Wins)
	require.Equal(t, 0.242, r1.WinRate)

	// Row 2 checks
	r2 := resp.Data[1]
	require.Equal(t, camp2, r2.CampaignID)
	require.Equal(t, 0.02, r2.CTR)
	require.Equal(t, 0.808, r2.Spend)
	require.Equal(t, 0.2, r2.WinRate)

	// Row 3 checks (division by zero safeguards)
	r3 := resp.Data[2]
	require.Equal(t, 0.0, r3.CTR)
	require.Equal(t, 0.0, r3.WinRate)
	require.Equal(t, 0.0, r3.Spend)

	// Totals check
	require.Equal(t, int64(2000), resp.Totals.Impressions)
	require.Equal(t, int64(52), resp.Totals.Clicks)
	require.Equal(t, 52.0/2000.0, resp.Totals.CTR)
	require.Equal(t, 2.62, resp.Totals.Spend)
	require.Equal(t, int64(9000), resp.Totals.Bids)
	require.Equal(t, int64(2010), resp.Totals.Wins)
	require.Equal(t, 2010.0/9000.0, resp.Totals.WinRate)

	mockRepo.AssertExpectations(t)
}

func TestService_GenerateReport_EmptyData(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	ctx := context.Background()

	from := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	params := QueryParams{
		From:        from,
		To:          to,
		Granularity: GranularityTotal,
	}

	mockRepo.On("QueryStats", ctx, params).Return([]RawStatsRow{}, nil)

	resp, err := svc.GenerateReport(ctx, params)
	require.NoError(t, err)
	require.Empty(t, resp.Data)
	require.NotNil(t, resp.Data) // must serialize as [] not null
	require.Equal(t, int64(0), resp.Totals.Impressions)
	require.Equal(t, int64(0), resp.Totals.Clicks)
	require.Equal(t, 0.0, resp.Totals.CTR)
	require.Equal(t, 0.0, resp.Totals.Spend)
	require.Equal(t, int64(0), resp.Totals.Bids)
	require.Equal(t, int64(0), resp.Totals.Wins)
	require.Equal(t, 0.0, resp.Totals.WinRate)
}
