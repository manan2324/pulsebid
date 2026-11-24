package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedExistingDataIsIdempotent(t *testing.T) {
	var advertiserLists, campaignLists int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/bidders", "POST /v1/advertisers":
			w.WriteHeader(http.StatusConflict)
		case "PATCH /v1/bidders/bidder-1":
			_, _ = w.Write([]byte(`{}`))
		case "GET /v1/advertisers":
			advertiserLists++
			_, _ = fmt.Fprint(w, `{"data":[{"id":"existing-advertiser","name":"Seed Advertiser","status":"active"}],"next_cursor":null}`)
		case "GET /v1/campaigns":
			campaignLists++
			if r.URL.Query().Get("advertiser_id") != "existing-advertiser" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":[{"id":"existing-campaign","name":"Seed Campaign 1","status":"active"}],"next_cursor":null}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("CAMPAIGN_URL", server.URL)
	t.Setenv("BIDDER_COUNT", "1")
	t.Setenv("SEED_CAMPAIGNS", "1")
	require.NoError(t, runSeed())
	require.Equal(t, 1, advertiserLists)
	require.Equal(t, 1, campaignLists)
}

func TestSeedFreshCampaignIsActivatedWithCreativeAndTargeting(t *testing.T) {
	var createdCampaigns, targeting, creatives, activations int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/bidders":
			w.WriteHeader(http.StatusCreated)
		case "POST /v1/advertisers":
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"id":"advertiser"}`)
		case "GET /v1/campaigns":
			_, _ = fmt.Fprint(w, `{"data":[],"next_cursor":null}`)
		case "POST /v1/campaigns":
			createdCampaigns++
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"id":"campaign"}`)
		case "PUT /v1/campaigns/campaign/targeting":
			targeting++
			_, _ = fmt.Fprint(w, `{}`)
		case "POST /v1/campaigns/campaign/creatives":
			creatives++
			w.WriteHeader(http.StatusCreated)
		case "POST /v1/campaigns/campaign/activate":
			activations++
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("CAMPAIGN_URL", server.URL)
	t.Setenv("BIDDER_COUNT", "1")
	t.Setenv("SEED_CAMPAIGNS", "1")
	require.NoError(t, runSeed())
	require.Equal(t, 1, createdCampaigns)
	require.Equal(t, 1, targeting)
	require.Equal(t, 1, creatives)
	require.Equal(t, 1, activations)
}
