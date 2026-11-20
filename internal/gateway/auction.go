package gateway

import (
	"sort"
)

// Bid represents a candidate bid participating in the auction.
type Bid struct {
	CampaignID   string
	CreativeID   string
	AdvertiserID string
	BidderID     string
	CPMMicros    int64
	Adm          string
	LandingURL   string
	Width        int
	Height       int
}

// AuctionResult contains the outcome of a second-price auction.
type AuctionResult struct {
	Winner      Bid
	SecondCPM   int64
	ClearingCPM int64
}

// RunAuction executes a deterministic second-price auction.
// Only bids >= floorMicros qualify. Ties on CPM are broken by ascending CampaignID,
// then ascending BidderID.
// Clearing price is: min(winner.CPM, max(secondCPM, floorMicros) + $0.01).
func RunAuction(bids []Bid, floorMicros int64) (AuctionResult, bool) {
	if len(bids) == 0 {
		return AuctionResult{}, false
	}

	valid := make([]Bid, 0, len(bids))
	for _, b := range bids {
		if b.CPMMicros >= floorMicros {
			valid = append(valid, b)
		}
	}
	if len(valid) == 0 {
		return AuctionResult{}, false
	}

	sort.Slice(valid, func(i, j int) bool {
		if valid[i].CPMMicros != valid[j].CPMMicros {
			return valid[i].CPMMicros > valid[j].CPMMicros
		}
		if valid[i].CampaignID != valid[j].CampaignID {
			return valid[i].CampaignID < valid[j].CampaignID
		}
		return valid[i].BidderID < valid[j].BidderID
	})

	winner := valid[0]
	var second int64
	if len(valid) > 1 {
		second = valid[1].CPMMicros
	}

	const incrementMicros = 10_000 // $0.01 in micros
	baseline := second
	if floorMicros > baseline {
		baseline = floorMicros
	}

	clearing := baseline + incrementMicros
	if clearing > winner.CPMMicros {
		clearing = winner.CPMMicros
	}

	return AuctionResult{
		Winner:      winner,
		SecondCPM:   second,
		ClearingCPM: clearing,
	}, true
}

// Without returns a slice of bids with all entries from the specified winner's campaign removed.
func (r AuctionResult) Without(winner Bid, bids []Bid) []Bid {
	out := make([]Bid, 0, len(bids))
	for _, b := range bids {
		if b.CampaignID != winner.CampaignID {
			out = append(out, b)
		}
	}
	return out
}
