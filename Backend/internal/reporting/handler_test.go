package reporting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/httpx"
)

type errorEnvelope struct {
	Error httpx.Error `json:"error"`
}

func TestHandler_Auth(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, []string{"secret-key"})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/reports/campaigns?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z", nil)

	// Missing auth header -> 401
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Invalid auth header -> 401
	req.Header.Set("X-API-Key", "wrong-key")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Valid auth header -> 200
	mockRepo.On("QueryStats", mock.Anything, mock.Anything).Return([]RawStatsRow{}, nil)
	req.Header.Set("X-API-Key", "secret-key")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_CampaignReports_Validation(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, []string{"dev-key"})

	doGet := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(context.Background(), "GET", target, nil)
		req.Header.Set("X-API-Key", "dev-key")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}

	t.Run("missing from", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?to=2026-11-19T00:00:00Z")
		require.Equal(t, http.StatusBadRequest, w.Code)
		var errResp errorEnvelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
		require.Equal(t, "invalid_request", errResp.Error.Code)
		require.Contains(t, errResp.Error.Message, "from parameter is required")
	})

	t.Run("invalid from timestamp", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=invalid-date&to=2026-11-19T00:00:00Z")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing to", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=2026-11-18T00:00:00Z")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid to timestamp", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=2026-11-18T00:00:00Z&to=invalid-date")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("from after to", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=2026-11-20T00:00:00Z&to=2026-11-19T00:00:00Z")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid granularity", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z&granularity=month")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid campaign_id UUID", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z&campaign_id=not-a-uuid")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("hour range exceeds 31 days", func(t *testing.T) {
		w := doGet("/v1/reports/campaigns?from=2026-10-01T00:00:00Z&to=2026-11-19T00:00:00Z&granularity=hour")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_SingleCampaignReport(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, []string{"dev-key"})

	t.Run("invalid UUID in path", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/reports/campaigns/not-a-uuid?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z", nil)
		req.Header.Set("X-API-Key", "dev-key")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown campaign returns 200 with zero totals", func(t *testing.T) {
		unknownID := uuid.New()
		mockRepo.On("QueryStats", mock.Anything, mock.MatchedBy(func(p QueryParams) bool {
			return len(p.CampaignIDs) == 1 && p.CampaignIDs[0] == unknownID
		})).Return([]RawStatsRow{}, nil)

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/reports/campaigns/"+unknownID.String()+"?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z", nil)
		req.Header.Set("X-API-Key", "dev-key")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp ReportResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Empty(t, resp.Data)
		require.Equal(t, int64(0), resp.Totals.Impressions)
		require.Equal(t, 0.0, resp.Totals.CTR)
		require.Equal(t, 0.0, resp.Totals.Spend)
	})
}

func TestHandler_GoldenJSONResponses(t *testing.T) {
	mockRepo := new(mockRepository)
	svc := NewService(mockRepo)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, []string{"dev-key"})

	campID := uuid.MustParse("0193f0a2-ca00-7000-8000-000000000001")
	bTime := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

	t.Run("granularity hour includes bucket", func(t *testing.T) {
		mockRepo.On("QueryStats", mock.Anything, mock.MatchedBy(func(p QueryParams) bool {
			return p.Granularity == GranularityHour
		})).Return([]RawStatsRow{
			{
				CampaignID:  campID,
				Bucket:      &bTime,
				Impressions: 1200,
				Clicks:      36,
				SpendMicros: 1812000,
				Bids:        5000,
				Wins:        1210,
			},
		}, nil).Once()

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/reports/campaigns?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z&granularity=hour", nil)
		req.Header.Set("X-API-Key", "dev-key")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		expectedJSON := `{
			"from": "2026-11-18T00:00:00Z",
			"to": "2026-11-19T00:00:00Z",
			"granularity": "hour",
			"data": [
				{
					"campaign_id": "0193f0a2-ca00-7000-8000-000000000001",
					"bucket": "2026-11-18T10:00:00Z",
					"impressions": 1200,
					"clicks": 36,
					"ctr": 0.03,
					"spend": 1.812,
					"currency": "USD",
					"bids": 5000,
					"wins": 1210,
					"win_rate": 0.242
				}
			],
			"totals": {
				"impressions": 1200,
				"clicks": 36,
				"ctr": 0.03,
				"spend": 1.812,
				"bids": 5000,
				"wins": 1210,
				"win_rate": 0.242
			}
		}`
		require.JSONEq(t, expectedJSON, w.Body.String())
	})

	t.Run("granularity total omits bucket", func(t *testing.T) {
		mockRepo.On("QueryStats", mock.Anything, mock.MatchedBy(func(p QueryParams) bool {
			return p.Granularity == GranularityTotal
		})).Return([]RawStatsRow{
			{
				CampaignID:  campID,
				Bucket:      nil, // total has no bucket
				Impressions: 1200,
				Clicks:      36,
				SpendMicros: 1812000,
				Bids:        5000,
				Wins:        1210,
			},
		}, nil).Once()

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/reports/campaigns?from=2026-11-18T00:00:00Z&to=2026-11-19T00:00:00Z&granularity=total", nil)
		req.Header.Set("X-API-Key", "dev-key")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var raw map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
		dataList := raw["data"].([]any)
		firstRow := dataList[0].(map[string]any)
		_, hasBucket := firstRow["bucket"]
		require.False(t, hasBucket, "bucket must be omitted for total granularity")
	})
}
