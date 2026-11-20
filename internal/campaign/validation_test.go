package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/httpx"
)

func ptr[T any](v T) *T { return &v }
func testCampaign() Campaign {
	return Campaign{AdvertiserID: "0193f0a2-0000-7000-8000-000000000001", BidderID: "bidder-1", Name: "Campaign", Status: "draft", DailyBudget: 50_000_000, MaxBidCPM: 3_000_000, PacingMode: "even"}
}
func patch(raw string) map[string]json.RawMessage {
	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		panic(err)
	}
	return result
}

func TestValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		check func() error
	}{
		{"advertiser name", func() error { return validateAdvertiser(Advertiser{Name: "", Status: "active"}) }},
		{"advertiser long name", func() error { return validateAdvertiser(Advertiser{Name: strings.Repeat("x", 201), Status: "active"}) }},
		{"advertiser status", func() error { return validateAdvertiser(Advertiser{Name: "ok", Status: "paused"}) }},
		{"email", func() error {
			return validateAdvertiser(Advertiser{Name: "ok", Status: "active", ContactEmail: ptr("invalid")})
		}},
		{"bidder id", func() error { return validateBidder(Bidder{ID: "BAD"}) }},
		{"bidder name", func() error { return validateBidder(Bidder{ID: "valid-id"}) }},
		{"bidder URL", func() error { return validateBidder(Bidder{ID: "valid-id", Name: "ok", EndpointUrl: "file:///tmp"}) }},
		{"bidder timeout", func() error {
			return validateBidder(Bidder{ID: "valid-id", Name: "ok", EndpointUrl: "http://localhost/bid", TimeoutMs: ptr(int32(9))})
		}},
		{"creative name", func() error { return validateCreative(Creative{}) }},
		{"creative format", func() error { return validateCreative(Creative{Name: "ok", Format: "unknown"}) }},
		{"creative width", func() error { return validateCreative(Creative{Name: "ok", Format: "banner", Width: ptr(int32(0))}) }},
		{"creative height", func() error { return validateCreative(Creative{Name: "ok", Format: "banner", Height: ptr(int32(-1))}) }},
		{"creative markup", func() error {
			return validateCreative(Creative{Name: "ok", Format: "banner", Markup: strings.Repeat("x", 65537)})
		}},
		{"creative URL", func() error {
			return validateCreative(Creative{Name: "ok", Format: "banner", Markup: "ad", LandingUrl: "javascript:alert(1)"})
		}},
		{"creative status", func() error {
			return validateCreative(Creative{Name: "ok", Format: "banner", Markup: "ad", LandingUrl: "https://example.test", Status: "archived"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) { t.Parallel(); require.ErrorIs(t, tc.check(), ErrValidation) })
	}
	require.NoError(t, validateAdvertiser(Advertiser{Name: "名字", Status: "active", ContactEmail: ptr("ads@example.test")}))
	require.NoError(t, validateBidder(Bidder{ID: "bidder-1", Name: "ok", EndpointUrl: "http://localhost:9101/bid", TimeoutMs: ptr(int32(1000))}))
	require.NoError(t, validateCreative(Creative{Name: "ok", Format: "banner", Markup: "ad", LandingUrl: "https://example.test", Status: "active"}))
	for _, bad := range []string{"a", "a_", "-ab", "Ab", strings.Repeat("a", 64)} {
		require.False(t, validSlug(bad))
	}
	for _, bad := range []string{"/relative", "ftp://example.test", "https://user:pass@example.test", "https:///path"} {
		require.False(t, validURL(bad))
	}
}

func TestCampaignValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*Campaign)
	}{
		{"advertiser", func(c *Campaign) { c.AdvertiserID = "bad" }}, {"bidder", func(c *Campaign) { c.BidderID = "!" }},
		{"name", func(c *Campaign) { c.Name = " " }}, {"status", func(c *Campaign) { c.Status = "invalid" }},
		{"zero budget", func(c *Campaign) { c.DailyBudget = 0 }}, {"large budget", func(c *Campaign) { c.DailyBudget = 1_000_000_000_001 }},
		{"zero bid", func(c *Campaign) { c.MaxBidCPM = 0 }}, {"large bid", func(c *Campaign) { c.MaxBidCPM = 1_000_000_001 }},
		{"pacing", func(c *Campaign) { c.PacingMode = "invalid" }},
		{"window", func(c *Campaign) { c.StartAt = ptr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); c.EndAt = c.StartAt }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := testCampaign()
			tc.change(&c)
			require.ErrorIs(t, validateCampaign(&c), ErrValidation)
		})
	}
	c := testCampaign()
	c.StartAt = ptr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("offset", 19800)))
	c.EndAt = ptr(c.StartAt.Add(time.Hour))
	require.NoError(t, validateCampaign(&c))
	require.Equal(t, time.UTC, c.StartAt.Location())
	require.Equal(t, time.UTC, c.EndAt.Location())
}

