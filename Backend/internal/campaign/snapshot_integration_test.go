//go:build integration

package campaign

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/httpx"
	"pulsebid/migrations"
)

func TestCatalogSnapshotPostgres(t *testing.T) {
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
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	store := NewStore(pool)
	service := NewService(store)
	snapshots := NewSnapshotService(store, clock.Real{})
	mux := service.Handler([]string{"test-key"})
	mux.Handle("GET /v1/snapshot", httpx.APIKey([]string{"test-key"})(snapshots.Handler()))
	handler := httpx.RequestID(mux)
	get := func() *catalogv1.GetSnapshotResponse {
		snapshot, err := snapshots.GetSnapshot(ctx, 0)
		require.NoError(t, err)
		return snapshot
	}
	contains := func(id string) bool {
		for _, c := range get().Campaigns {
			if c.Id == id {
				return true
			}
		}
		return false
	}
	empty := get()
	require.Equal(t, int64(1), empty.Version)
	require.Empty(t, empty.Bidders)
	require.Empty(t, empty.Campaigns)
	baseAdvertiser := bodyAs[Advertiser](t, send(t, handler, "POST", "/v1/advertisers", `{"name":"Snapshot advertiser"}`, 201))
	send(t, handler, "POST", "/v1/bidders", `{"id":"bidder-1","name":"Bidder","endpoint_url":"http://localhost:9101/bid"}`, 201)
	send(t, handler, "POST", "/v1/bidders", `{"id":"unused-bidder","name":"Unused","endpoint_url":"http://localhost:9199/bid"}`, 201)
	createCampaign := func(name, advertiser, bidder string) Campaign {
		return bodyAs[Campaign](t, send(t, handler, "POST", "/v1/campaigns", fmt.Sprintf(`{"advertiser_id":%q,"bidder_id":%q,"name":%q,"daily_budget":50.123456,"max_bid_cpm":3.000001}`, advertiser, bidder, name), 201))
	}
	createCreative := func(id, creativeStatus string) Creative {
		return bodyAs[Creative](t, send(t, handler, "POST", "/v1/campaigns/"+id+"/creatives", fmt.Sprintf(`{"name":"Banner","format":"banner","width":300,"height":250,"markup":"ad","landing_url":"https://example.test","status":%q}`, creativeStatus), 201))
	}
	activate := func(id string) { send(t, handler, "POST", "/v1/campaigns/"+id+"/activate", "", 200) }
	valid := createCampaign("Valid", baseAdvertiser.ID, "bidder-1")
	firstCreative := createCreative(valid.ID, "active")
	secondCreative := createCreative(valid.ID, "active")
	createCreative(valid.ID, "paused")
	activate(valid.ID)
	send(t, handler, "PUT", "/v1/campaigns/"+valid.ID+"/targeting", `{"countries":["IN"],"device_types":["mobile"],"placement_ids":["slot"],"hours_of_day":[3],"timezone":"Asia/Kolkata"}`, 200)

	t.Run("inclusion and exact mapping", func(t *testing.T) {
		snapshot := get()
		require.Len(t, snapshot.Campaigns, 1)
		require.Len(t, snapshot.Bidders, 1)
		require.Equal(t, int32(0), snapshot.Bidders[0].TimeoutMs)
		c := snapshot.Campaigns[0]
		require.Equal(t, valid.ID, c.Id)
		require.Equal(t, baseAdvertiser.ID, c.AdvertiserId)
		require.Equal(t, int64(50123456), c.DailyBudgetMicros)
		require.Equal(t, int64(3000001), c.MaxBidCpmMicros)
		require.Equal(t, catalogv1.PacingMode_PACING_MODE_EVEN, c.PacingMode)
		require.Len(t, c.Creatives, 2)
		require.Equal(t, firstCreative.ID, c.Creatives[0].Id)
		require.Equal(t, secondCreative.ID, c.Creatives[1].Id)
		require.Equal(t, int32(300), c.Creatives[0].Width)
		require.Equal(t, "https://example.test", c.Creatives[0].LandingUrl)
		require.Equal(t, []string{"IN"}, c.Targeting.Countries)
		require.Equal(t, []string{"mobile"}, c.Targeting.DeviceTypes)
		require.Equal(t, []string{"slot"}, c.Targeting.PlacementIds)
		require.Equal(t, []int32{3}, c.Targeting.HoursOfDay)
		require.Equal(t, "Asia/Kolkata", c.Targeting.Timezone)
		unchanged, err := snapshots.GetSnapshot(ctx, snapshot.Version)
		require.NoError(t, err)
		require.True(t, unchanged.Unchanged)
		require.Nil(t, unchanged.GeneratedAt)
		require.Nil(t, unchanged.Campaigns)
	})

	t.Run("exclude inactive parents and missing active creatives", func(t *testing.T) {
		draft := createCampaign("Draft", baseAdvertiser.ID, "bidder-1")
		createCreative(draft.ID, "active")
		require.False(t, contains(draft.ID))
		paused := createCampaign("Paused", baseAdvertiser.ID, "bidder-1")
		createCreative(paused.ID, "active")
		activate(paused.ID)
		send(t, handler, "POST", "/v1/campaigns/"+paused.ID+"/pause", "", 200)
		require.False(t, contains(paused.ID))
		archived := createCampaign("Archived", baseAdvertiser.ID, "bidder-1")
		createCreative(archived.ID, "active")
		activate(archived.ID)
		send(t, handler, "DELETE", "/v1/campaigns/"+archived.ID, "", 204)
		require.False(t, contains(archived.ID))
		advertiser := bodyAs[Advertiser](t, send(t, handler, "POST", "/v1/advertisers", `{"name":"Archived parent"}`, 201))
		orphan := createCampaign("Archived advertiser", advertiser.ID, "bidder-1")
		createCreative(orphan.ID, "active")
		activate(orphan.ID)
		send(t, handler, "DELETE", "/v1/advertisers/"+advertiser.ID, "", 204)
		require.False(t, contains(orphan.ID))
		send(t, handler, "POST", "/v1/bidders", `{"id":"disabled-bidder","name":"Disabled","endpoint_url":"http://localhost:9102/bid"}`, 201)
		disabled := createCampaign("Disabled bidder", baseAdvertiser.ID, "disabled-bidder")
		createCreative(disabled.ID, "active")
		activate(disabled.ID)
		send(t, handler, "PATCH", "/v1/bidders/disabled-bidder", `{"enabled":false}`, 200)
		require.False(t, contains(disabled.ID))
		for _, state := range []string{"none", "paused"} {
			missing := createCampaign("Missing creative "+state, baseAdvertiser.ID, "bidder-1")
			if state == "paused" {
				createCreative(missing.ID, "paused")
			}
			_, err := pool.Exec(ctx, "UPDATE campaigns SET status='active' WHERE id=$1", missing.ID)
			require.NoError(t, err)
			require.False(t, contains(missing.ID))
		}
		require.Len(t, get().Campaigns, 1)
	})

	t.Run("date windows stay in snapshot and missing targeting defaults", func(t *testing.T) {
		for _, year := range []int{2000, 2099} {
			c := createCampaign(fmt.Sprintf("Window %d", year), baseAdvertiser.ID, "bidder-1")
			createCreative(c.ID, "active")
			activate(c.ID)
			send(t, handler, "PATCH", "/v1/campaigns/"+c.ID, fmt.Sprintf(`{"start_at":"%d-01-01T00:00:00Z","end_at":"%d-01-02T00:00:00Z","pacing_mode":"asap"}`, year, year), 200)
			require.True(t, contains(c.ID))
			for _, item := range get().Campaigns {
				if item.Id == c.ID {
					require.Equal(t, "UTC", item.Targeting.Timezone)
					require.Empty(t, item.Targeting.Countries)
					require.Equal(t, catalogv1.PacingMode_PACING_MODE_ASAP, item.PacingMode)
					require.Equal(t, year, item.StartAt.AsTime().Year())
					require.NotNil(t, item.EndAt)
				}
			}
		}
	})

	t.Run("every successful mutating route advances version", func(t *testing.T) {
		seen := map[string]bool{}
		mutate := func(label, method, path, body string, status int) *httptest.ResponseRecorder {
			before, err := store.CatalogVersion(ctx)
			require.NoError(t, err)
			response := send(t, handler, method, path, body, status)
			after, err := store.CatalogVersion(ctx)
			require.NoError(t, err)
			require.Greater(t, after, before, label)
			updated, err := snapshots.GetSnapshot(ctx, before)
			require.NoError(t, err)
			require.False(t, updated.Unchanged)
			require.Equal(t, after, updated.Version)
			seen[label] = true
			return response
		}
		a := bodyAs[Advertiser](t, mutate("create advertiser", "POST", "/v1/advertisers", `{"name":"Version advertiser"}`, 201))
		mutate("patch advertiser", "PATCH", "/v1/advertisers/"+a.ID, `{"name":"Version advertiser renamed"}`, 200)
		mutate("create bidder", "POST", "/v1/bidders", `{"id":"version-bidder","name":"Version bidder","endpoint_url":"http://localhost:9103/bid"}`, 201)
		mutate("patch bidder", "PATCH", "/v1/bidders/version-bidder", `{"timeout_ms":100}`, 200)
		c := bodyAs[Campaign](t, mutate("create campaign", "POST", "/v1/campaigns", fmt.Sprintf(`{"advertiser_id":%q,"bidder_id":"version-bidder","name":"Version campaign","daily_budget":1,"max_bid_cpm":1}`, a.ID), 201))
		mutate("patch campaign", "PATCH", "/v1/campaigns/"+c.ID, `{"name":"Version campaign renamed"}`, 200)
		mutate("put targeting", "PUT", "/v1/campaigns/"+c.ID+"/targeting", `{"countries":["US"]}`, 200)
		mutate("put targeting", "PUT", "/v1/campaigns/"+c.ID+"/targeting", `{"countries":["IN"]}`, 200)
		cr := bodyAs[Creative](t, mutate("create creative", "POST", "/v1/campaigns/"+c.ID+"/creatives", `{"name":"Version creative","format":"native","markup":"ad","landing_url":"https://example.test"}`, 201))
		mutate("patch creative", "PATCH", "/v1/creatives/"+cr.ID, `{"markup":"updated"}`, 200)
		mutate("activate", "POST", "/v1/campaigns/"+c.ID+"/activate", "", 200)
		mutate("pause", "POST", "/v1/campaigns/"+c.ID+"/pause", "", 200)
		mutate("delete creative", "DELETE", "/v1/creatives/"+cr.ID, "", 204)
		mutate("archive campaign", "DELETE", "/v1/campaigns/"+c.ID, "", 204)
		mutate("archive advertiser", "DELETE", "/v1/advertisers/"+a.ID, "", 204)
		mutate("delete bidder", "DELETE", "/v1/bidders/unused-bidder", "", 204)
		require.Len(t, seen, 15)
		before, err := store.CatalogVersion(ctx)
		require.NoError(t, err)
		send(t, handler, "POST", "/v1/campaigns/"+c.ID+"/activate", "", 409)
		after, err := store.CatalogVersion(ctx)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})

	t.Run("REST changes invalidate ETags", func(t *testing.T) {
		full := send(t, handler, "GET", "/v1/snapshot", "", 200)
		etag := full.Header().Get("ETag")
		require.NotEmpty(t, etag)
		request := func() *httptest.ResponseRecorder {
			r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/snapshot", nil)
			r.Header.Set("X-API-Key", "test-key")
			r.Header.Set("If-None-Match", etag)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			return w
		}
		unchanged := request()
		require.Equal(t, 304, unchanged.Code)
		require.Empty(t, unchanged.Body.String())
		send(t, handler, "POST", "/v1/campaigns/"+valid.ID+"/pause", "", 200)
		changed := request()
		require.Equal(t, 200, changed.Code)
		require.NotEqual(t, etag, changed.Header().Get("ETag"))
		require.False(t, contains(valid.ID))
		activate(valid.ID)
	})

	t.Run("one MVCC view under concurrent commits", func(t *testing.T) {
		writeName := func() error {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = tx.Rollback(cleanup)
			}()
			var version int64
			if err = tx.QueryRow(ctx, "SELECT version FROM catalog_version FOR UPDATE").Scan(&version); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "UPDATE campaigns SET name=$1 WHERE id=$2", strconv.FormatInt(version+1, 10), valid.ID); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
		require.NoError(t, writeName())
		finished := make(chan struct{})
		var writerError error
		go func() {
			defer close(finished)
			for range 25 {
				if err := writeName(); err != nil {
					writerError = err
					return
				}
			}
		}()
		defer func() { <-finished }()
		for range 30 {
			snapshot, err := store.CatalogSnapshot(ctx)
			require.NoError(t, err)
			for _, c := range snapshot.Campaigns {
				if c.Id == valid.ID {
					require.Equal(t, strconv.FormatInt(snapshot.Version, 10), c.Name)
				}
			}
		}
		<-finished
		require.NoError(t, writerError)
	})

	// Ensure the cache never hides an unavailable catalog, including conditional reads.
	pool.Close()
	_, err = snapshots.GetSnapshot(ctx, 0)
	require.ErrorIs(t, err, ErrUnavailable)
	failed := send(t, handler, "GET", "/v1/snapshot", "", 503)
	require.True(t, strings.Contains(failed.Body.String(), "unavailable"))
}
