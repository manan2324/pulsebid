//go:build integration

package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/budget"
	"pulsebid/internal/platform/config"
	"pulsebid/internal/platform/grpcx"
	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/tracing"
	"pulsebid/migrations"
)

type mockBudgetReserver struct {
	spent int64
}

func (m *mockBudgetReserver) Reserve(context.Context, budget.CampaignBudget, int64, time.Time) (budget.Result, error) {
	return budget.Reserved, nil
}
func (m *mockBudgetReserver) Release(context.Context, string, int64, time.Time) error { return nil }
func (m *mockBudgetReserver) Spent(context.Context, string, time.Time) (int64, error) {
	return m.spent, nil
}
func (m *mockBudgetReserver) Reset(context.Context, string, time.Time) error {
	m.spent = 0
	return nil
}

func bodyAs[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result), w.Body.String())
	return result
}
func send(t *testing.T, h http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	require.Equal(t, status, w.Code, w.Body.String())
	require.NotEmpty(t, w.Header().Get("X-Request-Id"))
	if status >= 400 {
		envelope := bodyAs[struct {
			Error httpx.Error `json:"error"`
		}](t, w)
		require.NotEmpty(t, envelope.Error.Code)
		require.Equal(t, w.Header().Get("X-Request-Id"), envelope.Error.RequestID)
	}
	return w
}

func TestPostgresIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "postgres:16.8-alpine", postgres.WithDatabase("pulsebid"), postgres.WithUsername("pulsebid"), postgres.WithPassword("pulsebid"), postgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		require.NoError(t, container.Terminate(cleanup))
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, migrations.Apply(ctx, dsn, "up"))
	require.NoError(t, migrations.Apply(ctx, dsn, "up"))
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	service := NewService(NewStore(pool)).WithBudget(&mockBudgetReserver{spent: 1000}, true)
	handler := httpx.Chain(service.Handler([]string{"test-key"}), httpx.RequestID, httpx.Recover)
	var advertiser Advertiser
	var bidder Bidder
	var campaign Campaign
	var creative Creative

	t.Run("full CRUD and exact money", func(t *testing.T) {
		var version int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT version FROM catalog_version").Scan(&version))
		require.Equal(t, int64(1), version)
		advertiser = bodyAs[Advertiser](t, send(t, handler, "POST", "/v1/advertisers", `{"name":"Integration advertiser","contact_email":"ads@example.test"}`, 201))
		require.Equal(t, uuid.Version(7), uuid.MustParse(advertiser.ID).Version())
		require.NoError(t, pool.QueryRow(ctx, "SELECT version FROM catalog_version").Scan(&version))
		require.Equal(t, int64(2), version)
		send(t, handler, "GET", "/v1/advertisers/"+advertiser.ID, "", 200)
		updated := bodyAs[Advertiser](t, send(t, handler, "PATCH", "/v1/advertisers/"+advertiser.ID, `{"contact_email":null}`, 200))
		require.Nil(t, updated.ContactEmail)
		require.Equal(t, advertiser.Name, updated.Name)
		bidder = bodyAs[Bidder](t, send(t, handler, "POST", "/v1/bidders", `{"id":"bidder-1","name":"Bidder","endpoint_url":"http://localhost:9101/bid","timeout_ms":100}`, 201))
		require.True(t, bidder.Enabled)
		updatedBidder := bodyAs[Bidder](t, send(t, handler, "PATCH", "/v1/bidders/bidder-1", `{"enabled":false,"timeout_ms":null}`, 200))
		require.False(t, updatedBidder.Enabled)
		require.Nil(t, updatedBidder.TimeoutMs)
		send(t, handler, "PATCH", "/v1/bidders/bidder-1", `{"enabled":true}`, 200)
		send(t, handler, "GET", "/v1/bidders/bidder-1", "", 200)
		input := fmt.Sprintf(`{"advertiser_id":%q,"bidder_id":"bidder-1","name":"Campaign","daily_budget":50.123456,"max_bid_cpm":"3.000001","start_at":"2026-11-18T10:00:00+05:30"}`, advertiser.ID)
		campaign = bodyAs[Campaign](t, send(t, handler, "POST", "/v1/campaigns", input, 201))
		require.Equal(t, "draft", campaign.Status)
		require.Equal(t, int64(50_123456), int64(campaign.DailyBudget))
		require.Equal(t, int64(3_000001), int64(campaign.MaxBidCPM))
		require.Equal(t, time.UTC, campaign.StartAt.Location())
		send(t, handler, "GET", "/v1/campaigns/"+campaign.ID, "", 200)
		base := "/v1/campaigns/" + campaign.ID
		send(t, handler, "POST", base+"/activate", "", 409)
		rule := bodyAs[Targeting](t, send(t, handler, "GET", base+"/targeting", "", 200))
		require.Equal(t, []string{}, rule.Countries)
		require.Equal(t, "UTC", rule.Timezone)
		rule = bodyAs[Targeting](t, send(t, handler, "PUT", base+"/targeting", `{"countries":["in","IN","US"],"device_types":["mobile"],"hours_of_day":[3,3,5],"timezone":"Asia/Kolkata"}`, 200))
		require.Equal(t, []string{"IN", "US"}, rule.Countries)
		require.Equal(t, []int16{3, 5}, rule.HoursOfDay)
		rule = bodyAs[Targeting](t, send(t, handler, "PUT", base+"/targeting", `{"timezone":"UTC"}`, 200))
		require.Empty(t, rule.Countries)
		require.Empty(t, rule.HoursOfDay)
		creative = bodyAs[Creative](t, send(t, handler, "POST", base+"/creatives", `{"name":"Banner","format":"banner","width":300,"height":250,"markup":"<b>ad</b>","landing_url":"https://example.test"}`, 201))
		send(t, handler, "GET", "/v1/creatives/"+creative.ID, "", 200)
		send(t, handler, "GET", base+"/creatives", "", 200)
		paused := bodyAs[Creative](t, send(t, handler, "PATCH", "/v1/creatives/"+creative.ID, `{"status":"paused","width":null}`, 200))
		require.Nil(t, paused.Width)
		send(t, handler, "POST", base+"/activate", "", 409)
		send(t, handler, "PATCH", "/v1/creatives/"+creative.ID, `{"status":"active","width":300}`, 200)
		active := bodyAs[Campaign](t, send(t, handler, "POST", base+"/activate", "", 200))
		require.Equal(t, "active", active.Status)
		send(t, handler, "POST", base+"/pause", "", 200)
		send(t, handler, "POST", base+"/activate", "", 200)
		modified := bodyAs[Campaign](t, send(t, handler, "PATCH", base, `{"start_at":null,"daily_budget":0.01,"pacing_mode":"asap"}`, 200))
		require.Nil(t, modified.StartAt)
		require.Equal(t, int64(10000), int64(modified.DailyBudget))
		require.Equal(t, int64(campaign.MaxBidCPM), int64(modified.MaxBidCPM))
		page := bodyAs[Page[Campaign]](t, send(t, handler, "GET", "/v1/campaigns?advertiser_id="+advertiser.ID+"&status=active", "", 200))
		require.Len(t, page.Data, 1)
		page = bodyAs[Page[Campaign]](t, send(t, handler, "GET", "/v1/campaigns?advertiser_id="+strings.ToUpper(advertiser.ID), "", 200))
		require.Len(t, page.Data, 1)
		send(t, handler, "GET", "/v1/advertisers", "", 200)
		send(t, handler, "GET", "/v1/bidders", "", 200)
	})

	t.Run("validation auth foreign keys and not found", func(t *testing.T) {
		request := httptest.NewRequestWithContext(ctx, "GET", "/v1/advertisers", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		require.Equal(t, 401, w.Code)
		missing := uuid.NewString()
		for _, resource := range []string{"advertisers", "campaigns", "creatives"} {
			for _, method := range []string{"GET", "PATCH", "DELETE"} {
				body := ""
				if method == "PATCH" {
					body = `{}`
				}
				send(t, handler, method, "/v1/"+resource+"/"+missing, body, 404)
				send(t, handler, method, "/v1/"+resource+"/not-uuid", body, 400)
			}
		}
		for _, method := range []string{"GET", "PATCH", "DELETE"} {
			send(t, handler, method, "/v1/bidders/missing-bidder", `{}`, 404)
			send(t, handler, method, "/v1/bidders/INVALID", `{}`, 400)
		}
		for _, suffix := range []string{"targeting", "creatives"} {
			send(t, handler, "GET", "/v1/campaigns/"+missing+"/"+suffix, "", 404)
		}
		send(t, handler, "PUT", "/v1/campaigns/"+missing+"/targeting", `{}`, 404)
		send(t, handler, "POST", "/v1/advertisers", `{"name":"INTEGRATION ADVERTISER"}`, 409)
		send(t, handler, "POST", "/v1/bidders", `{"id":"bidder-1","name":"same","endpoint_url":"https://example.test"}`, 409)
		send(t, handler, "DELETE", "/v1/bidders/bidder-1", "", 409)
		base := fmt.Sprintf(`"advertiser_id":%q,"bidder_id":"bidder-1","name":"Negative","daily_budget":1,"max_bid_cpm":1`, advertiser.ID)
		for _, body := range []string{`{` + strings.Replace(base, `"daily_budget":1`, `"daily_budget":-1`, 1) + `}`, `{` + strings.Replace(base, `"max_bid_cpm":1`, `"max_bid_cpm":0.1234567`, 1) + `}`, `{` + base + `,"status":"active"}`} {
			send(t, handler, "POST", "/v1/campaigns", body, 400)
		}
		send(t, handler, "POST", "/v1/campaigns", fmt.Sprintf(`{"advertiser_id":%q,"bidder_id":"bidder-1","name":"FK","daily_budget":1,"max_bid_cpm":1}`, missing), 409)
		send(t, handler, "POST", "/v1/campaigns", `{`+strings.Replace(base, "bidder-1", "missing-bidder", 1)+`}`, 409)
		for _, body := range []string{`{"typo":true}`, `{"name":null}`, `{"daily_budget":null}`, `{"start_at":"bad"}`, `{"start_at":"2026-01-02T00:00:00Z","end_at":"2026-01-01T00:00:00Z"}`} {
			send(t, handler, "PATCH", "/v1/campaigns/"+campaign.ID, body, 400)
		}
		send(t, handler, "PUT", "/v1/campaigns/"+campaign.ID+"/targeting", `{"timezone":"bad/timezone"}`, 400)
		send(t, handler, "PATCH", "/v1/bidders/bidder-1", `{"enabled":null}`, 400)
		send(t, handler, "POST", "/v1/advertisers", `{"name":"bad","extra":1}`, 400)
		for _, query := range []string{"limit=201", "limit=-1", "cursor=garbage", "status=unknown", "advertiser_id=bad"} {
			send(t, handler, "GET", "/v1/campaigns?"+query, "", 400)
		}
		send(t, handler, "POST", "/v1/advertisers", strings.Repeat("x", 512<<10+1), 413)
		request = httptest.NewRequestWithContext(ctx, "POST", "/v1/advertisers", strings.NewReader(`{}`))
		request.Header.Set("X-API-Key", "test-key")
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		require.Equal(t, 415, w.Code)
		bView := bodyAs[map[string]any](t, send(t, handler, "GET", "/v1/campaigns/"+campaign.ID+"/budget", "", 200))
		require.Equal(t, campaign.ID, bView["campaign_id"])
		// The CRUD subtest changed this campaign's daily budget to 0.01.
		require.Equal(t, float64(0.01), bView["daily_budget"])
		require.Equal(t, float64(10000), bView["daily_budget_micros"])
		require.Equal(t, float64(0.001), bView["spent"])
		require.Equal(t, float64(1000), bView["spent_micros"])
		send(t, handler, "POST", "/v1/campaigns/"+campaign.ID+"/budget/reset", "", 200)
	})

	t.Run("pagination ties and insertion stability", func(t *testing.T) {
		var identifiers []string
		for i := range 3 {
			a := bodyAs[Advertiser](t, send(t, handler, "POST", "/v1/advertisers", fmt.Sprintf(`{"name":"Page %d"}`, i), 201))
			identifiers = append(identifiers, a.ID)
			_, err := pool.Exec(ctx, "UPDATE advertisers SET created_at='2030-01-01T00:00:00Z' WHERE id=$1", a.ID)
			require.NoError(t, err)
		}
		slices.Sort(identifiers)
		slices.Reverse(identifiers)
		first := bodyAs[Page[Advertiser]](t, send(t, handler, "GET", "/v1/advertisers?limit=2", "", 200))
		require.Equal(t, identifiers[0], first.Data[0].ID)
		require.Equal(t, identifiers[1], first.Data[1].ID)
		require.NotNil(t, first.NextCursor)
		newer := bodyAs[Advertiser](t, send(t, handler, "POST", "/v1/advertisers", `{"name":"Inserted after page"}`, 201))
		_, err := pool.Exec(ctx, "UPDATE advertisers SET created_at='2031-01-01T00:00:00Z' WHERE id=$1", newer.ID)
		require.NoError(t, err)
		seen := map[string]bool{first.Data[0].ID: true, first.Data[1].ID: true}
		cursor := first.NextCursor
		for cursor != nil {
			page := bodyAs[Page[Advertiser]](t, send(t, handler, "GET", "/v1/advertisers?limit=2&cursor="+url.QueryEscape(*cursor), "", 200))
			for _, row := range page.Data {
				require.False(t, seen[row.ID])
				require.NotEqual(t, newer.ID, row.ID)
				seen[row.ID] = true
			}
			cursor = page.NextCursor
		}
		require.True(t, seen[identifiers[2]])
		send(t, handler, "GET", "/v1/bidders?cursor="+*first.NextCursor, "", 400)
		send(t, handler, "POST", "/v1/bidders", `{"id":"bidder-2","name":"Second","endpoint_url":"http://localhost:9102/bid"}`, 201)
		bidders := bodyAs[Page[Bidder]](t, send(t, handler, "GET", "/v1/bidders?limit=1", "", 200))
		require.NotNil(t, bidders.NextCursor)
		send(t, handler, "GET", "/v1/bidders?limit=1&cursor="+*bidders.NextCursor, "", 200)
		extra := bodyAs[Creative](t, send(t, handler, "POST", "/v1/campaigns/"+campaign.ID+"/creatives", `{"name":"Extra","format":"native","markup":"ad","landing_url":"https://example.test"}`, 201))
		creatives := bodyAs[Page[Creative]](t, send(t, handler, "GET", "/v1/campaigns/"+campaign.ID+"/creatives?limit=1", "", 200))
		require.NotNil(t, creatives.NextCursor)
		send(t, handler, "GET", "/v1/campaigns/"+campaign.ID+"/creatives?limit=1&cursor="+*creatives.NextCursor, "", 200)
		send(t, handler, "DELETE", "/v1/creatives/"+extra.ID, "", 204)
		send(t, handler, "DELETE", "/v1/bidders/bidder-2", "", 204)
	})

	t.Run("concurrent patches and terminal state", func(t *testing.T) {
		results := make(chan error, 2)
		for _, body := range []string{`{"name":"Concurrent name"}`, `{"max_bid_cpm":2.123456}`} {
			go func() { _, err := service.PatchCampaign(ctx, campaign.ID, patch(body)); results <- err }()
		}
		require.NoError(t, <-results)
		require.NoError(t, <-results)
		updated, err := service.Campaign(ctx, campaign.ID)
		require.NoError(t, err)
		require.Equal(t, "Concurrent name", updated.Name)
		require.Equal(t, int64(2_123456), int64(updated.MaxBidCPM))
		send(t, handler, "DELETE", "/v1/campaigns/"+campaign.ID, "", 204)
		send(t, handler, "POST", "/v1/campaigns/"+campaign.ID+"/activate", "", 409)
		send(t, handler, "PATCH", "/v1/campaigns/"+campaign.ID, `{"status":"draft"}`, 409)
		send(t, handler, "DELETE", "/v1/advertisers/"+advertiser.ID, "", 204)
		send(t, handler, "POST", "/v1/advertisers", `{"name":"Integration advertiser"}`, 201)
		// Archived campaigns still reference bidders, so deletion must still conflict.
		send(t, handler, "DELETE", "/v1/bidders/bidder-1", "", 409)
	})

	t.Run("down then up", func(t *testing.T) {
		require.NoError(t, migrations.Apply(ctx, dsn, "down"))
		var table *string
		require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass('advertisers')::text").Scan(&table))
		require.Nil(t, table)
		require.NoError(t, migrations.Apply(ctx, dsn, "up"))
		require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass('advertisers')::text").Scan(&table))
		require.NotNil(t, table)
	})

	t.Run("runtime readiness and database outage", func(t *testing.T) {
		apiAddr, adminAddr, grpcAddr := freeAddress(t), freeAddress(t), freeAddress(t)
		runCtx, stop := context.WithCancel(ctx)
		defer stop()
		done := make(chan error, 1)
		common := config.Common{ServiceName: "campaign-integration", LogLevel: "error", AdminAddr: adminAddr, OTLPEndpoint: "localhost:1", ShutdownTimeout: time.Second}
		go func() {
			done <- Run(runCtx, common, Config{HTTPAddr: apiAddr, GRPCAddr: grpcAddr, DatabaseURL: dsn, APIKeys: []string{"test-key"}}, io.Discard)
		}()
		t.Cleanup(func() {
			stop()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Error("runtime did not stop")
			}
		})
		check := func(address, path string) int {
			requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(requestCtx, "GET", "http://"+address+path, nil)
			if err != nil {
				return 0
			}
			req.Header.Set("X-API-Key", "test-key")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return 0
			}
			defer func() { require.NoError(t, res.Body.Close()) }()
			_, err = io.Copy(io.Discard, res.Body)
			require.NoError(t, err)
			return res.StatusCode
		}
		require.Eventually(t, func() bool { return check(adminAddr, "/readyz") == 200 }, 5*time.Second, 20*time.Millisecond)
		require.Equal(t, 200, check(apiAddr, "/v1/advertisers"))
		provider := tracing.NewProvider("snapshot-test", "dev", 0)
		defer func() { require.NoError(t, provider.Shutdown(ctx)) }()
		connection, err := grpcx.NewClient(grpcAddr, insecure.NewCredentials(), provider)
		require.NoError(t, err)
		defer func() { require.NoError(t, connection.Close()) }()
		catalogClient := catalogv1.NewCatalogServiceClient(connection)
		grpcSnapshot, err := catalogClient.GetSnapshot(ctx, &catalogv1.GetSnapshotRequest{})
		require.NoError(t, err)
		require.Equal(t, int64(1), grpcSnapshot.Version)
		unchanged, err := catalogClient.GetSnapshot(ctx, &catalogv1.GetSnapshotRequest{KnownVersion: grpcSnapshot.Version})
		require.NoError(t, err)
		require.True(t, unchanged.Unchanged)
		require.Nil(t, unchanged.GeneratedAt)
		require.Empty(t, unchanged.Campaigns)
		requestCtx, cancelRequest := context.WithTimeout(ctx, 5*time.Second)
		defer cancelRequest()
		request, err := http.NewRequestWithContext(requestCtx, "GET", "http://"+apiAddr+"/v1/snapshot", nil)
		require.NoError(t, err)
		request.Header.Set("X-API-Key", "test-key")
		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		data, readErr := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, readErr)
		require.Equal(t, 200, response.StatusCode)
		var restSnapshot catalogv1.GetSnapshotResponse
		require.NoError(t, protojson.Unmarshal(data, &restSnapshot))
		require.True(t, proto.Equal(grpcSnapshot, &restSnapshot))
		healthClient := healthpb.NewHealthClient(connection)
		require.Eventually(t, func() bool {
			response, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: catalogv1.CatalogService_ServiceDesc.ServiceName})
			return err == nil && response.Status == healthpb.HealthCheckResponse_SERVING
		}, 3*time.Second, 20*time.Millisecond)
		require.NoError(t, container.Stop(ctx, nil))
		require.Eventually(t, func() bool { return check(adminAddr, "/readyz") == 503 }, 10*time.Second, 50*time.Millisecond)
		require.Equal(t, 200, check(adminAddr, "/healthz"))
		require.Equal(t, 503, check(apiAddr, "/v1/advertisers"))
		require.Equal(t, 503, check(apiAddr, "/v1/snapshot"))
		_, err = catalogClient.GetSnapshot(ctx, &catalogv1.GetSnapshotRequest{KnownVersion: 1})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Eventually(t, func() bool {
			response, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
			return err == nil && response.Status == healthpb.HealthCheckResponse_NOT_SERVING
		}, 3*time.Second, 20*time.Millisecond)
	})
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}
