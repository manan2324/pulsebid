package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	db "pulsebid/gen/campaigndb"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/platform/clock"
	"pulsebid/internal/platform/httpx"
)

type snapshotRepoFuncs struct {
	version func(context.Context) (int64, error)
	load    func(context.Context) (*catalogv1.GetSnapshotResponse, error)
}

func (f snapshotRepoFuncs) CatalogVersion(ctx context.Context) (int64, error) { return f.version(ctx) }
func (f snapshotRepoFuncs) CatalogSnapshot(ctx context.Context) (*catalogv1.GetSnapshotResponse, error) {
	return f.load(ctx)
}

func TestSnapshotCacheAndUnchanged(t *testing.T) {
	t.Parallel()
	var version atomic.Int64
	version.Store(1)
	var loads atomic.Int32
	c := clock.NewFake(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))
	repo := snapshotRepoFuncs{version: func(context.Context) (int64, error) { return version.Load(), nil }, load: func(context.Context) (*catalogv1.GetSnapshotResponse, error) {
		loads.Add(1)
		return &catalogv1.GetSnapshotResponse{Version: version.Load(), Campaigns: []*catalogv1.Campaign{{Id: "one", Name: "original", Creatives: []*catalogv1.Creative{{Id: "creative", Markup: "ad"}}}}}, nil
	}}
	snapshots := NewSnapshotService(repo, c)
	unchanged, err := snapshots.GetSnapshot(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, unchanged.Unchanged)
	require.Nil(t, unchanged.Campaigns)
	require.Nil(t, unchanged.Bidders)
	require.Nil(t, unchanged.GeneratedAt)
	require.Zero(t, loads.Load())
	first, err := snapshots.GetSnapshot(t.Context(), 0)
	require.NoError(t, err)
	require.Equal(t, c.Now(), first.GeneratedAt.AsTime())
	require.Equal(t, int32(1), loads.Load())
	first.Campaigns[0].Name = "mutated"
	first.Campaigns[0].Creatives[0].Markup = "mutated"
	c.Advance(time.Hour)
	second, err := snapshots.GetSnapshot(t.Context(), 0)
	require.NoError(t, err)
	require.Equal(t, "original", second.Campaigns[0].Name)
	require.Equal(t, "ad", second.Campaigns[0].Creatives[0].Markup)
	require.Equal(t, int32(1), loads.Load())
	require.NotEqual(t, c.Now(), second.GeneratedAt.AsTime())
	version.Store(2)
	third, err := snapshots.GetSnapshot(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), third.Version)
	require.Equal(t, c.Now(), third.GeneratedAt.AsTime())
	require.Equal(t, int32(2), loads.Load())
	unchanged, err = snapshots.GetSnapshot(t.Context(), 2)
	require.NoError(t, err)
	require.True(t, unchanged.Unchanged)
	_, err = snapshots.GetSnapshot(t.Context(), -1)
	require.ErrorIs(t, err, ErrValidation)
}

