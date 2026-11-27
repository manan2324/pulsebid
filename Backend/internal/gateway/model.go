package gateway

import (
	"encoding/json"
	"strings"
	"unicode"

	"pulsebid/internal/platform/httpx"
	"pulsebid/internal/platform/money"
)

// BidRequest represents an incoming simplified OpenRTB-like bid request.
type BidRequest struct {
	ID     string     `json:"id"`
	Imp    Impression `json:"imp"`
	Device *Device    `json:"device,omitempty"`
	Geo    *Geo       `json:"geo,omitempty"`
	Site   *Site      `json:"site,omitempty"`
	User   *User      `json:"user,omitempty"`
	TMax   int        `json:"tmax,omitempty"`
}

// Impression describes the ad placement being auctioned.
type Impression struct {
	ID          string      `json:"id"`
	PlacementID string      `json:"placement_id"`
	W           int         `json:"w,omitempty"`
	H           int         `json:"h,omitempty"`
	BidFloor    json.Number `json:"bidfloor,omitempty"`
	BidFloorCur string      `json:"bidfloorcur,omitempty"`
}

// Device specifies the device environment of the end user.
type Device struct {
	Type string `json:"type,omitempty"`
	OS   string `json:"os,omitempty"`
	UA   string `json:"ua,omitempty"`
}

// Geo specifies geographic location data.
type Geo struct {
	Country string `json:"country,omitempty"`
	Region  string `json:"region,omitempty"`
	City    string `json:"city,omitempty"`
}

// Site specifies publisher site metadata.
type Site struct {
	ID     string `json:"id,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// User specifies user identifier.
type User struct {
	ID string `json:"id,omitempty"`
}

// BidResponse is returned on a successful ad fill (HTTP 200).
type BidResponse struct {
	RequestID     string  `json:"request_id"`
	BidID         string  `json:"bid_id"`
	CampaignID    string  `json:"campaign_id"`
	CreativeID    string  `json:"creative_id"`
	AdvertiserID  string  `json:"advertiser_id"`
	BidderID      string  `json:"bidder_id"`
	Price         float64 `json:"price"`
	Currency      string  `json:"currency"`
	Adm           string  `json:"adm"`
	ImpressionURL string  `json:"impression_url"`
	ClickURL      string  `json:"click_url"`
	W             int     `json:"w"`
	H             int     `json:"h"`
}

// Validate validates request semantics per Section 11.1.
func (r *BidRequest) Validate() error {
	var details []httpx.Detail

	if len(strings.TrimSpace(r.ID)) < 1 || len(r.ID) > 128 {
		details = append(details, httpx.Detail{Field: "id", Issue: "must be between 1 and 128 characters"})
	}

	if len(strings.TrimSpace(r.Imp.ID)) < 1 || len(r.Imp.ID) > 128 {
		details = append(details, httpx.Detail{Field: "imp.id", Issue: "must be between 1 and 128 characters"})
	}
	if len(strings.TrimSpace(r.Imp.PlacementID)) < 1 || len(r.Imp.PlacementID) > 128 {
		details = append(details, httpx.Detail{Field: "imp.placement_id", Issue: "must be between 1 and 128 characters"})
	}

	if r.Imp.W < 0 {
		details = append(details, httpx.Detail{Field: "imp.w", Issue: "must be positive"})
	}
	if r.Imp.H < 0 {
		details = append(details, httpx.Detail{Field: "imp.h", Issue: "must be positive"})
	}

	if r.Imp.BidFloor.String() != "" {
		floorMicros, err := money.FromDecimalString(r.Imp.BidFloor.String())
		if err != nil {
			details = append(details, httpx.Detail{Field: "imp.bidfloor", Issue: "amount must be valid decimal with at most 6 places"})
		} else if floorMicros < 0 || floorMicros > 1000*1_000_000 {
			details = append(details, httpx.Detail{Field: "imp.bidfloor", Issue: "must be between 0 and 1000 USD"})
		}
	}

	cur := strings.TrimSpace(r.Imp.BidFloorCur)
	if cur != "" && cur != "USD" {
		details = append(details, httpx.Detail{Field: "imp.bidfloorcur", Issue: "only USD currency supported"})
	}

	if r.Device != nil && r.Device.Type != "" {
		switch r.Device.Type {
		case "mobile", "desktop", "tablet", "ctv":
		default:
			details = append(details, httpx.Detail{Field: "device.type", Issue: "must be mobile, desktop, tablet, or ctv"})
		}
	}

	if r.Geo != nil && r.Geo.Country != "" {
		c := strings.ToUpper(r.Geo.Country)
		if len(c) != 2 || !unicode.IsLetter(rune(c[0])) || !unicode.IsLetter(rune(c[1])) {
			details = append(details, httpx.Detail{Field: "geo.country", Issue: "must be ISO alpha-2 country code"})
		} else {
			r.Geo.Country = c
		}
	}

	if len(details) > 0 {
		return &httpx.Error{
			Code:    "invalid_request",
			Message: "request validation failed",
			Details: details,
		}
	}
	return nil
}

// FloorMicros parses the impression bidfloor into micro-dollars.
func (r *BidRequest) FloorMicros() int64 {
	if r.Imp.BidFloor.String() == "" {
		return 0
	}
	m, err := money.FromDecimalString(r.Imp.BidFloor.String())
	if err != nil || m < 0 {
		return 0
	}
	return int64(m)
}
