package gateway

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAuction_Section12Examples(t *testing.T) {
	tests := []struct {
		name        string
		bids        []Bid
		floorMicros int64
		wantOk      bool
		wantWinner  string
		wantSecond  int64
		wantClear   int64
	}{
		{
			name: "two bids 2.00 and 1.50 with floor 1.00",
			bids: []Bid{
				{CampaignID: "c1", BidderID: "b1", CPMMicros: 2_000_000},
				{CampaignID: "c2", BidderID: "b2", CPMMicros: 1_500_000},
			},
			floorMicros: 1_000_000,
			wantOk:      true,
			wantWinner:  "c1",
			wantSecond:  1_500_000,
			wantClear:   1_510_000, // 1.50 + 0.01 = 1.51
		},
		{
			name: "single bid 3.00 with floor 1.00",
			bids: []Bid{
				{CampaignID: "c1", BidderID: "b1", CPMMicros: 3_000_000},
			},
			floorMicros: 1_000_000,
			wantOk:      true,
			wantWinner:  "c1",
			wantSecond:  0,
			wantClear:   1_010_000, // max(0, 1.00) + 0.01 = 1.01
		},
		{
			name: "tie 2.00 and 2.00 campaign A < B",
			bids: []Bid{
				{CampaignID: "camp-b", BidderID: "b2", CPMMicros: 2_000_000},
				{CampaignID: "camp-a", BidderID: "b1", CPMMicros: 2_000_000},
			},
			floorMicros: 1_000_000,
			wantOk:      true,
			wantWinner:  "camp-a",
			wantSecond:  2_000_000,
			wantClear:   2_000_000, // min(2.00, 2.00 + 0.01) = 2.00
		},
		{
			name: "all bids below floor 1.00",
			bids: []Bid{
				{CampaignID: "c1", BidderID: "b1", CPMMicros: 900_000},
				{CampaignID: "c2", BidderID: "b2", CPMMicros: 800_000},
			},
			floorMicros: 1_000_000,
			wantOk:      false,
		},
		{
			name: "bid equal to floor 1.00",
			bids: []Bid{
				{CampaignID: "c1", BidderID: "b1", CPMMicros: 1_000_000},
			},
			floorMicros: 1_000_000,
			wantOk:      true,
			wantWinner:  "c1",
			wantSecond:  0,
			wantClear:   1_000_000, // min(1.00, 1.00 + 0.01) = 1.00
		},
		{
			name: "tie on CPM and CampaignID breaks on BidderID",
			bids: []Bid{
				{CampaignID: "c1", BidderID: "bidder-z", CPMMicros: 2_500_000},
				{CampaignID: "c1", BidderID: "bidder-a", CPMMicros: 2_500_000},
			},
			floorMicros: 500_000,
			wantOk:      true,
			wantWinner:  "c1",
			wantSecond:  2_500_000,
			wantClear:   2_500_000,
		},
		{
			name:        "empty bids",
			bids:        nil,
			floorMicros: 1_000_000,
			wantOk:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, ok := RunAuction(tc.bids, tc.floorMicros)
			require.Equal(t, tc.wantOk, ok)
			if tc.wantOk {
				assert.Equal(t, tc.wantWinner, res.Winner.CampaignID)
				assert.Equal(t, tc.wantSecond, res.SecondCPM)
				assert.Equal(t, tc.wantClear, res.ClearingCPM)
			}
		})
	}
}

func TestRunAuction_RandomPropertyTests(t *testing.T) {
	// #nosec G404 -- Reproducible auction property tests do not require cryptographic randomness.
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 100; i++ {
		numBids := rng.Intn(10) + 1
		floorMicros := int64(rng.Intn(5_000_000)) // $0.00 to $5.00 floor

		bids := make([]Bid, numBids)
		for j := 0; j < numBids; j++ {
			bids[j] = Bid{
				CampaignID: fmt.Sprintf("camp-%03d", rng.Intn(50)),
				BidderID:   fmt.Sprintf("bidder-%d", rng.Intn(8)+1),
				CPMMicros:  int64(rng.Intn(10_000_000) + 10_000), // $0.01 to $10.00
			}
		}

		res, ok := RunAuction(bids, floorMicros)

		// Check if any bid qualified
		var maxValid int64 = -1
		for _, b := range bids {
			if b.CPMMicros >= floorMicros && b.CPMMicros > maxValid {
				maxValid = b.CPMMicros
			}
		}

		if maxValid < 0 {
			assert.False(t, ok, "iteration %d: expected no fill when no bid meets floor", i)
			continue
		}

		require.True(t, ok, "iteration %d: expected fill", i)
		assert.Equal(t, maxValid, res.Winner.CPMMicros, "iteration %d: winner must have max CPM", i)
		assert.LessOrEqual(t, res.ClearingCPM, res.Winner.CPMMicros, "iteration %d: clearing must not exceed winner CPM", i)
		assert.GreaterOrEqual(t, res.ClearingCPM, floorMicros, "iteration %d: clearing must be at least floor", i)
	}
}

func TestAuctionResult_Without(t *testing.T) {
	bids := []Bid{
		{CampaignID: "c1", CPMMicros: 3_000_000},
		{CampaignID: "c2", CPMMicros: 2_000_000},
		{CampaignID: "c1", CPMMicros: 1_000_000},
		{CampaignID: "c3", CPMMicros: 500_000},
	}

	res, ok := RunAuction(bids, 0)
	require.True(t, ok)
	assert.Equal(t, "c1", res.Winner.CampaignID)

	remaining := res.Without(res.Winner, bids)
	assert.Len(t, remaining, 2)
	for _, b := range remaining {
		assert.NotEqual(t, "c1", b.CampaignID)
	}
}

func BenchmarkRunAuction(b *testing.B) {
	bids := make([]Bid, 20)
	for i := range bids {
		bids[i] = Bid{
			CampaignID: fmt.Sprintf("camp-%d", i),
			BidderID:   fmt.Sprintf("bidder-%d", i%8),
			CPMMicros:  int64((i + 1) * 200_000),
		}
	}
	floor := int64(500_000)

	b.ReportAllocs()
	for b.Loop() {
		_, _ = RunAuction(bids, floor)
	}
}