func TestConcurrentSnapshotRebuildAndCanceledWaiter(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	var loads atomic.Int32
	repo := snapshotRepoFuncs{version: func(context.Context) (int64, error) { return 1, nil }, load: func(ctx context.Context) (*catalogv1.GetSnapshotResponse, error) {
		loads.Add(1)
		close(entered)
		select {
		case <-release:
			return &catalogv1.GetSnapshotResponse{Version: 1}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	snapshots := NewSnapshotService(repo, clock.Real{})
	results := make(chan error, 21)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() { defer workers.Done(); _, err := snapshots.GetSnapshot(t.Context(), 0); results <- err }()
	<-entered
	waitCtx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	_, err := snapshots.GetSnapshot(waitCtx, 0)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	for range 20 {
		workers.Add(1)
		go func() { defer workers.Done(); _, err := snapshots.GetSnapshot(t.Context(), 0); results <- err }()
	}
	close(release)
	workers.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), loads.Load())
}

func TestSnapshotFailuresAndVersionAdvance(t *testing.T) {
	t.Parallel()
	for _, failAt := range []string{"first version", "second version", "load", "nil snapshot", "regression", "clock"} {
		t.Run(failAt, func(t *testing.T) {
			t.Parallel()
			calls := 0
			repo := snapshotRepoFuncs{version: func(context.Context) (int64, error) {
				calls++
				if failAt == "first version" || failAt == "second version" && calls == 2 {
					return 0, ErrUnavailable
				}
				return 5, nil
			}, load: func(context.Context) (*catalogv1.GetSnapshotResponse, error) {
				if failAt == "load" {
					return nil, ErrUnavailable
				}
				if failAt == "nil snapshot" {
					return nil, nil
				}
				if failAt == "regression" {
					return &catalogv1.GetSnapshotResponse{Version: 4}, nil
				}
				return &catalogv1.GetSnapshotResponse{Version: 5}, nil
			}}
			var c clock.Clock = clock.Real{}
			if failAt == "clock" {
				c = clock.NewFake(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC))
			}
			s := NewSnapshotService(repo, c)
			_, err := s.GetSnapshot(t.Context(), 0)
			require.Error(t, err)
			require.Nil(t, s.cached.Load())
		})
	}
	// The catalog can advance between its cheap version query and the full read.
	repo := snapshotRepoFuncs{version: func(context.Context) (int64, error) { return 1, nil }, load: func(context.Context) (*catalogv1.GetSnapshotResponse, error) {
		return &catalogv1.GetSnapshotResponse{Version: 2}, nil
	}}
	s := NewSnapshotService(repo, clock.Real{})
	response, err := s.GetSnapshot(t.Context(), 2)
	require.NoError(t, err)
	require.True(t, response.Unchanged)
	require.Nil(t, response.GeneratedAt)
	// A canceled queued caller whose known version becomes current takes the
	// second-version unchanged path without loading another full snapshot.
	reads := 0
	repo.version = func(context.Context) (int64, error) {
		reads++
		if reads == 1 {
			return 1, nil
		}
		return 2, nil
	}
	s = NewSnapshotService(repo, clock.Real{})
	response, err = s.GetSnapshot(t.Context(), 2)
	require.NoError(t, err)
	require.True(t, response.Unchanged)
}

func TestSnapshotDecoding(t *testing.T) {
	t.Parallel()
	row := db.CatalogSnapshotRow{Version: 12, Bidders: []byte(`[{"id":"bidder-1","endpoint_url":"http://localhost:9101/bid","timeout_ms":0}]`), Campaigns: []byte(`[{"id":"one","advertiser_id":"advertiser","bidder_id":"bidder-1","daily_budget_micros":9223372036854775807,"max_bid_cpm_micros":3000001,"pacing_mode":"asap","start_at":"2026-11-18T10:00:00+05:30","end_at":null,"targeting":{"countries":["IN"],"hours_of_day":[3],"timezone":"Asia/Kolkata"},"creatives":[{"id":"creative","width":300,"height":250,"markup":"ad"}]}]`)}
	snapshot, err := decodeSnapshot(row)
	require.NoError(t, err)
	require.Equal(t, int64(9223372036854775807), snapshot.Campaigns[0].DailyBudgetMicros)
	require.Equal(t, int64(3000001), snapshot.Campaigns[0].MaxBidCpmMicros)
	require.Equal(t, catalogv1.PacingMode_PACING_MODE_ASAP, snapshot.Campaigns[0].PacingMode)
	require.Equal(t, "2026-11-18T04:30:00Z", snapshot.Campaigns[0].StartAt.AsTime().Format(time.RFC3339))
	require.Nil(t, snapshot.Campaigns[0].EndAt)
	require.Equal(t, []int32{3}, snapshot.Campaigns[0].Targeting.HoursOfDay)
	for _, tc := range []db.CatalogSnapshotRow{{Bidders: []byte(`{`), Campaigns: []byte(`[]`)}, {Bidders: []byte(`[]`), Campaigns: []byte(`{`)}, {Bidders: []byte(`[]`), Campaigns: []byte(`[{"daily_budget_micros":1.2}]`)}, {Bidders: []byte(`[]`), Campaigns: []byte(`[{"pacing_mode":"unknown"}]`)}} {
		_, err := decodeSnapshot(tc)
		require.Error(t, err)
	}
	empty, err := decodeSnapshot(db.CatalogSnapshotRow{Version: 1, Bidders: []byte(`[]`), Campaigns: []byte(`[]`)})
	require.NoError(t, err)
	require.Empty(t, empty.Campaigns)
	_, err = snapshotTimestamp(ptr(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)))
	require.Error(t, err)
}

