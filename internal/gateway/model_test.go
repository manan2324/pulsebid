package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/httpx"
)

func TestBidRequest_ValidationTable(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(r *BidRequest)
		wantError bool
		errorFrag string
	}{
		{
			name:      "valid minimal request",
			modify:    func(r *BidRequest) {},
			wantError: false,
		},
		{
			name: "missing id",
			modify: func(r *BidRequest) {
				r.ID = ""
			},
			wantError: true,
			errorFrag: "id",
		},
		{
			name: "id too long (> 128)",
			modify: func(r *BidRequest) {
				r.ID = strings.Repeat("a", 129)
			},
			wantError: true,
			errorFrag: "id",
		},
		{
			name: "missing imp.id",
			modify: func(r *BidRequest) {
				r.Imp.ID = ""
			},
			wantError: true,
			errorFrag: "imp.id",
		},
		{
			name: "missing imp.placement_id",
			modify: func(r *BidRequest) {
				r.Imp.PlacementID = ""
			},
			wantError: true,
			errorFrag: "imp.placement_id",
		},
		{
			name: "negative imp.w",
			modify: func(r *BidRequest) {
				r.Imp.W = -1
			},
			wantError: true,
			errorFrag: "imp.w",
		},
		{
			name: "negative imp.h",
			modify: func(r *BidRequest) {
				r.Imp.H = -1
			},
			wantError: true,
			errorFrag: "imp.h",
		},
		{
			name: "negative bidfloor",
			modify: func(r *BidRequest) {
				r.Imp.BidFloor = json.Number("-0.01")
			},
			wantError: true,
			errorFrag: "imp.bidfloor",
		},
		{
			name: "bidfloor exceeds 1000",
			modify: func(r *BidRequest) {
				r.Imp.BidFloor = json.Number("1000.01")
			},
			wantError: true,
			errorFrag: "imp.bidfloor",
		},
		{
			name: "bidfloor with > 6 decimals",
			modify: func(r *BidRequest) {
				r.Imp.BidFloor = json.Number("1.1234567")
			},
			wantError: true,
			errorFrag: "imp.bidfloor",
		},
		{
			name: "bidfloorcur not USD",
			modify: func(r *BidRequest) {
				r.Imp.BidFloorCur = "EUR"
			},
			wantError: true,
			errorFrag: "imp.bidfloorcur",
		},
		{
			name: "invalid device.type",
			modify: func(r *BidRequest) {
				r.Device = &Device{Type: "smartwatch"}
			},
			wantError: true,
			errorFrag: "device.type",
		},
		{
			name: "valid device types",
			modify: func(r *BidRequest) {
				r.Device = &Device{Type: "mobile"}
			},
			wantError: false,
		},
		{
			name: "invalid geo.country (3 letters)",
			modify: func(r *BidRequest) {
				r.Geo = &Geo{Country: "USA"}
			},
			wantError: true,
			errorFrag: "geo.country",
		},
		{
			name: "invalid geo.country (numbers)",
			modify: func(r *BidRequest) {
				r.Geo = &Geo{Country: "12"}
			},
			wantError: true,
			errorFrag: "geo.country",
		},
		{
			name: "valid geo.country gets uppercased",
			modify: func(r *BidRequest) {
				r.Geo = &Geo{Country: "in"}
			},
			wantError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := BidRequest{
				ID: "req-1",
				Imp: Impression{
					ID:          "imp-1",
					PlacementID: "placement-1",
					W:           300,
					H:           250,
					BidFloor:    json.Number("0.50"),
				},
			}
			tc.modify(&req)
			err := req.Validate()
			if tc.wantError {
				require.Error(t, err)
				var httpErr *httpx.Error
				require.ErrorAs(t, err, &httpErr)
				assert.Equal(t, "invalid_request", httpErr.Code)
				found := false
				for _, d := range httpErr.Details {
					if strings.Contains(d.Field, tc.errorFrag) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected detail field containing %q, got: %+v", tc.errorFrag, httpErr.Details)
			} else {
				assert.NoError(t, err)
				if req.Geo != nil && req.Geo.Country == "in" {
					assert.Equal(t, "IN", req.Geo.Country)
				}
			}
		})
	}
}

func FuzzDecodeAndValidateRequest(f *testing.F) {
	// Seed corpus with valid and invalid payloads
	f.Add([]byte(`{"id":"r1","imp":{"id":"i1","placement_id":"p1","w":300,"h":250,"bidfloor":0.50}}`))
	f.Add([]byte(`{"id":"r2","imp":{"id":"i2","placement_id":"p2"},"device":{"type":"mobile"},"geo":{"country":"US"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not a json`))
	f.Add([]byte(`{"id":"","imp":{"id":""}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		req := httptest.NewRequest("POST", "/v1/bid", strings.NewReader(string(data)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		var bidReq BidRequest
		err := httpx.DecodeJSON(rec, req, &bidReq, 16384, false)
		if err != nil {
			return // rejected at decode level
		}
		_ = bidReq.Validate()
		_ = bidReq.FloorMicros()
	})
}
