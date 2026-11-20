package gateway

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
)

func TestTargeting_EvaluateCampaign(t *testing.T) {
	kolkataLoc, err := time.LoadLocation("Asia/Kolkata")
	require.NoError(t, err)

	startAt := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)

	baseCampaign := &catalogv1.Campaign{
		Id:           "camp-1",
		BidderId:     "bidder-1",
		StartAt:      timestamppb.New(startAt),
		EndAt:        timestamppb.New(endAt),
		MaxBidCpmMicros: 3_000_000,
		Targeting: &catalogv1.Targeting{
			Countries:    []string{"IN", "US"},
			DeviceTypes:  []string{"mobile"},
			PlacementIds: []string{"top-banner"},
			HoursOfDay:   []int32{9, 10, 11, 12, 13, 14, 15, 16, 17},
			Timezone:     "Asia/Kolkata",
		},
		Creatives: []*catalogv1.Creative{
			{Id: "cr-2", Width: 300, Height: 250, Markup: "<div>300x250</div>"},
			{Id: "cr-1", Width: 728, Height: 90, Markup: "<div>728x90</div>"},
		},
	}
	compiled := CompileCampaign(baseCampaign)

	// In Asia/Kolkata (UTC+5:30), UTC 04:00 is 09:30 (hour 9).
	validTime := time.Date(2026, 11, 15, 4, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		req        BidRequest
		now        time.Time
		isExh      func(string) bool
		wantMatch  bool
		wantCreat  string
	}{
		{
			name: "exact match on all targeting criteria",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       validTime,
			wantMatch: true,
			wantCreat: "cr-2",
		},
		{
			name: "country match second country US",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 728, H: 90},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "US"},
			},
			now:       validTime,
			wantMatch: true,
			wantCreat: "cr-1",
		},
		{
			name: "geo mismatch DE",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "DE"},
			},
			now:       validTime,
			wantMatch: false,
		},
		{
			name: "missing geo when countries required",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
			},
			now:       validTime,
			wantMatch: false,
		},
		{
			name: "device mismatch desktop",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "desktop"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       validTime,
			wantMatch: false,
		},
		{
			name: "placement mismatch",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "sidebar", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       validTime,
			wantMatch: false,
		},
		{
			name: "creative size mismatch 160x600",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 160, H: 600},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       validTime,
			wantMatch: false,
		},
		{
			name: "no size in request picks first creative sorted by ID",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner"},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       validTime,
			wantMatch: true,
			wantCreat: "cr-1", // cr-1 < cr-2
		},
		{
			name: "Asia/Kolkata half-hour offset: UTC 03:30 is 09:00 local (in range)",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       time.Date(2026, 11, 15, 3, 30, 0, 0, time.UTC),
			wantMatch: true,
			wantCreat: "cr-2",
		},
		{
			name: "Asia/Kolkata half-hour offset: UTC 03:29 is 08:59 local (out of range)",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       time.Date(2026, 11, 15, 3, 29, 59, 0, time.UTC),
			wantMatch: false,
		},
		{
			name: "time before start_at",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC),
			wantMatch: false,
		},
		{
			name: "time at or after end_at",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
			wantMatch: false,
		},
		{
			name: "exhausted campaign",
			req: BidRequest{
				Imp:    Impression{ID: "i1", PlacementID: "top-banner", W: 300, H: 250},
				Device: &Device{Type: "mobile"},
				Geo:    &Geo{Country: "IN"},
			},
			now:       validTime,
			isExh:     func(id string) bool { return id == "camp-1" },
			wantMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cand, ok := EvaluateCampaign(compiled, &tc.req, tc.now, tc.isExh)
			require.Equal(t, tc.wantMatch, ok)
			if tc.wantMatch {
				assert.Equal(t, tc.wantCreat, cand.Creative.GetId())
			}
		})
	}

	_ = kolkataLoc
}

func TestTargeting_BoundaryHours(t *testing.T) {
	cMidnight := CompileCampaign(&catalogv1.Campaign{
		Id:       "c-midnight",
		BidderId: "b1",
		Targeting: &catalogv1.Targeting{
			HoursOfDay: []int32{0},
			Timezone:   "UTC",
		},
		Creatives: []*catalogv1.Creative{{Id: "cr1", Width: 300, Height: 250}},
	})

	cNight := CompileCampaign(&catalogv1.Campaign{
		Id:       "c-night",
		BidderId: "b1",
		Targeting: &catalogv1.Targeting{
			HoursOfDay: []int32{23},
			Timezone:   "UTC",
		},
		Creatives: []*catalogv1.Creative{{Id: "cr1", Width: 300, Height: 250}},
	})

	req := BidRequest{Imp: Impression{ID: "i1", W: 300, H: 250}}

	// Hour 00:30 UTC
	t0 := time.Date(2026, 11, 15, 0, 30, 0, 0, time.UTC)
	_, okMid := EvaluateCampaign(cMidnight, &req, t0, nil)
	assert.True(t, okMid)
	_, okNight := EvaluateCampaign(cNight, &req, t0, nil)
	assert.False(t, okNight)

	// Hour 23:45 UTC
	t23 := time.Date(2026, 11, 15, 23, 45, 0, 0, time.UTC)
	_, okMid2 := EvaluateCampaign(cMidnight, &req, t23, nil)
	assert.False(t, okMid2)
	_, okNight2 := EvaluateCampaign(cNight, &req, t23, nil)
	assert.True(t, okNight2)
}

func BenchmarkTargeting(b *testing.B) {
	campaigns := make([]*CompiledCampaign, 100)
	for i := 0; i < 100; i++ {
		campaigns[i] = CompileCampaign(&catalogv1.Campaign{
			Id:       fmt.Sprintf("camp-%d", i),
			BidderId: fmt.Sprintf("bidder-%d", i%8),
			Targeting: &catalogv1.Targeting{
				Countries:    []string{"IN", "US"},
				DeviceTypes:  []string{"mobile", "desktop"},
				PlacementIds: []string{fmt.Sprintf("slot-%d", i%10)},
				HoursOfDay:   []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23},
			},
			Creatives: []*catalogv1.Creative{
				{Id: "cr1", Width: 300, Height: 250},
			},
		})
	}

	req := &BidRequest{
		Imp:    Impression{ID: "imp-1", PlacementID: "slot-3", W: 300, H: 250},
		Device: &Device{Type: "mobile"},
		Geo:    &Geo{Country: "IN"},
	}
	now := time.Now().UTC()

	
	b.ReportAllocs()
	for b.Loop() {
		_ = GroupCandidates(campaigns, req, now, nil)
	}
}