func TestSnapshotHTTPAndETags(t *testing.T) {
	t.Parallel()
	var unavailable atomic.Bool
	repo := snapshotRepoFuncs{version: func(context.Context) (int64, error) {
		if unavailable.Load() {
			return 0, ErrUnavailable
		}
		return 7, nil
	}, load: func(context.Context) (*catalogv1.GetSnapshotResponse, error) {
		return &catalogv1.GetSnapshotResponse{Version: 7}, nil
	}}
	s := NewSnapshotService(repo, clock.Real{})
	handler := httpx.Chain(s.Handler(), httpx.RequestID, httpx.APIKey([]string{"test-key"}))
	call := func(tag, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), "GET", "/v1/snapshot", nil)
		r.Header.Set("If-None-Match", tag)
		r.Header.Set("X-API-Key", key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	require.Equal(t, 401, call("", "").Code)
	full := call("", "test-key")
	require.Equal(t, 200, full.Code)
	require.Equal(t, `"7"`, full.Header().Get("ETag"))
	require.NotEmpty(t, full.Header().Get("X-Request-Id"))
	var payload struct {
		Version     string `json:"version"`
		Unchanged   bool   `json:"unchanged"`
		Bidders     []any  `json:"bidders"`
		Campaigns   []any  `json:"campaigns"`
		GeneratedAt string `json:"generated_at"`
	}
	require.NoError(t, json.Unmarshal(full.Body.Bytes(), &payload))
	require.Equal(t, "7", payload.Version)
	require.False(t, payload.Unchanged)
	require.NotNil(t, payload.Campaigns)
	require.NotNil(t, payload.Bidders)
	require.NotEmpty(t, payload.GeneratedAt)
	for _, tag := range []string{`"7"`, `W/"7"`, `"other", "7"`, `*`} {
		w := call(tag, "test-key")
		require.Equal(t, 304, w.Code)
		require.Empty(t, w.Body.String())
		require.Equal(t, `"7"`, w.Header().Get("ETag"))
	}
	require.Equal(t, 200, call(`"6"`, "test-key").Code)
	unavailable.Store(true)
	failed := call(`"7"`, "test-key")
	require.Equal(t, 503, failed.Code)
	require.Contains(t, failed.Body.String(), `"unavailable"`)
}

func TestETagParsing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		header string
		match  bool
	}{{`"7"`, true}, {` W/"7" `, true}, {`"one,two", W/"7"`, true}, {`"other"`, false}, {`7`, false}, {`"7`, false}, {`"7",`, false}, {`"7" extra`, false}, {`"white space"`, false}, {`*`, true}, {``, false}, {`"other",`, false}, {`"other", bare`, false}} {
		t.Run(tc.header, func(t *testing.T) { t.Parallel(); require.Equal(t, tc.match, matchesETag(tc.header, `"7"`)) })
	}
}

func TestCatalogGRPCAdapter(t *testing.T) {
	t.Parallel()
	repo := snapshotRepoFuncs{version: func(context.Context) (int64, error) { return 1, nil }, load: func(context.Context) (*catalogv1.GetSnapshotResponse, error) {
		return &catalogv1.GetSnapshotResponse{Version: 1}, nil
	}}
	s := NewCatalogServer(NewSnapshotService(repo, clock.Real{}))
	_, err := s.GetSnapshot(t.Context(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = s.GetSnapshot(t.Context(), &catalogv1.GetSnapshotRequest{KnownVersion: -1})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	response, err := s.GetSnapshot(t.Context(), &catalogv1.GetSnapshotRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(1), response.Version)
	unchanged, err := s.GetSnapshot(t.Context(), &catalogv1.GetSnapshotRequest{KnownVersion: 1})
	require.NoError(t, err)
	require.True(t, proto.Equal(unchanged, unchangedSnapshot(1)))
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded}, {ErrUnavailable, codes.Unavailable}, {ErrValidation, codes.InvalidArgument}, {errors.New("private details"), codes.Internal}} {
		mapped := snapshotGRPCError(fmt.Errorf("wrapped: %w", tc.err))
		require.Equal(t, tc.code, status.Code(mapped))
		require.NotContains(t, mapped.Error(), "private details")
	}
}