func TestTargeting(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"countries":["USA"]}`, `{"countries":["ZZ"]}`, `{"device_types":["watch"]}`, `{"placement_ids":[""]}`, `{"hours_of_day":[24]}`, `{"hours_of_day":[-1]}`, `{"timezone":"Not/AZone"}`, `{"timezone":"Local"}`, `{"timezone":""}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			rule := Targeting{Timezone: "UTC"}
			require.NoError(t, json.Unmarshal([]byte(raw), &rule))
			require.ErrorIs(t, normalizeTargeting(&rule), ErrValidation)
		})
	}
	rule := Targeting{Timezone: "Asia/Kolkata", Countries: []string{"in", "US", "IN"}, HoursOfDay: []int16{3, 1, 3}}
	require.NoError(t, normalizeTargeting(&rule))
	require.Equal(t, []string{"IN", "US"}, rule.Countries)
	require.Equal(t, []int16{1, 3}, rule.HoursOfDay)
	require.NotNil(t, rule.DeviceTypes)
	require.NotNil(t, rule.PlacementIds)
}

func TestMergeAndPagination(t *testing.T) {
	t.Parallel()
	old := Advertiser{Name: "old", Status: "active", ContactEmail: ptr("a@example.test")}
	next, err := merge(old, patch(`{"contact_email":null}`), "name contact_email", "contact_email")
	require.NoError(t, err)
	require.Nil(t, next.ContactEmail)
	require.Equal(t, "old", next.Name)
	for _, raw := range []string{`{"name":null}`, `{"extra":true}`, `{"name":123}`} {
		_, err := merge(old, patch(raw), "name contact_email", "contact_email")
		require.ErrorIs(t, err, ErrValidation)
	}
	for _, query := range []url.Values{{"limit": {"0"}}, {"limit": {"201"}}, {"limit": {"x"}}, {"limit": {"1", "2"}}, {"cursor": {""}}, {"cursor": {"bad!"}}, {"cursor": {"a", "b"}}, {"cursor": {strings.Repeat("x", 2049)}}} {
		_, err := parsePage(query, "advertisers")
		require.ErrorIs(t, err, ErrValidation)
	}
	p, err := parsePage(url.Values{}, "advertisers")
	require.NoError(t, err)
	require.Equal(t, int32(50), p.Limit)
	p.Limit = 1
	rows := []Advertiser{{ID: "0193f0a2-0000-7000-8000-000000000001", CreatedAt: time.Now().UTC()}, {ID: "0193f0a2-0000-7000-8000-000000000002"}}
	page := paginate(rows, p, func(a Advertiser) (time.Time, string) { return a.CreatedAt, a.ID })
	require.Len(t, page.Data, 1)
	require.NotNil(t, page.NextCursor)
	parsed, err := parsePage(url.Values{"cursor": {*page.NextCursor}}, "advertisers")
	require.NoError(t, err)
	require.True(t, parsed.HasCursor)
	require.Equal(t, rows[0].ID, parsed.ID)
	_, err = parsePage(url.Values{"cursor": {*page.NextCursor}}, "campaigns")
	require.Error(t, err)
	empty := paginate([]Advertiser(nil), p, func(a Advertiser) (time.Time, string) { return a.CreatedAt, a.ID })
	require.Empty(t, empty.Data)
	require.NotNil(t, empty.Data)
}

func TestErrorsAndConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		err  error
		want error
		code string
	}{
		{pgx.ErrNoRows, ErrNotFound, "not_found"}, {&pgconn.PgError{Code: "23505"}, ErrConflict, "conflict"}, {&pgconn.PgError{Code: "23503"}, ErrConflict, "conflict"},
		{&pgconn.PgError{Code: "23514"}, ErrValidation, "invalid_request"}, {&pgconn.PgError{Code: "08006"}, ErrUnavailable, "unavailable"}, {&pgconn.PgError{Code: "57P01"}, ErrUnavailable, "unavailable"},
		{context.DeadlineExceeded, ErrUnavailable, "unavailable"}, {puddle.ErrClosedPool, ErrUnavailable, "unavailable"},
		{io.EOF, ErrUnavailable, "unavailable"}, {io.ErrUnexpectedEOF, ErrUnavailable, "unavailable"},
	} {
		mapped := databaseError(tc.err)
		require.ErrorIs(t, mapped, tc.want)
		var httpErr *httpx.Error
		require.ErrorAs(t, publicError(mapped), &httpErr)
		require.Equal(t, tc.code, httpErr.Code)
	}
	require.Nil(t, databaseError(nil))
	private := errors.New("secret SQL")
	require.Equal(t, private, databaseError(private))
	require.Equal(t, "internal", publicError(private).(*httpx.Error).Code)
	public := &httpx.Error{Code: "payload_too_large"}
	require.Same(t, public, publicError(public))
	err := invalid("field", "required")
	require.Contains(t, err.Error(), "field")
	require.Equal(t, "field", publicError(fmt.Errorf("wrapped: %w", err)).(*httpx.Error).Details[0].Field)
	for _, cfg := range []Config{{HTTPAddr: "bad"}, {HTTPAddr: ":70000"}, {HTTPAddr: ":8081", GRPCAddr: "bad"}, {HTTPAddr: ":8081", GRPCAddr: ":0"}, {HTTPAddr: ":8081", GRPCAddr: ":8082"}, {HTTPAddr: ":8081", GRPCAddr: ":8082", DatabaseURL: "dsn", APIKeys: []string{" "}}} {
		require.Error(t, cfg.Validate())
	}
	require.NoError(t, (Config{HTTPAddr: ":8081", GRPCAddr: ":8082", DatabaseURL: "dsn", APIKeys: []string{"key"}}).Validate())
}
